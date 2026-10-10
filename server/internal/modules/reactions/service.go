package reactions

import (
	"context"
)

// Service holds the reaction business rules. It has no HTTP dependency:
// handlers parse input and format responses, the service decides.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// React records the caller's reaction to a pin and reports the resulting state.
//
// The order of the checks is the contract:
//  1. the pin must exist and be visible, so a hidden pin answers 404;
//  2. it must not be the caller's own pin;
//  3. only then does the row get written.
//
// All three run before the insert, so a rejected reaction never reaches the
// database and never moves the count. A duplicate reaction is not an error —
// the primary key absorbs it — so a double tap returns the same state with the
// same count instead of counting twice.
func (s *Service) React(ctx context.Context, userID, pinID string) (Result, error) {
	if err := s.checkPin(ctx, userID, pinID); err != nil {
		return Result{}, err
	}
	if err := s.repo.React(ctx, userID, pinID); err != nil {
		return Result{}, err
	}
	return s.state(ctx, userID, pinID)
}

// Unreact removes the caller's reaction and reports the resulting state.
//
// Removing a reaction that was never there is not an error: the caller is then
// unreacted, which is exactly the state the request asked for. Only a hidden or
// missing pin is refused, because the toggle has to agree with React about what
// exists.
func (s *Service) Unreact(ctx context.Context, userID, pinID string) (Result, error) {
	if err := s.checkPin(ctx, userID, pinID); err != nil {
		return Result{}, err
	}
	if err := s.repo.Unreact(ctx, userID, pinID); err != nil {
		return Result{}, err
	}
	// Read the stored state rather than assuming: the delete is idempotent, so
	// "was it there?" is a question for the database, and reacting is only
	// reported when a row actually existed afterwards.
	reacted, err := s.repo.ReactedByMe(ctx, userID, pinID)
	if err != nil {
		return Result{}, err
	}
	count, err := s.repo.Count(ctx, pinID)
	if err != nil {
		return Result{}, err
	}
	return Result{Reacted: reacted, GoodSpotCount: count}, nil
}

// checkPin enforces the existence and ownership rules shared by both
// operations.
func (s *Service) checkPin(ctx context.Context, userID, pinID string) error {
	exists, err := s.repo.PinExistsVisible(ctx, pinID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	owner, err := s.repo.PinOwner(ctx, pinID)
	if err != nil {
		return err
	}
	if owner != nil && *owner == userID {
		return ErrOwnPin
	}
	return nil
}

// state reads back both halves of the answer after a write: the caller's
// membership and the pin's authoritative count.
func (s *Service) state(ctx context.Context, userID, pinID string) (Result, error) {
	reacted, err := s.repo.ReactedByMe(ctx, userID, pinID)
	if err != nil {
		return Result{}, err
	}
	count, err := s.repo.Count(ctx, pinID)
	if err != nil {
		return Result{}, err
	}
	return Result{Reacted: reacted, GoodSpotCount: count}, nil
}
