package twitchUtils

import "mitoboat/internal/types"

func GetStreamerContext(ctx *types.BotContext, streamer *types.Streamer) *types.StreamerContext {
	for _, sctx := range ctx.StreamerContexts {
		if streamer.ID == sctx.Streamer.ID {
			return sctx
		}
	}

	return nil
}
