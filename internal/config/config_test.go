package config

import (
	"strings"
	"testing"
	"time"
)

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("TWITCH_ID", "id")
	t.Setenv("TWITCH_SECRET", "secret")
	t.Setenv("IRC_USER", "mitoboat")
	t.Setenv("DB_NAME", "mitoboat")
	t.Setenv("DB_USER", "postgres")
	t.Setenv("DB_PSSWD", "password")
}

func TestLoadDefaults(t *testing.T) {
	setRequired(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.DBHost != "127.0.0.1" || cfg.DBPort != "5432" {
		t.Errorf("database host/port = %s:%s, want 127.0.0.1:5432", cfg.DBHost, cfg.DBPort)
	}
	if cfg.DBMaxOpenConns != 10 {
		t.Errorf("DBMaxOpenConns = %d, want 10", cfg.DBMaxOpenConns)
	}
	if cfg.SayBurst != 20 || cfg.SayWindow != 30*time.Second {
		t.Errorf("say limit = %d/%s, want 20/30s", cfg.SayBurst, cfg.SayWindow)
	}
	if cfg.CommandCacheTTL != 5*time.Minute {
		t.Errorf("CommandCacheTTL = %s, want 5m", cfg.CommandCacheTTL)
	}
}

func TestLoadMissingRequired(t *testing.T) {
	setRequired(t)
	t.Setenv("TWITCH_ID", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load must fail when TWITCH_ID is empty")
	}
}

func TestLoadRejectsBadPoolBounds(t *testing.T) {
	tests := []struct {
		name string
		key  string
		val  string
		want string
	}{
		{"zero open conns", "DB_MAX_OPEN_CONNS", "0", "DB_MAX_OPEN_CONNS"},
		{"idle above open", "DB_MAX_IDLE_CONNS", "99", "DB_MAX_IDLE_CONNS"},
		{"zero burst", "SAY_BURST", "0", "SAY_BURST"},
		{"zero cache ttl", "COMMAND_CACHE_TTL", "0s", "COMMAND_CACHE_TTL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequired(t)
			t.Setenv(tt.key, tt.val)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load must reject %s=%s", tt.key, tt.val)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it to mention %s", err, tt.want)
			}
		})
	}
}

func TestDSN(t *testing.T) {
	cfg := &Config{
		DBHost: "db.internal", DBPort: "5433", DBUser: "bot",
		DBPassword: "hunter2", DBName: "mitoboat", DBSSLMode: "require",
	}

	want := "host=db.internal port=5433 user=bot password=hunter2 dbname=mitoboat sslmode=require"
	if got := cfg.DSN(); got != want {
		t.Errorf("DSN() = %q, want %q", got, want)
	}
}
