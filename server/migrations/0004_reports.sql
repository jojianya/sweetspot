CREATE TABLE reports (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pin_id      UUID REFERENCES pins(id) ON DELETE CASCADE,
    reporter_id UUID REFERENCES users(id) ON DELETE SET NULL,
    reason      TEXT NOT NULL,
    status      TEXT NOT NULL CHECK (status IN ('pending', 'reviewed', 'actioned')) DEFAULT 'pending',
    resolved_by UUID REFERENCES users(id) ON DELETE SET NULL,
    resolved_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ DEFAULT now(),
    UNIQUE (pin_id, reporter_id)
);