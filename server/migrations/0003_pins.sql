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
    photo_url   TEXT NOT NULL,
    location    GEOGRAPHY(POINT, 4326) NOT NULL,
    geohash     TEXT NOT NULL,
    caption     TEXT,
    category_id INT REFERENCES categories(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX pins_location_idx  ON pins USING GIST (location);
CREATE INDEX pins_geohash_idx   ON pins (geohash);
CREATE INDEX pins_category_idx  ON pins (category_id);
