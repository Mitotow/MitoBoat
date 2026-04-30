package main

import (
	"mitoboat/internal/bot"
	"mitoboat/internal/flags"
	"os"
)

func main() {
	var err error

	args := flags.GetFlags()
	if *args.SetupDb {
		err = bot.SetupDb()
	} else {
		ctx, err := bot.SetupBot()

		if err == nil {
			bot.Listen(ctx)
		}
	}

	if err != nil {
		os.Exit(1)
	}
}
