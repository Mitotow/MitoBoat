package twitchUtils

import (
	"fmt"
	"log/slog"
	"mitoboat/internal/types"
	"os"

	"github.com/gempir/go-twitch-irc/v4"
)

// GetIrcClient return an IRC client, require GlobalHelix client to be initialized in context
func GetIrcClient(ctx *types.BotContext) (*twitch.Client, error) {
	clientUser := os.Getenv("IRC_USER")
	logger := slog.With("scope", "IRC", "irc_user", clientUser)

	var token types.BotToken
	if err := ctx.Db.First(&token, 1).Error; err != nil {
		logger.Error("No token found on Database", "error", err)
		return nil, err
	}

	resp, err := ctx.GlobalHelix.RefreshUserAccessToken(token.RefreshToken)
	if err != nil || resp.Data.AccessToken == "" || resp.Data.RefreshToken == "" {
		logger.Error("Token Refresh failed", "status", resp.StatusCode, "error", err)
		return nil, err
	}

	token.AccessToken = resp.Data.AccessToken
	token.RefreshToken = resp.Data.RefreshToken
	ctx.Db.Save(&token)
	logger.Debug("IRC Token refreshed and saved in Database")

	ircClient := twitch.NewClient(clientUser, fmt.Sprintf("oauth:%s", token.AccessToken))
	logger.Info("IRC client ready")

	return ircClient, nil
}
