-- 0008_pin_edit.sql
-- Track when a pin was last edited (caption/category/photo changes).

ALTER TABLE pins ADD COLUMN updated_at TIMESTAMPTZ DEFAULT now();