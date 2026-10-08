package pins

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mime/multipart"

	users "github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
	"github.com/jojianya/sweetspot247-backend/pkg/geohash"
)

// Upload validation errors. The messages are the exact client-facing strings
// the handler responds with; the handler maps each to its status code.
var (
	ErrPhotoTooLarge  = errors.New("one or more photos exceed 10MB")
	ErrPhotoUnreadable = errors.New("could not read uploaded file")
	ErrPhotoInvalid   = errors.New("invalid photo")
	ErrPhotoSave      = errors.New("could not save uploaded file")
	ErrAccountMissing = errors.New("account no longer exists, please sign in again")
)

// photoProblem is an invalid-photo failure for one upload: the exact
// client-facing message ("photo %d: <reason>"). It matches ErrPhotoInvalid
// via errors.Is so the handler answers 400.
type photoProblem struct {
	msg string
}

func (e *photoProblem) Error() string { return e.msg }
func (e *photoProblem) Is(target error) bool { return target == ErrPhotoInvalid }

// Service holds the pin business rules: handlers parse input and format
// responses, the service decides. It owns the repository, the file store and
// the event publisher so multi-step writes (stage files, insert row, clean up
// on failure, publish) live in one testable place with no HTTP dependency.
type Service struct {
	repo   Repository
	store  *storage.Local
	events Events
	// roles resolves moderation rights from the database rather than the JWT,
	// so a demotion takes effect on the caller's next request.
	roles users.RoleReader
}

func NewService(repo Repository, store *storage.Local, events Events, roles users.RoleReader) *Service {
	if events == nil {
		events = nopEvents{}
	}
	return &Service{repo: repo, store: store, events: events, roles: roles}
}

func (s *Service) ListCategories(ctx context.Context) ([]Category, error) {
	return s.repo.ListCategories(ctx)
}

func (s *Service) ListPins(ctx context.Context, bbox [4]float64, categoryID *int, limit int) ([]PinListEntry, error) {
	if categoryID != nil {
		exists, err := s.repo.CategoryExists(ctx, *categoryID)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrCategoryNotFound
		}
	}
	return s.repo.ListPins(ctx, bbox, categoryID, limit)
}

func (s *Service) ListTrending(ctx context.Context, bbox [4]float64, limit int) ([]TrendingPin, error) {
	return s.repo.ListTrending(ctx, bbox, limit)
}

func (s *Service) SearchPins(ctx context.Context, query string, limit int) ([]PinListEntry, error) {
	return s.repo.SearchPins(ctx, query, limit)
}

