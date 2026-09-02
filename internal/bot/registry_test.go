package bot

import (
	"sync"
	"testing"

	"mitoboat/internal/domain"
)

func streamer(id, username string) domain.Streamer {
	return domain.Streamer{ID: id, Username: username}
}

func TestRegistryByID(t *testing.T) {
	r := NewRegistry()
	r.Add(NewStreamerContext(streamer("12345678", "mitoboat")))

	if r.ByID("") != nil {
		t.Error("ByID(\"\") must be nil")
	}
	if r.ByID("1234") != nil {
		t.Error("ByID must not match a prefix")
	}
	if r.ByID("12345678") == nil {
		t.Error("ByID(12345678) must find the streamer")
	}

	r.Add(NewStreamerContext(streamer("87654321", "mitotow")))
	if r.ByID("12345678") == nil || r.ByID("87654321") == nil {
		t.Error("both streamers must be findable")
	}
	if got := r.Len(); got != 2 {
		t.Errorf("Len() = %d, want 2", got)
	}
}

func TestRegistryByUsername(t *testing.T) {
	r := NewRegistry()
	r.Add(NewStreamerContext(streamer("1", "mitoboat")))

	if r.ByUsername("") != nil {
		t.Error("ByUsername(\"\") must be nil")
	}
	if r.ByUsername("mito") != nil {
		t.Error("ByUsername must not match a prefix")
	}
	if r.ByUsername("mitoboat") == nil {
		t.Error("ByUsername(mitoboat) must find the streamer")
	}
}

// Twitch IRC reports channel names lowercased while a streamer row may hold a
// display-cased name; matching them exactly silently ignored those channels.
func TestRegistryUsernameLookupIsCaseInsensitive(t *testing.T) {
	r := NewRegistry()
	r.Add(NewStreamerContext(streamer("1", "MitoBoat")))

	for _, name := range []string{"mitoboat", "MitoBoat", "MITOBOAT"} {
		if r.ByUsername(name) == nil {
			t.Errorf("ByUsername(%q) must find the streamer", name)
		}
	}
}

// The old lookup returned the address of a range variable, so writes through
// the returned pointer were lost.
func TestRegistryReturnsSharedPointer(t *testing.T) {
	r := NewRegistry()
	r.Add(NewStreamerContext(streamer("1", "mitoboat")))

	r.ByUsername("mitoboat").SetToken(domain.Token{AccessToken: "refreshed"})

	if got := r.ByID("1").Token().AccessToken; got != "refreshed" {
		t.Errorf("token after update = %q, want %q; the write did not reach the registry", got, "refreshed")
	}
}

func TestRegistryRemove(t *testing.T) {
	r := NewRegistry()
	r.Add(NewStreamerContext(streamer("1", "mitoboat")))
	r.Remove("1")

	if r.ByID("1") != nil || r.ByUsername("mitoboat") != nil {
		t.Error("Remove must clear both indexes")
	}
	if got := r.Len(); got != 0 {
		t.Errorf("Len() = %d, want 0", got)
	}
}

// Re-adding under a new name must not leave the old name resolving to the
// streamer, or the bot would answer in a channel it no longer tracks.
func TestRegistryRenameDropsOldUsername(t *testing.T) {
	r := NewRegistry()
	r.Add(NewStreamerContext(streamer("1", "oldname")))
	r.Add(NewStreamerContext(streamer("1", "newname")))

	if r.ByUsername("oldname") != nil {
		t.Error("the previous username must stop resolving")
	}
	if r.ByUsername("newname") == nil {
		t.Error("the new username must resolve")
	}
	if got := r.Len(); got != 1 {
		t.Errorf("Len() = %d, want 1", got)
	}
}

func TestRegistryAll(t *testing.T) {
	r := NewRegistry()
	r.Add(NewStreamerContext(streamer("1", "a")))
	r.Add(NewStreamerContext(streamer("2", "b")))

	if got := len(r.All()); got != 2 {
		t.Errorf("len(All()) = %d, want 2", got)
	}
}

// The IRC read loop reads the registry while the token refresher writes to it.
func TestRegistryConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	r.Add(NewStreamerContext(streamer("1", "mitoboat")))

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		for range 200 {
			r.ByUsername("mitoboat")
			r.ByID("1")
		}
	}()
	go func() {
		defer wg.Done()
		for range 200 {
			if sctx := r.ByID("1"); sctx != nil {
				sctx.SetToken(domain.Token{AccessToken: "x"})
				sctx.SetHelix(nil)
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := range 200 {
			r.Add(NewStreamerContext(streamer("2", "other")))
			if i%2 == 0 {
				r.Remove("2")
			}
			r.All()
		}
	}()

	wg.Wait()
}
