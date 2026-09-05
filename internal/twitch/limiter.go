package twitch

import (
	"sync"
	"time"
)

// rateLimiter is a token bucket allowing burst events per window.
type rateLimiter struct {
	burst  int
	window time.Duration

	tokens   float64
	lastFill time.Time
}

func newRateLimiter(burst int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		burst:    burst,
		window:   window,
		tokens:   float64(burst),
		lastFill: time.Now(),
	}
}

// allow consumes a token if one is available.
func (l *rateLimiter) allow(now time.Time) bool {
	elapsed := now.Sub(l.lastFill)
	if elapsed > 0 {
		refillRate := float64(l.burst) / l.window.Seconds()
		l.tokens += elapsed.Seconds() * refillRate
		if l.tokens > float64(l.burst) {
			l.tokens = float64(l.burst)
		}
		l.lastFill = now
	}

	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}

// idle reports whether the bucket has been full and untouched for a while, so
// the limiter for a channel the bot has left can be dropped.
func (l *rateLimiter) idle(now time.Time) bool {
	return l.tokens >= float64(l.burst) && now.Sub(l.lastFill) > 10*l.window
}

// channelLimiter applies a separate rate limit to each channel.
//
// Twitch counts outbound chat per account: exceeding roughly 20 messages per
// 30 seconds gets the bot account silenced everywhere at once, so a single busy
// channel must not be able to spend the budget of all the others.
// go-twitch-irc rate limits JOINs but not messages, so this is the only thing
// standing between a popular command and a global timeout.
type channelLimiter struct {
	burst  int
	window time.Duration

	mu       sync.Mutex
	limiters map[string]*rateLimiter
	lastGC   time.Time
}

func newChannelLimiter(burst int, window time.Duration) *channelLimiter {
	return &channelLimiter{
		burst:    burst,
		window:   window,
		limiters: make(map[string]*rateLimiter),
		lastGC:   time.Now(),
	}
}

// allow reports whether a message may be sent to channel right now.
func (c *channelLimiter) allow(channel string) bool {
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	limiter, ok := c.limiters[channel]
	if !ok {
		limiter = newRateLimiter(c.burst, c.window)
		c.limiters[channel] = limiter
	}

	c.collectLocked(now)
	return limiter.allow(now)
}

// collectLocked drops limiters for channels that have gone quiet, so the map
// does not retain an entry for every channel the bot has ever spoken in.
func (c *channelLimiter) collectLocked(now time.Time) {
	if now.Sub(c.lastGC) < 10*c.window {
		return
	}
	c.lastGC = now

	for channel, limiter := range c.limiters {
		if limiter.idle(now) {
			delete(c.limiters, channel)
		}
	}
}
