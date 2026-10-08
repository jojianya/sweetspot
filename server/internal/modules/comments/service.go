package comments

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

// ErrForbidden reports a delete by someone who is neither the comment's
// author nor a moderator.
var ErrForbidden = errors.New("you can only delete your own comments")

// Service holds the comment business rules: handlers parse input and format
// responses, the service decides. It has no HTTP dependency.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListByPin(ctx context.Context, pinID string, limit, offset int) ([]Comment, int, error) {
	exists, err := s.repo.PinExistsVisible(ctx, pinID)
	if err != nil {
		return nil, 0, err
	}
	if !exists {
		return nil, 0, ErrPinNotFound
	}
	return s.repo.ListByPin(ctx, pinID, limit, offset)
}

// Create validates the body, checks the pin is visible, and inserts the
// comment. The repository translates a raced pin-hide into ErrPinNotFound.
func (s *Service) Create(ctx context.Context, pinID, userID, body string) (Comment, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return Comment{}, errEmptyBody
	}
	if utf8.RuneCountInString(body) > maxCommentLength {
		return Comment{}, errBodyTooLong
	}

	exists, err := s.repo.PinExistsVisible(ctx, pinID)
	if err != nil {
		return Comment{}, err
	}
	if !exists {
		return Comment{}, ErrPinNotFound
	}

	return s.repo.Create(ctx, pinID, userID, body)
}

var (
	errEmptyBody   = errors.New("body is required")
	errBodyTooLong = errors.New("body must be at most 500 characters")
)

// Delete enforces the delete policy: moderators soft-delete (keeps an audit
// trail); the author removes the comment outright; anyone else is forbidden.
func (s *Service) Delete(ctx context.Context, id, userID string, isModerator bool) error {
	comment, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}

	if isModerator {
		return s.repo.Hide(ctx, comment.ID)
	}
	if comment.UserID == userID {
		return s.repo.Delete(ctx, comment.ID)
	}
	return ErrForbidden
}