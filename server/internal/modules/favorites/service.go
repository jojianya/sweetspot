package favorites

import (
	"context"
)

// Service holds the favorites business rules: handlers parse input and format
// responses, the service decides. It has no HTTP dependency.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Save records a save after verifying the pin exists and is visible, so
// saving a hidden or missing pin answers 404 instead of silently succeeding.
func (s *Service) Save(ctx context.Context, userID, pinID string) error {
	exists, err := s.repo.PinExists(ctx, pinID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return s.repo.Save(ctx, userID, pinID)
}

// Unsave removes a save, answering 404 when the pin was never saved so an
// unsave of a missing favorite is distinguishable from a removed one.
func (s *Service) Unsave(ctx context.Context, userID, pinID string) error {
	saved, err := s.repo.IsSaved(ctx, userID, pinID)
	if err != nil {
		return err
	}
	if !saved {
		return ErrFavoriteNotFound
	}
	return s.repo.Unsave(ctx, userID, pinID)
}

func (s *Service) List(ctx context.Context, userID string, limit, offset int) ([]Entry, int, error) {
	return s.repo.List(ctx, userID, limit, offset)
}

func (s *Service) ListIDs(ctx context.Context, userID string) ([]string, error) {
	return s.repo.ListIDs(ctx, userID)
}
