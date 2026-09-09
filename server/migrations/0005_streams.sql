CREATE TABLE streams (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pin_id            UUID REFERENCES pins(id) ON DELETE SET NULL,
    broadcaster_id    UUID REFERENCES users(id) ON DELETE SET NULL,
    livekit_room_name TEXT NOT NULL,
    status            TEXT CHECK (status IN ('live', 'ended')) DEFAULT 'live',
    started_at        TIMESTAMPTZ DEFAULT now(),
    ended_at          TIMESTAMPTZ
);
