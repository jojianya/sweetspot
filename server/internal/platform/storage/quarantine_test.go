package storage

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func mustSave(t *testing.T, l *Local, data string) string {
	t.Helper()
	url, err := l.Save([]byte(data), "webp")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	return url
}

// TestQuarantineMovesFileAndServes404 proves a hidden pin's bytes leave the
// static root: gone from uploads, present in quarantine with identical
// content, and GET /uploads/<file> answers 404 afterwards.
func TestQuarantineMovesFileAndServes404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	uploads := filepath.Join(root, "uploads")
	l := NewLocalWithQuarantine(uploads, "http://api.test", filepath.Join(root, "quarantine"))

	url := mustSave(t, l, "hidden bytes")
	name := filepath.Base(url)

	if _, err := os.Stat(filepath.Join(uploads, name)); err != nil {
		t.Fatalf("source should exist before quarantine: %v", err)
	}

	moved, err := l.Quarantine(url)
	if err != nil {
		t.Fatalf("Quarantine: %v", err)
	}
	if !moved {
		t.Fatal("expected moved=true on first quarantine")
	}
	if _, err := os.Stat(filepath.Join(uploads, name)); !os.IsNotExist(err) {
		t.Fatalf("source should be gone from uploads, stat err: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "quarantine", name))
	if err != nil {
		t.Fatalf("quarantined file missing: %v", err)
	}
	if string(got) != "hidden bytes" {
		t.Fatalf("quarantined content = %q, want %q", got, "hidden bytes")
	}

	r := gin.New()
	r.Static("/uploads", uploads)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/uploads/"+name, nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GET /uploads/%s = %d, want 404", name, w.Code)
	}
}

// TestQuarantineIdempotent proves a retry or background sweep finishes safely:
// the second run moves nothing and changes nothing.
func TestQuarantineIdempotent(t *testing.T) {
	root := t.TempDir()
	l := NewLocalWithQuarantine(filepath.Join(root, "uploads"), "http://api.test", filepath.Join(root, "quarantine"))

	url := mustSave(t, l, "x")
	name := filepath.Base(url)

	if moved, err := l.Quarantine(url); err != nil || !moved {
		t.Fatalf("first Quarantine = (%v, %v), want (true, nil)", moved, err)
	}
	moved, err := l.Quarantine(url)
	if err != nil {
		t.Fatalf("second Quarantine: %v", err)
	}
	if moved {
		t.Fatal("second Quarantine should be a no-op (moved=false)")
	}
	if _, err := os.Stat(filepath.Join(root, "quarantine", name)); err != nil {
		t.Fatalf("quarantined file should still exist: %v", err)
	}
}

// TestQuarantineRejectsTraversal proves DB values cannot escape the uploads
// root: .. segments, absolute filesystem paths, and non-uploads schemes move
// nothing and touch nothing outside.
func TestQuarantineRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	uploads := filepath.Join(root, "uploads")
	quarantine := filepath.Join(root, "quarantine")
	l := NewLocalWithQuarantine(uploads, "http://api.test", quarantine)

	outside := filepath.Join(root, "outside.txt")
	if err := os.WriteFile(outside, []byte("do not touch"), 0o644); err != nil {
		t.Fatal(err)
	}
	legit := mustSave(t, l, "legit")

	bad := []string{
		"http://host/uploads/../../etc/passwd",
		"http://host/uploads/%2e%2e/x",
		"/etc/passwd",
		"/etc/shadow",
		"/uploads/../../outside.txt",
		"javascript:alert(1)",
		"//evil.com/uploads/x.webp",
		"data:text/html,<h1>x</h1>",
		"ftp://host/uploads/x.webp",
		"",
		"   ",
	}
	for _, v := range bad {
		moved, err := l.Quarantine(v)
		if err != nil {
			t.Errorf("Quarantine(%q): unexpected error %v (should skip, not fail)", v, err)
		}
		if moved {
			t.Errorf("Quarantine(%q): moved=true, want false (reject)", v)
		}
	}

	if got, _ := os.ReadFile(outside); string(got) != "do not touch" {
		t.Fatal("file outside the uploads root was touched")
	}
	entries, err := os.ReadDir(quarantine)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == "passwd" || e.Name() == "shadow" || e.Name() == "outside.txt" {
			t.Fatalf("traversal artifact %q landed in quarantine", e.Name())
		}
	}
	// The legitimate file must be untouched by the bad inputs above.
	if _, err := os.Stat(filepath.Join(uploads, filepath.Base(legit))); err != nil {
		t.Fatalf("legit file should be untouched: %v", err)
	}
}

// TestQuarantineMoveFailureReturnsError proves a broken quarantine target
// surfaces an error (the caller logs it and keeps the pin hidden) instead of
// panicking or silently dropping the file.
func TestQuarantineMoveFailureReturnsError(t *testing.T) {
	root := t.TempDir()
	uploads := filepath.Join(root, "uploads")
	// Point the quarantine dir at an existing FILE so MkdirAll/Rename fails.
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	l := NewLocalWithQuarantine(uploads, "http://api.test", blocker)

	url := mustSave(t, l, "y")
	if _, err := l.Quarantine(url); err == nil {
		t.Fatal("expected an error when the quarantine target is unusable, got nil")
	}
	// Source must still be in uploads (nothing was deleted on failure).
	if _, err := os.Stat(filepath.Join(uploads, filepath.Base(url))); err != nil {
		t.Fatalf("source should remain on move failure: %v", err)
	}
}
