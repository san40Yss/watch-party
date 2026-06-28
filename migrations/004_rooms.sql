-- Watch-party rooms. A room is hosted by one user and bound to a video; the
-- host's playback (position + paused) is the shared state. Connected members
-- (presence) live in-memory in the app's WS hub, not here.
--
-- position/updated_at form a playback "anchor": the current position while
-- playing is position + (now - updated_at). Written only on play/pause/seek,
-- never on every tick.

CREATE TABLE IF NOT EXISTS rooms (
    id         TEXT PRIMARY KEY,                                   -- short shareable code
    host_id    INTEGER NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    video_id   INTEGER          REFERENCES videos(id) ON DELETE SET NULL,
    position   DOUBLE PRECISION NOT NULL DEFAULT 0,                -- anchor position (seconds)
    paused     BOOLEAN          NOT NULL DEFAULT true,
    updated_at TIMESTAMPTZ      NOT NULL DEFAULT NOW(),            -- when the anchor was set
    created_at TIMESTAMPTZ      NOT NULL DEFAULT NOW()
);
