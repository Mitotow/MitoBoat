// Command mitoboat runs the Twitch chat bot.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"mitoboat/internal/bot"
	"mitoboat/internal/config"
	"mitoboat/internal/flags"
	"mitoboat/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("Fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	opts := flags.Parse()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	initLogger(cfg.LogLevel)

	// Ctrl-C and SIGTERM cancel the context, which unwinds the bot: the IRC
	// connection is closed, the background refreshers stop, and the database
	// pool is drained before the process exits.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if opts.SetupDB {
		return setupDB(cfg, opts.Verbose)
	}

	b, err := bot.New(ctx, cfg, opts.Verbose)
	if err != nil {
		return err
	}
	defer func() {
		if err := b.Close(); err != nil {
			slog.Warn("Could not close the bot cleanly", "error", err)
		}
	}()

	slog.Info("Starting MitoBoat", "version", bot.Version)
	return b.Run(ctx)
}

// setupDB runs the schema migration and returns.
func setupDB(cfg *config.Config, verbose bool) error {
	db, err := store.Open(cfg, verbose)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Warn("Could not close the database cleanly", "error", err)
		}
	}()

	if err := db.Migrate(); err != nil {
		return err
	}

	slog.Info("Database migration complete", "scope", "DB")
	return nil
}

// initLogger installs the process-wide structured logger.
func initLogger(logLevel string) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(logLevel)); err != nil {
		level = slog.LevelInfo
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
}
