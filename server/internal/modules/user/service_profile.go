package users

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"

	"github.com/jojianya/sweetspot247-backend/internal/platform/imaging"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
)

// Avatar upload errors. The messages are the exact client-facing strings the
// handler responds with; the handler maps each to its status code.
var (
	ErrAvatarTooLarge = errors.New("avatar exceeds 5MB")
	ErrAvatarInvalid  = errors.New("invalid avatar")
)

// profileUpdater applies profile updates including avatar replacement. It is
// built from the handler's service and store so the update orchestration
// (stage avatar, write profile, clean up on failure, drop the superseded
// file) lives outside the handler without changing the Service interface or
// its constructor.
type profileUpdater struct {
	svc   Service
	store *storage.Local
}

// UpdateProfileWithAvatar validates the avatar bytes, stages the new file,
// writes the profile patch, and cleans up: a staged file is removed when the
// profile write fails, and the superseded avatar is dropped on success. Both
// cleanups are best-effort and logged for the operator sweep, never fatal.
// A nil avatar means "leave unchanged".
func (u *profileUpdater) UpdateProfileWithAvatar(ctx context.Context, userID string, patch UpdateProfilePatch, avatar *multipart.FileHeader) (User, error) {
	current, err := u.svc.GetByID(ctx, userID)
	if err != nil {
		return User{}, err
	}

	var newAvatar *string
	if avatar != nil {
		url, err := stageAvatar(u.store, avatar)
		if err != nil {
			return User{}, err
		}
		newAvatar = &url
		patch.AvatarURL = &url
	}

	updated, err := u.svc.UpdateProfile(ctx, userID, patch)
	if err != nil {
		if newAvatar != nil {
			if derr := u.store.Delete(*newAvatar); derr != nil {
				slog.Warn("update profile cleanup: remove new avatar", "error", derr.Error(), "url", *newAvatar, "user_id", userID)
			}
		}
		return User{}, err
	}

	// The previous avatar file is superseded; drop it (best-effort).
	if newAvatar != nil && current.AvatarURL != nil && *current.AvatarURL != *newAvatar {
		if derr := u.store.Delete(*current.AvatarURL); derr != nil {
			slog.Warn("update profile: remove old avatar", "error", derr.Error(), "url", *current.AvatarURL, "user_id", userID)
		}
	}

	return updated, nil
}

// avatarProblem is an avatar validation failure carrying the exact
// client-facing message. It matches ErrAvatarInvalid via errors.Is.
type avatarProblem struct {
	msg string
}

func (e *avatarProblem) Error() string { return e.msg }
func (e *avatarProblem) Is(target error) bool { return target == ErrAvatarInvalid }

// stageAvatar validates, re-encodes, and stores one avatar upload, returning
// its URL. Size is enforced on actual bytes, not the client-claimed header.
func stageAvatar(store *storage.Local, fh *multipart.FileHeader) (string, error) {
	if fh.Size > maxAvatarSize {
		return "", ErrAvatarTooLarge
	}
	src, err := fh.Open()
	if err != nil {
		return "", err
	}
	// Cap at max+1 so the length check below enforces actual bytes, not
	// the client-claimed FileHeader.Size.
	data, readErr := io.ReadAll(io.LimitReader(src, maxAvatarSize+1))
	src.Close()
	if readErr != nil {
		return "", readErr
	}
	if len(data) > maxAvatarSize {
		return "", ErrAvatarTooLarge
	}
	if err := imaging.Validate(data); err != nil {
		return "", &avatarProblem{msg: err.Error()}
	}
	processed, err := imaging.Avatar(data)
	if err != nil {
		return "", err
	}
	return store.Save(processed, "webp")
}
