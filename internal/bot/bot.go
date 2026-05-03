package bot

import (
	"fmt"
	"log/slog"
	"mitoboat/internal/db"
	"mitoboat/internal/domain"
	"mitoboat/internal/flags"
	"mitoboat/internal/twitch"

	irc "github.com/gempir/go-twitch-irc/v4"
)

type MitoBoat struct {
	Context *domain.BotContext
	Version string
}

var version = "1.0.0"

// SetupDb initialize a connection and run a migration on the Database.
func SetupDb(flags *flags.BotFlags) error {
	database, err := db.ConnectDb(*flags.Verbose)
	if err != nil {
		return err
	}

	err = db.AutoMigrate(database)
	if err != nil {
		return err
	}

	return nil
}

// Create initialize the bot context by creating logger, loading environment variables,
// connect to database, create global helix client and create IRC client.
func Create(flags *flags.BotFlags) (*MitoBoat, error) {
	ctx := &domain.BotContext{}

	ds, err := db.ConnectDb(*flags.Verbose)
	if err != nil {
		return nil, err
	}
	ctx.Db = ds

	helixClient, err := twitch.GetGlobalHelixClient()
	if err != nil {
		return nil, err
	}
	ctx.GlobalHelix = helixClient

	ircClient, err := twitch.GetIrcClient(ctx)
	if err != nil {
		return nil, err
	}
	ctx.IrcClient = ircClient

	return &MitoBoat{
		Context: ctx,
		Version: version,
	}, nil
}

// Listen connect irc client and listen on websocket
func (bot *MitoBoat) Listen() error {
	logger := slog.With("scope", "IRC")

	bot.Context.IrcClient.OnConnect(func() { logger.Info("IRC connection established") })
	bot.Context.IrcClient.OnPrivateMessage(func(message irc.PrivateMessage) { handlePrivateMessage(bot.Context, message) })

	var streamers []domain.Streamer
	bot.Context.Db.Find(&streamers)

	logger.Info("All streamers found in database", "count", len(streamers))
	for _, streamer := range streamers {
		logger.Debug("Joining streamer channel", "username", streamer.Username)

		bot.Context.IrcClient.Join(streamer.Username)
		helixClient, err := twitch.GetStreamerHelixClient(bot.Context, &streamer)
		if err != nil {
			logger.Warn("Cannot create helix client for streamer", "username", streamer.Username)
		}

		bot.Context.StreamerContexts = append(bot.Context.StreamerContexts, domain.StreamerContext{
			Streamer:  &streamer,
			UserHelix: helixClient,
		})
	}
	logger.Info("Joined all registered streamers channel")

	err := bot.Context.IrcClient.Connect()
	if err != nil {
		return fmt.Errorf("irc connection crashed: %w", err)
	}
	return nil
}
