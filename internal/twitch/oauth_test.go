package twitch

import (
	"strings"
	"testing"
	"time"

	"mitoboat/internal/config"
)

func testOAuth(t *testing.T) *OAuth {
	t.Helper()
	o, err := NewOAuth(&config.Config{
		TwitchID:       "client-id",
		TwitchSecret:   "client-secret",
		RedirectURI:    "http://localhost:8080/auth/callback",
		BotScopes:      []string{"chat:read", "chat:edit"},
		StreamerScopes: []string{"moderator:read:followers"},
	})
	if err != nil {
		t.Fatalf("NewOAuth: %v", err)
	}
	return o
}

func TestAuthorizationURL(t *testing.T) {
	o := testOAuth(t)

	url, err := o.AuthorizationURL(RoleBot)
	if err != nil {
		t.Fatalf("AuthorizationURL: %v", err)
	}

	for _, want := range []string{"client_id=client-id", "response_type=code", "state=", "chat"} {
		if !strings.Contains(url, want) {
			t.Errorf("authorization URL %q is missing %q", url, want)
		}
	}
}

func TestAuthorizationURLUsesRoleScopes(t *testing.T) {
	o := testOAuth(t)

	botURL, err := o.AuthorizationURL(RoleBot)
	if err != nil {
		t.Fatalf("AuthorizationURL(bot): %v", err)
	}
	streamerURL, err := o.AuthorizationURL(RoleStreamer)
	if err != nil {
		t.Fatalf("AuthorizationURL(streamer): %v", err)
	}

	// A streamer must not be asked for the scopes that let the bot post as
	// them; those belong to the bot account alone.
	if strings.Contains(streamerURL, "chat%3Aedit") || strings.Contains(streamerURL, "chat:edit") {
		t.Error("the streamer flow must not request the bot's chat scopes")
	}
	if !strings.Contains(botURL, "chat") {
		t.Error("the bot flow must request the chat scopes")
	}
}

func TestStatesAreUnique(t *testing.T) {
	o := testOAuth(t)
	seen := make(map[string]bool)

	for range 50 {
		if _, err := o.AuthorizationURL(RoleStreamer); err != nil {
			t.Fatalf("AuthorizationURL: %v", err)
		}
	}

	o.mu.Lock()
	defer o.mu.Unlock()
	for state := range o.states {
		if seen[state] {
			t.Fatal("a state token was reused")
		}
		seen[state] = true
	}
	if len(seen) != 50 {
		t.Errorf("stored states = %d, want 50", len(seen))
	}
}

func TestConsumeStateRejectsUnknown(t *testing.T) {
	o := testOAuth(t)
	if _, err := o.consumeState("never-issued"); err == nil {
		t.Error("an unknown state must be rejected")
	}
}

// A state must work exactly once, or a leaked callback URL could be replayed.
func TestConsumeStateIsSingleUse(t *testing.T) {
	o := testOAuth(t)
	if _, err := o.AuthorizationURL(RoleStreamer); err != nil {
		t.Fatalf("AuthorizationURL: %v", err)
	}

	var state string
	o.mu.Lock()
	for s := range o.states {
		state = s
	}
	o.mu.Unlock()

	role, err := o.consumeState(state)
	if err != nil {
		t.Fatalf("first use of the state failed: %v", err)
	}
	if role != RoleStreamer {
		t.Errorf("role = %q, want %q", role, RoleStreamer)
	}
	if _, err := o.consumeState(state); err == nil {
		t.Error("the state must not be accepted a second time")
	}
}

func TestConsumeStateRejectsExpired(t *testing.T) {
	o := testOAuth(t)
	o.mu.Lock()
	o.states["stale"] = pendingState{role: RoleBot, createdAt: time.Now().Add(-stateTTL - time.Minute)}
	o.mu.Unlock()

	if _, err := o.consumeState("stale"); err == nil {
		t.Error("an expired state must be rejected")
	}
}

// Abandoned flows must not accumulate: the map is only bounded by pruning.
func TestAbandonedStatesArePruned(t *testing.T) {
	o := testOAuth(t)

	o.mu.Lock()
	for i := range 100 {
		o.states[string(rune('a'+i%26))+string(rune('a'+i/26))] =
			pendingState{role: RoleStreamer, createdAt: time.Now().Add(-stateTTL - time.Minute)}
	}
	o.mu.Unlock()

	if _, err := o.AuthorizationURL(RoleStreamer); err != nil {
		t.Fatalf("AuthorizationURL: %v", err)
	}

	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.states) != 1 {
		t.Errorf("states after pruning = %d, want 1 (only the fresh one)", len(o.states))
	}
}
