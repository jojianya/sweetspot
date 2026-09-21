package social

import (
	"context"

	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
)

type Service interface {
	UserExists(ctx context.Context, id string) (bool, error)
	Follow(ctx context.Context, followerID, followeeID string) error
	Unfollow(ctx context.Context, followerID, followeeID string) error
	IsFollowing(ctx context.Context, followerID, followeeID string) (bool, error)
	CountFollowers(ctx context.Context, userID string) (int, error)
	CountFollowing(ctx context.Context, userID string) (int, error)
	CountPins(ctx context.Context, userID string) (int, error)
	Feed(ctx context.Context, userID string, limit int) ([]pins.PinListEntry, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) UserExists(ctx context.Context, id string) (bool, error) {
	return s.repo.UserExists(ctx, id)
}

func (s *service) Follow(ctx context.Context, followerID, followeeID string) error {
	return s.repo.Follow(ctx, followerID, followeeID)
}

func (s *service) Unfollow(ctx context.Context, followerID, followeeID string) error {
	return s.repo.Unfollow(ctx, followerID, followeeID)
}

func (s *service) IsFollowing(ctx context.Context, followerID, followeeID string) (bool, error) {
	return s.repo.IsFollowing(ctx, followerID, followeeID)
}

func (s *service) CountFollowers(ctx context.Context, userID string) (int, error) {
	return s.repo.CountFollowers(ctx, userID)
}

func (s *service) CountFollowing(ctx context.Context, userID string) (int, error) {
	return s.repo.CountFollowing(ctx, userID)
}

func (s *service) CountPins(ctx context.Context, userID string) (int, error) {
	return s.repo.CountPins(ctx, userID)
}

func (s *service) Feed(ctx context.Context, userID string, limit int) ([]pins.PinListEntry, error) {
	return s.repo.Feed(ctx, userID, limit)
}
