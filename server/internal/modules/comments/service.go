package comments

import "context"

type Service interface {
	Create(ctx context.Context, pinID, userID, body string) (Comment, error)
	ListByPin(ctx context.Context, pinID string) ([]Comment, error)
	Get(ctx context.Context, id string) (Comment, error)
	// Hide soft-deletes a comment (moderation); Delete removes it entirely.
	Hide(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Create(ctx context.Context, pinID, userID, body string) (Comment, error) {
	return s.repo.Create(ctx, pinID, userID, body)
}

func (s *service) ListByPin(ctx context.Context, pinID string) ([]Comment, error) {
	return s.repo.ListByPin(ctx, pinID)
}

func (s *service) Get(ctx context.Context, id string) (Comment, error) {
	return s.repo.Get(ctx, id)
}

func (s *service) Hide(ctx context.Context, id string) error {
	return s.repo.Hide(ctx, id)
}

func (s *service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
