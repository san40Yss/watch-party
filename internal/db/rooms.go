package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Room struct {
	ID        string    `json:"id"`
	HostID    int       `json:"host_id"`
	VideoID   *int      `json:"video_id"`
	Position  float64   `json:"position"`
	Paused    bool      `json:"paused"`
	UpdatedAt time.Time `json:"updated_at"`
}

func CreateRoom(ctx context.Context, pool *pgxpool.Pool, id string, hostID int, videoID *int) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO rooms (id, host_id, video_id) VALUES ($1, $2, $3)`,
		id, hostID, videoID)
	return err
}

func GetRoom(ctx context.Context, pool *pgxpool.Pool, id string) (*Room, error) {
	var r Room
	err := pool.QueryRow(ctx,
		`SELECT id, host_id, video_id, position, paused, updated_at
		 FROM rooms WHERE id = $1`, id).
		Scan(&r.ID, &r.HostID, &r.VideoID, &r.Position, &r.Paused, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// UpdateRoomState records a new playback anchor: position as of NOW(). Called on
// the host's play/pause/seek.
func UpdateRoomState(ctx context.Context, pool *pgxpool.Pool, id string, position float64, paused bool) error {
	_, err := pool.Exec(ctx,
		`UPDATE rooms SET position = $1, paused = $2, updated_at = NOW() WHERE id = $3`,
		position, paused, id)
	return err
}

// LivePosition extrapolates the current playback position from the stored
// anchor (position advances with wall-clock time while not paused).
func (r *Room) LivePosition() float64 {
	if r.Paused {
		return r.Position
	}
	return r.Position + time.Since(r.UpdatedAt).Seconds()
}

// DeleteStaleRooms removes rooms whose playback anchor hasn't moved for
// olderThan — a party is a one-evening thing, but rows (and shareable codes)
// would otherwise live forever.
func DeleteStaleRooms(ctx context.Context, pool *pgxpool.Pool, olderThan time.Duration) (int64, error) {
	tag, err := pool.Exec(ctx,
		`DELETE FROM rooms WHERE updated_at < NOW() - make_interval(secs => $1)`,
		olderThan.Seconds())
	return tag.RowsAffected(), err
}
