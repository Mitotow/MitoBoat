package main

import (
	"log/slog"
	"mitoboat/internal/bot"
	"mitoboat/internal/env"
	"mitoboat/internal/flags"
	"os"
)

// initLogger Create the logger and set it as the default instance of slog logger
func initLogger() {
	var level slog.Level
	err := level.UnmarshalText([]byte(env.DefaultEnv.LogLevel))
	if err != nil {
		level = slog.LevelInfo
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
}

func main() {
	err := env.Load()

	if err == nil {
		initLogger()
		args := flags.GetFlags()
		if *args.SetupDb {
			err = bot.SetupDb(args)
		} else {
			var b *bot.MitoBoat
			b, err = bot.Create(args)
			if err == nil {
				err = b.Listen()
			}
		}
	}

	if err != nil {
		slog.Error("Fatal", "error", err)
		os.Exit(1)
	}
}
