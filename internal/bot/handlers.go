package bot

import (
	"log/slog"
	"time"

	"mitoboat/internal/commands"

	irc "github.com/gempir/go-twitch-irc/v4"
)

// handlePrivateMessage answers chat commands.
//
// go-twitch-irc invokes this inline on the single goroutine that reads the
// connection, so every millisecond spent here delays messages for every joined
// channel. It must stay allocation light and must never do I/O: the command
// lookup is served from memory and the reply is handed to the client's own
// writer goroutine.
func (b *MitoBoat) handlePrivateMessage(message irc.PrivateMessage) {
	// Never react to our own messages: a reply that itself starts with '!'
	// would otherwise trigger the bot forever.
	if b.chat.IsSelf(message.User.Name) {
		return
	}

	invocation, ok := commands.Parse(message.Message)
	if !ok {
		return
	}

	sctx := b.registry.ByUsername(message.Channel)
	if sctx == nil {
		// A message from a channel we do not track. This is normal briefly
		// after a streamer is deactivated but before the PART lands.
		return
	}

	text, ok := b.cache.Lookup(sctx.ID(), invocation.Name)
	if !ok {
		return
	}

	if !b.cache.Allow(message.Channel, invocation.Name, time.Now()) {
		slog.Debug("Command on cooldown",
			"scope", "COMMANDS", "channel", message.Channel, "command", invocation.Name)
		return
	}

	slog.Debug("Answering command",
		"scope", "COMMANDS",
		"channel", message.Channel,
		"sender", message.User.Name,
		"command", invocation.Name)

	b.chat.Say(message.Channel, text)
}
