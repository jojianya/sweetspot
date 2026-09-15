-- 0006_pin_photo_thumbnails.sql
-- Add a thumbnail URL for list/map views alongside the full-size photo.

ALTER TABLE pin_photos ADD COLUMN thumbnail_url TEXT;

-- Backfill existing rows so older pins still render covers.
UPDATE pin_photos SET thumbnail_url = photo_url WHERE thumbnail_url IS NULL;