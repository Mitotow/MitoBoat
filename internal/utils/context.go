package utils

import "mitoboat/internal/types"

func getStreamerContext(ctx *types.BotContext, filter func(*types.StreamerContext) bool) *types.StreamerContext {
	for _, sctx := range ctx.StreamerContexts {
		if filter(sctx) {
			return sctx
		}
	}

	return nil
}

func GetStreamerContextById(ctx *types.BotContext, ID string) *types.StreamerContext {
	return getStreamerContext(ctx, func(sctx *types.StreamerContext) bool {
		return sctx.Streamer.ID == ID
	})
}

func GetStreamerContextByUser(ctx *types.BotContext, username string) *types.StreamerContext {
	return getStreamerContext(ctx, func(sctx *types.StreamerContext) bool {
		return sctx.Streamer.Username == username
	})
}
