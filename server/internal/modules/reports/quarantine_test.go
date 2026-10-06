package reports

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
)

type stubPhotos struct {
	detail pins.PinDetail
	err    error
}

func (s *stubPhotos) GetPin(context.Context, string) (pins.PinDetail, error) {
	return s.detail, s.err
}

func quarantineStore(t *testing.T) (*storage.Local, string, string) {
	t.Helper()
	root := t.TempDir()
	uploads := filepath.Join(root, "uploads")
	q := filepath.Join(root, "quarantine")
	return storage.NewLocalWithQuarantine(uploads, "http://api.test", q), uploads, q
}

func saveFile(t *testing.T, store *storage.Local, data string) string {
	t.Helper()
	url, err := store.Save([]byte(data), "webp")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	return url
}

// TestReviewApproveQuarantinesPinFiles proves the moderation hide path moves
// bytes out of /uploads: the review still commits (200) and the files are
// gone from the static root, present in quarantine.
func TestReviewApproveQuarantinesPinFiles(t *testing.T) {
	store, uploads, qdir := quarantineStore(t)
	photo := saveFile(t, store, "photo-bytes")
	thumb := saveFile(t, store, "thumb-bytes")

	loc := "POINT(1 2)"
	svc := &stubReviewService{
		report:   Report{PinID: uuidPtr("11111111-1111-1111-1111-111111111111")},
		location: &loc,
	}
	photos := &stubPhotos{detail: pins.PinDetail{
		Photos: []pins.PinPhoto{{PhotoURL: photo, ThumbnailURL: thumb}},
	}}
	h := NewHandler(svc, &recordingPublisher{}).WithQuarantine(store, photos)

	w := reviewRequest(t, h, "rep-1", "approve")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}
	for _, u := range []string{photo, thumb} {
		if _, err := os.Stat(filepath.Join(uploads, filepath.Base(u))); !os.IsNotExist(err) {
			t.Fatalf("%s should be gone from uploads", u)
		}
		if _, err := os.Stat(filepath.Join(qdir, filepath.Base(u))); err != nil {
			t.Fatalf("%s should be in quarantine: %v", u, err)
		}
	}
}

// TestReviewApproveQuarantinesAllPhotos proves full photo coverage: a pin
// with 3 photos plus thumbnails (6 files) leaves /uploads entirely — every
// file is quarantined and every old URL 404s.
func TestReviewApproveQuarantinesAllPhotos(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store, uploads, qdir := quarantineStore(t)
	var photos []pins.PinPhoto
	var urls []string
	for i := 0; i < 3; i++ {
		photo := saveFile(t, store, string(rune('a'+i))+`-full`)
		thumb := saveFile(t, store, string(rune('a'+i))+`-thumb`)
		photos = append(photos, pins.PinPhoto{PhotoURL: photo, ThumbnailURL: thumb})
		urls = append(urls, photo, thumb)
	}

	loc := "POINT(1 2)"
	svc := &stubReviewService{
		report:   Report{PinID: uuidPtr("11111111-1111-1111-1111-111111111111")},
		location: &loc,
	}
	h := NewHandler(svc, &recordingPublisher{}).
		WithQuarantine(store, &stubPhotos{detail: pins.PinDetail{Photos: photos}})

	w := reviewRequest(t, h, "rep-1", "approve")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	r := gin.New()
	r.Static("/uploads", uploads)
	for _, u := range urls {
		name := filepath.Base(u)
		if _, err := os.Stat(filepath.Join(uploads, name)); !os.IsNotExist(err) {
			t.Fatalf("%s should be gone from uploads", name)
		}
		if _, err := os.Stat(filepath.Join(qdir, name)); err != nil {
			t.Fatalf("%s should be in quarantine: %v", name, err)
		}
		sw := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/uploads/"+name, nil)
		r.ServeHTTP(sw, req)
		if sw.Code != http.StatusNotFound {
			t.Fatalf("GET /uploads/%s = %d, want 404", name, sw.Code)
		}
	}
}
// TestReviewApproveQuarantineFailureStillHides proves a storage failure never
// un-hides the pin: the DB commit stands, the request is still 200, the miss
// is left for the sweep, and nothing panics.
func TestReviewApproveQuarantineFailureStillHides(t *testing.T) {
	root := t.TempDir()
	uploads := filepath.Join(root, "uploads")
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := storage.NewLocalWithQuarantine(uploads, "http://api.test", blocker)
	photo := saveFile(t, store, "photo-bytes")

	loc := "POINT(1 2)"
	svc := &stubReviewService{
		report:   Report{PinID: uuidPtr("11111111-1111-1111-1111-111111111111")},
		location: &loc,
	}
	photos := &stubPhotos{detail: pins.PinDetail{
		Photos: []pins.PinPhoto{{PhotoURL: photo, ThumbnailURL: photo}},
	}}
	h := NewHandler(svc, &recordingPublisher{}).WithQuarantine(store, photos)

	w := reviewRequest(t, h, "rep-1", "approve")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 despite quarantine failure, got %d (body: %s)", w.Code, w.Body.String())
	}
	// Source stays put on failure (never deleted by the quarantine path).
	if _, err := os.Stat(filepath.Join(uploads, filepath.Base(photo))); err != nil {
		t.Fatalf("source should remain on move failure: %v", err)
	}
}

// TestSweepDryRunReportsWithoutMoving proves the pre-enable report path:
// dry-run counts files that would move and moves nothing.
func TestSweepDryRunReportsWithoutMoving(t *testing.T) {
	store, uploads, _ := quarantineStore(t)
	a := saveFile(t, store, "a")
	b := saveFile(t, store, "b")
	urls := []string{a, b}

	checked, moved, err := SweepURLs(context.Background(), urls, store, true)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if checked != 2 || moved != 2 {
		t.Fatalf("dry-run = (%d, %d), want (2, 2)", checked, moved)
	}
	for _, u := range urls {
		if _, err := os.Stat(filepath.Join(uploads, filepath.Base(u))); err != nil {
			t.Fatalf("dry-run must not move %s: %v", u, err)
		}
	}

	checked, moved, err = SweepURLs(context.Background(), urls, store, false)
	if err != nil {
		t.Fatalf("real sweep: %v", err)
	}
	if checked != 2 || moved != 2 {
		t.Fatalf("real sweep = (%d, %d), want (2, 2)", checked, moved)
	}

	// Second real sweep is a no-op.
	checked, moved, err = SweepURLs(context.Background(), urls, store, false)
	if err != nil {
		t.Fatalf("resweep: %v", err)
	}
	if moved != 0 {
		t.Fatalf("resweep moved = %d, want 0", moved)
	}
}
