package bot

import (
	"log/slog"
	"mitoboat/internal/types"
	"strings"

	"github.com/gempir/go-twitch-irc/v4"
)

func handlePrivateMessage(ctx *types.BotContext, message twitch.PrivateMessage) {
	cleanedMessage := strings.TrimSpace(message.Message)
	logger := slog.With("scope", "PRIVATE_MESSAGE_HANDLER", "channel", message.Channel,
		"sender", message.User.DisplayName)

	logger.Debug(cleanedMessage)
	if cleanedMessage == "!ping" {
		logger.Debug("Ping received")
		ctx.IrcClient.Say(message.Channel, "Pong!")
	}
}

func handleNoticeMessage(message twitch.NoticeMessage) {
	slog.With("scope", "NOTICE_MESSAGE_HANDLER", "channel", message.Channel,
		"cause", message.Message).Warn("Twitch blocked an IRC message")
}