func (s *Service) ListByUser(ctx context.Context, userID string, limit int) ([]PinListEntry, error) {
	exists, err := s.repo.UserExists(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	return s.repo.ListByUser(ctx, userID, limit)
}

// GetVisible loads a pin and enforces the hidden-pin rule: hidden pins do
// not exist unless the viewer owns the pin or moderates. The owner check is
// free; the moderator lookup runs only for hidden pins owned by someone
// else, keeping it off the common path.
func (s *Service) GetVisible(ctx context.Context, id, viewerID string) (PinDetail, error) {
	pin, err := s.repo.GetPin(ctx, id)
	if err != nil {
		return PinDetail{}, err
	}
	if pin.IsHidden && !s.canViewHidden(ctx, viewerID, pin) {
		return PinDetail{}, ErrNotFound
	}
	return pin, nil
}

func (s *Service) canViewHidden(ctx context.Context, viewerID string, pin PinDetail) bool {
	if viewerID == "" {
		return false
	}
	if viewerID == pin.UserID.String() {
		return true
	}
	if s.roles == nil {
		return false
	}
	// Live lookup rather than the gin-context cached role: the service has no
	// HTTP dependency, and the cached value came from this same database read
	// so the two cannot disagree.
	u, err := s.roles.GetByID(ctx, viewerID)
	if err != nil {
		return false
	}
	return u.Role == users.RoleAdmin || u.Role == users.RoleOwner
}

func (s *Service) RegisterView(ctx context.Context, id, viewerID string) (int64, error) {
	return s.repo.RegisterView(ctx, id, viewerID)
}

// CreatePinInput carries the validated scalar fields of a create-pin form
// plus the raw uploaded files. Field validation stays with the handler;
// existence checks, photo processing, staging, insert, cleanup and publish
// are the service's job.
type CreatePinInput struct {
	UserID     string
	Lat        float64
	Lng        float64
	Caption    *string
	CategoryID int
	Files      []*multipart.FileHeader
}

// CreatePinResult is the created pin plus the staged photo URLs, so the
// handler can build the {"pin", "photos"} response envelope.
type CreatePinResult struct {
	Pin         Pin
	PhotoURLs   []string
	ThumbURLs   []string
}

func (s *Service) CreatePin(ctx context.Context, in CreatePinInput) (CreatePinResult, error) {
	userExists, err := s.repo.UserExists(ctx, in.UserID)
	if err != nil {
		return CreatePinResult{}, err
	}
	if !userExists {
		return CreatePinResult{}, ErrAccountMissing
	}

	validated, err := processPhotos(in.Files)
	if err != nil {
		return CreatePinResult{}, err
	}

	exists, err := s.repo.CategoryExists(ctx, in.CategoryID)
	if err != nil {
		return CreatePinResult{}, err
	}
	if !exists {
		return CreatePinResult{}, ErrCategoryNotFound
	}

	photoURLs, thumbURLs, err := savePhotos(s.store, validated)
	if err != nil {
		return CreatePinResult{}, err
	}

	pin, err := s.repo.CreatePin(ctx, NewPin{
		UserID:        in.UserID,
		Lat:           in.Lat,
		Lng:           in.Lng,
		Caption:       in.Caption,
		CategoryID:    in.CategoryID,
		PhotoURLs:     photoURLs,
		ThumbnailURLs: thumbURLs,
		Geohash:       geohash.Encode(in.Lat, in.Lng),
	})
	if err != nil {
		// The photos are already on disk; remove them so a failed insert
		// cannot orphan files. Use min length to guard against mismatched
		// slices. Deletion failures are logged, not discarded.
		n := len(photoURLs)
		if len(thumbURLs) < n {
			n = len(thumbURLs)
		}
		for i := 0; i < n; i++ {
			if derr := s.store.Delete(photoURLs[i]); derr != nil {
				slog.Warn("cleanup failed: photo", "url", photoURLs[i], "error", derr)
			}
			if derr := s.store.Delete(thumbURLs[i]); derr != nil {
				slog.Warn("cleanup failed: thumbnail", "url", thumbURLs[i], "error", derr)
			}
		}
		return CreatePinResult{}, err
	}

	// Broadcast to connected maps (best-effort; never fails the create).
	cover := ""
	if len(thumbURLs) > 0 {
		cover = thumbURLs[0]
	} else if len(photoURLs) > 0 {
		cover = photoURLs[0]
	}
	s.events.PinCreated(ctx, Event{
		ID:         pin.ID.String(),
		UserID:     pin.UserID.String(),
		Location:   fmt.Sprintf("POINT(%v %v)", in.Lng, in.Lat),
		Caption:    pin.Caption,
		CategoryID: pin.CategoryID,
		CoverURL:   cover,
		CreatedAt:  pin.CreatedAt,
	})

	return CreatePinResult{Pin: pin, PhotoURLs: photoURLs, ThumbURLs: thumbURLs}, nil
}

// UpdatePinInput carries an update-pin request after field validation. A nil
// Caption/CategoryID keeps the current value; a nil Photos keeps the current
// photo set.
type UpdatePinInput struct {
	ID          string
	UserID      string
	IsModerator bool
	Caption     *string
	CategoryID  *int
	Files       []*multipart.FileHeader
}

// UpdatePinResult is the pin plus its photo set after the change, so the
// handler can answer with the current photos without a refetch.
type UpdatePinResult struct {
	Pin    Pin
	Photos []PinPhoto
}

func (s *Service) UpdatePin(ctx context.Context, in UpdatePinInput) (UpdatePinResult, error) {
	existing, err := s.repo.GetPin(ctx, in.ID)
	if err != nil {
		return UpdatePinResult{}, err
	}

	// Hidden pins do not exist for edits. Checked before any work is done so
	// a hidden pin answers 404 rather than revealing that it exists; the
	// UPDATE predicate repeats the check.
	visible, err := s.repo.PinVisible(ctx, in.ID)
	if err != nil {
		return UpdatePinResult{}, err
	}
	if !visible {
		return UpdatePinResult{}, ErrNotFound
	}

	if in.CategoryID != nil {
		exists, err := s.repo.CategoryExists(ctx, *in.CategoryID)
		if err != nil {
			return UpdatePinResult{}, err
		}
		if !exists {
			return UpdatePinResult{}, ErrCategoryNotFound
		}
	}

	patch := UpdatePinPatch{
		Caption:    in.Caption,
		CategoryID: in.CategoryID,
	}

	if len(in.Files) > 0 {
		validated, err := processPhotos(in.Files)
		if err != nil {
			return UpdatePinResult{}, err
		}

		photoURLs, thumbURLs, err := savePhotos(s.store, validated)
		if err != nil {
			return UpdatePinResult{}, err
		}

		patch.Photos = make([]NewPhoto, 0, len(photoURLs))
		for i := range photoURLs {
			patch.Photos = append(patch.Photos, NewPhoto{PhotoURL: photoURLs[i], ThumbnailURL: thumbURLs[i]})
		}
	}

	updated, photos, err := s.repo.UpdatePin(ctx, in.ID, in.UserID, in.IsModerator, patch)
	if err != nil {
		// The new photos are already on disk; remove them so a failed update
		// cannot orphan files. Deletion failures are logged, not discarded.
		for _, ph := range patch.Photos {
			if derr := s.store.Delete(ph.PhotoURL); derr != nil {
				slog.Warn("update pin cleanup: remove photo", "error", derr.Error(), "url", ph.PhotoURL, "pin_id", in.ID)
			}
			if derr := s.store.Delete(ph.ThumbnailURL); derr != nil {
				slog.Warn("update pin cleanup: remove thumbnail", "error", derr.Error(), "url", ph.ThumbnailURL, "pin_id", in.ID)
			}
		}
		return UpdatePinResult{}, err
	}

	// Best-effort cleanup of the replaced photos now that the swap succeeded.
	if patch.Photos != nil {
		for _, ph := range existing.Photos {
			if derr := s.store.Delete(ph.PhotoURL); derr != nil {
				slog.Warn("update pin: remove replaced photo", "error", derr.Error(), "url", ph.PhotoURL, "pin_id", in.ID)
			}
			if derr := s.store.Delete(ph.ThumbnailURL); derr != nil {
				slog.Warn("update pin: remove replaced thumbnail", "error", derr.Error(), "url", ph.ThumbnailURL, "pin_id", in.ID)
			}
		}
	}

	return UpdatePinResult{Pin: updated, Photos: photos}, nil
}

// DeletePin hides the pin and cleans up after it: removes the staged files
// and publishes the removal. The hide already committed before cleanup runs,
// so cleanup failures are logged and left for the sweep rather than failing
// the request. A nil error means the handler answers 204.
func (s *Service) DeletePin(ctx context.Context, id, userID string, isModerator bool) error {
	if err := s.repo.DeletePin(ctx, id, userID, isModerator); err != nil {
		return err
	}

	// Load the (now hidden) pin for photo cleanup and moderator audit
	// logging. pin_photos rows survive the soft-hide, so this read sees the
	// authoritative set. A read failure here must not fail the delete; files
	// are unreferenced either way and the miss is logged for a sweep.
	existing, err := s.repo.GetPin(ctx, id)
	if err != nil {
		slog.Warn("delete pin: load for cleanup", "error", err.Error(), "pin_id", id, "user_id", userID)
		return nil
	}

	if existing.UserID.String() != userID {
		slog.Info("moderator deleted pin", "moderator_id", userID, "pin_id", id, "owner_id", existing.UserID.String())
	}

	// Best-effort cleanup. The pin is already gone, so a storage failure
	// must not fail the request; the files are unreferenced either way.
	// Logged so an operator can sweep them.
	for _, ph := range existing.Photos {
		if derr := s.store.Delete(ph.PhotoURL); derr != nil {
			slog.Warn("delete pin: remove photo", "error", derr.Error(), "url", ph.PhotoURL, "pin_id", id)
		}
		if derr := s.store.Delete(ph.ThumbnailURL); derr != nil {
			slog.Warn("delete pin: remove thumbnail", "error", derr.Error(), "url", ph.ThumbnailURL, "pin_id", id)
		}
	}

	// Publish pin_removed for realtime updates (best-effort; never fatal).
	if s.events != nil {
		s.events.PinRemoved(ctx, PinRemoved{
			ID:       id,
			Location: existing.Location,
		})
	}
	return nil
}
