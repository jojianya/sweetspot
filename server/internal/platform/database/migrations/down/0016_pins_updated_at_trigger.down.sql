-- Rollback for 0016_pins_updated_at_trigger.sql
--
-- The trigger is removed entirely. Column values are left as they are: the
-- trigger overwrote updated_at on real edits, and there is no way to tell
-- those post-migration edits from the pre-migration values, so reverting them
-- is not possible. Restoring pins.updated_at to a pre-migration state would
-- require discarding genuine edit history.
DROP TRIGGER IF EXISTS pins_updated_at ON pins;

DROP FUNCTION IF EXISTS set_pins_updated_at();