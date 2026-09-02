package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mitoboat/internal/config"
	"mitoboat/internal/domain"
	"mitoboat/internal/twitch"
)

const testSecret = "a-sufficiently-long-admin-secret"

type recordingStore struct {
	botTokens []domain.Token
	streamers []domain.Streamer
}

func (r *recordingStore) UpsertBotToken(_ context.Context, token domain.Token) error {
	r.botTokens = append(r.botTokens, token)
	return nil
}

func (r *recordingStore) UpsertStreamer(_ context.Context, streamer domain.Streamer) error {
	r.streamers = append(r.streamers, streamer)
	return nil
}

func testServer(t *testing.T) (*Server, *recordingStore) {
	t.Helper()

	cfg := &config.Config{
		TwitchID:       "client-id",
		TwitchSecret:   "client-secret",
		RedirectURI:    "http://localhost:8080/auth/callback",
		AdminSecret:    testSecret,
		BotScopes:      []string{"chat:read", "chat:edit"},
		StreamerScopes: []string{"moderator:read:followers"},
	}

	oauth, err := twitch.NewOAuth(cfg)
	if err != nil {
		t.Fatalf("NewOAuth: %v", err)
	}

	store := &recordingStore{}
	return New(cfg, oauth, NewStoreRegistrar(store)), store
}

func do(t *testing.T, s *Server, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestIndexIsPublic(t *testing.T) {
	s, _ := testServer(t)

	rec := do(t, s, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "/auth/streamer") {
		t.Error("the index must link streamers to the registration flow")
	}
}

// A streamer authorizing grants access to their own channel only, so the flow
// is deliberately open; gating it would defeat self-service registration.
func TestStreamerAuthorizeIsOpen(t *testing.T) {
	s, _ := testServer(t)

	rec := do(t, s, "/auth/streamer")
	if rec.Code != http.StatusFound {
		t.Fatalf("GET /auth/streamer = %d, want 302", rec.Code)
	}
	if location := rec.Header().Get("Location"); !strings.HasPrefix(location, "https://id.twitch.tv/") {
		t.Errorf("redirect went to %q, want Twitch", location)
	}
}

// The bot token is what lets the bot speak in every channel at once, so minting
// one must require the admin secret.
func TestBotAuthorizeRequiresAdminSecret(t *testing.T) {
	s, _ := testServer(t)

	tests := []struct {
		name   string
		target string
		want   int
	}{
		{"no key", "/auth/bot", http.StatusForbidden},
		{"empty key", "/auth/bot?key=", http.StatusForbidden},
		{"wrong key", "/auth/bot?key=guess", http.StatusForbidden},
		{"prefix of the key", "/auth/bot?key=a-sufficiently", http.StatusForbidden},
		{"correct key", "/auth/bot?key=" + testSecret, http.StatusFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := do(t, s, tt.target).Code; got != tt.want {
				t.Errorf("GET %s = %d, want %d", tt.target, got, tt.want)
			}
		})
	}
}

func TestCallbackRejectsBadRequests(t *testing.T) {
	tests := []struct {
		name   string
		target string
	}{
		{"missing everything", "/auth/callback"},
		{"missing state", "/auth/callback?code=abc"},
		{"missing code", "/auth/callback?state=abc"},
		{"unknown state", "/auth/callback?code=abc&state=never-issued"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, store := testServer(t)

			if got := do(t, s, tt.target).Code; got != http.StatusBadRequest {
				t.Errorf("GET %s = %d, want 400", tt.target, got)
			}
			if len(store.botTokens) != 0 || len(store.streamers) != 0 {
				t.Error("a rejected callback must not write anything")
			}
		})
	}
}

// Twitch reports a declined consent screen as a redirect carrying ?error.
func TestCallbackHandlesDeniedConsent(t *testing.T) {
	s, store := testServer(t)

	rec := do(t, s, "/auth/callback?error=access_denied&error_description=denied")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("denied consent = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Nothing has been saved") {
		t.Error("the page should say nothing was saved")
	}
	if len(store.streamers) != 0 {
		t.Error("a denied consent must not register anyone")
	}
}

func TestSecurityHeaders(t *testing.T) {
	s, _ := testServer(t)
	rec := do(t, s, "/")

	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "no-referrer",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Error("a Content-Security-Policy must be set")
	}
}

func TestRunShutsDownOnCancel(t *testing.T) {
	s, _ := testServer(t)
	s.httpServer.Addr = "127.0.0.1:0"

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	cancel()
	if err := <-done; err != nil {
		t.Errorf("Run returned %v, want nil after cancellation", err)
	}
}
