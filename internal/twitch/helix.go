package twitch

import (
	"mitoboat/internal/domain"
	"mitoboat/internal/env"

	"github.com/nicklaw5/helix/v2"
)

func getBaseHelixClient() (*helix.Client, error) {
	client, err := helix.NewClient(&helix.Options{
		ClientID:     env.DefaultEnv.TwitchId,
		ClientSecret: env.DefaultEnv.TwitchSecret,
	})

	if err != nil {
		return nil, err
	}
	return client, nil
}

// GetGlobalHelixClient return a none streamer related helix client
func GetGlobalHelixClient() (*helix.Client, error) {
	client, err := getBaseHelixClient()
	resp, err := client.RequestAppAccessToken([]string{})
	if err != nil {
		return nil, err
	}

	client.SetAppAccessToken(resp.Data.AccessToken)
	return client, err
}

// GetStreamerHelixClient return an helix client configured with the UserAccessToken of a streamer
func GetStreamerHelixClient(ctx *domain.BotContext, streamer *domain.Streamer) (*helix.Client, error) {
	client, err := getBaseHelixClient()
	if err != nil {
		return nil, err
	}

	changed, err := ValidateAccessToken(ctx.GlobalHelix, &streamer.Token)
	if err != nil {
		return nil, err
	}

	if changed {
		if err = ctx.Db.Save(streamer).Error; err != nil {
			return nil, err
		}
	}

	client.SetUserAccessToken(streamer.Token.AccessToken)
	client.SetRefreshToken(streamer.Token.RefreshToken)
	return client, err
}
