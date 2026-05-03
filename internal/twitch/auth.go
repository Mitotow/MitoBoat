package twitch

import (
	"mitoboat/internal/domain"

	"github.com/nicklaw5/helix/v2"
)

// ValidateAccessToken ask via helix if the given token is valid.
// If not, it will call the RefreshUserAccessToken function and set
// the new Access Token and Refresh Token in the token struct given
func ValidateAccessToken(helixClient *helix.Client, token *domain.Token) (bool, error) {
	v, vres, verr := helixClient.ValidateToken(token.AccessToken)
	if !v && vres.StatusCode == 200 && verr == nil {
		res, err := helixClient.RefreshUserAccessToken(token.RefreshToken)
		if err != nil {
			return true, err
		}

		token.AccessToken = res.Data.AccessToken
		token.RefreshToken = res.Data.RefreshToken
	}

	return !v, verr
}
