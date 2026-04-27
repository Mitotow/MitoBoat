package main

import (
	"mitoboat/internal/bot"
	"os"
)

func main() {
	ctx, err := bot.SetupBot()
	if err != nil {
		ctx.Logger.Error("Critical error when starting", "error", err)
		os.Exit(1)
	}

	bot.Listen(ctx)
}
