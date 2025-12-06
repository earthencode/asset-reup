package ide

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"log"

	"github.com/earthencode/asset-reup/internal/roblox"
)

var logger = log.Default()

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

	req, err := http.NewRequest("POST", "https://apis.roblox.com/assets/v1/audio", bytes.NewReader(bodyBytes))
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

	return func() (int64, error) {
		encodedFile := base64.StdEncoding.EncodeToString(data.Bytes())

		reqBody := uploadAudioRequest{
			Name:              name,
			File:              encodedFile,
			GroupID:           g,
			PaymentSource:     "User",
			EstimatedFileSize: int64(data.Len()),
			AssetPrivacy:      1, // public
		}

		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			return 0, err
		}

		req, err := http.NewRequest("POST", "https://apis.roblox.com/assets/v1/audio", bytes.NewReader(bodyBytes))
		if err != nil {
			return 0, err
		}

		// Authentication
		req.AddCookie(&http.Cookie{
			Name:  ".ROBLOSECURITY",
			Value: c.Cookie,
		})
		req.Header.Set("User-Agent", "RobloxStudio/WinInet")
		req.Header.Set("Content-Type", "application/json")

		// Set CSRF token if available
		if token := c.GetToken(); token != "" {
			req.Header.Set("x-csrf-token", token)
		}

		resp, err := c.DoRequest(req)
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()

		// Retry if CSRF token is missing or invalid
		if resp.StatusCode == http.StatusForbidden {
			newToken := resp.Header.Get("x-csrf-token")
			if newToken != "" {
				c.SetToken(newToken)
				req.Header.Set("x-csrf-token", newToken)

				resp, err = c.DoRequest(req)
				if err != nil {
					return 0, err
				}
				defer resp.Body.Close()
			} else {
				return 0, errors.New("forbidden: CSRF token missing or invalid")
			}
		}

		// Decode JSON response
		var result struct {
			AssetID int64 `json:"assetId"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return 0, fmt.Errorf("failed to decode response: %w", err)
		}

		// Handle HTTP status codes
		switch resp.StatusCode {
		case http.StatusOK:
			return result.AssetID, nil
		case http.StatusUnprocessableEntity:
			return 0, errors.New("unprocessable entity: invalid name, description, or file")
		default:
			return 0, fmt.Errorf("upload failed: %s", resp.Status)
		}
	}, nil
}