package reactions

import "time"

// Result is the answer to one toggle. Reacted is the caller's state after the
// operation, and GoodSpotCount is the pin's authoritative count — read back
// from pins rather than computed here, because the 0023 trigger is what
// maintains it and the client must never be told a number the database does
// not agree with.
type Result struct {
	Reacted       bool `json:"reacted"`
	GoodSpotCount int  `json:"good_spot_count"`
}

// Reaction is one stored reaction, used by the "my reactions" read.
type Reaction struct {
	PinID     string    `json:"pin_id"`
	CreatedAt time.Time `json:"created_at"`
}
