-- App-managed key-value settings (e.g. the HMAC secret that signs direct
-- stream links for external/VR players). Values are written by the app on
-- startup; this file only defines structure.

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
