-- 0003_pins.sql

CREATE TABLE categories (
    id   SERIAL PRIMARY KEY,
    name TEXT UNIQUE NOT NULL
);

INSERT INTO categories (name) VALUES
    ('Food'),
    ('Nature'),
    ('Event'),
    ('Nightlife'),
    ('Art'),
    ('Sports'),
    ('Travel'),
    ('Other');

CREATE TABLE pins (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    location    GEOGRAPHY(POINT, 4326) NOT NULL,
    geohash     TEXT NOT NULL,
    caption     TEXT,
    category_id INT NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    is_hidden   BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX pins_location_idx  ON pins USING GIST (location);
CREATE INDEX pins_geohash_idx   ON pins (geohash);
CREATE INDEX pins_category_idx  ON pins (category_id);

CREATE TABLE pin_photos (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pin_id     UUID NOT NULL REFERENCES pins(id) ON DELETE CASCADE,
    photo_url  TEXT NOT NULL,
    position   SMALLINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX pin_photos_pin_id_idx ON pin_photos (pin_id);
CREATE UNIQUE INDEX pin_photos_pin_id_position_idx ON pin_photos (pin_id, position);