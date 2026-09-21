package pins

import (
	"context"
	"time"
)

// Event describes a newly created public pin, broadcast to connected maps
// through the realtime stream.
type Event struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Location   string    `json:"location"`
	Caption    *string   `json:"caption"`
	CategoryID int       `json:"category_id"`
	CoverURL   string    `json:"cover_url"`
	CreatedAt  time.Time `json:"created_at"`
}

// Events lets the create flow publish pin events without depending on a
// concrete pub/sub implementation. Nil implementations are allowed (the
// realtime feature is off); failures are logged, never fatal.
type Events interface {
	PinCreated(ctx context.Context, ev Event)
}
