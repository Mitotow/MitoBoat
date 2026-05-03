package utils

import (
	"mitoboat/internal/types"
	"strings"
)

func GetCommandFromMessage(message string) *string {
	cleanedMessage := strings.TrimSpace(message)
	if len(cleanedMessage) <= 1 || cleanedMessage[0] != '!' {
		return nil
	}

	parts := strings.Fields(cleanedMessage)
	cmd := strings.ToLower(parts[0][1:])
	return &cmd
}

func ExecuteIfFound(ctx *types.BotContext, channel string, dest types.ReplyableCommand, query string, args ...any) bool {
	err := ctx.Db.Where(query, args...).First(dest).Error
	if err == nil {
		ctx.IrcClient.Say(channel, dest.GetText())
		return true
	}

	return false
}
