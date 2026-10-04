package imaging

import (
	"bytes"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/h2non/bimg"
)

func withResizeStub(t *testing.T, stub func([]byte, bimg.Options) ([]byte, error)) {
	t.Helper()
	orig := resizeFn
	resizeFn = stub
	t.Cleanup(func() { resizeFn = orig })
}

func TestProcessPanicReturnsError(t *testing.T) {
	withResizeStub(t, func([]byte, bimg.Options) ([]byte, error) {
		panic("libvips exploded")
	})

	result, err := Process([]byte("data"))
	if err == nil {
		t.Fatal("expected error from panicking resize, got nil")
	}
	if len(result.Full) != 0 || len(result.Thumb) != 0 {
		t.Fatalf("expected empty result on panic, got %+v", result)
	}

	// The semaphore must have been released: a follow-up call must not block.
	withResizeStub(t, func([]byte, bimg.Options) ([]byte, error) {
		return []byte("ok"), nil
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = Process([]byte("data"))
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Process blocked after panic: semaphore was not released")
	}
}

func TestAvatarPanicReturnsError(t *testing.T) {
	withResizeStub(t, func([]byte, bimg.Options) ([]byte, error) {
		panic("libvips exploded")
	})

	img, err := Avatar([]byte("data"))
	if err == nil {
		t.Fatal("expected error from panicking resize, got nil")
	}
	if len(img) != 0 {
		t.Fatalf("expected nil image on panic, got %d bytes", len(img))
	}
}

func TestProcessResizeErrorIsReturned(t *testing.T) {
	withResizeStub(t, func([]byte, bimg.Options) ([]byte, error) {
		return nil, errors.New("decode failed")
	})

	if _, err := Process([]byte("data")); err == nil {
		t.Fatal("expected resize error to be returned, got nil")
	}
	if _, err := Avatar([]byte("data")); err == nil {
		t.Fatal("expected avatar resize error to be returned, got nil")
	}
}

func assertNoGPS(t *testing.T, name string, buf []byte) {
	t.Helper()
	if bytes.Contains(buf, []byte("Exif\x00\x00")) || bytes.Contains(buf, []byte("EXIF")) {
		t.Errorf("%s: output still contains an EXIF marker", name)
	}
	if bytes.Contains(buf, []byte("GPSLatitude")) {
		t.Errorf("%s: output still contains GPS metadata", name)
	}
	meta, err := bimg.Metadata(buf)
	if err != nil {
		t.Fatalf("%s: read output metadata: %v", name, err)
	}
	if meta.EXIF.GPSLatitude != "" || meta.EXIF.GPSLongitude != "" {
		t.Errorf("%s: output GPS EXIF not stripped: lat=%q lng=%q", name, meta.EXIF.GPSLatitude, meta.EXIF.GPSLongitude)
	}
}

// TestProcessStripsGPSMetadata proves uploaded photos cannot leak precise
// locations through EXIF: the fixture carries real GPS tags (verified with
// vipsheader), and every persisted output (full, thumbnail, avatar) must be
// free of them. Only re-encoded bytes are ever stored (see savePhotos), so
// stripping here covers the whole persistence path.
func TestProcessStripsGPSMetadata(t *testing.T) {
	raw, err := os.ReadFile("testdata/gps-exif.jpg")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	sanity, err := bimg.Metadata(raw)
	if err != nil {
		t.Fatalf("read fixture metadata: %v", err)
	}
	if sanity.EXIF.GPSLatitude == "" {
		t.Fatal("fixture has no GPS EXIF; test would prove nothing")
	}

	result, err := Process(raw)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	assertNoGPS(t, "full", result.Full)
	assertNoGPS(t, "thumbnail", result.Thumb)

	avatar, err := Avatar(raw)
	if err != nil {
		t.Fatalf("Avatar: %v", err)
	}
	assertNoGPS(t, "avatar", avatar)
}

func TestStripImage(t *testing.T) {
	raw, err := os.ReadFile("testdata/gps-exif.jpg")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if !NeedsStrip(raw) {
		t.Fatal("fixture should need stripping; test would prove nothing")
	}

	clean, err := StripImage(raw)
	if err != nil {
		t.Fatalf("StripImage: %v", err)
	}
	assertNoGPS(t, "stripped", clean)

	// Idempotent: already-clean bytes need no further work.
	if NeedsStrip(clean) {
		t.Fatal("stripped output still needs stripping; not idempotent")
	}

	if _, err := StripImage([]byte("not an image")); err == nil {
		t.Fatal("expected an error for non-image input, got nil")
	}
}

// TestProcessAutorotatesOrientation proves stripping is safe for rotated
// phone photos: the fixture is stored 64x32 with EXIF orientation 6 (rotate
// 90 CW, i.e. portrait content), so an upright output must be taller than
// wide with the orientation flag consumed.
func TestProcessAutorotatesOrientation(t *testing.T) {
	raw, err := os.ReadFile("testdata/orient-6.jpg")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	sanity, err := bimg.Metadata(raw)
	if err != nil {
		t.Fatalf("read fixture metadata: %v", err)
	}
	if sanity.EXIF.Orientation != 6 {
		t.Fatalf("fixture orientation = %d, want 6; test would prove nothing", sanity.EXIF.Orientation)
	}

	result, err := Process(raw)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	meta, err := bimg.Metadata(result.Full)
	if err != nil {
		t.Fatalf("read output metadata: %v", err)
	}
	if meta.Size.Width >= meta.Size.Height {
		t.Errorf("output not upright: got %dx%d, want portrait (height > width)", meta.Size.Width, meta.Size.Height)
	}
	assertNoGPS(t, "rotated full", result.Full)
}
