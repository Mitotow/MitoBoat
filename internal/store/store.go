// Package store owns the database connection and all SQL access.
//
// Nothing outside this package builds queries: callers ask for entities and get
// plain domain values back. That keeps the GORM dependency in one place and
// makes it obvious which code paths can hit the database.
package store

import (
	"context"
	"fmt"
	"log/slog"

	"mitoboat/internal/config"
	"mitoboat/internal/domain"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Store is a handle on the database.
type Store struct {
	db *gorm.DB
}

// Open connects to PostgreSQL and applies the configured pool bounds.
func Open(cfg *config.Config, verbose bool) (*Store, error) {
	logLevel := gormlogger.Silent
	if verbose {
		logLevel = gormlogger.Info
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: gormlogger.Default.LogMode(logLevel),
		// The bot never relies on GORM's implicit transaction per write, and
		// disabling it removes a round trip from every statement.
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("access underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.DBMaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.DBMaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.DBConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.DBConnMaxIdleTime)

	slog.Info("Connected to database",
		"scope", "DB",
		"host", cfg.DBHost,
		"name", cfg.DBName,
		"max_open_conns", cfg.DBMaxOpenConns)

	return &Store{db: db}, nil
}

// Close releases the connection pool.
func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("access underlying sql.DB: %w", err)
	}
	return sqlDB.Close()
}

// Ping verifies the database is reachable.
func (s *Store) Ping(ctx context.Context) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("access underlying sql.DB: %w", err)
	}
	return sqlDB.PingContext(ctx)
}

// Migrate creates or updates the schema.
func (s *Store) Migrate() error {
	err := s.db.AutoMigrate(
		&domain.BotToken{},
		&domain.Streamer{},
		&domain.GlobalCommand{},
		&domain.CustomTextCommand{},
	)
	if err != nil {
		return fmt.Errorf("run automigration: %w", err)
	}
	return nil
}
