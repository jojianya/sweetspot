package storage

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func exdevRename(_, _ string) error {
	return &os.LinkError{Op: "rename", Old: "a", New: "b", Err: syscall.EXDEV}
}

// TestQuarantineCrossFilesystemFallback proves separate volumes don't strand
// files: when rename fails with EXDEV the file is copied (temp name in the
// destination, fsynced, atomically renamed, size-verified) and only then is
// the source removed. The copy is always owner-only (0600), never preserving
// the world-readable source mode.
func TestQuarantineCrossFilesystemFallback(t *testing.T) {
	root := t.TempDir()
	uploads := filepath.Join(root, "uploads")
	qdir := filepath.Join(root, "quarantine")
	l := NewLocalWithQuarantine(uploads, "http://api.test", qdir)

	url := mustSave(t, l, "cross-volume bytes")
	name := filepath.Base(url)

	// Only the cross-volume leg fails: same-dir renames still work, exactly
	// like a real EXDEV world.
	oldRename := renameFn
	renameFn = func(oldpath, newpath string) error {
		if oldpath == filepath.Join(uploads, name) {
			return exdevRename(oldpath, newpath)
		}
		return os.Rename(oldpath, newpath)
	}
	defer func() { renameFn = oldRename }()

	moved, err := l.Quarantine(url)
	if err != nil {
		t.Fatalf("Quarantine over EXDEV: %v", err)
	}
	if !moved {
		t.Fatal("expected moved=true via copy fallback")
	}
	if _, err := os.Stat(filepath.Join(uploads, name)); !os.IsNotExist(err) {
		t.Fatalf("source should be gone, stat err: %v", err)
	}
	dst := filepath.Join(qdir, name)
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("quarantined file missing: %v", err)
	}
	if string(got) != "cross-volume bytes" {
		t.Fatalf("content = %q, want %q", got, "cross-volume bytes")
	}
	if st, err := os.Stat(dst); err != nil {
		t.Fatalf("stat quarantine copy: %v", err)
	} else if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", st.Mode().Perm())
	}
	// No temp file may remain under any name.
	entries, err := os.ReadDir(qdir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), quarantineTempPrefix) {
			t.Fatalf("stale temp file %q left behind", e.Name())
		}
	}
}

// TestQuarantineFailedCopyKeepsSource proves a failed copy deletes nothing:
// the source stays intact and no temp file remains in quarantine.
func TestQuarantineFailedCopyKeepsSource(t *testing.T) {
	oldRename, oldCopy := renameFn, copyFileFn
	renameFn = exdevRename
	copyFileFn = func(_, _ string, _ os.FileMode) error {
		return os.ErrInvalid
	}
	defer func() { renameFn, copyFileFn = oldRename, oldCopy }()

	root := t.TempDir()
	uploads := filepath.Join(root, "uploads")
	qdir := filepath.Join(root, "quarantine")
	l := NewLocalWithQuarantine(uploads, "http://api.test", qdir)

	url := mustSave(t, l, "precious bytes")
	name := filepath.Base(url)

	if _, err := l.Quarantine(url); err == nil {
		t.Fatal("expected copy error, got nil")
	}
	if got, err := os.ReadFile(filepath.Join(uploads, name)); err != nil || string(got) != "precious bytes" {
		t.Fatalf("source intact check: content=%q err=%v", got, err)
	}
	entries, err := os.ReadDir(qdir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("quarantine dir should hold no temp files, has %d", len(entries))
	}
}

// TestCleanStaleTemps proves crash leftovers never collide with real files:
// temp-prefix files are removed, final-name files are untouched.
func TestCleanStaleTemps(t *testing.T) {
	root := t.TempDir()
	qdir := filepath.Join(root, "quarantine")
	l := NewLocalWithQuarantine(filepath.Join(root, "uploads"), "http://api.test", qdir)
	if err := os.MkdirAll(qdir, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(qdir, quarantineTempPrefix+"deadbeef")
	if err := os.WriteFile(stale, []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	legit := filepath.Join(qdir, "abc123.webp")
	if err := os.WriteFile(legit, []byte("real"), 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := l.CleanStaleTemps()
	if err != nil {
		t.Fatalf("CleanStaleTemps: %v", err)
	}
	if n != 1 {
		t.Fatalf("cleaned = %d, want 1", n)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("stale temp should be gone")
	}
	if got, err := os.ReadFile(legit); err != nil || string(got) != "real" {
		t.Fatalf("legit file must survive: %q %v", got, err)
	}
}

// TestEnsureQuarantineDirTightensExisting proves boot heals volumes created
// before the 0700 default: a pre-existing 0755 dir comes back 0700.
func TestEnsureQuarantineDirTightensExisting(t *testing.T) {
	root := t.TempDir()
	qdir := filepath.Join(root, "quarantine")
	if err := os.MkdirAll(qdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(qdir, 0o755); err != nil {
		t.Fatal(err)
	}
	l := NewLocalWithQuarantine(filepath.Join(root, "uploads"), "http://api.test", qdir)
	if err := l.EnsureQuarantineDir(); err != nil {
		t.Fatalf("EnsureQuarantineDir: %v", err)
	}
	if st, err := os.Stat(qdir); err != nil {
		t.Fatal(err)
	} else if st.Mode().Perm() != 0o700 {
		t.Fatalf("mode = %o, want 700", st.Mode().Perm())
	}
}
