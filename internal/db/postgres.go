package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL) // cfg is a pointer to a struct with all the connection settings
	if err != nil {
		return nil, fmt.Errorf("parse db url: %w", err)
	}

	// Connection pool tuning
	cfg.MaxConns = 20                      //maximum # of connection at a time.
	cfg.MinConns = 2                       //keep at least 2 connections alive for low traffic periods to avoid cold starts.
	cfg.MaxConnLifetime = 30 * time.Minute // after 30 mins, connection is replaced with a new one to prevent stale connections.
	cfg.MaxConnIdleTime = 5 * time.Minute  // if a connection is idle for 5 mins, close it. helps to free resources during low traffic.

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg) // pool is a pointer to a struct that manages a pool of connections to the database, and provides methods for acquiring and releasing connections.
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	// Verify connection on startup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // Max 5 seconds to test connection on startup
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return pool, nil
}
