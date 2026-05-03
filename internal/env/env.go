package env

import (
	"sync"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Env struct {
	LogLevel     string `envconfig:"LOG_LEVEL" default:"INFO"`
	TwitchId     string `envconfig:"TWITCH_ID" required:"true"`
	TwitchSecret string `envconfig:"TWITCH_SECRET" required:"true"`
	IrcUser      string `envconfig:"IRC_USER" required:"true"`
	HttpPort     string `envconfig:"HTTP_PORT" default:"8080"`
	DBHost       string `envconfig:"DB_HOST" default:"127.0.0.1"`
	DBPort       string `envconfig:"DB_PORT" default:"5432"` // 5432 => default psql port
	DBName       string `envconfig:"DB_NAME" required:"true"`
	DBUser       string `envconfig:"DB_USER" required:"true"`
	DBPsswd      string `envconfig:"DB_PSSWD" required:"true"`
}

var (
	DefaultEnv *Env
	once       sync.Once
)

// Load read environment variables and initialize the DefaultEnv instance
func Load() error {
	var err error
	godotenv.Load()

	once.Do(func() {
		var e Env
		err = envconfig.Process("", &e)
		if err == nil {
			DefaultEnv = &e
		}
	})

	return err
}
