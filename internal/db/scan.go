package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ListVideoPaths returns the set of source file paths already registered, so a
// library scan can skip files it has seen before.
func ListVideoPaths(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	rows, err := pool.Query(ctx, `SELECT file_path FROM videos`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	paths := map[string]bool{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths[p] = true
	}
	return paths, rows.Err()
}

// InsertScannedVideo registers a file found on disk as a pending video. is_vr
// routes it to the personal VR library. Ignores a duplicate path (unique race
// with a concurrent scan) rather than erroring.
func InsertScannedVideo(ctx context.Context, pool *pgxpool.Pool, title, path string, isVR bool) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO videos (title, file_path, is_vr) VALUES ($1, $2, $3)
		 ON CONFLICT DO NOTHING`,
		title, path, isVR)
	return err
}
