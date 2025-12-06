package clientutils

import (
	"errors"
	"fmt"

	"github.com/earthencode/asset-reup/internal/app/assets/shared/permissions"
	"github.com/earthencode/asset-reup/internal/app/config"
	"github.com/earthencode/asset-reup/internal/app/context"
	"github.com/earthencode/asset-reup/internal/app/request"
	"github.com/earthencode/asset-reup/internal/color"
	"github.com/earthencode/asset-reup/internal/console"
	"github.com/earthencode/asset-reup/internal/files"
)

var cookieFile = config.Get("cookie_file")

func GetNewCookie(ctx *context.Context, r *request.Request, m string) {
	pauseController := ctx.PauseController

	if !pauseController.Pause() {
		pauseController.WaitIfPaused()
		return
	}

	console.ClearScreen()

	client := ctx.Client
	inputErr := errors.New(m)
	for {
		fmt.Print(ctx.Logger.History.String())
		color.Error.Println(inputErr)

		i, err := console.LongInput("ROBLOSECURITY: ")
		console.ClearScreen()
		if err != nil {
			inputErr = err
			continue
		}

		fmt.Println("Authenticating cookie...")
		err = client.SetCookie(i)
		console.ClearScreen()
		if err != nil {
			inputErr = err
			continue
		}

		fmt.Println("Checking if account can edit universe...")
		err = permissions.CanEditUniverse(ctx, r)
		console.ClearScreen()
		if err != nil {
			inputErr = err
			continue
		}

		break
	}

	fmt.Print(ctx.Logger.History.String())

	if err := files.Write(cookieFile, client.Cookie); err != nil {
		ctx.Logger.Error("Failed to save cookie: ", err)
	}

	pauseController.Unpause()
}
