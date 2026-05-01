package types

import "gorm.io/gorm"

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
