package social

import (
	"context"
)

// statsFailure carries which fan-out call failed so the handler keeps its
// exact per-call log message and args.
type statsFailure struct {
	log  string
	args []any
	err  error
}

// collectStats fans out to the count queries behind the stats endpoint,
// plus the conditional is-following lookup for other viewers' profiles.
// Extracted unchanged from the Stats handler.
func collectStats(ctx context.Context, repo Repository, viewerID, userID string) (Stats, *statsFailure) {
	var stats Stats
	var err error
	if stats.Followers, err = repo.CountFollowers(ctx, userID); err != nil {
		return Stats{}, &statsFailure{log: "social: count followers", args: []any{"user_id", userID}, err: err}
	}
	if stats.Following, err = repo.CountFollowing(ctx, userID); err != nil {
		return Stats{}, &statsFailure{log: "social: count following", args: []any{"user_id", userID}, err: err}
	}
	if stats.PinsCount, err = repo.CountPins(ctx, userID); err != nil {
		return Stats{}, &statsFailure{log: "social: count pins", args: []any{"user_id", userID}, err: err}
	}
	if viewerID != "" && viewerID != userID {
		if stats.IsFollowing, err = repo.IsFollowing(ctx, viewerID, userID); err != nil {
			return Stats{}, &statsFailure{log: "social: is following", args: []any{"user_id", viewerID, "target", userID}, err: err}
		}
	}
	return stats, nil
}
