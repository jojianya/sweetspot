// Package session holds the per-user session posture shared between the
// user module, which loads it from the database, and the auth middleware,
// which enforces it. Keeping the type here avoids an import cycle: the
// middleware cannot import the user module (user routes import the
// middleware), and the user module must not import the HTTP layer.
package session

import "time"

// State is the per-user session posture: the instant before which tokens
// are dead, plus the live role for context caching.
type State struct {
	ValidAfter time.Time
	Role       string
}
