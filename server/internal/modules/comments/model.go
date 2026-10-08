package comments

import (
	"time"
)

// Comment is a comment on a pin, joined with its author's public identity.
type Comment struct {
	ID        string    `json:"id"`
	PinID     string    `json:"pin_id"`
	UserID    string    `json:"user_id"`
	Body      string    `json:"body"`
	IsHidden  bool      `json:"is_hidden"`
	CreatedAt time.Time `json:"created_at"`
	Username  *string   `json:"username"`
	AvatarURL *string   `json:"avatar_url"`
}
