-- Rollback for 0015_pin_photo_thumbnail_not_null.sql
--
-- LOSSY: this restores nullability but cannot undo the backfill. Rows that
-- received photo_url as their thumbnail during the up migration are
-- indistinguishable from rows that always had one, so they keep the substituted
-- value. That is harmless — the value is the same image — but it means down
-- then up is not a true round trip for data, only for schema.
ALTER TABLE pin_photos ALTER COLUMN thumbnail_url DROP NOT NULL;