package types

import (
	"time"

	"gorm.io/gorm"
)

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
