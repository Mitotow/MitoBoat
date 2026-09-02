package commands

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubLoader struct {
	global map[string]string
	custom map[string]map[string]string
	err    error
	calls  int
}

func (s *stubLoader) GlobalCommands(context.Context) (map[string]string, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.global, nil
}

func (s *stubLoader) CustomCommands(context.Context) (map[string]map[string]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.custom, nil
}

func TestCacheLookup(t *testing.T) {
	loader := &stubLoader{
		global: map[string]string{"ping": "pong"},
		custom: map[string]map[string]string{
			"123": {"hello": "hi from 123", "ping": "custom pong"},
		},
	}

	cache := NewCache(loader, time.Second)
	if err := cache.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	t.Run("global command is visible to every streamer", func(t *testing.T) {
		if text, ok := cache.Lookup("999", "ping"); !ok || text != "pong" {
			t.Errorf("Lookup(999, ping) = %q, %v; want \"pong\", true", text, ok)
		}
	})

	t.Run("custom command shadows the global one", func(t *testing.T) {
		if text, ok := cache.Lookup("123", "ping"); !ok || text != "custom pong" {
			t.Errorf("Lookup(123, ping) = %q, %v; want \"custom pong\", true", text, ok)
		}
	})

	t.Run("custom command is private to its streamer", func(t *testing.T) {
		if _, ok := cache.Lookup("999", "hello"); ok {
			t.Error("Lookup(999, hello) found another streamer's custom command")
		}
	})

	t.Run("unknown command", func(t *testing.T) {
		if _, ok := cache.Lookup("123", "nope"); ok {
			t.Error("Lookup(123, nope) = true, want false")
		}
	})
}

func TestCacheReloadError(t *testing.T) {
	wantErr := errors.New("database down")
	loader := &stubLoader{global: map[string]string{"ping": "pong"}}

	cache := NewCache(loader, time.Second)
	if err := cache.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	// A failed refresh must leave the previous contents in place: serving
	// slightly stale commands is better than serving none.
	loader.err = wantErr
	if err := cache.Reload(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("Reload error = %v, want %v", err, wantErr)
	}

	if text, ok := cache.Lookup("123", "ping"); !ok || text != "pong" {
		t.Errorf("after a failed reload, Lookup = %q, %v; want \"pong\", true", text, ok)
	}
}

func TestCacheAllowCooldown(t *testing.T) {
	cache := NewCache(&stubLoader{}, 3*time.Second)
	start := time.Now()

	if !cache.Allow("chan", "ping", start) {
		t.Fatal("first invocation must be allowed")
	}
	if cache.Allow("chan", "ping", start.Add(time.Second)) {
		t.Error("second invocation within the cooldown must be refused")
	}
	if !cache.Allow("chan", "ping", start.Add(3*time.Second)) {
		t.Error("invocation after the cooldown must be allowed")
	}

	// Cooldowns are per channel and per command, so one busy channel does not
	// mute a command everywhere else.
	if !cache.Allow("other", "ping", start.Add(time.Second)) {
		t.Error("cooldown must not carry across channels")
	}
	if !cache.Allow("chan", "other", start.Add(time.Second)) {
		t.Error("cooldown must not carry across commands")
	}
}

func TestCacheAllowDisabled(t *testing.T) {
	cache := NewCache(&stubLoader{}, 0)
	now := time.Now()

	for i := range 3 {
		if !cache.Allow("chan", "ping", now) {
			t.Fatalf("invocation %d refused with the cooldown disabled", i)
		}
	}
}

func TestCacheReloadPrunesCooldowns(t *testing.T) {
	loader := &stubLoader{}
	cache := NewCache(loader, time.Millisecond)

	cache.Allow("chan", "ping", time.Now().Add(-time.Hour))
	if err := cache.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	cache.mu.RLock()
	remaining := len(cache.cooldowns)
	cache.mu.RUnlock()

	if remaining != 0 {
		t.Errorf("expired cooldowns = %d, want 0; the map would grow without bound", remaining)
	}
}

// The IRC read loop and the refresh ticker touch the cache at the same time.
func TestCacheConcurrentAccess(t *testing.T) {
	loader := &stubLoader{
		global: map[string]string{"ping": "pong"},
		custom: map[string]map[string]string{"123": {"hi": "hello"}},
	}
	cache := NewCache(loader, time.Millisecond)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 200 {
			_ = cache.Reload(context.Background())
		}
	}()

	for range 200 {
		cache.Lookup("123", "ping")
		cache.Allow("chan", "ping", time.Now())
	}
	<-done
}
