package db

type BotToken struct {
	ID           uint   `gorm:"primaryKey"`
	AccessToken  string `gorm:"not null"`
	RefreshToken string `gorm:"not null"`
}
