// Package config loads and validates the bot configuration from the environment.
//
// Configuration is returned as an explicit value rather than stored in a package
// level singleton, so that every component receives its dependencies through its
// constructor and can be exercised in tests without touching the environment.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// Config holds every tunable the bot reads at startup.
type Config struct {
	LogLevel string `envconfig:"LOG_LEVEL" default:"INFO"`

	TwitchID     string `envconfig:"TWITCH_ID" required:"true"`
	TwitchSecret string `envconfig:"TWITCH_SECRET" required:"true"`
	IRCUser      string `envconfig:"IRC_USER" required:"true"`

	DBHost     string `envconfig:"DB_HOST" default:"127.0.0.1"`
	DBPort     string `envconfig:"DB_PORT" default:"5432"`
	DBName     string `envconfig:"DB_NAME" required:"true"`
	DBUser     string `envconfig:"DB_USER" required:"true"`
	DBPassword string `envconfig:"DB_PSSWD" required:"true"`
	DBSSLMode  string `envconfig:"DB_SSLMODE" default:"disable"`

	// Connection pool bounds. database/sql defaults to an unlimited number of
	// open connections, which lets a burst of traffic allocate one connection
	// (and its buffers) per concurrent query. Bounding it keeps the footprint
	// flat regardless of how many channels the bot has joined.
	DBMaxOpenConns    int           `envconfig:"DB_MAX_OPEN_CONNS" default:"10"`
	DBMaxIdleConns    int           `envconfig:"DB_MAX_IDLE_CONNS" default:"2"`
	DBConnMaxLifetime time.Duration `envconfig:"DB_CONN_MAX_LIFETIME" default:"30m"`
	DBConnMaxIdleTime time.Duration `envconfig:"DB_CONN_MAX_IDLE_TIME" default:"5m"`

	// CommandCacheTTL is how often the in-memory command cache is rebuilt from
	// the database. Chat messages are served entirely from that cache, so this
	// is the only thing standing between a command edit and it going live.
	CommandCacheTTL time.Duration `envconfig:"COMMAND_CACHE_TTL" default:"5m"`

	// CommandCooldown is the minimum delay between two executions of the same
	// command in the same channel. It stops a command from being used to flood
	// chat and breaks feedback loops between bots.
	CommandCooldown time.Duration `envconfig:"COMMAND_COOLDOWN" default:"3s"`

	// Outbound chat rate limit, applied per channel. Twitch allows 20 messages
	// per 30s for a regular account and 100 for a moderator; exceeding it gets
	// the whole bot account timed out, on every channel at once.
	SayBurst  int           `envconfig:"SAY_BURST" default:"20"`
	SayWindow time.Duration `envconfig:"SAY_WINDOW" default:"30s"`
}

// Load reads the .env file if present, then populates a Config from the
// environment. A missing .env file is not an error: in production the values
// are expected to come from the real environment.
func Load() (*Config, error) {
	// Values already present in the environment win over the .env file.
	_ = godotenv.Load()

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("read configuration from environment: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	// envconfig's `required` only checks that a variable is set, so an empty
	// value satisfies it and then fails much later as a confusing auth error.
	required := map[string]string{
		"TWITCH_ID":     c.TwitchID,
		"TWITCH_SECRET": c.TwitchSecret,
		"IRC_USER":      c.IRCUser,
		"DB_NAME":       c.DBName,
		"DB_USER":       c.DBUser,
		"DB_PSSWD":      c.DBPassword,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s must not be empty", name)
		}
	}

	if c.DBMaxOpenConns < 1 {
		return fmt.Errorf("DB_MAX_OPEN_CONNS must be at least 1, got %d", c.DBMaxOpenConns)
	}
	if c.DBMaxIdleConns > c.DBMaxOpenConns {
		return fmt.Errorf("DB_MAX_IDLE_CONNS (%d) cannot exceed DB_MAX_OPEN_CONNS (%d)",
			c.DBMaxIdleConns, c.DBMaxOpenConns)
	}
	if c.SayBurst < 1 {
		return fmt.Errorf("SAY_BURST must be at least 1, got %d", c.SayBurst)
	}
	if c.SayWindow <= 0 {
		return fmt.Errorf("SAY_WINDOW must be positive, got %s", c.SayWindow)
	}
	if c.CommandCacheTTL <= 0 {
		return fmt.Errorf("COMMAND_CACHE_TTL must be positive, got %s", c.CommandCacheTTL)
	}
	return nil
}

// DSN builds the PostgreSQL connection string.
func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode)
}
