package imaging

import (
	"errors"
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
