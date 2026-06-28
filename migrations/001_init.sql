CREATE TABLE IF NOT EXISTS videos (
    id             SERIAL PRIMARY KEY,
    title          TEXT        NOT NULL,
    file_path      TEXT        NOT NULL,           -- original source (/media/...)
    processed_path TEXT,                           -- browser-ready MP4 (/media/processed/...)
    status         TEXT        NOT NULL DEFAULT 'pending', -- pending|processing|ready|error
    progress       REAL        NOT NULL DEFAULT 0,         -- 0..100, % of processing done
    error          TEXT,
    -- probe summary (filled in during processing)
    video_codec    TEXT,
    width          INTEGER,
    height         INTEGER,
    hdr            BOOLEAN     NOT NULL DEFAULT FALSE,
    duration       DOUBLE PRECISION,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audio_tracks (
    id          SERIAL PRIMARY KEY,
    video_id    INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    track_index INTEGER NOT NULL,   -- output track index inside the processed MP4
    src_index   INTEGER NOT NULL,   -- stream index in the source file
    codec       TEXT,
    language    TEXT,
    title       TEXT,
    channels    INTEGER
);

CREATE INDEX IF NOT EXISTS idx_audio_tracks_video ON audio_tracks(video_id);

-- Seed: points at the source file; processing fills in processed_path + status.
INSERT INTO videos (title, file_path)
VALUES ('Waterfall 4K', '/media/YTDown_YouTube_Free-Waterfall-4k-Videoclip-Nature-Video_Media_w6uX9jamcwQ_001_1080p.mp4')
ON CONFLICT DO NOTHING;
