package bot

import "mitoboat/internal/domain"

func getStreamerContext(ctx *domain.BotContext, filter func(*domain.StreamerContext) bool) *domain.StreamerContext {
	for _, sctx := range ctx.StreamerContexts {
		if filter(sctx) {
			return sctx
		}
	}

	return nil
}

// GetStreamerContextById return the StreamerContext using the streamer's id
func GetStreamerContextById(ctx *domain.BotContext, ID string) *domain.StreamerContext {
	return getStreamerContext(ctx, func(sctx *domain.StreamerContext) bool {
		return sctx.Streamer.ID == ID
	})
}

// GetStreamerContextByUser return the StreamerContext using the streamer's username
func GetStreamerContextByUser(ctx *domain.BotContext, username string) *domain.StreamerContext {
	return getStreamerContext(ctx, func(sctx *domain.StreamerContext) bool {
		return sctx.Streamer.Username == username
	})
}
