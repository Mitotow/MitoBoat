package bot

import (
	"log/slog"
	"mitoboat/internal/commands"
	"mitoboat/internal/domain"

	"github.com/gempir/go-twitch-irc/v4"
)

func handlePrivateMessage(ctx *domain.BotContext, message twitch.PrivateMessage) {
	logger := slog.With("scope", "PRIVATE_MESSAGE_HANDLER", "channel", message.Channel,
		"sender", message.User.DisplayName)

	cmdName := commands.GetCommandFromMessage(message.Message)
	if cmdName == nil {
		return
	}

	sctx := GetStreamerContextByUser(ctx, message.Channel)
	if sctx == nil {
		return
	}

	if commands.ExecuteTextCommand(ctx, message.Channel, &domain.TextCommand{}, "name = ?", *cmdName) {
		logger.Debug("TextCommand", "cmdName", *cmdName)
		return
	}

	if commands.ExecuteTextCommand(ctx, message.Channel, &domain.CustomTextCommand{}, "streamer_id = ? AND name = ?", sctx.Streamer.ID, *cmdName) {
		logger.Debug("CustomTextCommand", "cmdName", *cmdName)
		return
	}
}
