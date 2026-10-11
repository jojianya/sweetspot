-- Rolls back 0023_good_spots.sql.
--
-- Order matters. The set_pins_updated_at redefinition is restored to its 0016
-- body BEFORE the column is dropped: leaving the good_spot_count guard in place
-- would reference a column that no longer exists, and the pins_updated_at
-- trigger would then error on every genuine pin edit. That is why this file
-- carries a copy of the 0016 function body rather than just dropping things.

CREATE OR REPLACE FUNCTION set_pins_updated_at() RETURNS TRIGGER AS $$
BEGIN
	IF NEW.views IS DISTINCT FROM OLD.views THEN
		RETURN NEW;
	END IF;
	NEW.updated_at = now();
	RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS good_spots_count ON good_spots;

DROP FUNCTION IF EXISTS bump_good_spot_count();

ALTER TABLE pins DROP COLUMN IF EXISTS good_spot_count;

DROP TABLE IF EXISTS good_spots;
