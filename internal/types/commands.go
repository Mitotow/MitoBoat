package types

import "gorm.io/gorm"

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
