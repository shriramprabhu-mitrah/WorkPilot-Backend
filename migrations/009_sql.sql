-- Expand logo_url column to accommodate S3 public URLs (previously VARCHAR(150))

ALTER TABLE organizations
    ALTER COLUMN logo_url TYPE VARCHAR(500);

-- Allow logo_url to be NULL (logo is now optional; existing rows keep their value)

ALTER TABLE organizations
    ALTER COLUMN logo_url DROP NOT NULL;

-- Expand avatar_url column in users table to accommodate S3 public URLs (previously VARCHAR(255))

ALTER TABLE users
    ALTER COLUMN avatar_url TYPE VARCHAR(500);