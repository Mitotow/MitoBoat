package bot

import (
	"log/slog"
	"mitoboat/internal/types"
	"mitoboat/internal/utils"

	"github.com/gempir/go-twitch-irc/v4"
	"gorm.io/gorm"
)

func handlePrivateMessage(ctx *types.BotContext, message twitch.PrivateMessage) {
	logger := slog.With("scope", "PRIVATE_MESSAGE_HANDLER", "channel", message.Channel,
		"sender", message.User.DisplayName)

	if len(message.Message) > 0 && message.Message[0] == '!' {
		logger.Debug("Message is prefixed by !, searching for command ...")
		sctx := utils.GetStreamerContextByUser(ctx, message.Channel)
		if sctx == nil {
			logger.Debug("No Streamer Context found")
			return
		}

		cmdName := utils.GetCommandFromMessage(message.Message)
		if cmdName == nil {
			logger.Debug("Command not found")
			return
		}

		var cmd types.TextCommand
		result := ctx.Db.First(&cmd, "name = ?", cmdName)

		if result.Error == nil {
			ctx.IrcClient.Say(message.Channel, cmd.Text)
			return
		} else if result.Error != gorm.ErrRecordNotFound {
			logger.Error("Database error searching default command", "error", result.Error)
			return
		}
		logger.Debug("No default command found, searching for custom command ...")

		var ccmd types.CustomTextCommand
		resultCustom := ctx.Db.First(&ccmd, "streamer_id = ? AND name = ?", sctx.Streamer.ID, cmdName)
		if resultCustom.Error == nil {
			ctx.IrcClient.Say(message.Channel, ccmd.Command.Text)
		} else if resultCustom.Error != gorm.ErrRecordNotFound {
			logger.Error("Database error searching custom command", "error", resultCustom.Error)
		} else {
			logger.Debug("No custom command found")
		}
	}
}
