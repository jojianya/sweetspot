package favorites

import "context"

type Service interface {
	SavePin(ctx context.Context, userID, pinID string) error
	UnsavePin(ctx context.Context, userID, pinID string) error
	IsSaved(ctx context.Context, userID, pinID string) (bool, error)
	PinExists(ctx context.Context, pinID string) (bool, error)
	List(ctx context.Context, userID string) ([]Entry, error)
	ListIDs(ctx context.Context, userID string) ([]string, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) SavePin(ctx context.Context, userID, pinID string) error {
	return s.repo.Save(ctx, userID, pinID)
}

func (s *service) UnsavePin(ctx context.Context, userID, pinID string) error {
	return s.repo.Unsave(ctx, userID, pinID)
}

func (s *service) IsSaved(ctx context.Context, userID, pinID string) (bool, error) {
	return s.repo.IsSaved(ctx, userID, pinID)
}

func (s *service) PinExists(ctx context.Context, pinID string) (bool, error) {
	return s.repo.PinExists(ctx, pinID)
}

func (s *service) List(ctx context.Context, userID string) ([]Entry, error) {
	return s.repo.List(ctx, userID)
}

func (s *service) ListIDs(ctx context.Context, userID string) ([]string, error) {
	return s.repo.ListIDs(ctx, userID)
}