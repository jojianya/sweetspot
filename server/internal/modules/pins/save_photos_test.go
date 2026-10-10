package pins

// savePhotos failure-path coverage: a storage error must stay reachable through
// the ErrPhotoSave wrapper, so a caller can tell "could not save uploaded file"
// from the actual cause (disk full, permissions, no space left).
//
// The wrapper used %v for the cause, which flattened it to a string: err.Error()
// still read correctly, but errors.Is/As on the underlying storage error always
// failed. That matters because ErrPhotoSave is what the handler maps to a 500,
// so the wrapped cause is the only way to tell a full disk from a bug.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
)

// readOnlyStore returns a store whose uploads directory cannot be written to,
// so Save fails on the first file.
func readOnlyStore(t *testing.T) *storage.Local {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "uploads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	return storage.NewLocal(dir, "http://api.test")
}

func TestSavePhotosWrapsTheStorageCause(t *testing.T) {
	_, _, err := savePhotos(readOnlyStore(t), []validatedFile{{data: []byte("x"), thumb: []byte("y"), ext: "webp"}})
	if err == nil {
		t.Fatal("savePhotos returned no error for an unwritable uploads dir")
	}

	// The handler's mapping still works.
	if !errors.Is(err, ErrPhotoSave) {
		t.Errorf("errors.Is(err, ErrPhotoSave) = false, want true: the handler answers 500 on this sentinel")
	}

	// The cause is still there. Before the fix this was a string in the
	// message and unreachable: errors.Is(err, os.ErrPermission) was false.
	if !errors.Is(err, os.ErrPermission) {
		t.Errorf("errors.Is(err, os.ErrPermission) = false, want true: the storage cause was flattened by %%v")
	}
}

// The cleanup path must still run on failure: a partially written set leaves
// nothing behind, and no orphan file appears in the uploads dir.
func TestSavePhotosCleansUpAfterAFailure(t *testing.T) {
	store := readOnlyStore(t)
	photos, thumbs, err := savePhotos(store, []validatedFile{
		{data: []byte("x"), thumb: []byte("y"), ext: "webp"},
		{data: []byte("x"), thumb: []byte("y"), ext: "webp"},
	})
	if err == nil {
		t.Fatal("expected an error, got none")
	}
	if photos != nil || thumbs != nil {
		t.Errorf("returned (%v, %v), want (nil, nil) on failure", photos, thumbs)
	}

	entries, rerr := os.ReadDir(store.Dir())
	if rerr != nil {
		t.Fatalf("read uploads dir: %v", rerr)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("uploads dir is not empty after a failed save: %v", names)
	}
}
