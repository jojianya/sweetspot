-- 0011_collections.sql

CREATE TABLE collections (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX collections_user_idx ON collections (user_id, created_at DESC);

CREATE TABLE collection_pins (
    collection_id UUID NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    pin_id        UUID NOT NULL REFERENCES pins(id) ON DELETE CASCADE,
    position      INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (collection_id, pin_id)
);

CREATE INDEX collection_pins_collection_idx ON collection_pins (collection_id, position);