package twitch

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	"mitoboat/internal/config"
	"mitoboat/internal/domain"

	"github.com/nicklaw5/helix/v2"
)

// Role is which kind of account is being authorized.
type Role string

const (
	// RoleBot is the account the bot posts as.
	RoleBot Role = "bot"
	// RoleStreamer is a channel owner registering their channel.
	RoleStreamer Role = "streamer"
)

// stateTTL is how long an authorization may sit half finished before its state
// token is rejected.
const stateTTL = 10 * time.Minute

// AuthorizedUser is the result of a completed authorization.
type AuthorizedUser struct {
	ID          string
	Login       string
	DisplayName string
	Token       domain.Token
	Role        Role
}

// OAuth runs the Twitch authorization code flow.
//
// Twitch will only redirect back to a URI registered on the application, and
// only issues a user token in exchange for a single-use code, so this needs a
// listening HTTP server; see internal/web.
type OAuth struct {
	cfg    *config.Config
	client *helix.Client

	mu     sync.Mutex
	states map[string]pendingState
}

type pendingState struct {
	role      Role
	createdAt time.Time
}

// NewOAuth builds the authorization flow.
func NewOAuth(cfg *config.Config) (*OAuth, error) {
	client, err := helix.NewClient(&helix.Options{
		ClientID:     cfg.TwitchID,
		ClientSecret: cfg.TwitchSecret,
		RedirectURI:  cfg.RedirectURI,
		HTTPClient:   sharedHTTPClient,
	})
	if err != nil {
		return nil, fmt.Errorf("create oauth client: %w", err)
	}

	return &OAuth{
		cfg:    cfg,
		client: client,
		states: make(map[string]pendingState),
	}, nil
}

// AuthorizationURL starts a flow and returns the Twitch URL to send the user to.
//
// The state token ties the callback back to this request. Without it, an
// attacker could feed the callback their own code and bind their account to
// someone else's session.
func (o *OAuth) AuthorizationURL(role Role) (string, error) {
	state, err := randomState()
	if err != nil {
		return "", err
	}

	o.mu.Lock()
	o.pruneStatesLocked()
	o.states[state] = pendingState{role: role, createdAt: time.Now()}
	o.mu.Unlock()

	url := o.client.GetAuthorizationURL(&helix.AuthorizationURLParams{
		ResponseType: "code",
		Scopes:       o.scopesFor(role),
		State:        state,
		// Force the consent screen so re-authorizing a different account does
		// not silently reuse whichever one the browser is already signed in as.
		ForceVerify: true,
	})
	return url, nil
}

// Exchange completes a flow: it consumes the state, trades the code for a
// token, and looks up who authorized.
func (o *OAuth) Exchange(state, code string) (*AuthorizedUser, error) {
	role, err := o.consumeState(state)
	if err != nil {
		return nil, err
	}

	resp, err := o.client.RequestUserAccessToken(code)
	if err != nil {
		return nil, fmt.Errorf("exchange authorization code: %w", err)
	}
	if resp.ErrorMessage != "" {
		return nil, fmt.Errorf("exchange authorization code: twitch returned %d %s: %s",
			resp.StatusCode, resp.Error, resp.ErrorMessage)
	}

	token := domain.Token{
		AccessToken:  resp.Data.AccessToken,
		RefreshToken: resp.Data.RefreshToken,
	}
	if resp.Data.ExpiresIn > 0 {
		token.ExpiresAt = time.Now().Add(time.Duration(resp.Data.ExpiresIn) * time.Second)
	}

	user, err := o.identify(token.AccessToken)
	if err != nil {
		return nil, err
	}

	return &AuthorizedUser{
		ID:          user.ID,
		Login:       strings.ToLower(user.Login),
		DisplayName: user.DisplayName,
		Token:       token,
		Role:        role,
	}, nil
}

// identify asks Twitch who a token belongs to.
//
// The user id comes from Twitch rather than from anything the caller supplied:
// it is the primary key streamers are stored under, and unlike the login name
// it never changes.
func (o *OAuth) identify(accessToken string) (*helix.User, error) {
	client, err := helix.NewClient(&helix.Options{
		ClientID:        o.cfg.TwitchID,
		ClientSecret:    o.cfg.TwitchSecret,
		UserAccessToken: accessToken,
		HTTPClient:      sharedHTTPClient,
	})
	if err != nil {
		return nil, fmt.Errorf("create identification client: %w", err)
	}

	resp, err := client.GetUsers(&helix.UsersParams{})
	if err != nil {
		return nil, fmt.Errorf("identify the authorized user: %w", err)
	}
	if resp.ErrorMessage != "" {
		return nil, fmt.Errorf("identify the authorized user: twitch returned %d %s: %s",
			resp.StatusCode, resp.Error, resp.ErrorMessage)
	}
	if len(resp.Data.Users) == 0 {
		return nil, fmt.Errorf("identify the authorized user: twitch returned no user")
	}

	return &resp.Data.Users[0], nil
}

func (o *OAuth) scopesFor(role Role) []string {
	if role == RoleBot {
		return o.cfg.BotScopes
	}
	return o.cfg.StreamerScopes
}

// consumeState validates a state token and removes it, so a callback cannot be
// replayed.
func (o *OAuth) consumeState(state string) (Role, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	pending, ok := o.states[state]
	if !ok {
		return "", fmt.Errorf("unknown or already used authorization state")
	}
	delete(o.states, state)

	if time.Since(pending.createdAt) > stateTTL {
		return "", fmt.Errorf("authorization expired, please start again")
	}
	return pending.role, nil
}

// pruneStatesLocked drops abandoned flows so the map cannot grow without bound.
// The caller must hold the lock.
func (o *OAuth) pruneStatesLocked() {
	for state, pending := range o.states {
		if time.Since(pending.createdAt) > stateTTL {
			delete(o.states, state)
		}
	}
}

func randomState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate authorization state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
