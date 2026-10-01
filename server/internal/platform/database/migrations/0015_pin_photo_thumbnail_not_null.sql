-- 0015_pin_photo_thumbnail_not_null.sql
-- Make pin_photos.thumbnail_url mandatory.
--
-- 0006 added the column as nullable and backfilled it, so on a database that
-- has been running the app this backfill should match zero rows. It is kept
-- here anyway because the constraint and the backfill have to ship together:
-- the NOT NULL below is exactly what would reject a NULL, and dropping the
-- UPDATE to "save" a statement would turn a data problem into a failed deploy.

-- Default for a missing thumbnail is the row's own full-size photo_url.
-- Not '' and not a placeholder path: thumbnail and full-size share one storage
-- path scheme, so pointing the thumbnail at the original degrades to a larger
-- payload, whereas an empty or invented URL renders as a broken <img src="">.
-- This is also what 0006 backfilled with, so both migrations agree on what a
-- missing thumbnail means.
UPDATE pin_photos SET thumbnail_url = photo_url WHERE thumbnail_url IS NULL;

-- SAFETY: if any row still has a NULL thumbnail after the backfill, its
-- photo_url must be NULL too. photo_url is NOT NULL, so that is impossible
-- today, and the constraint would fail loudly rather than silently if it ever
-- became possible.
--
-- LOCKING: SET NOT NULL takes ACCESS EXCLUSIVE. On PostgreSQL 12+ it is
-- implemented as add-NOT-VALID-check, validate, then a brief SET NOT NULL, so
-- the table is not fully rescanned while holding the strongest lock.
ALTER TABLE pin_photos ALTER COLUMN thumbnail_url SET NOT NULL;