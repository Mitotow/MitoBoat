package bot

import (
	"strings"
	"sync"

	"mitoboat/internal/domain"

	"github.com/nicklaw5/helix/v2"
)

// StreamerContext is everything the bot knows about one joined channel.
//
// It is read by the IRC read loop and written by the token refresh goroutine,
// so its fields are unexported and reached through accessors that hold a lock.
type StreamerContext struct {
	mu       sync.RWMutex
	streamer domain.Streamer
	// helix is the streamer-scoped API client, nil when the streamer's token
	// could not be validated. Chat commands work without it.
	helix *helix.Client
}

// NewStreamerContext builds a context for a streamer.
func NewStreamerContext(streamer domain.Streamer) *StreamerContext {
	streamer.Username = strings.ToLower(streamer.Username)
	return &StreamerContext{streamer: streamer}
}

// ID returns the Twitch user id, which never changes and so needs no lock.
func (s *StreamerContext) ID() string { return s.streamer.ID }

// Username returns the lowercased channel name.
func (s *StreamerContext) Username() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.streamer.Username
}

// Streamer returns a copy of the persisted streamer.
func (s *StreamerContext) Streamer() domain.Streamer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.streamer
}

// Token returns a copy of the streamer's current token.
func (s *StreamerContext) Token() domain.Token {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.streamer.Token
}

// SetToken records a refreshed token.
func (s *StreamerContext) SetToken(token domain.Token) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.streamer.Token = token
}

// Helix returns the streamer-scoped API client, which may be nil.
func (s *StreamerContext) Helix() *helix.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.helix
}

// SetHelix replaces the streamer-scoped API client.
func (s *StreamerContext) SetHelix(client *helix.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.helix = client
}

// Registry indexes the joined streamers for lookup on the message hot path.
//
// The previous revision walked a slice on every chat message and returned the
// address of the loop variable, which is a pointer to a copy: writing to the
// returned context (a refreshed token, for instance) updated nothing. Storing
// pointers in maps fixes both the cost and the aliasing, and lets the set of
// streamers change while the bot is running.
type Registry struct {
	mu sync.RWMutex
	// byID and byUsername hold the same pointers, so an update through one is
	// visible through the other.
	byID       map[string]*StreamerContext
	byUsername map[string]*StreamerContext
}

// NewRegistry builds an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		byID:       make(map[string]*StreamerContext),
		byUsername: make(map[string]*StreamerContext),
	}
}

// Add registers a streamer, replacing any existing entry for the same id.
func (r *Registry) Add(sctx *StreamerContext) {
	username := sctx.Username()

	r.mu.Lock()
	defer r.mu.Unlock()

	// Drop a stale username index if the streamer was renamed.
	if existing, ok := r.byID[sctx.ID()]; ok {
		delete(r.byUsername, existing.Username())
	}

	r.byID[sctx.ID()] = sctx
	r.byUsername[username] = sctx
}

// Remove deregisters a streamer by id.
func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.byID[id]; ok {
		delete(r.byUsername, existing.Username())
		delete(r.byID, id)
	}
}

// ByID returns the context for a streamer id, or nil.
func (r *Registry) ByID(id string) *StreamerContext {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byID[id]
}

// ByUsername returns the context for a channel name, or nil. Lookups are case
// insensitive because Twitch IRC reports channels lowercased while the name a
// streamer registers with may not be.
func (r *Registry) ByUsername(username string) *StreamerContext {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byUsername[strings.ToLower(username)]
}

// Len reports how many streamers are registered.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byID)
}

// All returns the registered contexts. The slice is a snapshot, so callers can
// range over it without holding the registry lock.
func (r *Registry) All() []*StreamerContext {
	r.mu.RLock()
	defer r.mu.RUnlock()

	all := make([]*StreamerContext, 0, len(r.byID))
	for _, sctx := range r.byID {
		all = append(all, sctx)
	}
	return all
}
