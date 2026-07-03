package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// GetOrCreateSetting returns the value stored under key, initializing it with
// candidate when absent. First writer wins, so the value is stable across
// restarts (used for app-generated secrets).
func GetOrCreateSetting(ctx context.Context, pool *pgxpool.Pool, key, candidate string) (string, error) {
	if _, err := pool.Exec(ctx,
		`INSERT INTO settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO NOTHING`,
		key, candidate); err != nil {
		return "", err
	}
	var v string
	err := pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = $1`, key).Scan(&v)
	return v, err
}
