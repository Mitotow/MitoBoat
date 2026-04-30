package db

import (
	"fmt"
	"log/slog"
	"mitoboat/internal/types"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&types.BotToken{}, &types.Streamer{})
}

// ConnectDb format the dsn and initialize session to db
func ConnectDb(migrate bool) (*gorm.DB, error) {
	logger := slog.With("scope", "DB")
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable", os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"), os.Getenv("DB_PSSWD"), os.Getenv("DB_NAME"), os.Getenv("DB_PORT"))

	ds, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Error("Could not connect to Database", "error", err)
		return nil, err
	}
	logger.Info("Connected to Database")

	if migrate {
		err = autoMigrate(ds)
		if err != nil {
			logger.Error("Could not auto migrate Database", "error", err)
			return nil, err
		}
		logger.Info("Database as been automatically migrated")
	}

	return ds, nil
}
