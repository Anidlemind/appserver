package db

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool() (*pgxpool.Pool, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, err
	}

	if err := InitSchema(context.Background(), pool); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}

func InitSchema(ctx context.Context, pool *pgxpool.Pool) error {
	const query = `
	CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		password_salt TEXT NOT NULL,
		password_hash TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS auth_tokens (
		token TEXT PRIMARY KEY,
		user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		expires_at TIMESTAMPTZ NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS raw_vehicle_stats (
		id BIGSERIAL PRIMARY KEY,
		user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		vehicle_type TEXT NOT NULL,
		fuel_liters DOUBLE PRECISION NOT NULL CHECK (fuel_liters >= 0),
		distance_km DOUBLE PRECISION NOT NULL CHECK (distance_km >= 0),
		recorded_at TIMESTAMPTZ NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_raw_vehicle_stats_vehicle_type
		ON raw_vehicle_stats(vehicle_type);

	CREATE INDEX IF NOT EXISTS idx_auth_tokens_user_id
		ON auth_tokens(user_id);

	CREATE TABLE IF NOT EXISTS vehicle_type_aggregates (
		vehicle_type TEXT PRIMARY KEY,
		total_fuel_liters DOUBLE PRECISION NOT NULL DEFAULT 0,
		total_distance_km DOUBLE PRECISION NOT NULL DEFAULT 0,
		total_records BIGINT NOT NULL DEFAULT 0,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	`

	_, err := pool.Exec(ctx, query)
	return err
}
