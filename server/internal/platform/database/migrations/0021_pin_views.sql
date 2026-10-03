-- 0021_pin_views.sql
-- Unique per-account view tracking: one row per (pin, viewer). The primary
-- key is the uniqueness guarantee, so concurrent first opens serialize on
-- it and a pin's view count moves exactly once per account. No backfill:
-- existing view counts stand as-is and only grow from new unique opens.
-- View increments touch only `views`, so the pins_updated_at trigger (0016)
-- keeps ignoring them and updated_at stays honest.

CREATE TABLE pin_views (
    pin_id     UUID NOT NULL REFERENCES pins(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (pin_id, user_id)
);
