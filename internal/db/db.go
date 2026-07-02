package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IsUniqueViolation reports whether err is a Postgres unique-constraint error.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// DeleteExpiredSessions purges sessions past their expiry; without this the
// table grows forever (one row per login, 30-day TTL).
func DeleteExpiredSessions(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	tag, err := pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at < NOW()`)
	return tag.RowsAffected(), err
}

type Video struct {
	ID            int       `json:"id"`
	Title         string    `json:"title"`
	FilePath      string    `json:"file_path"`
	ProcessedPath *string   `json:"processed_path"`
	PlaybackType  string    `json:"playback_type"` // "mp4" | "hls"
	Status        string    `json:"status"`
	Progress      float64   `json:"progress"`
	Error         *string   `json:"error,omitempty"`
	VideoCodec    *string   `json:"video_codec"`
	Width         *int      `json:"width"`
	Height        *int      `json:"height"`
	HDR           bool      `json:"hdr"`
	Duration      *float64  `json:"duration"`
	CreatedAt     time.Time `json:"created_at"`
}

type AudioTrack struct {
	ID         int     `json:"id"`
	VideoID    int     `json:"video_id"`
	TrackIndex int     `json:"track_index"`
	SrcIndex   int     `json:"src_index"`
	Codec      *string `json:"codec"`
	Language   *string `json:"language"`
	Title      *string `json:"title"`
	Channels   *int    `json:"channels"`
}

type SubtitleTrack struct {
	ID         int     `json:"id"`
	VideoID    int     `json:"video_id"`
	TrackIndex int     `json:"track_index"`
	SrcIndex   int     `json:"src_index"`
	Language   *string `json:"language"`
	Title      *string `json:"title"`
	Forced     bool    `json:"forced"`
	Path       *string `json:"path"`
}

const videoCols = `id, title, file_path, processed_path, playback_type, status,
	progress, error, video_codec, width, height, hdr, duration, created_at`

func scanVideo(row pgx.Row) (*Video, error) {
	var v Video
	err := row.Scan(&v.ID, &v.Title, &v.FilePath, &v.ProcessedPath, &v.PlaybackType,
		&v.Status, &v.Progress, &v.Error, &v.VideoCodec, &v.Width, &v.Height, &v.HDR,
		&v.Duration, &v.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func ListVideos(ctx context.Context, pool *pgxpool.Pool) ([]Video, error) {
	rows, err := pool.Query(ctx,
		`SELECT `+videoCols+` FROM videos ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []Video
	for rows.Next() {
		v, err := scanVideo(rows)
		if err != nil {
			return nil, err
		}
		videos = append(videos, *v)
	}
	return videos, rows.Err()
}

func GetVideo(ctx context.Context, pool *pgxpool.Pool, id int) (*Video, error) {
	return scanVideo(pool.QueryRow(ctx,
		`SELECT `+videoCols+` FROM videos WHERE id = $1`, id))
}

// ResetStuckProcessing marks videos left mid-encode (e.g. the app was
// restarted while ffmpeg was running) as errored so they can be retried,
// instead of being stuck in 'processing' forever. Returns rows affected.
func ResetStuckProcessing(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	tag, err := pool.Exec(ctx,
		`UPDATE videos SET status = 'error', error = 'interrupted by restart'
		 WHERE status = 'processing'`)
	return tag.RowsAffected(), err
}

// DeleteVideo removes the video row (audio/subtitle tracks cascade). The caller
// is responsible for removing any processed files on disk.
func DeleteVideo(ctx context.Context, pool *pgxpool.Pool, id int) error {
	_, err := pool.Exec(ctx, `DELETE FROM videos WHERE id = $1`, id)
	return err
}

func SetStatus(ctx context.Context, pool *pgxpool.Pool, id int, status string, errMsg *string) error {
	_, err := pool.Exec(ctx,
		`UPDATE videos SET status = $1, error = $2 WHERE id = $3`,
		status, errMsg, id)
	return err
}

// SetProgress records processing completion percentage (0..100).
func SetProgress(ctx context.Context, pool *pgxpool.Pool, id int, percent float64) error {
	_, err := pool.Exec(ctx,
		`UPDATE videos SET progress = $1 WHERE id = $2`, percent, id)
	return err
}

// SetProcessed records the result of a successful processing run and marks
// the video ready in a single update.
func SetProcessed(ctx context.Context, pool *pgxpool.Pool, id int,
	processedPath, playbackType, videoCodec string, width, height int, hdr bool, duration float64) error {
	_, err := pool.Exec(ctx,
		`UPDATE videos
		 SET status = 'ready', error = NULL, progress = 100, processed_path = $1,
		     playback_type = $2, video_codec = $3, width = $4, height = $5,
		     hdr = $6, duration = $7
		 WHERE id = $8`,
		processedPath, playbackType, videoCodec, width, height, hdr, duration, id)
	return err
}

func ReplaceAudioTracks(ctx context.Context, pool *pgxpool.Pool, videoID int, tracks []AudioTrack) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM audio_tracks WHERE video_id = $1`, videoID); err != nil {
		return err
	}
	for _, t := range tracks {
		if _, err := tx.Exec(ctx,
			`INSERT INTO audio_tracks (video_id, track_index, src_index, codec, language, title, channels)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			videoID, t.TrackIndex, t.SrcIndex, t.Codec, t.Language, t.Title, t.Channels); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func ListAudioTracks(ctx context.Context, pool *pgxpool.Pool, videoID int) ([]AudioTrack, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, video_id, track_index, src_index, codec, language, title, channels
		 FROM audio_tracks WHERE video_id = $1 ORDER BY track_index`, videoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []AudioTrack
	for rows.Next() {
		var t AudioTrack
		if err := rows.Scan(&t.ID, &t.VideoID, &t.TrackIndex, &t.SrcIndex,
			&t.Codec, &t.Language, &t.Title, &t.Channels); err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

func ReplaceSubtitleTracks(ctx context.Context, pool *pgxpool.Pool, videoID int, tracks []SubtitleTrack) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM subtitle_tracks WHERE video_id = $1`, videoID); err != nil {
		return err
	}
	for _, t := range tracks {
		if _, err := tx.Exec(ctx,
			`INSERT INTO subtitle_tracks (video_id, track_index, src_index, language, title, forced, path)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			videoID, t.TrackIndex, t.SrcIndex, t.Language, t.Title, t.Forced, t.Path); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func ListSubtitleTracks(ctx context.Context, pool *pgxpool.Pool, videoID int) ([]SubtitleTrack, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, video_id, track_index, src_index, language, title, forced, path
		 FROM subtitle_tracks WHERE video_id = $1 ORDER BY track_index`, videoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []SubtitleTrack
	for rows.Next() {
		var t SubtitleTrack
		if err := rows.Scan(&t.ID, &t.VideoID, &t.TrackIndex, &t.SrcIndex,
			&t.Language, &t.Title, &t.Forced, &t.Path); err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}
