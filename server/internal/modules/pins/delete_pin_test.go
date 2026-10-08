package pins

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
)

func init() { gin.SetMode(gin.TestMode) }

// stubRepo implements Repository with only the methods the handler tests
// touch. The embedded nil interface means any other call panics, which is
// what we want from a test double.
type stubRepo struct {
	Repository

	detail PinDetail
	getErr error
	delErr error

	deletedID          string
	deletedUserID      string
	deletedIsModerator bool

	userExists     bool
	userExistsErr  error
	categoryExists bool
	createErr      error
	created        bool
	updateErr      error
	updated        bool
	updatePhotos   int
	updatedID      string
	// lastPatch records the exact patch the handler sent to the repository.
	lastPatch UpdatePinPatch
	// updateResultPhotos is what UpdatePin reports as the pin's photo set
	// after the update; visible answers PinVisible.
	updateResultPhotos []PinPhoto
	visible            bool
}

func (s *stubRepo) GetPin(context.Context, string) (PinDetail, error) {
	return s.detail, s.getErr
}

func (s *stubRepo) DeletePin(_ context.Context, id, userID string, isModerator bool) error {
	s.deletedID, s.deletedUserID, s.deletedIsModerator = id, userID, isModerator
	return s.delErr
}

func (s *stubRepo) UserExists(context.Context, string) (bool, error) {
	return s.userExists, s.userExistsErr
}

func (s *stubRepo) CategoryExists(context.Context, int) (bool, error) {
	return s.categoryExists, nil
}

func (s *stubRepo) CreatePin(_ context.Context, _ NewPin) (Pin, error) {
	s.created = true
	return Pin{}, s.createErr
}

func (s *stubRepo) UpdatePin(_ context.Context, id, userID string, isModerator bool, patch UpdatePinPatch) (Pin, []PinPhoto, error) {
	s.updated = true
	s.lastPatch = patch
	if patch.Photos != nil {
		s.updatePhotos = len(patch.Photos)
	}
	if s.updateErr != nil {
		return Pin{}, nil, s.updateErr
	}
	// Mirror the repository's UPDATE predicate: owner or moderator, visible only.
	if !s.visible {
		return Pin{}, nil, ErrNotFound
	}
	if s.detail.UserID != userID && !isModerator {
		return Pin{}, nil, ErrForbidden
	}
	s.updatedID = id
	return Pin{}, s.updateResultPhotos, nil
}

func (s *stubRepo) PinVisible(context.Context, string) (bool, error) {
	return s.visible, nil
}

// stubRoles is a one-method RoleReader returning a fixed role for any caller.
type stubRoles struct {
	role string
}

func (s stubRoles) GetByID(context.Context, string) (users.User, error) {
	return users.User{Role: s.role}, nil
}

// newDeleteHarness wires a handler with a stub repo and a real local storage in
// dir, behind a gin route whose auth middleware is stubbed to report userID.
func newDeleteHarness(t *testing.T, repo *stubRepo, dir string) *gin.Engine {
	t.Helper()
	h := &Handler{
		repo:   repo,
		store:  storage.NewLocal(dir, "http://api.test"),
		events: nopEvents{},
	}

	r := gin.New()
	r.DELETE("/pins/:id", func(c *gin.Context) {
		c.Set("user_id", "owner-1")
		h.DeletePin(c)
	})
	return r
}

// stagePhoto writes a real file into the storage dir and returns the URL Save
// would have returned for it.
func stagePhoto(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("img"), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return "http://api.test/uploads/" + name
}

func onDisk(t *testing.T, dir, name string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

func deletePin(r *gin.Engine, id string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/pins/"+id, nil))
	return w
}

// TestDeletePinRemovesPhotoFiles is the regression test for the audit finding
// that DeletePin left every uploaded file on disk, still publicly served, after
// removing the pin row. pin_photos is ON DELETE CASCADE, so the handler has to
// capture the URLs before the delete.
func TestDeletePinRemovesPhotoFiles(t *testing.T) {
	dir := t.TempDir()
	photoURL := stagePhoto(t, dir, "photo-1.webp")
	thumbURL := stagePhoto(t, dir, "thumb-1.webp")

	r := newDeleteHarness(t, &stubRepo{detail: PinDetail{
		Photos: []PinPhoto{
			{PhotoURL: photoURL, ThumbnailURL: thumbURL},
		},
		Pin: Pin{Location: "POINT(0 0)"},
	}}, dir)

	w := deletePin(r, "pin-1")
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d (body: %s)", w.Code, w.Body.String())
	}
	for _, name := range []string{"photo-1.webp", "thumb-1.webp"} {
		if onDisk(t, dir, name) {
			t.Errorf("%s still on disk after the pin was deleted", name)
		}
	}
}

