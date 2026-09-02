// Package domain holds the persisted entities of the bot.
//
// It deliberately depends only on GORM (for the struct tags that describe the
// schema) and never on the Twitch API or IRC clients: those are infrastructure
// concerns and live in internal/twitch. Keeping them out means the entities can
// be constructed and asserted on in tests without any network client.
package domain

import (
	"time"

	"gorm.io/gorm"
)

// Token is an OAuth token pair. It is embedded into the rows that own one
// rather than stored in its own table.
type Token struct {
	AccessToken  string `gorm:"not null"`
	RefreshToken string `gorm:"not null"`
	// ExpiresAt is the moment the access token stops being accepted. A zero
	// value means "unknown", which callers treat as "validate before use".
	ExpiresAt time.Time
}

// Expired reports whether the token is past its expiry, with a safety margin so
// a token is refreshed slightly before Twitch starts rejecting it.
func (t Token) Expired(margin time.Duration) bool {
	if t.ExpiresAt.IsZero() {
		return true
	}
	return time.Now().Add(margin).After(t.ExpiresAt)
}

// BotToken is the single user token the bot authenticates to IRC with.
type BotToken struct {
	ID        uint  `gorm:"primaryKey"`
	Token     Token `gorm:"embedded"`
	UpdatedAt time.Time
}

// Streamer is a channel the bot has been invited to.
type Streamer struct {
	// ID is the Twitch user id, which is stable across renames. Usernames are
	// not, which is why they are never used as the primary key.
	ID string `gorm:"primaryKey"`
	// Username is stored lowercased: Twitch IRC reports channel names in lower
	// case, and lookups happen on every chat message.
	Username string `gorm:"uniqueIndex;not null"`
	Token    Token  `gorm:"embedded"`
	// Active lets a streamer be parked without deleting their commands.
	Active    bool `gorm:"not null;default:true"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// GlobalCommand is a static text command available in every joined channel.
type GlobalCommand struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"uniqueIndex;not null"`
	Text      string `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CustomTextCommand is a text command defined by one streamer for their own
// channel. It shadows a GlobalCommand of the same name.
//
// The previous revision embedded a TextCommand struct that carried its own ID
// field alongside gorm.Model, so two fields mapped to the "id" column. The
// columns are declared flat here to keep the schema unambiguous.
type CustomTextCommand struct {
	ID         uint   `gorm:"primaryKey"`
	StreamerID string `gorm:"index;uniqueIndex:idx_streamer_command;not null"`
	Name       string `gorm:"uniqueIndex:idx_streamer_command;not null"`
	Text       string `gorm:"not null"`
	// Streamer is the owning channel. It is not preloaded on the hot path; the
	// command cache is keyed by StreamerID directly.
	Streamer  Streamer `gorm:"foreignKey:StreamerID;constraint:OnDelete:CASCADE"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
