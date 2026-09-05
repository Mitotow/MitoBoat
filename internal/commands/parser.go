// Package commands parses chat commands and serves their replies from memory.
package commands

import "strings"

// Prefix marks a chat message as a command invocation.
const Prefix = '!'

// Invocation is a parsed command call.
type Invocation struct {
	// Name is the command name, lowercased and without the prefix.
	Name string
	// Args are the whitespace separated arguments that followed the name.
	Args []string
}

// Parse extracts a command invocation from a chat message. It reports false if
// the message is not a command.
//
// It returns a value rather than a pointer so that the common case (a message
// that is not a command, which is most of chat) costs no allocation at all.
func Parse(message string) (Invocation, bool) {
	message = strings.TrimSpace(message)
	if len(message) < 2 || message[0] != Prefix {
		return Invocation{}, false
	}

	fields := strings.Fields(message[1:])
	if len(fields) == 0 {
		return Invocation{}, false
	}

	return Invocation{
		Name: strings.ToLower(fields[0]),
		Args: fields[1:],
	}, true
}
