package social

import (
	"context"
	"errors"

	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
)

// ErrSelfFollow reports a follow of the caller's own account.
var ErrSelfFollow = errors.New("you cannot follow yourself")

// Service holds the social business rules: handlers parse input and format
// responses, the service decides. It has no HTTP dependency.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// FollowTarget validates a follow target: the caller cannot follow
// themselves, and the target must exist.
func (s *Service) FollowTarget(ctx context.Context, callerID, targetID string) (string, error) {
	if targetID == callerID {
		return "", ErrSelfFollow
	}

	exists, err := s.repo.UserExists(ctx, targetID)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", ErrNotFound
	}
	return targetID, nil
}

func (s *Service) Follow(ctx context.Context, followerID, followeeID string) error {
	return s.repo.Follow(ctx, followerID, followeeID)
}

func (s *Service) Unfollow(ctx context.Context, followerID, followeeID string) error {
	return s.repo.Unfollow(ctx, followerID, followeeID)
}

func (s *Service) UserStats(ctx context.Context, viewerID, userID string) (Stats, error) {
	exists, err := s.repo.UserExists(ctx, userID)
	if err != nil {
		return Stats{}, &statsFailure{log: "social: user exists", args: []any{"user_id", userID}, err: err}
	}
	if !exists {
		return Stats{}, ErrNotFound
	}
	stats, fail := collectStats(ctx, s.repo, viewerID, userID)
	if fail != nil {
		return Stats{}, fail
	}
	return stats, nil
}

func (s *Service) Feed(ctx context.Context, userID string, limit int) ([]pins.PinListEntry, error) {
	return s.repo.Feed(ctx, userID, limit)
}

// statsFailure carries which fan-out call failed so the handler keeps its
// exact per-call log message and args. It implements error so UserStats can
// return it alongside the ErrNotFound domain error.
type statsFailure struct {
	log  string
	args []any
	err  error
}

func (f *statsFailure) Error() string { return f.err.Error() }
func (f *statsFailure) Unwrap() error { return f.err }

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
