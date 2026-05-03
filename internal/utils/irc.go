package utils

import (
	"fmt"
	"log/slog"
	"mitoboat/internal/env"
	"mitoboat/internal/types"

	"github.com/gempir/go-twitch-irc/v4"
)

// GetIrcClient return an IRC client, require GlobalHelix client to be initialized in context
func GetIrcClient(ctx *types.BotContext) (*twitch.Client, error) {
	var token types.BotToken
	if err := ctx.Db.First(&token, 1).Error; err != nil {
		return nil, fmt.Errorf("No token found on Database: %w", err)
	}

	changed, err := ValidateAccessToken(ctx, &token.Token)
	if err != nil {
		return nil, fmt.Errorf("Error while trying to validate the UserAccessToken: %w", err)
	}

	if changed {
		if err = ctx.Db.Save(&token).Error; err != nil {
			return nil, fmt.Errorf("Cannot save new UserAccessToken to Database: %w", err)
		}
	}

	accessToken := token.Token.AccessToken
	if len(accessToken) > 8 {
		slog.Debug("IRC Debug",
			"user", env.DefaultEnv.IrcUser,
			"token_start", accessToken[:4],
			"token_end", accessToken[len(accessToken)-4:])
	}
	ircClient := twitch.NewClient(env.DefaultEnv.IrcUser, fmt.Sprintf("oauth:%s", token.Token.AccessToken))
	return ircClient, nil
}
