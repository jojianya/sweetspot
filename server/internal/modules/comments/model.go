package comments

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Comment is a comment on a pin, joined with its author's public identity.
type Comment struct {
	ID        pgtype.UUID `json:"id"`
	PinID     pgtype.UUID `json:"pin_id"`
	UserID    pgtype.UUID `json:"user_id"`
	Body      string      `json:"body"`
	IsHidden  bool        `json:"is_hidden"`
	CreatedAt time.Time   `json:"created_at"`
	Username  *string     `json:"username"`
	AvatarURL *string     `json:"avatar_url"`
}
