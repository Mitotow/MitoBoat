package types

import (
	"time"

	"gorm.io/gorm"
)

type BotToken struct {
	ID           uint
	AccessToken  string
	RefreshToken string
	ExpiresAt    string
}

type StreamerToken struct {
	ID           uint
	AccessToken  string
	RefreshToken string
	ExpiresAt    string
}

type Streamer struct {
	ID        string
	Username  string
	Token     StreamerToken `gorm:"embedded"`
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
