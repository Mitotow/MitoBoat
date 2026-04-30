package types

import (
	"github.com/gempir/go-twitch-irc/v4"
	"github.com/nicklaw5/helix/v2"
	"gorm.io/gorm"
)

type StreamerContext struct {
	Streamer  *Streamer
	UserHelix *helix.Client
}

type BotContext struct {
	Db               *gorm.DB
	IrcClient        *twitch.Client
	GlobalHelix      *helix.Client
	StreamerContexts []*StreamerContext
}
