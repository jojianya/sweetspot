CREATE TABLE streams (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pin_id             UUID REFERENCES pins(id) ON DELETE SET NULL,
    broadcaster_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    livekit_room_name  TEXT NOT NULL UNIQUE,
    status             TEXT NOT NULL CHECK (status IN ('live', 'ended')) DEFAULT 'live',
    peak_viewer_count  INT NOT NULL DEFAULT 0,
    started_at         TIMESTAMPTZ DEFAULT now(),
    ended_at           TIMESTAMPTZ
);

CREATE INDEX streams_status_idx ON streams (status) WHERE status = 'live';