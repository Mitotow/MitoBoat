package twitch

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"mitoboat/internal/config"
	"mitoboat/internal/domain"

	irc "github.com/gempir/go-twitch-irc/v4"
)

// Chat wraps the IRC client with the outbound rate limiting Twitch requires.
//
// The bot's own username is kept here so incoming messages from the bot can be
// dropped: without that check a command whose reply itself starts with '!'
// makes the bot answer itself forever.
type Chat struct {
	client   *irc.Client
	username string
	limiter  *channelLimiter

	mu     sync.RWMutex
	joined map[string]struct{}
}

// NewChat builds a chat client authenticated with the bot's user token.
func NewChat(cfg *config.Config, token domain.Token) *Chat {
	username := strings.ToLower(cfg.IRCUser)
	client := irc.NewClient(username, "oauth:"+token.AccessToken)

	return &Chat{
		client:   client,
		username: username,
		limiter:  newChannelLimiter(cfg.SayBurst, cfg.SayWindow),
		joined:   make(map[string]struct{}),
	}
}

// Client exposes the underlying client so callers can register handlers.
func (c *Chat) Client() *irc.Client { return c.client }

// Username is the bot's own login, lowercased.
func (c *Chat) Username() string { return c.username }

// IsSelf reports whether a message was sent by the bot itself.
func (c *Chat) IsSelf(login string) bool {
	return strings.EqualFold(login, c.username)
}

// Join subscribes to a channel, ignoring a repeat join.
func (c *Chat) Join(channel string) {
	channel = strings.ToLower(channel)

	c.mu.Lock()
	if _, ok := c.joined[channel]; ok {
		c.mu.Unlock()
		return
	}
	c.joined[channel] = struct{}{}
	c.mu.Unlock()

	c.client.Join(channel)
}

// Part leaves a channel.
func (c *Chat) Part(channel string) {
	channel = strings.ToLower(channel)

	c.mu.Lock()
	delete(c.joined, channel)
	c.mu.Unlock()

	c.client.Depart(channel)
}

// Say sends a message, dropping it if the channel is over its rate limit.
//
// Dropping is deliberate: queueing would let a burst build an unbounded backlog
// of replies that arrive long after the message that triggered them.
func (c *Chat) Say(channel, text string) {
	channel = strings.ToLower(channel)
	if text == "" {
		return
	}

	if !c.limiter.allow(channel) {
		slog.Warn("Dropped message, channel is over its rate limit",
			"scope", "IRC", "channel", channel)
		return
	}

	c.client.Say(channel, text)
}

// Connect blocks until the connection drops or Disconnect is called.
func (c *Chat) Connect() error {
	if err := c.client.Connect(); err != nil {
		return fmt.Errorf("irc connection: %w", err)
	}
	return nil
}

// Disconnect closes the connection, unblocking Connect.
func (c *Chat) Disconnect() error {
	if err := c.client.Disconnect(); err != nil {
		return fmt.Errorf("irc disconnect: %w", err)
	}
	return nil
}
