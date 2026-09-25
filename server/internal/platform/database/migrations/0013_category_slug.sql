-- 0013_category_slug.sql
-- Give categories a stable public URL identifier while pins keep using numeric IDs.

ALTER TABLE categories ADD COLUMN slug TEXT;

UPDATE categories
SET slug = trim(BOTH '-' FROM regexp_replace(lower(name), '[^a-z0-9]+', '-', 'g'));

ALTER TABLE categories
    ALTER COLUMN slug SET NOT NULL,
    ADD CONSTRAINT categories_slug_key UNIQUE (slug);
