-- 0012_pin_views.sql
-- Track how many times a pin has been viewed.

ALTER TABLE pins ADD COLUMN views BIGINT NOT NULL DEFAULT 0;