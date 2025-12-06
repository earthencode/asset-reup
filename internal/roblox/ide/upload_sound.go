package ide

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"mime/multipart"
	"github.com/earthencode/asset-reup/internal/roblox"
)

var UploadSoundErrors = struct {
	ErrNotLoggedIn       error
	ErrTokenInvalid      error
	ErrInappropriateName error
}{
	ErrNotLoggedIn:       errors.New("not logged in"),
	ErrTokenInvalid:      errors.New("XSRF token validation failed"),
	ErrInappropriateName: errors.New("inappropriate name or description"),
}

// request body for Open Cloud API
type uploadAudioRequest struct {
	Name              string `json:"name"`
	File              string `json:"file"` // base64-encoded audio
	GroupID           int64  `json:"groupId,omitempty"`
	PaymentSource     string `json:"paymentSource,omitempty"`
	EstimatedFileSize int64  `json:"estimatedFileSize,omitempty"`
	EstimatedDuration int64  `json:"estimatedDuration,omitempty"`
	AssetPrivacy      int    `json:"assetPrivacy,omitempty"`
}

func newSoundURL(groupID int64, name, description string) string {
    base := "https://apis.roblox.com/developer-tools/v1/assets/upload"

    values := url.Values{}
    values.Set("assetType", "Audio")
    values.Set("name", name)
    values.Set("description", description)

    if groupID > 0 {
        values.Set("groupId", fmt.Sprintf("%d", groupID))
    }

    return base + "?" + values.Encode()
}

func newUploadSoundRequest(groupID int64, name, description string, fileData *bytes.Buffer) (*http.Request, error) {
	encodedFile := base64.StdEncoding.EncodeToString(fileData.Bytes())

	reqBody := uploadAudioRequest{
		Name:              name,
		File:              encodedFile,
		GroupID:           groupID,
		PaymentSource:     "User",
		EstimatedFileSize: int64(fileData.Len()),
		AssetPrivacy:      1, // public
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "https://apis.roblox.com/developer-tools/v1/audio", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "RobloxStudio/WinInet")
	req.Header.Set("Content-Type", "application/json")
	// CSRF token and .ROBLOSECURITY cookie will be set in handler

	return req, nil
}


// NewUploadSoundHandler returns a function to upload audio and get the Asset ID
func NewUploadSoundHandler(
	c *roblox.Client,
	name, description string,
	data *bytes.Buffer,
	groupID ...int64,
) (func() (int64, error), error) {
	g := int64(0)
	if len(groupID) > 0 {
		g = groupID[0]
	}

	req, err := newUploadSoundRequest(g, name, description, data)
	if err != nil {
		return func() (int64, error) { return 0, nil }, err
	}

	return func() (int64, error) {
		// Set authentication
		req.AddCookie(&http.Cookie{
			Name:  ".ROBLOSECURITY",
			Value: c.Cookie,
		})
		req.Header.Set("x-csrf-token", c.GetToken())

		resp, err := c.DoRequest(req)
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()

		var result struct {
			AssetID int64 `json:"assetId"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return 0, err
		}

		switch resp.StatusCode {
		case http.StatusOK:
			return result.AssetID, nil
		case http.StatusForbidden:
			return 0, errors.New("forbidden: check CSRF token or login")
		case http.StatusUnprocessableEntity:
			return 0, errors.New("unprocessable entity: invalid name/description or file")
		default:
			return 0, errors.New("upload failed: " + resp.Status)
		}
	}, nil
}