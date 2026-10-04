-- 0016_pins_updated_at_trigger.sql
-- Keep pins.updated_at honest.
--
-- 0008 added the column with DEFAULT now(), which backfilled every existing
-- pin to the moment 0008 ran and nothing has maintained it since: only
-- UpdatePin (pins/repository.go) moved it, and only because that one statement
-- sets updated_at = now() by hand. Any other UPDATE left the column stale.

-- Deliberately its own function rather than a reuse of set_updated_at() from
-- 0002: pins need a view-count exemption that users do not.
CREATE OR REPLACE FUNCTION set_pins_updated_at()
RETURNS TRIGGER AS $$
BEGIN
	-- A view increment is not an edit. Without this guard every pin detail page
	-- view would rewrite updated_at, and any "recently edited" ordering would
	-- track traffic instead of edits. Every other UPDATE — caption, category,
	-- soft-delete by owner or by a moderator hiding a reported pin — is a real
	-- change and does move the column.
	--
	-- Only `views` is special-cased rather than listing the columns that count
	-- as edits. A column added later would be missing from such a list, and its
	-- edits would then be misread as view churn and silently skipped. Failing
	-- safe here costs at most a hypothetical edit-and-view-in-one-statement not
	-- bumping updated_at; no such statement exists, since UpdatePin and
	-- RegisterView are separate statements against separate code paths.
	IF NEW.views IS DISTINCT FROM OLD.views THEN
		RETURN NEW;
	END IF;

	NEW.updated_at = now();
	RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER pins_updated_at
    BEFORE UPDATE ON pins
    FOR EACH ROW
    EXECUTE FUNCTION set_pins_updated_at();

-- NO BACKFILL, DELIBERATELY. Existing rows all carry updated_at = the instant
-- 0008 ran, which is not a real edit time, and there is no way to recover the
-- true value: pins had no update history before this trigger existed. Writing
-- anything else in (created_at, NULL, now()) would invent data. The practical
-- consequence, which callers must know: ordering by updated_at is only
-- meaningful for pins edited after this migration deployed. Legacy pins all
-- share one timestamp and will sort as a tie.