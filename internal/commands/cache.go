package commands

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Loader supplies the cache with command definitions. It is satisfied by
// *store.Store; declaring it here keeps the cache testable without a database.
type Loader interface {
	GlobalCommands(ctx context.Context) (map[string]string, error)
	CustomCommands(ctx context.Context) (map[string]map[string]string, error)
}

// Cache holds every text command in memory.
//
// The previous revision issued up to two database queries for every chat
// message beginning with '!', on a handler that go-twitch-irc invokes inline on
// its single connection read loop. A slow query therefore stalled message
// processing for every joined channel at once. Serving lookups from a map turns
// the hot path into a pair of map reads under a read lock.
//
// The whole command set is small (a string pair per command), so holding it
// resident costs far less than the connection churn it replaces.
type Cache struct {
	loader Loader

	mu sync.RWMutex
	// global maps command name to reply text, for every channel.
	global map[string]string
	// custom maps streamer id to that streamer's own commands, which shadow
	// the global ones.
	custom map[string]map[string]string
	// cooldowns records the last time a command replied in a channel, keyed by
	// channel and name. It is pruned on every reload so it cannot grow without
	// bound as channels come and go.
	cooldowns map[cooldownKey]time.Time
	cooldown  time.Duration
}

type cooldownKey struct {
	channel string
	command string
}

// NewCache builds an empty cache. Call Reload before serving lookups.
func NewCache(loader Loader, cooldown time.Duration) *Cache {
	return &Cache{
		loader:    loader,
		global:    make(map[string]string),
		custom:    make(map[string]map[string]string),
		cooldowns: make(map[cooldownKey]time.Time),
		cooldown:  cooldown,
	}
}

// Reload rebuilds the cache from the loader.
//
// The new maps are built outside the write lock so readers are only blocked for
// the pointer swap, not for the duration of the queries.
func (c *Cache) Reload(ctx context.Context) error {
	global, err := c.loader.GlobalCommands(ctx)
	if err != nil {
		return fmt.Errorf("reload global commands: %w", err)
	}

	custom, err := c.loader.CustomCommands(ctx)
	if err != nil {
		return fmt.Errorf("reload custom commands: %w", err)
	}

	customCount := 0
	for _, byName := range custom {
		customCount += len(byName)
	}

	c.mu.Lock()
	c.global = global
	c.custom = custom
	c.pruneCooldownsLocked()
	c.mu.Unlock()

	slog.Debug("Command cache reloaded",
		"scope", "COMMANDS",
		"global", len(global),
		"custom", customCount,
		"streamers_with_custom", len(custom))
	return nil
}

// Lookup returns the reply text for a command in a streamer's channel. A custom
// command shadows a global one of the same name.
func (c *Cache) Lookup(streamerID, name string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if byName, ok := c.custom[streamerID]; ok {
		if text, ok := byName[name]; ok {
			return text, true
		}
	}

	text, ok := c.global[name]
	return text, ok
}

// Allow reports whether a command may reply in a channel right now, recording
// the invocation when it may. It rate limits per command per channel so a
// single command cannot be used to flood chat, and so two bots answering each
// other cannot spin into a loop.
func (c *Cache) Allow(channel, name string, now time.Time) bool {
	if c.cooldown <= 0 {
		return true
	}

	key := cooldownKey{channel: channel, command: name}

	c.mu.Lock()
	defer c.mu.Unlock()

	if last, ok := c.cooldowns[key]; ok && now.Sub(last) < c.cooldown {
		return false
	}
	c.cooldowns[key] = now
	return true
}

// pruneCooldownsLocked drops cooldown entries that have already elapsed. The
// caller must hold the write lock.
func (c *Cache) pruneCooldownsLocked() {
	now := time.Now()
	for key, last := range c.cooldowns {
		if now.Sub(last) >= c.cooldown {
			delete(c.cooldowns, key)
		}
	}
}

// Run keeps the cache fresh until ctx is cancelled. A failed refresh is logged
// and retried on the next tick rather than taking the bot down: the previous
// contents stay valid and serving stale commands beats serving none.
func (c *Cache) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.Reload(ctx); err != nil && ctx.Err() == nil {
				slog.Warn("Could not refresh command cache",
					"scope", "COMMANDS", "error", err)
			}
		}
	}
}
