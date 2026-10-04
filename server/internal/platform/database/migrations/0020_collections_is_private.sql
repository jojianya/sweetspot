-- 0020_collections_is_private.sql
--
-- Per-collection visibility. Private collections are readable only by their
-- owner (GET /collections/:id answers 404 to anyone else, and
-- GET /users/:id/collections omits them for non-owners). Existing collections
-- stay public: the default only affects rows, never anyone's expectations.

ALTER TABLE collections ADD COLUMN is_private BOOLEAN NOT NULL DEFAULT false;
