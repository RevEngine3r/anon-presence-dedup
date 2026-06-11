package db

import (
	"context"
	"time"

	"github.com/RevEngine3r/anon-presence-dedup/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(cfg config.PostgresConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, err
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return pgxpool.NewWithConfig(ctx, poolCfg)
}
