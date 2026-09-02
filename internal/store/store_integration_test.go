package store_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"mitoboat/internal/commands"
	"mitoboat/internal/config"
	"mitoboat/internal/domain"
	"mitoboat/internal/store"
)

// newTestStore connects to the database described by the usual DB_* variables.
//
// These tests exercise the SQL itself: the upsert conflict clauses and the
// cache queries cannot be covered without a real PostgreSQL, and they are
// exactly where a schema change breaks silently. They skip when no database is
// configured, so `go test ./...` still works on a bare checkout.
func newTestStore(t *testing.T) *store.Store {
	t.Helper()

	if os.Getenv("DB_NAME") == "" {
		t.Skip("no database configured; set the DB_* variables to run the store integration tests")
	}

	t.Setenv("TWITCH_ID", "test")
	t.Setenv("TWITCH_SECRET", "test")
	t.Setenv("IRC_USER", "test")
	t.Setenv("ADMIN_SECRET", "a-sufficiently-long-admin-secret")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	db, err := store.Open(cfg, false)
	if err != nil {
		t.Skipf("cannot reach the database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

func TestBotTokenRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := newTestStore(t)

	expiry := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	if err := db.UpsertBotToken(ctx, domain.Token{
		AccessToken: "access-1", RefreshToken: "refresh-1", ExpiresAt: expiry,
	}); err != nil {
		t.Fatalf("UpsertBotToken: %v", err)
	}

	got, err := db.BotToken(ctx)
	if err != nil {
		t.Fatalf("BotToken: %v", err)
	}
	if got.Token.AccessToken != "access-1" || got.Token.RefreshToken != "refresh-1" {
		t.Errorf("token = %+v, want access-1/refresh-1", got.Token)
	}

	// Re-authorizing must replace the row rather than add a second one the bot
	// would then have to choose between.
	if err := db.UpsertBotToken(ctx, domain.Token{
		AccessToken: "access-2", RefreshToken: "refresh-2", ExpiresAt: expiry,
	}); err != nil {
		t.Fatalf("second UpsertBotToken: %v", err)
	}

	got, err = db.BotToken(ctx)
	if err != nil {
		t.Fatalf("BotToken after re-authorization: %v", err)
	}
	if got.Token.AccessToken != "access-2" {
		t.Errorf("access token = %q, want access-2", got.Token.AccessToken)
	}
	if got.ID != 1 {
		t.Errorf("bot token id = %d, want the single pinned row 1", got.ID)
	}
}

func TestStreamerUpsertKeepsIdentityAcrossRename(t *testing.T) {
	ctx := context.Background()
	db := newTestStore(t)

	streamer := domain.Streamer{
		ID: "900001", Username: "OldName",
		Token: domain.Token{AccessToken: "a", RefreshToken: "r"},
	}
	if err := db.UpsertStreamer(ctx, streamer); err != nil {
		t.Fatalf("UpsertStreamer: %v", err)
	}

	stored, err := db.StreamerByID(ctx, "900001")
	if err != nil {
		t.Fatalf("StreamerByID: %v", err)
	}
	// Usernames are lowercased on write because IRC reports channels lowercased
	// and the lookup happens on every message.
	if stored.Username != "oldname" {
		t.Errorf("username = %q, want it lowercased", stored.Username)
	}
	if !stored.Active {
		t.Error("a freshly registered streamer must be active")
	}

	// A rename must update the row, not create a second one, or the streamer
	// would lose their custom commands.
	streamer.Username = "NewName"
	if err := db.UpsertStreamer(ctx, streamer); err != nil {
		t.Fatalf("UpsertStreamer after rename: %v", err)
	}

	stored, err = db.StreamerByID(ctx, "900001")
	if err != nil {
		t.Fatalf("StreamerByID after rename: %v", err)
	}
	if stored.Username != "newname" {
		t.Errorf("username = %q, want newname", stored.Username)
	}

	active, err := db.ActiveStreamers(ctx)
	if err != nil {
		t.Fatalf("ActiveStreamers: %v", err)
	}
	seen := 0
	for _, s := range active {
		if s.ID == "900001" {
			seen++
		}
	}
	if seen != 1 {
		t.Errorf("streamer appears %d times, want exactly 1", seen)
	}
}

func TestSaveStreamerToken(t *testing.T) {
	ctx := context.Background()
	db := newTestStore(t)

	err := db.UpsertStreamer(ctx, domain.Streamer{
		ID: "900002", Username: "tokenowner",
		Token: domain.Token{AccessToken: "old", RefreshToken: "old-r"},
	})
	if err != nil {
		t.Fatalf("UpsertStreamer: %v", err)
	}

	expiry := time.Now().Add(4 * time.Hour).UTC().Truncate(time.Second)
	err = db.SaveStreamerToken(ctx, "900002", domain.Token{
		AccessToken: "new", RefreshToken: "new-r", ExpiresAt: expiry,
	})
	if err != nil {
		t.Fatalf("SaveStreamerToken: %v", err)
	}

	stored, err := db.StreamerByID(ctx, "900002")
	if err != nil {
		t.Fatalf("StreamerByID: %v", err)
	}
	if stored.Token.AccessToken != "new" || stored.Token.RefreshToken != "new-r" {
		t.Errorf("token = %+v, want new/new-r", stored.Token)
	}
	// The targeted update must not disturb the rest of the row.
	if stored.Username != "tokenowner" || !stored.Active {
		t.Errorf("streamer = %+v, want the other columns untouched", stored)
	}
}

func TestStreamerByIDNotFound(t *testing.T) {
	db := newTestStore(t)

	_, err := db.StreamerByID(context.Background(), "no-such-streamer")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

// The cache is fed by these two queries, so they are checked through it.
func TestCommandCacheAgainstDatabase(t *testing.T) {
	ctx := context.Background()
	db := newTestStore(t)

	err := db.UpsertStreamer(ctx, domain.Streamer{
		ID: "900003", Username: "cacheowner",
		Token: domain.Token{AccessToken: "a", RefreshToken: "r"},
	})
	if err != nil {
		t.Fatalf("UpsertStreamer: %v", err)
	}

	cache := commands.NewCache(db, time.Second)
	if err := cache.Reload(ctx); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	// An unknown command must miss cleanly rather than error.
	if _, ok := cache.Lookup("900003", "definitely-not-a-command"); ok {
		t.Error("an unknown command must not resolve")
	}
	if _, ok := cache.Lookup("no-such-streamer", "anything"); ok {
		t.Error("a lookup for an unknown streamer must not resolve")
	}
}
