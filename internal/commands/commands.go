package commands

import (
	"mitoboat/internal/domain"
	"strings"
)

// GetCommandFromMessage return the command in message
func GetCommandFromMessage(message string) *string {
	cleanedMessage := strings.TrimSpace(message)
	if len(cleanedMessage) <= 1 || cleanedMessage[0] != '!' {
		return nil
	}

	parts := strings.Fields(cleanedMessage)
	cmd := strings.ToLower(parts[0][1:])
	return &cmd
}

// ExecuteTextCommand find the command by command name and send the text related to the command via IRC client
func ExecuteTextCommand(ctx *domain.BotContext, channel string, dest domain.ReplyableCommand, query string, args ...any) bool {
	err := ctx.Db.Where(query, args...).First(dest).Error
	if err == nil {
		ctx.IrcClient.Say(channel, dest.GetText())
		return true
	}

	return false
}
