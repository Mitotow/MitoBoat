package twitch

import (
	"sync"
	"testing"
	"time"
)

func TestRateLimiterBurst(t *testing.T) {
	l := newRateLimiter(3, time.Second)
	now := time.Now()

	for i := range 3 {
		if !l.allow(now) {
			t.Fatalf("call %d within the burst was refused", i)
		}
	}
	if l.allow(now) {
		t.Error("a call past the burst must be refused")
	}
}

func TestRateLimiterRefills(t *testing.T) {
	l := newRateLimiter(2, time.Second)
	now := time.Now()

	l.allow(now)
	l.allow(now)
	if l.allow(now) {
		t.Fatal("bucket should be empty")
	}

	// Half a window refills one of the two tokens.
	if !l.allow(now.Add(500 * time.Millisecond)) {
		t.Error("a token should have refilled after half a window")
	}
	if l.allow(now.Add(500 * time.Millisecond)) {
		t.Error("only one token should have refilled")
	}
}

func TestRateLimiterDoesNotOverfill(t *testing.T) {
	l := newRateLimiter(2, time.Second)
	now := time.Now()

	// A long idle period must not let the bucket accumulate past its burst,
	// or the bot would come back from a quiet spell and flood chat.
	for i := range 2 {
		if !l.allow(now.Add(time.Hour)) {
			t.Fatalf("call %d after idling was refused", i)
		}
	}
	if l.allow(now.Add(time.Hour)) {
		t.Error("the bucket accumulated more than its burst while idle")
	}
}

// Twitch counts messages per account, so one busy channel must not be able to
// spend the budget belonging to the others.
func TestChannelLimiterIsolatesChannels(t *testing.T) {
	c := newChannelLimiter(2, time.Second)

	if !c.allow("busy") || !c.allow("busy") {
		t.Fatal("the burst for the busy channel was refused")
	}
	if c.allow("busy") {
		t.Fatal("the busy channel should be exhausted")
	}

	if !c.allow("quiet") {
		t.Error("a quiet channel must keep its own budget")
	}
}

func TestChannelLimiterConcurrentAccess(t *testing.T) {
	c := newChannelLimiter(100, time.Second)

	var wg sync.WaitGroup
	for i := range 4 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for range 100 {
				c.allow("channel")
				c.allow(string(rune('a' + n)))
			}
		}(i)
	}
	wg.Wait()
}
