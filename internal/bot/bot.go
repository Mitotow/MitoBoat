package bot

import (
	"fmt"
	"log/slog"
	"mitoboat/internal/db"
	"os"

	"github.com/gempir/go-twitch-irc/v4"
	"github.com/joho/godotenv"
	"github.com/nicklaw5/helix/v2"
	"gorm.io/gorm"
)

type BotContext struct {
	Logger    *slog.Logger
	db        *gorm.DB
	ircClient *twitch.Client
	apiClient *helix.Client
}

// createLogger Create the logger
func createLogger() *slog.Logger {
	var level slog.Level

	envLevel := os.Getenv("LOG_LEVEL")
	if envLevel == "" {
		envLevel = "INFO"
	}

	err := level.UnmarshalText([]byte(envLevel))
	if err != nil {
		level = slog.LevelInfo
	}

	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

// initDb Initialize the database connection
func initDb(ctx *BotContext) error {
	logger := ctx.Logger.With("scope", "DB")

	database, err := db.ConnectDb()
	if err != nil {
		logger.Error("Could not connect to Database", "error", err)
		return err
	}
	ctx.db = database
	logger.Info("Connected to Database", "dbName", database.Name())

	err = db.AutoMigrate(database)
	if err != nil {
		logger.Error("Could not auto migrate Database", "error", err)
		return err
	}
	logger.Info("Database as been automatically migrated", "dbName", database.Name())

	return nil
}

// initApiClient Initialize the API Client
func initApiClient(ctx *BotContext) error {
	logger := ctx.Logger.With("scope", "API")
	clientId := os.Getenv("TWITCH_ID")
	clientSecret := os.Getenv("TWITCH_SECRET")

	client, err := helix.NewClient(&helix.Options{ClientID: clientId, ClientSecret: clientSecret})
	if err != nil {
		logger.Error("Could not connect to Helix API", "error", err)
		return err
	}

	resp, err := client.RequestAppAccessToken([]string{})
	if err != nil {
		logger.Error("Could not get an App Access Token", "error", err)
		return err
	}
	client.SetAppAccessToken(resp.Data.AccessToken)

	ctx.apiClient = client
	logger.Info("API client connected", "client_id", clientId)
	return nil
}

// initIrcClient Initialize the IRC client
func initIrcClient(ctx *BotContext) error {
	logger := ctx.Logger.With("scope", "IRC")

	var token db.BotToken
	if err := ctx.db.First(&token, 1).Error; err != nil {
		logger.Error("No token found on Database", "error", err)
		return err
	}

	resp, err := ctx.apiClient.RefreshUserAccessToken(token.RefreshToken)
	if err != nil {
		logger.Error("Token Refresh failed", "error", err)
	}

	token.AccessToken = resp.Data.AccessToken
	token.RefreshToken = resp.Data.RefreshToken
	ctx.db.Save(&token)
	logger.Info("IRC Token refreshed and saved in Database")

	ctx.apiClient.SetUserAccessToken(token.AccessToken)

	clientUser := os.Getenv("IRC_USER")
	ircClient := twitch.NewClient(clientUser, fmt.Sprintf("oauth:%s", token.AccessToken))
	ctx.ircClient = ircClient

	logger.Info("IRC client created", "username", clientUser)
	return nil
}

// SetupBot initialize the bot context by creating logger, loading environment variables,
// connect database and connect the bot to twitch.
func SetupBot() (*BotContext, error) {
	ctx := &BotContext{}
	envErr := godotenv.Load()

	logger := createLogger()
	slog.SetDefault(logger)
	ctx.Logger = logger

	// Handle envErr after logger creation because createLogger may require the .env file to be load
	if envErr != nil {
		logger.With("scope", "ENV").Warn("Aucun fichier .env trouvé, utilisation des variables système",
			"erreur", envErr)
	}

	err := initDb(ctx)
	if err != nil {
		return nil, err
	}

	err = initApiClient(ctx)
	if err != nil {
		return nil, err
	}

	err = initIrcClient(ctx)
	if err != nil {
		return nil, err
	}

	return ctx, nil
}

func Listen(ctx *BotContext) {
	logger := ctx.Logger.With("scope", "RUNTIME")

	ctx.ircClient.OnConnect(func() { logger.Info("IRC connection established") })

	err := ctx.ircClient.Connect()
	if err != nil {
		logger.Error("IRC connection crashed", "error", err)
	}
}
