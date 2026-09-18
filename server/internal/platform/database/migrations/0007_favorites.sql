-- 0007_favorites.sql

CREATE TABLE favorites (
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    pin_id     UUID NOT NULL REFERENCES pins(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (user_id, pin_id)
);

CREATE INDEX favorites_user_created_idx ON favorites (user_id, created_at DESC);