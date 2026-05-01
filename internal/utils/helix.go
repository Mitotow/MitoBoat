package utils

import (
	"log/slog"
	"mitoboat/internal/types"
	"os"

	"github.com/nicklaw5/helix/v2"
)

func getHelixClient(ctx *types.BotContext, logger *slog.Logger, streamer *types.Streamer) (*helix.Client, error) {
	clientId := os.Getenv("TWITCH_ID")
	clientSecret := os.Getenv("TWITCH_SECRET")

	client, err := helix.NewClient(&helix.Options{ClientID: clientId, ClientSecret: clientSecret})
	if err != nil {
		logger.Error("Could not connect to Helix API", "error", err)
		return nil, err
	}

	if streamer != nil {
		if v, _, _ := client.ValidateToken(streamer.Token.AccessToken); !v {
			logger.Debug("UserAccessToken as expired")
			resp, err := client.RefreshUserAccessToken(streamer.Token.RefreshToken)
			if err != nil {
				logger.Error("Cannot refresh UserAccessToken")
				return nil, err
			}

			streamer.Token.AccessToken = resp.Data.AccessToken
			streamer.Token.RefreshToken = resp.Data.RefreshToken
			ctx.Db.Save(streamer)
		}

		client.SetUserAccessToken(streamer.Token.AccessToken)
		client.SetRefreshToken(streamer.Token.RefreshToken)
		logger.Info("Streamer Helix Client ready")
	} else {
		resp, err := client.RequestAppAccessToken([]string{})
		if err != nil {
			logger.Error("Could not get an App Access Token", "error", err)
			return nil, err
		}

		client.SetAppAccessToken(resp.Data.AccessToken)
		logger.Info("Global Helix Client ready")
	}

	return client, nil
}

// GetGlobalHelixClient return a none streamer related helix client
func GetGlobalHelixClient() (*helix.Client, error) {
	return getHelixClient(nil, slog.With("scope", "HELIX"), nil)
}

// GetStreamerHelixClient return an helix client configured with the UserAccessToken of a streamer
func GetStreamerHelixClient(ctx *types.BotContext, streamer *types.Streamer) (*helix.Client, error) {
	return getHelixClient(ctx, slog.With("scope", "HELIX", "username", streamer.Username), streamer)
}
