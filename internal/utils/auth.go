package utils

import "mitoboat/internal/types"

// ValidateAccessToken ask via helix if the given token is valid.
// If not, it will call the RefreshUserAccessToken function and set
// the new Access Token and Refresh Token in the token struct given
func ValidateAccessToken(ctx *types.BotContext, token *types.Token) (bool, error) {
	v, vres, verr := ctx.GlobalHelix.ValidateToken(token.AccessToken)
	if !v && vres.StatusCode == 200 && verr == nil {
		res, err := ctx.GlobalHelix.RefreshUserAccessToken(token.RefreshToken)
		if err != nil {
			return true, err
		}

		token.AccessToken = res.Data.AccessToken
		token.RefreshToken = res.Data.RefreshToken
	}

	return !v, verr
}
