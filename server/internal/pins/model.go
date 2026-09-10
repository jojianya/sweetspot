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
	CreatedAt  time.Time   `json:"created_at"`
}

type PinPhoto struct {
	ID        pgtype.UUID `json:"id"`
	PinID     pgtype.UUID `json:"pin_id"`
	PhotoURL  string      `json:"photo_url"`
	Position  int16       `json:"position"`
	CreatedAt time.Time   `json:"created_at"`
}
