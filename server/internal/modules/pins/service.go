package pins

import "context"

type Service interface {
	ListCategories(ctx context.Context) ([]Category, error)
	CategoryExists(ctx context.Context, id int) (bool, error)
	DeletePin(ctx context.Context, id, userID string) error
	GetPin(ctx context.Context, id string) (PinDetail, error)
	ListPins(ctx context.Context, bbox [4]float64, categoryID *int, limit int) ([]PinListEntry, error)
	ListByUser(ctx context.Context, userID string, limit int) ([]PinListEntry, error)
	SearchPins(ctx context.Context, query string, limit int) ([]PinListEntry, error)
	CreatePin(ctx context.Context, pin NewPin) (Pin, error)
	UpdatePin(ctx context.Context, id string, patch UpdatePinPatch) (Pin, error)
	UserExists(ctx context.Context, id string) (bool, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) ListCategories(ctx context.Context) ([]Category, error) {
	return s.repo.ListCategories(ctx)
}

func (s *service) CategoryExists(ctx context.Context, id int) (bool, error) {
	return s.repo.CategoryExists(ctx, id)
}

func (s *service) GetPin(ctx context.Context, id string) (PinDetail, error) {
	return s.repo.GetPin(ctx, id)
}

func (s *service) ListPins(ctx context.Context, bbox [4]float64, categoryID *int, limit int) ([]PinListEntry, error) {
	return s.repo.ListPins(ctx, bbox, categoryID, limit)
}

func (s *service) SearchPins(ctx context.Context, query string, limit int) ([]PinListEntry, error) {
	return s.repo.SearchPins(ctx, query, limit)
}

func (s *service) CreatePin(ctx context.Context, pin NewPin) (Pin, error) {
	return s.repo.CreatePin(ctx, pin)
}

func (s *service) UpdatePin(ctx context.Context, id string, patch UpdatePinPatch) (Pin, error) {
	return s.repo.UpdatePin(ctx, id, patch)
}

func (s *service) ListByUser(ctx context.Context, userID string, limit int) ([]PinListEntry, error) {
	return s.repo.ListByUser(ctx, userID, limit)
}

func (s *service) DeletePin(ctx context.Context, id, userID string) error {
	return s.repo.DeletePin(ctx, id, userID)
}

func (s *service) UserExists(ctx context.Context, id string) (bool, error) {
	return s.repo.UserExists(ctx, id)
}
