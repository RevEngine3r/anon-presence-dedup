package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level structure mirroring server.yml.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Postgres  PostgresConfig  `yaml:"postgres"`
	Dedup     DedupConfig     `yaml:"dedup"`
	WebSocket WebSocketConfig `yaml:"websocket"`
	Log       LogConfig       `yaml:"log"`
}

type ServerConfig struct {
	Addr         string        `yaml:"addr"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
}

type PostgresConfig struct {
	DSN             string        `yaml:"dsn"`
	MaxConns        int32         `yaml:"max_conns"`
	MinConns        int32         `yaml:"min_conns"`
	MaxConnIdleTime time.Duration `yaml:"max_conn_idle_time"`
}

type DedupConfig struct {
	FlushInterval time.Duration `yaml:"flush_interval"`
	CleanInterval time.Duration `yaml:"clean_interval"`
	ViewTTL       time.Duration `yaml:"view_ttl"`
	MaxEntries    int           `yaml:"max_entries"`
}

type WebSocketConfig struct {
	BroadcastThrottle time.Duration `yaml:"broadcast_throttle"`
}

type LogConfig struct {
	Level string `yaml:"level"`
}

// Defaults applied before parsing the file.
func defaults() Config {
	return Config{
		Server: ServerConfig{
			Addr:         ":8080",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		Postgres: PostgresConfig{
			MaxConns:        20,
			MinConns:        2,
			MaxConnIdleTime: 30 * time.Minute,
		},
		Dedup: DedupConfig{
			FlushInterval: 30 * time.Second,
			CleanInterval: 15 * time.Minute,
			ViewTTL:       24 * time.Hour,
			MaxEntries:    2000,
		},
		WebSocket: WebSocketConfig{
			BroadcastThrottle: 350 * time.Millisecond,
		},
		Log: LogConfig{Level: "info"},
	}
}

// Load reads the YAML file at path and merges it over defaults.
// Path resolution order:
//  1. Explicit path argument (if non-empty)
//  2. CONFIG_FILE env var
//  3. ./server.yml  (next to binary)
func Load(path string) (*Config, error) {
	if path == "" {
		path = os.Getenv("CONFIG_FILE")
	}
	if path == "" {
		path = "server.yml"
	}

	cfg := defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No config file found — use pure defaults.
			// DSN must come from env in this case.
			if dsn := os.Getenv("POSTGRES_DSN"); dsn != "" {
				cfg.Postgres.DSN = dsn
			}
			return &cfg, nil
		}
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}

	// Allow env var to override DSN from file (useful in Docker).
	if dsn := os.Getenv("POSTGRES_DSN"); dsn != "" {
		cfg.Postgres.DSN = dsn
	}

	if cfg.Postgres.DSN == "" {
		return nil, fmt.Errorf("config: postgres.dsn is required (set in server.yml or POSTGRES_DSN env)")
	}

	return &cfg, nil
}
