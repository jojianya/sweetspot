package collections

import (
	"context"
)

// Service holds the collection business rules: handlers parse input and
// format responses, the service decides. It has no HTTP dependency; the
// caller resolves moderation to a bool up front.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListMine(ctx context.Context, userID string) ([]Collection, error) {
	return s.repo.ListByUser(ctx, userID)
}

func (s *Service) ListByUser(ctx context.Context, userID, callerID string) ([]Collection, error) {
	exists, err := s.repo.UserExists(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrUserNotFound
	}

	// Private collections are visible only to their owner; everyone else,
	// logged in or not, sees the public subset.
	if callerID != "" && callerID == userID {
		return s.repo.ListByUser(ctx, userID)
	}
	return s.repo.ListPublicByUser(ctx, userID)
}

func (s *Service) Create(ctx context.Context, userID, name string, description *string, isPrivate bool) (Collection, error) {
	return s.repo.Create(ctx, userID, name, description, isPrivate)
}

// GetDetail loads a collection with a page of its pins. Private collections
// exist only for their owner: anyone else gets ErrNotFound, the same
// convention as hidden pins, so privacy is not enumerable.
func (s *Service) GetDetail(ctx context.Context, id, callerID string, limit, offset int) (CollectionDetail, error) {
	collection, err := s.repo.Get(ctx, id)
	if err != nil {
		return CollectionDetail{}, err
	}

	if collection.IsPrivate && collection.UserID.String() != callerID {
		return CollectionDetail{}, ErrNotFound
	}

	pins, total, err := s.repo.ListPins(ctx, id, limit, offset)
	if err != nil {
		return CollectionDetail{}, err
	}

	return CollectionDetail{Collection: collection, Pins: pins, PinTotal: total}, nil
}

// RequireOwner loads the collection and enforces owner-or-moderator access,
// returning the collection so callers can reuse the read.
func (s *Service) RequireOwner(ctx context.Context, id, userID string, isModerator bool) (Collection, error) {
	collection, err := s.repo.Get(ctx, id)
	if err != nil {
		return Collection{}, err
	}

	// Only non-owners need the moderator decision, keeping it off the common
	// path. The caller resolves moderation up front.
	if collection.UserID.String() != userID && !isModerator {
		return Collection{}, ErrForbidden
	}
	return collection, nil
}

// Update applies a rename/description/visibility change. Absent isPrivate
// means unchanged, so partial updates cannot flip visibility by accident.
func (s *Service) Update(ctx context.Context, id, userID string, isModerator bool, name string, description *string, isPrivate *bool) error {
	existing, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}

	// Only the owner can change visibility (RequireOwner already enforced
	// ownership or moderation, and moderators editing a private collection
	// they can already see keep its flag unless they set it).
	visible := existing.IsPrivate
	if isPrivate != nil {
		visible = *isPrivate
	}

	return s.repo.Update(ctx, id, name, description, visible, userID, isModerator)
}

func (s *Service) Delete(ctx context.Context, id, userID string, isModerator bool) error {
	return s.repo.Delete(ctx, id, userID, isModerator)
}

func (s *Service) AddPin(ctx context.Context, id, pinID string) error {
	exists, err := s.repo.PinExists(ctx, pinID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrPinNotFound
	}
	return s.repo.AddPin(ctx, id, pinID)
}

func (s *Service) RemovePin(ctx context.Context, id, pinID string) error {
	return s.repo.RemovePin(ctx, id, pinID)
}
