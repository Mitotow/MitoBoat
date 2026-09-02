package twitch

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"mitoboat/internal/config"
	"mitoboat/internal/domain"

	"github.com/nicklaw5/helix/v2"
)

// expiryMargin is how long before a token's stated expiry it is treated as
// already expired, so a refresh happens before requests start failing.
const expiryMargin = 5 * time.Minute

// ErrNoRefreshToken is returned when a token has expired and cannot be renewed.
var ErrNoRefreshToken = errors.New("token expired and no refresh token is available")

// Authenticator validates and refreshes OAuth tokens.
//
// helix.Client.ValidateToken temporarily writes the token under test into the
// client's own options and restores it with a defer, so calling it concurrently
// on a shared client races on that field and can leave the wrong token behind.
// The Authenticator therefore owns a private client used for nothing else, and
// serialises access to it.
type Authenticator struct {
	mu     sync.Mutex
	client *helix.Client
}

// NewAuthenticator builds an Authenticator from the application credentials.
func NewAuthenticator(cfg *config.Config) (*Authenticator, error) {
	client, err := helix.NewClient(&helix.Options{
		ClientID:     cfg.TwitchID,
		ClientSecret: cfg.TwitchSecret,
	})
	if err != nil {
		return nil, fmt.Errorf("create authentication client: %w", err)
	}
	return &Authenticator{client: client}, nil
}

// EnsureValid makes sure token is usable, refreshing it in place when it is
// not. It reports whether the token was changed and therefore needs persisting.
//
// The previous revision dereferenced the validation response before checking
// the error. helix returns a nil response alongside any transport error, so any
// network blip panicked the bot; the error is checked first here.
func (a *Authenticator) EnsureValid(token *domain.Token) (bool, error) {
	if token.AccessToken == "" {
		return false, ErrNoRefreshToken
	}

	// Trust a recorded expiry when there is one: it saves a round trip to
	// Twitch on every startup and on every periodic check.
	if !token.ExpiresAt.IsZero() && !token.Expired(expiryMargin) {
		return false, nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	valid, _, err := a.client.ValidateToken(token.AccessToken)
	if err != nil {
		return false, fmt.Errorf("validate access token: %w", err)
	}
	if valid {
		return false, nil
	}

	return a.refreshLocked(token)
}

// Refresh renews a token unconditionally.
func (a *Authenticator) Refresh(token *domain.Token) (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.refreshLocked(token)
}

func (a *Authenticator) refreshLocked(token *domain.Token) (bool, error) {
	if token.RefreshToken == "" {
		return false, ErrNoRefreshToken
	}

	resp, err := a.client.RefreshUserAccessToken(token.RefreshToken)
	if err != nil {
		return false, fmt.Errorf("refresh user access token: %w", err)
	}
	if resp.ErrorMessage != "" {
		return false, fmt.Errorf("refresh user access token: twitch returned %d %s: %s",
			resp.StatusCode, resp.Error, resp.ErrorMessage)
	}

	token.AccessToken = resp.Data.AccessToken
	if resp.Data.RefreshToken != "" {
		token.RefreshToken = resp.Data.RefreshToken
	}
	if resp.Data.ExpiresIn > 0 {
		token.ExpiresAt = time.Now().Add(time.Duration(resp.Data.ExpiresIn) * time.Second)
	}

	return true, nil
}
