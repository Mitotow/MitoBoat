package bot

import (
	"log/slog"
	"mitoboat/internal/db"
	"mitoboat/internal/flags"
	"mitoboat/internal/types"
	"mitoboat/internal/utils"
	"os"

	"github.com/gempir/go-twitch-irc/v4"
	"github.com/joho/godotenv"
)

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

func init() {
	envErr := godotenv.Load()
	logger := createLogger()
	slog.SetDefault(logger)

	// Handle envErr after logger creation because createLogger may require the .env file to be load
	if envErr != nil {
		logger.With("scope", "ENV").Warn("Aucun fichier .env trouvé, utilisation des variables système",
			"erreur", envErr)
	}
}

// SetupDb initialize a connection and run a migration on the Database.
func SetupDb(flags *flags.BotFlags) error {
	_, err := db.ConnectDb(true, *flags.Verbose)
	if err != nil {
		return err
	}

	return nil
}

// SetupBot initialize the bot context by creating logger, loading environment variables,
// connect to database, create global helix client and create IRC client.
func SetupBot(flags *flags.BotFlags) (*types.BotContext, error) {
	ctx := &types.BotContext{}

	ds, err := db.ConnectDb(false, *flags.Verbose)
	if err != nil {
		return nil, err
	}
	ctx.Db = ds

	helixClient, err := utils.GetGlobalHelixClient()
	if err != nil {
		return nil, err
	}
	ctx.GlobalHelix = helixClient

	ircClient, err := utils.GetIrcClient(ctx)
	if err != nil {
		return nil, err
	}
	ctx.IrcClient = ircClient

	return ctx, nil
}

// Listen connect irc client and listen on websocket
func Listen(ctx *types.BotContext) {
	logger := slog.With("scope", "IRC")

	ctx.IrcClient.OnConnect(func() { logger.Info("IRC connection established") })
	ctx.IrcClient.OnPrivateMessage(func(message twitch.PrivateMessage) { handlePrivateMessage(ctx, message) })

	var streamers []types.Streamer
	ctx.Db.Debug().Find(&streamers)

	logger.Debug("All streamers found in database", "count", len(streamers))
	for _, streamer := range streamers {
		logger.Debug("Joining streamer channel", "username", streamer.Username)

		ctx.IrcClient.Join(streamer.Username)
		helixClient, _ := utils.GetStreamerHelixClient(ctx, &streamer)
		ctx.StreamerContexts = append(ctx.StreamerContexts, &types.StreamerContext{
			Streamer:  &streamer,
			UserHelix: helixClient,
		})
	}
	logger.Info("Joined all registered streamers channel")

	err := ctx.IrcClient.Connect()
	if err != nil {
		logger.Error("IRC connection crashed", "error", err)
	}
}
