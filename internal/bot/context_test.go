package bot

import (
	"mitoboat/internal/domain"
	"testing"
)

func createStreamerCtxId(id string) domain.StreamerContext {
	return domain.StreamerContext{Streamer: &domain.Streamer{ID: id}}
}

func createStreamerCtxName(name string) domain.StreamerContext {
	return domain.StreamerContext{Streamer: &domain.Streamer{Username: name}}
}

func TestGetStreamerContextById(t *testing.T) {
	var streamerContexts []domain.StreamerContext
	streamerContexts = append(streamerContexts, createStreamerCtxId("12345678"))
	ctx := &domain.BotContext{StreamerContexts: streamerContexts}

	if GetStreamerContextById(ctx, "") != nil {
		t.Errorf("Expected streamer with id %s to be nil", "")
	}

	if GetStreamerContextById(ctx, "1234") != nil {
		t.Errorf("Expected streamer with id %s to be nil", "1234")
	}

	if GetStreamerContextById(ctx, "12345678") == nil {
		t.Errorf("Expected streamer with id %s not to be nil", "12345678")
	}

	streamerContexts = append(streamerContexts, createStreamerCtxId("87654321"))
	ctx.StreamerContexts = streamerContexts

	if GetStreamerContextById(ctx, "12345678") == nil {
		t.Errorf("Expected streamer with id %s not to be nil", "12345678")
	}

	if GetStreamerContextById(ctx, "87654321") == nil {
		t.Errorf("Expected streamer with id %s not to be nil", "87654321")
	}
}

func TestGetStreamerContextByUser(t *testing.T) {
	var streamerContexts []domain.StreamerContext
	streamerContexts = append(streamerContexts, createStreamerCtxName("mitoboat"))
	ctx := &domain.BotContext{StreamerContexts: streamerContexts}

	if GetStreamerContextByUser(ctx, "") != nil {
		t.Errorf("Expected streamer with id %s to be nil", "")
	}

	if GetStreamerContextByUser(ctx, "mito") != nil {
		t.Errorf("Expected streamer with id %s to be nil", "mito")
	}

	if GetStreamerContextByUser(ctx, "mitoboat") == nil {
		t.Errorf("Expected streamer with id %s not to be nil", "mitoboat")
	}

	streamerContexts = append(streamerContexts, createStreamerCtxName("mitotow"))
	ctx.StreamerContexts = streamerContexts

	if GetStreamerContextByUser(ctx, "mitoboat") == nil {
		t.Errorf("Expected streamer with id %s not to be nil", "mitoboat")
	}

	if GetStreamerContextByUser(ctx, "mitotow") == nil {
		t.Errorf("Expected streamer with id %s not to be nil", "mitotow")
	}
}
