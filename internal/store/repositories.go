package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"mitoboat/internal/domain"

	"gorm.io/gorm"
)

// ErrNotFound is returned when a lookup matches no row.
var ErrNotFound = errors.New("not found")

// BotToken loads the single token the bot authenticates to IRC with.
func (s *Store) BotToken(ctx context.Context) (*domain.BotToken, error) {
	var token domain.BotToken
	err := s.db.WithContext(ctx).Order("id").First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("load bot token: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("load bot token: %w", err)
	}
	return &token, nil
}

// SaveBotToken persists a refreshed bot token.
func (s *Store) SaveBotToken(ctx context.Context, token *domain.BotToken) error {
	if err := s.db.WithContext(ctx).Save(token).Error; err != nil {
		return fmt.Errorf("save bot token: %w", err)
	}
	return nil
}

// ActiveStreamers lists the channels the bot should join.
func (s *Store) ActiveStreamers(ctx context.Context) ([]domain.Streamer, error) {
	var streamers []domain.Streamer
	err := s.db.WithContext(ctx).Where("active = ?", true).Find(&streamers).Error
	if err != nil {
		return nil, fmt.Errorf("list active streamers: %w", err)
	}
	return streamers, nil
}

// SaveStreamerToken persists a refreshed streamer token without touching the
// rest of the row, so a concurrent update to e.g. Active is not clobbered.
func (s *Store) SaveStreamerToken(ctx context.Context, id string, token domain.Token) error {
	err := s.db.WithContext(ctx).
		Model(&domain.Streamer{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"access_token":  token.AccessToken,
			"refresh_token": token.RefreshToken,
			"expires_at":    token.ExpiresAt,
		}).Error
	if err != nil {
		return fmt.Errorf("save token for streamer %s: %w", id, err)
	}
	return nil
}

// GlobalCommands loads every channel-independent text command.
//
// Only the two columns the cache needs are selected: the bot never has a reason
// to pull timestamps into memory for commands it only ever reads the text of.
func (s *Store) GlobalCommands(ctx context.Context) (map[string]string, error) {
	var rows []domain.GlobalCommand
	err := s.db.WithContext(ctx).
		Model(&domain.GlobalCommand{}).
		Select("name", "text").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list global commands: %w", err)
	}

	commands := make(map[string]string, len(rows))
	for _, row := range rows {
		commands[strings.ToLower(row.Name)] = row.Text
	}
	return commands, nil
}

// CustomCommands loads every per-streamer text command, grouped by streamer id.
//
// This is one query for all streamers rather than one per streamer: the result
// is small enough to hold in memory and the bot would otherwise issue N queries
// on every cache refresh.
func (s *Store) CustomCommands(ctx context.Context) (map[string]map[string]string, error) {
	var rows []domain.CustomTextCommand
	err := s.db.WithContext(ctx).
		Model(&domain.CustomTextCommand{}).
		Select("streamer_id", "name", "text").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list custom commands: %w", err)
	}

	commands := make(map[string]map[string]string)
	for _, row := range rows {
		byName := commands[row.StreamerID]
		if byName == nil {
			byName = make(map[string]string)
			commands[row.StreamerID] = byName
		}
		byName[strings.ToLower(row.Name)] = row.Text
	}
	return commands, nil
}
