// Command mitoboat runs the Twitch chat bot and its authorization server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"mitoboat/internal/bot"
	"mitoboat/internal/config"
	"mitoboat/internal/flags"
	"mitoboat/internal/store"
	"mitoboat/internal/twitch"
	"mitoboat/internal/web"
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

	// Ctrl-C and SIGTERM cancel the context, which unwinds everything: the IRC
	// connection is closed, the HTTP server drains, the background refreshers
	// stop, and the database pool is closed before the process exits.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch {
	case opts.SetupDB:
		return setupDB(cfg, opts.Verbose)
	case opts.AuthOnly:
		return runAuthServer(ctx, cfg, opts.Verbose)
	default:
		return runBot(ctx, cfg, opts)
	}
}

// runBot serves chat and the authorization flow together, so a streamer who
// authorizes is joined without a restart.
func runBot(ctx context.Context, cfg *config.Config, opts flags.Flags) error {
	b, err := bot.New(ctx, cfg, opts.Verbose)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			slog.Error("No bot token stored yet. Run with -a and authorize the bot account first.")
		}
		return err
	}
	defer closeQuietly("bot", b.Close)

	oauth, err := twitch.NewOAuth(cfg)
	if err != nil {
		return err
	}

	slog.Info("Starting MitoBoat", "version", bot.Version)
	return runAll(ctx,
		func(ctx context.Context) error { return b.Run(ctx) },
		func(ctx context.Context) error { return web.New(cfg, oauth, b).Run(ctx) },
	)
}

// runAuthServer serves only the authorization flow, for the initial setup.
func runAuthServer(ctx context.Context, cfg *config.Config, verbose bool) error {
	db, err := store.Open(cfg, verbose)
	if err != nil {
		return err
	}
	defer closeQuietly("database", db.Close)

	oauth, err := twitch.NewOAuth(cfg)
	if err != nil {
		return err
	}

	slog.Info("Running in authorization-only mode",
		"authorize_bot", cfg.RedirectURI, "hint", "open /auth/bot?key=$ADMIN_SECRET")
	return web.New(cfg, oauth, web.NewStoreRegistrar(db)).Run(ctx)
}

// setupDB runs the schema migration and returns.
func setupDB(cfg *config.Config, verbose bool) error {
	db, err := store.Open(cfg, verbose)
	if err != nil {
		return err
	}
	defer closeQuietly("database", db.Close)

	if err := db.Migrate(); err != nil {
		return err
	}

	slog.Info("Database migration complete", "scope", "DB")
	return nil
}

// runAll runs every task until one returns or ctx is cancelled, then cancels
// the rest and waits for them. The first error is what the process reports.
func runAll(ctx context.Context, tasks ...func(context.Context) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		wg       sync.WaitGroup
		once     sync.Once
		firstErr error
	)

	for _, task := range tasks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := task(ctx); err != nil {
				once.Do(func() { firstErr = err })
			}
			// Whether it failed or returned cleanly, the process is done.
			cancel()
		}()
	}

	wg.Wait()
	return firstErr
}

func closeQuietly(what string, close func() error) {
	if err := close(); err != nil {
		slog.Warn("Could not close cleanly", "what", what, "error", err)
	}
}

// initLogger installs the process-wide structured logger.
func initLogger(logLevel string) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(logLevel)); err != nil {
		level = slog.LevelInfo
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
}
