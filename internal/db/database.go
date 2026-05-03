package db

import (
	"fmt"
	"log/slog"
	"mitoboat/internal/domain"
	"mitoboat/internal/env"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&domain.BotToken{}, &domain.Streamer{}, &domain.TextCommand{}, &domain.CustomTextCommand{})
}

func getConfig(verbose bool) *gorm.Config {
	var logLevel logger.LogLevel
	if verbose {
		logLevel = logger.Info
	} else {
		logLevel = logger.Silent
	}

	return &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	}
}

// ConnectDb format the dsn and initialize session to db
func ConnectDb(verbose bool) (*gorm.DB, error) {
	logger := slog.With("scope", "DB")
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable", env.DefaultEnv.DBHost,
		env.DefaultEnv.DBUser, env.DefaultEnv.DBPsswd, env.DefaultEnv.DBName, env.DefaultEnv.DBPort)

	ds, err := gorm.Open(postgres.Open(dsn), getConfig(verbose))

	if err != nil {
		logger.Error("Could not connect to Database", "error", err)
		return nil, err
	}
	logger.Info("Connected to Database")

	return ds, nil
}
