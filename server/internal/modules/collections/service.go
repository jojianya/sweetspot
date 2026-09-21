package collections

import (
	"context"
)

type Service interface {
	UserExists(ctx context.Context, id string) (bool, error)
	Create(ctx context.Context, userID, name string, description *string) (Collection, error)
	Get(ctx context.Context, id string) (Collection, error)
	ListByUser(ctx context.Context, userID string) ([]Collection, error)
	Update(ctx context.Context, id, name string, description *string) error
	Delete(ctx context.Context, id string) error
	CollectionDetail(ctx context.Context, id string) (CollectionDetail, error)
	PinExists(ctx context.Context, pinID string) (bool, error)
	AddPin(ctx context.Context, collectionID, pinID string) error
	RemovePin(ctx context.Context, collectionID, pinID string) error
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

func (s *service) Create(ctx context.Context, userID, name string, description *string) (Collection, error) {
	return s.repo.Create(ctx, userID, name, description)
}

func (s *service) Get(ctx context.Context, id string) (Collection, error) {
	return s.repo.Get(ctx, id)
}

func (s *service) ListByUser(ctx context.Context, userID string) ([]Collection, error) {
	return s.repo.ListByUser(ctx, userID)
}

func (s *service) Update(ctx context.Context, id, name string, description *string) error {
	return s.repo.Update(ctx, id, name, description)
}

func (s *service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// CollectionDetail returns a collection with its pins in one call.
func (s *service) CollectionDetail(ctx context.Context, id string) (CollectionDetail, error) {
	c, err := s.repo.Get(ctx, id)
	if err != nil {
		return CollectionDetail{}, err
	}
	pins, err := s.repo.ListPins(ctx, id)
	if err != nil {
		return CollectionDetail{}, err
	}
	return CollectionDetail{Collection: c, Pins: pins}, nil
}

func (s *service) PinExists(ctx context.Context, pinID string) (bool, error) {
	return s.repo.PinExists(ctx, pinID)
}

func (s *service) AddPin(ctx context.Context, collectionID, pinID string) error {
	return s.repo.AddPin(ctx, collectionID, pinID)
}

func (s *service) RemovePin(ctx context.Context, collectionID, pinID string) error {
	return s.repo.RemovePin(ctx, collectionID, pinID)
}
