package twitch

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"mitoboat/internal/config"
	"mitoboat/internal/domain"

	"github.com/nicklaw5/helix/v2"
)

// sharedHTTPClient is used by every helix client the bot creates.
//
// helix falls back to http.DefaultClient, whose transport has only 2 idle
// connections per host; with one client per streamer all of them contend for
// that pool and end up dialling fresh TLS connections. One explicitly sized
// transport, shared by every client, keeps both the connection count and the
// per-connection buffers bounded no matter how many streamers are registered.
var sharedHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        32,
		MaxIdleConnsPerHost: 16,
		IdleConnTimeout:     90 * time.Second,
	},
}

// AppClient is a helix client authenticated with the application's own token.
// It is safe for concurrent use and keeps its token fresh in the background.
type AppClient struct {
	cfg *config.Config

	mu        sync.RWMutex
	client    *helix.Client
	expiresAt time.Time
}

func newHelixClient(cfg *config.Config) (*helix.Client, error) {
	client, err := helix.NewClient(&helix.Options{
		ClientID:     cfg.TwitchID,
		ClientSecret: cfg.TwitchSecret,
		HTTPClient:   sharedHTTPClient,
	})
	if err != nil {
		return nil, fmt.Errorf("create helix client: %w", err)
	}
	return client, nil
}

// NewAppClient creates a client holding an application access token.
//
// The previous revision assigned the constructor's error and then immediately
// overwrote it with the next call's, so a failed construction was not detected
// and the nil client was dereferenced on the following line.
func NewAppClient(cfg *config.Config) (*AppClient, error) {
	client, err := newHelixClient(cfg)
	if err != nil {
		return nil, err
	}

	app := &AppClient{cfg: cfg, client: client}
	if err := app.requestToken(); err != nil {
		return nil, err
	}
	return app, nil
}

func (a *AppClient) requestToken() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	resp, err := a.client.RequestAppAccessToken(nil)
	if err != nil {
		return fmt.Errorf("request app access token: %w", err)
	}
	if resp.ErrorMessage != "" {
		return fmt.Errorf("request app access token: twitch returned %d %s: %s",
			resp.StatusCode, resp.Error, resp.ErrorMessage)
	}

	a.client.SetAppAccessToken(resp.Data.AccessToken)
	if resp.Data.ExpiresIn > 0 {
		a.expiresAt = time.Now().Add(time.Duration(resp.Data.ExpiresIn) * time.Second)
	}
	return nil
}

// Client returns the underlying helix client.
func (a *AppClient) Client() *helix.Client {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.client
}

// Run renews the application token before it expires, until ctx is cancelled.
// Application tokens last around 60 days, so a bot that never renews works for
// weeks in testing and then fails in production; the ticker is deliberately
// short enough to also recover from a token revoked out of band.
func (a *AppClient) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.mu.RLock()
			expiresAt := a.expiresAt
			a.mu.RUnlock()

			if !expiresAt.IsZero() && time.Until(expiresAt) > 24*time.Hour {
				continue
			}
			if err := a.requestToken(); err != nil && ctx.Err() == nil {
				slog.Warn("Could not renew app access token", "scope", "HELIX", "error", err)
			}
		}
	}
}

// NewUserClient builds a helix client bound to one streamer's user token. The
// caller is responsible for having validated the token first.
func NewUserClient(cfg *config.Config, token domain.Token) (*helix.Client, error) {
	client, err := newHelixClient(cfg)
	if err != nil {
		return nil, err
	}
	client.SetUserAccessToken(token.AccessToken)
	client.SetRefreshToken(token.RefreshToken)
	return client, nil
}
