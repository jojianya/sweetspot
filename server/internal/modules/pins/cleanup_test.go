package pins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
)

// TestDeleteFilesContinuesPastFailures proves best-effort cleanup: a failure
// on one URL never stops the rest, and blank URLs are skipped without
// touching storage.
func TestDeleteFilesContinuesPastFailures(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ok.webp"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A non-empty directory fails os.Remove, simulating a storage failure
	// for exactly one of the URLs.
	if err := os.Mkdir(filepath.Join(dir, "stuck"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stuck", "inner"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &Service{store: storage.NewLocal(dir, "http://t")}
	s.deleteFiles("test cleanup", nil,
		"http://t/uploads/stuck",
		"",
		"http://t/uploads/ok.webp",
	)

	if _, err := os.Stat(filepath.Join(dir, "ok.webp")); !os.IsNotExist(err) {
		t.Fatalf("ok.webp should be deleted, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "stuck")); err != nil {
		t.Fatalf("failed URL must be left for retry, stat err = %v", err)
	}
}
