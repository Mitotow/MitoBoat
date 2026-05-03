package domain

import (
	"time"

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
	StreamerContexts []StreamerContext
}

type Token struct {
	AccessToken  string
	RefreshToken string
}

type BotToken struct {
	ID        uint
	Token     Token `gorm:"embedded"`
	UpdatedAt time.Time
}

type Streamer struct {
	ID        string
	Username  string
	Token     Token `gorm:"embedded"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type ReplyableCommand interface {
	GetText() string
}

type TextCommand struct {
	ID   string
	Name string
	Text string
}

type CustomTextCommand struct {
	gorm.Model
	Command    TextCommand `gorm:"embedded"`
	StreamerID string
	Streamer   Streamer
}

func (c *TextCommand) GetText() string {
	return c.Text
}

func (c *CustomTextCommand) GetText() string {
	return c.Command.Text
}
