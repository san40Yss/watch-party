-- Admin flag. The library owner (admin) manages processing/deletion/uploads;
-- everyone else just watches. The earliest user (the seed "host") is the admin;
-- newly registered friends default to non-admin.

ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT false;

UPDATE users SET is_admin = true WHERE id = (SELECT MIN(id) FROM users);
