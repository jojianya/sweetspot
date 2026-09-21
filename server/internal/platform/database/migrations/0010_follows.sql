-- 0010_follows.sql

CREATE TABLE follows (
    follower_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    followee_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (follower_id, followee_id)
);

CREATE INDEX follows_follower_idx ON follows (follower_id, created_at DESC);
CREATE INDEX follows_followee_idx  ON follows (followee_id, created_at DESC);