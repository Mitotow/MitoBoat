package web

import (
	"context"
	"log/slog"

	"mitoboat/internal/domain"
	"mitoboat/internal/twitch"
)

// TokenStore is the persistence the bootstrap registrar needs.
type TokenStore interface {
	UpsertBotToken(ctx context.Context, token domain.Token) error
	UpsertStreamer(ctx context.Context, streamer domain.Streamer) error
}

// StoreRegistrar saves authorizations straight to the database, with no bot
// running.
//
// This breaks the bootstrap deadlock: the bot cannot start without a token, and
// the token can only be obtained through this flow, so the flow has to be able
// to run on its own first.
type StoreRegistrar struct {
	store TokenStore
}

// NewStoreRegistrar builds a registrar backed only by the database.
func NewStoreRegistrar(store TokenStore) *StoreRegistrar {
	return &StoreRegistrar{store: store}
}

// RegisterBot stores the bot token.
func (r *StoreRegistrar) RegisterBot(ctx context.Context, user *twitch.AuthorizedUser) error {
	if err := r.store.UpsertBotToken(ctx, user.Token); err != nil {
		return err
	}
	slog.Info("Stored the bot token, the bot can now be started",
		"scope", "AUTH", "login", user.Login)
	return nil
}

// RegisterStreamer registers a channel. It will be joined the next time the bot
// starts, since there is no running bot to join it now.
func (r *StoreRegistrar) RegisterStreamer(ctx context.Context, user *twitch.AuthorizedUser) error {
	err := r.store.UpsertStreamer(ctx, domain.Streamer{
		ID:       user.ID,
		Username: user.Login,
		Token:    user.Token,
		Active:   true,
	})
	if err != nil {
		return err
	}
	slog.Info("Registered a streamer, their channel is joined when the bot starts",
		"scope", "STREAMERS", "username", user.Login, "user_id", user.ID)
	return nil
}
