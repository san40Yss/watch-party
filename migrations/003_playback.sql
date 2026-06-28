-- Playback model: a video is delivered either as a single MP4 (direct play /
-- skip-if-ready) or as an HLS package (per-viewer audio + subtitle renditions).
ALTER TABLE videos ADD COLUMN IF NOT EXISTS playback_type TEXT NOT NULL DEFAULT 'mp4';

-- Subtitle renditions extracted during processing (populated in the subtitle step).
CREATE TABLE IF NOT EXISTS subtitle_tracks (
    id          SERIAL PRIMARY KEY,
    video_id    INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    track_index INTEGER NOT NULL,
    src_index   INTEGER NOT NULL,
    language    TEXT,
    title       TEXT,
    forced      BOOLEAN NOT NULL DEFAULT FALSE,
    path        TEXT  -- relative path within the video's processed dir
);

CREATE INDEX IF NOT EXISTS idx_subtitle_tracks_video ON subtitle_tracks(video_id);
