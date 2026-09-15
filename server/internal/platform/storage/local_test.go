package storage

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestLocalSave(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "uploads")
	l := NewLocal(dir, "http://localhost:8081")

	url, err := l.Save([]byte("fake image bytes"), "jpg")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	want := "http://localhost:8081/uploads/"
	if !strings.HasPrefix(url, want) {
		t.Fatalf("url %q does not have prefix %q", url, want)
	}
	name := strings.TrimPrefix(url, want)
	if !strings.HasSuffix(name, ".jpg") {
		t.Fatalf("expected .jpg suffix, got %q", name)
	}
	if strings.Contains(name, "/") {
		t.Fatalf("name contains slash: %q", name)
	}

	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(b) != "fake image bytes" {
		t.Fatalf("content mismatch: %q", b)
	}
}

func TestLocalSaveUniqueNames(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "uploads")
	l := NewLocal(dir, "http://localhost:8081")

	seen := map[string]bool{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			url, err := l.Save([]byte("x"), "png")
			if err != nil {
				t.Errorf("Save: %v", err)
				return
			}
			name := strings.TrimPrefix(url, "http://localhost:8081/uploads/")
			mu.Lock()
			seen[name] = true
			mu.Unlock()
		}()
	}
	wg.Wait()
	if len(seen) != 50 {
		t.Fatalf("expected 50 unique names, got %d", len(seen))
	}
}

func TestLocalDirCreated(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "uploads")
	l := NewLocal(dir, "http://localhost:8081")
	if _, err := l.Save([]byte("x"), "png"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".keep")); !os.IsNotExist(err) {
		t.Fatalf("nested dir should not exist yet: %v", err)
	}
	if _, httpErr := http.Get("http://localhost:8081/health"); httpErr != nil {
		t.Logf("note: server not required for this test; %v", httpErr)
	}
}
