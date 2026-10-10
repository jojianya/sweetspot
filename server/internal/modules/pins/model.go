package pins

import (
	"time"
)

type Category struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type Pin struct {
	ID string `json:"id"`
	// UserID is nil for orphan pins whose author was deleted: pins.user_id is
	// NULLable (ON DELETE SET NULL in 0003_pins.sql), so pgx cannot scan it
	// into a plain string.
	UserID     *string `json:"user_id"`
	Location   string  `json:"location"`
	Geohash    string  `json:"geohash"`
	Caption    *string `json:"caption"`
	CategoryID int     `json:"category_id"`
	IsHidden   bool    `json:"is_hidden"`
	Views      int64   `json:"views"`
	// GoodSpotCount is the pin's "Good spot" reaction total. It is a
	// denormalised column maintained by the 0023 trigger, so reading it costs
	// one column instead of a COUNT join — measured at roughly a fifth of the
	// time and buffers of the join on 536 pins and 102k reactions.
	//
	// It lives on Pin, not on the list-entry type, so every shape that embeds
	// it carries the count: list, trending, feed, favorites, collections, user
	// pins and the created-pin response. The trending *formula* is untouched.
	GoodSpotCount int       `json:"good_spot_count"`
	CreatedAt     time.Time `json:"created_at"`
}

type PinPhoto struct {
	ID           string    `json:"id"`
	PinID        string    `json:"pin_id"`
	PhotoURL     string    `json:"photo_url"`
	ThumbnailURL string    `json:"thumbnail_url"`
	Position     int16     `json:"position"`
	CreatedAt    time.Time `json:"created_at"`
}

type PinDetail struct {
	Pin
	Category  *string    `json:"category"`
	Username  *string    `json:"username"`
	AvatarURL *string    `json:"avatar_url"`
	Photos    []PinPhoto `json:"photos"`
	// ReactedByMe reports whether the requesting viewer already reacted. It is
	// only ever set on GET /pins/:id — the one pin endpoint that knows who is
	// asking — and is false for guests and for any wiring that never supplied a
	// reaction reader, so the field is always present rather than omitted.
	ReactedByMe bool `json:"reacted_by_me"`
}

type PinListEntry struct {
	Pin
	CoverURL string  `json:"cover_url"`
	Username *string `json:"username"`
}

// TrendingPin is a viewport list entry plus the engagement metrics that drive
// the trending ranking (surfaced to the client for display).
type TrendingPin struct {
	PinListEntry
	CommentCount int     `json:"comment_count"`
	Score        float64 `json:"score"`
}
