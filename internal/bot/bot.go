// Package bot wires the configuration, database, Twitch clients and command
// cache together and runs the message loop.
package bot

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"mitoboat/internal/commands"
	"mitoboat/internal/config"
	"mitoboat/internal/store"
	"mitoboat/internal/twitch"
)

// Version is the bot version, overridable at build time with
// -ldflags "-X mitoboat/internal/bot.Version=...".
var Version = "1.0.0"

// streamerTokenInterval is how often streamer tokens are revalidated. Twitch
// user tokens last about four hours, so checking twice an hour refreshes them
// well before they lapse without polling Twitch needlessly.
const streamerTokenInterval = 30 * time.Minute

// MitoBoat is the running bot.
type MitoBoat struct {
	cfg      *config.Config
	store    *store.Store
	auth     *twitch.Authenticator
	app      *twitch.AppClient
	chat     *twitch.Chat
	cache    *commands.Cache
	registry *Registry
}

// New builds a bot: it connects to the database, authenticates against Twitch
// and prepares the chat client, without joining anything yet.
func New(ctx context.Context, cfg *config.Config, verbose bool) (*MitoBoat, error) {
	db, err := store.Open(cfg, verbose)
	if err != nil {
		return nil, err
	}

	// Anything failing past this point must not leak the connection pool.
	success := false
	defer func() {
		if !success {
			_ = db.Close()
		}
	}()

	auth, err := twitch.NewAuthenticator(cfg)
	if err != nil {
		return nil, err
	}

	app, err := twitch.NewAppClient(cfg)
	if err != nil {
		return nil, err
	}

	botToken, err := db.BotToken(ctx)
	if err != nil {
		return nil, err
	}

	changed, err := auth.EnsureValid(&botToken.Token)
	if err != nil {
		return nil, fmt.Errorf("validate the bot token: %w", err)
	}
	if changed {
		if err := db.SaveBotToken(ctx, botToken); err != nil {
			return nil, err
		}
		slog.Info("Refreshed the bot token", "scope", "AUTH")
	}

	cache := commands.NewCache(db, cfg.CommandCooldown)
	if err := cache.Reload(ctx); err != nil {
		return nil, err
	}

	bot := &MitoBoat{
		cfg:      cfg,
		store:    db,
		auth:     auth,
		app:      app,
		chat:     twitch.NewChat(cfg, botToken.Token),
		cache:    cache,
		registry: NewRegistry(),
	}

	success = true
	return bot, nil
}

// Close releases every resource the bot holds.
func (b *MitoBoat) Close() error {
	return b.store.Close()
}

// Run joins the registered channels and serves chat until ctx is cancelled.
func (b *MitoBoat) Run(ctx context.Context) error {
	logger := slog.With("scope", "IRC")

	if err := b.loadStreamers(ctx); err != nil {
		return err
	}

	client := b.chat.Client()
	client.OnConnect(func() {
		logger.Info("IRC connection established",
			"user", b.chat.Username(), "channels", b.registry.Len())
	})
	client.OnPrivateMessage(b.handlePrivateMessage)

	for _, sctx := range b.registry.All() {
		b.chat.Join(sctx.Username())
	}
	logger.Info("Joined registered channels", "count", b.registry.Len())

	// Background maintenance shares the bot's lifetime and is cancelled with
	// it, so no goroutine outlives Run.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); b.app.Run(runCtx) }()
	go func() { defer wg.Done(); b.cache.Run(runCtx, b.cfg.CommandCacheTTL) }()
	go func() { defer wg.Done(); b.refreshStreamerTokens(runCtx) }()
	defer wg.Wait()

	// Connect blocks until the connection drops, so cancellation has to close
	// it explicitly to unblock.
	connErr := make(chan error, 1)
	go func() { connErr <- b.chat.Connect() }()

	select {
	case <-ctx.Done():
		logger.Info("Shutting down, closing IRC connection")
		if err := b.chat.Disconnect(); err != nil {
			logger.Warn("Could not close the IRC connection cleanly", "error", err)
		}
		<-connErr
		return nil
	case err := <-connErr:
		if err != nil {
			return fmt.Errorf("irc connection lost: %w", err)
		}
		return nil
	}
}

// loadStreamers reads the active streamers and builds their contexts.
func (b *MitoBoat) loadStreamers(ctx context.Context) error {
	logger := slog.With("scope", "STREAMERS")

	// The previous revision ignored this query's error and carried on with an
	// empty list, so a database problem was indistinguishable from having no
	// streamers registered.
	streamers, err := b.store.ActiveStreamers(ctx)
	if err != nil {
		return err
	}
	logger.Info("Loaded active streamers", "count", len(streamers))

	for _, streamer := range streamers {
		sctx := NewStreamerContext(streamer)

		// A streamer whose token cannot be validated is still joined: text
		// commands do not touch the Helix API, so leaving the channel entirely
		// would be a worse outcome than running without API access.
		if err := b.syncStreamerClient(ctx, sctx); err != nil {
			logger.Warn("Streamer joined without an API client",
				"username", sctx.Username(), "error", err)
		}

		b.registry.Add(sctx)
	}

	return nil
}

// syncStreamerClient validates a streamer's token, persists it if it changed,
// and rebuilds their Helix client.
func (b *MitoBoat) syncStreamerClient(ctx context.Context, sctx *StreamerContext) error {
	token := sctx.Token()

	changed, err := b.auth.EnsureValid(&token)
	if err != nil {
		return err
	}

	if changed {
		if err := b.store.SaveStreamerToken(ctx, sctx.ID(), token); err != nil {
			return err
		}
		sctx.SetToken(token)
		slog.Debug("Refreshed streamer token",
			"scope", "STREAMERS", "username", sctx.Username())
	}

	client, err := twitch.NewUserClient(b.cfg, token)
	if err != nil {
		return err
	}
	sctx.SetHelix(client)
	return nil
}

// refreshStreamerTokens keeps the streamer tokens alive.
//
// Twitch user access tokens last about four hours, so a bot that only validated
// them at startup lost API access for every streamer partway through the first
// long stream and did not recover until it was restarted.
func (b *MitoBoat) refreshStreamerTokens(ctx context.Context) {
	ticker := time.NewTicker(streamerTokenInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, sctx := range b.registry.All() {
				if err := b.syncStreamerClient(ctx, sctx); err != nil {
					if ctx.Err() != nil {
						return
					}
					slog.Warn("Could not refresh a streamer token",
						"scope", "STREAMERS", "username", sctx.Username(), "error", err)
				}
			}
		}
	}
}