// TestDeletePinForwardsOwnershipArgs guards the fetch-then-delete ordering: the
// delete must still be scoped to the caller's user id, so knowing an id is not
// enough to remove someone else's pin.
func TestDeletePinForwardsOwnershipArgs(t *testing.T) {
	repo := &stubRepo{}
	r := newDeleteHarness(t, repo, t.TempDir())

	if w := deletePin(r, "pin-42"); w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if repo.deletedID != "pin-42" {
		t.Errorf("DeletePin got id %q, want \"pin-42\"", repo.deletedID)
	}
	if repo.deletedUserID != "owner-1" {
		t.Errorf("DeletePin got userID %q, want \"owner-1\"", repo.deletedUserID)
	}
}

// TestDeletePinForbiddenLeavesFilesOnDisk proves cleanup only runs after a
// successful delete: a refused delete must not remove files belonging to a pin
// that still exists.
func TestDeletePinForbiddenLeavesFilesOnDisk(t *testing.T) {
	dir := t.TempDir()
	stagePhoto(t, dir, "photo-keep.webp")
	store := storage.NewLocal(dir, "http://api.test")

	repo := &stubRepo{
		detail: PinDetail{Photos: []PinPhoto{
			{PhotoURL: "http://api.test/uploads/photo-keep.webp", ThumbnailURL: "http://api.test/uploads/thumb-keep.webp"},
		}},
		delErr: ErrForbidden,
	}
	h := &Handler{repo: repo, store: store}
	r := gin.New()
	r.DELETE("/pins/:id", func(c *gin.Context) {
		c.Set("user_id", "owner-1")
		h.DeletePin(c)
	})

	if w := deletePin(r, "pin-1"); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d (body: %s)", w.Code, w.Body.String())
	}
	if !onDisk(t, dir, "photo-keep.webp") {
		t.Error("photo was deleted even though the pin delete was refused")
	}
}

// TestDeletePinNotFoundReturns404 keeps the 404 path intact: a missing or
// already-hidden pin must still be 404, not 500.
func TestDeletePinNotFoundReturns404(t *testing.T) {
	r := newDeleteHarness(t, &stubRepo{delErr: ErrNotFound}, t.TempDir())

	w := deletePin(r, "gone")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body: %s)", w.Code, w.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if body["error"] == "" {
		t.Error("404 body has no error message")
	}
}

// TestDeletePinHandlesPinWithNoPhotos covers the zero-photo case so the cleanup
// loop cannot panic on an empty slice.
func TestDeletePinHandlesPinWithNoPhotos(t *testing.T) {
	r := newDeleteHarness(t, &stubRepo{detail: PinDetail{Photos: []PinPhoto{}}}, t.TempDir())

	if w := deletePin(r, "pin-1"); w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// TestDeletePinForwardsModeratorFlag proves the moderator bypass reaches the
// repository as a flag (same IsModerator check as UpdatePin) instead of a
// handler-side owner comparison, so the SQL predicate decides atomically.
func TestDeletePinForwardsModeratorFlag(t *testing.T) {
	dir := t.TempDir()
	repo := &stubRepo{detail: PinDetail{Photos: []PinPhoto{}, Pin: Pin{Location: "POINT(0 0)"}}}
	h := &Handler{
		repo:   repo,
		store:  storage.NewLocal(dir, "http://api.test"),
		roles:  stubRoles{role: users.RoleAdmin},
		events: nopEvents{},
	}
	r := gin.New()
	r.DELETE("/pins/:id", func(c *gin.Context) {
		c.Set("user_id", "moderator-1")
		h.DeletePin(c)
	})

	if w := deletePin(r, "pin-9"); w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d (body: %s)", w.Code, w.Body.String())
	}
	if !repo.deletedIsModerator {
		t.Error("expected isModerator=true to reach the repository for an admin caller")
	}
	if repo.deletedUserID != "moderator-1" {
		t.Errorf("DeletePin got userID %q, want \"moderator-1\"", repo.deletedUserID)
	}
}

// TestDeletePinNonModeratorFlagIsFalse guards the common path: a plain owner
// delete must not claim moderation rights.
func TestDeletePinNonModeratorFlagIsFalse(t *testing.T) {
	repo := &stubRepo{detail: PinDetail{Photos: []PinPhoto{}}}
	r := newDeleteHarness(t, repo, t.TempDir())

	if w := deletePin(r, "pin-1"); w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d (body: %s)", w.Code, w.Body.String())
	}
	if repo.deletedIsModerator {
		t.Error("expected isModerator=false for a handler without roles")
	}
}
