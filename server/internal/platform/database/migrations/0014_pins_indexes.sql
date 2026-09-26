-- P1.2: Sort indexes for pins table
-- These support ORDER BY created_at DESC on both the full table and the
-- visible subset (is_hidden = false). CONCURRENTLY avoids long table locks
-- on deploy, at the cost of requiring two table scans. If CONCURRENTLY is
-- not supported (e.g. inside a transaction), the index will still be created
-- but will take an exclusive lock briefly.

CREATE INDEX CONCURRENTLY pins_created_at_idx ON pins (created_at DESC);

-- Partial index covering only visible pins, used by ListPins and ListTrending
-- which both filter is_hidden = false.
CREATE INDEX CONCURRENTLY pins_visible_created_idx ON pins (created_at DESC) WHERE is_hidden = false;