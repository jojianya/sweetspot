package pins

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type Category struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Pin struct {
	ID         pgtype.UUID `json:"id"`
	UserID     pgtype.UUID `json:"user_id"`
	Location   string      `json:"location"`
	Geohash    string      `json:"geohash"`
	Caption    *string     `json:"caption"`
	CategoryID int         `json:"category_id"`
	IsHidden   bool        `json:"is_hidden"`
	Views      int64       `json:"views"`
	CreatedAt  time.Time   `json:"created_at"`
}

type PinPhoto struct {
	ID           pgtype.UUID `json:"id"`
	PinID        pgtype.UUID `json:"pin_id"`
	PhotoURL     string      `json:"photo_url"`
	ThumbnailURL string      `json:"thumbnail_url"`
	Position     int16       `json:"position"`
	CreatedAt    time.Time   `json:"created_at"`
}

type PinDetail struct {
	Pin
	Category  *string    `json:"category"`
	Username  *string    `json:"username"`
	AvatarURL *string    `json:"avatar_url"`
	Photos    []PinPhoto `json:"photos"`
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
