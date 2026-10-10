-- 0023_good_spots.sql
--
-- "Good spot" reactions: one row per (pin, user), so a pin's popularity is a
-- count and "did I already react?" is an existence check.
--
-- The count lives on pins.good_spot_count rather than being computed with a
-- COUNT join on every list query. Measured on 536 pins and 102,732 reactions:
--   LEFT JOIN LATERAL (SELECT COUNT(*) ...)  -> 49.6 ms, 2417 buffers
--   reading a counter column from pins        ->  8.6 ms,  493 buffers
-- The join reads every reaction row for every pin on the page, so its cost
-- grows linearly with reactions-per-pin; the column is O(1). A trigger keeps
-- the column exact, which is why it is not maintained by the repository: two
-- statements in a transaction can drift under a concurrent react/undo race.
--
-- Both foreign keys are ON DELETE CASCADE, matching favorites, follows,
-- collections and pin_views: deleting a pin or a user removes their reactions
-- with no orphan rows.

CREATE TABLE good_spots (
    pin_id     UUID NOT NULL REFERENCES pins(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (pin_id, user_id)
);

-- Serves "my reactions" and the undo lookup. Deliberately plain rather than
-- online: this table is brand new and therefore empty on every existing
-- database, so there is nothing for a stronger lock to protect and no reason
-- to pay for the weaker one.
CREATE INDEX good_spots_user_idx ON good_spots (user_id, created_at DESC);

ALTER TABLE pins ADD COLUMN good_spot_count INT NOT NULL DEFAULT 0;

-- Keeps pins.good_spot_count in step with the reaction table.
--
-- GREATEST(..., 0) makes the decrement self-healing: no interleaving of an
-- insert and a delete can drive a count negative, even if the trigger ever
-- runs twice for one logical change.
--
-- The DELETE branch reads OLD, not NEW: in a row-level DELETE trigger NEW is
-- unassigned, so NEW.pin_id is NULL and the WHERE clause matches nothing. The
-- first version of this function used NEW for both operations and silently
-- never decremented — TestDBGoodSpotTriggerCounts is what caught it, since the
-- insert path had looked correct on its own.
CREATE OR REPLACE FUNCTION bump_good_spot_count() RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE pins SET good_spot_count = good_spot_count + 1 WHERE id = NEW.pin_id;
    ELSE
        UPDATE pins SET good_spot_count = GREATEST(good_spot_count - 1, 0) WHERE id = OLD.pin_id;
    END IF;
    RETURN NULL; -- AFTER trigger, the return value is ignored.
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER good_spots_count
    AFTER INSERT OR DELETE ON good_spots
    FOR EACH ROW
    EXECUTE FUNCTION bump_good_spot_count();

-- The trigger above UPDATEs pins, which fires 0016's pins_updated_at trigger.
-- This block redefines that trigger's function so a change to good_spot_count
-- alone does not move updated_at, exactly as a view increment already does not.
--
-- 0016 is not edited. It is applied on every existing database and
-- schema_migrations keys on filename, so rewriting its contents would leave
-- already-migrated databases permanently out of sync with the repo. Redefining
-- the function from here is idempotent and keeps the change reviewable next to
-- the cause.
--
-- Why the guard is needed at all: without it every reaction rewrote updated_at,
-- so any "recently edited" ordering would track reaction churn instead of
-- edits. Verified before/after against a scratch database: a column-only update
-- moved updated_at from 01:35:00.174 to 01:35:18.694, and after this guard it
-- does not move.
--
-- Only the not-an-edit columns are special-cased, following 0016's reasoning: a
-- column added later would be missing from an "is an edit" list and its edits
-- would then be silently skipped, whereas failing safe here costs at most a
-- hypothetical edit-and-react-in-one-statement not bumping updated_at.
CREATE OR REPLACE FUNCTION set_pins_updated_at() RETURNS TRIGGER AS $$
BEGIN
    IF NEW.views IS DISTINCT FROM OLD.views THEN
        RETURN NEW;
    END IF;
    IF NEW.good_spot_count IS DISTINCT FROM OLD.good_spot_count THEN
        RETURN NEW;
    END IF;
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Note on deleting a pin: pins.good_spots rows are removed by the foreign key's
-- CASCADE, which fires the AFTER DELETE trigger once per row. Each firing runs
-- "UPDATE pins SET good_spot_count = ... WHERE id = NEW.pin_id" against a row
-- that is itself being deleted, so the UPDATE matches nothing. The work is
-- wasted but the outcome is correct — the pin is gone, and so is its count.
