package bot

import (
	"context"
	"log/slog"

	"mitoboat/internal/domain"
	"mitoboat/internal/twitch"
)

// RegisterBot stores the token the bot posts as.
//
// The change only takes effect on restart: the IRC connection authenticates
// once, at connect time, so swapping the token underneath a live connection
// would not change the identity messages are sent as.
func (b *MitoBoat) RegisterBot(ctx context.Context, user *twitch.AuthorizedUser) error {
	if err := b.store.UpsertBotToken(ctx, user.Token); err != nil {
		return err
	}

	slog.Info("Stored a new bot token, restart to use it",
		"scope", "AUTH", "login", user.Login)
	return nil
}

// RegisterStreamer registers a channel and joins it immediately.
//
// Joining live is what makes the bot self-service: a streamer authorizes and
// the bot is in their chat seconds later, with no restart and no effect on the
// channels already being served.
func (b *MitoBoat) RegisterStreamer(ctx context.Context, user *twitch.AuthorizedUser) error {
	streamer := domain.Streamer{
		ID:       user.ID,
		Username: user.Login,
		Token:    user.Token,
		Active:   true,
	}

	if err := b.store.UpsertStreamer(ctx, streamer); err != nil {
		return err
	}

	sctx := NewStreamerContext(streamer)
	if err := b.syncStreamerClient(ctx, sctx); err != nil {
		// The token was just issued, so this is unexpected, but text commands
		// work without the API and the channel should still be joined.
		slog.Warn("Registered a streamer without an API client",
			"scope", "STREAMERS", "username", sctx.Username(), "error", err)
	}

	b.registry.Add(sctx)
	b.chat.Join(sctx.Username())

	// Custom commands for a brand new streamer are picked up by the next cache
	// refresh; a reload here makes them work immediately.
	if err := b.cache.Reload(ctx); err != nil {
		slog.Warn("Could not refresh the command cache after a registration",
			"scope", "COMMANDS", "error", err)
	}

	slog.Info("Registered a streamer and joined their channel",
		"scope", "STREAMERS", "username", sctx.Username(), "user_id", sctx.ID())
	return nil
}
