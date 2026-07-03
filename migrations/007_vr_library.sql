-- Personal VR library: VR videos are uploaded and watched through a separate
-- (hidden, admin-only) page and never appear in the shared film library.
ALTER TABLE videos ADD COLUMN IF NOT EXISTS is_vr BOOLEAN NOT NULL DEFAULT false;
