package types

type BotToken struct {
	ID           uint   `gorm:"primaryKey"`
	AccessToken  string `gorm:"not null"`
	RefreshToken string `gorm:"not null"`
	ExpiresAt    string `gorm:"not null"`
}

type Streamer struct {
	ID           string `gorm:"primaryKey"`
	Username     string `gorm:"not null"`
	AccessToken  string
	RefreshToken string
	ExpiresAt    string
}
