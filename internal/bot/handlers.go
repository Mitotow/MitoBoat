package bot

import (
	"log/slog"
	"mitoboat/internal/types"
	"mitoboat/internal/utils"

	"github.com/gempir/go-twitch-irc/v4"
)

func handlePrivateMessage(ctx *types.BotContext, message twitch.PrivateMessage) {
	logger := slog.With("scope", "PRIVATE_MESSAGE_HANDLER", "channel", message.Channel,
		"sender", message.User.DisplayName)

	cmdName := utils.GetCommandFromMessage(message.Message)
	if cmdName == nil {
		logger.Debug("Command not found")
		return
	}

	sctx := utils.GetStreamerContextByUser(ctx, message.Channel)
	if sctx == nil {
		logger.Debug("No Streamer Context found")
		return
	}

	logger.Debug("Searching for command", "command", *cmdName)
	if utils.ExecuteIfFound(ctx, message.Channel, &types.TextCommand{}, "name = ?", *cmdName) {
		logger.Debug("found")
		return
	}
	logger.Debug("No default command found, searching for custom command ...")

	if utils.ExecuteIfFound(ctx, message.Channel, &types.CustomTextCommand{}, "streamer_id = ? AND name = ?", sctx.Streamer.ID, *cmdName) {
		logger.Debug("found")
		return
	}
}
