package storage

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// quarantinePaths resolves a stored media URL to its (source, destination)
// filesystem paths, preserving the relative structure under /uploads so a
// restore is a plain move back. It reports ok=false for anything that must
// not move: empty values, values containing .. , absolute filesystem paths
// outside /uploads, and non-uploads schemes (javascript:, data:, blob:,
// protocol-relative, ftp:, ...). Only http/https uploads URLs and relative
// uploads paths move. Callers treat ok=false as a safe skip, never an error.
func (l *Local) quarantinePaths(raw string) (src, dst string, ok bool) {
	v := strings.TrimSpace(raw)
	if v == "" || strings.Contains(v, "..") {
		return "", "", false
	}
	var rel string
	switch {
	case strings.HasPrefix(v, "/uploads/"):
		rel = strings.TrimPrefix(v, "/uploads/")
	case strings.HasPrefix(v, "uploads/"):
		rel = strings.TrimPrefix(v, "uploads/")
	default:
		u, err := url.Parse(v)
		if err != nil {
			return "", "", false
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return "", "", false
		}
		if u.Host == "" || !strings.HasPrefix(u.Path, "/uploads/") {
			return "", "", false
		}
		rel = strings.TrimPrefix(u.Path, "/uploads/")
	}
	// Query/fragment never reach the filesystem; the pathname alone
	// addresses the stored file.
	if i := strings.IndexAny(rel, "?#"); i >= 0 {
		rel = rel[:i]
	}
	if rel == "" || rel == "." || rel == "/" {
		return "", "", false
	}
	if filepath.IsAbs(rel) || filepath.IsAbs(v) && !strings.HasPrefix(v, "/uploads/") {
		return "", "", false
	}
	clean := filepath.Clean(rel)
	if clean != rel || clean == "." {
		return "", "", false
	}
	// Flat stored names today, but keep any sub-structure instead of
	// flattening so derived variants survive the move.
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		if part == "" || part == "." || part == ".." {
			return "", "", false
		}
	}
	src = filepath.Join(l.dir, clean)
	dst = filepath.Join(l.quarantineDir, clean)
	return src, dst, true
}

// Quarantine moves one stored file out of the static root into the quarantine
// directory. It is idempotent: a missing source (already deleted) or an
// already-moved file is a no-op returning (false, nil). Unsafe values are
// skipped the same way. A real filesystem failure returns an error and moves
// nothing; the caller logs it and keeps the pin hidden so a retry or sweep
// can finish the move. It never deletes.
func (l *Local) Quarantine(rawURL string) (bool, error) {
	src, dst, ok := l.quarantinePaths(rawURL)
	if !ok {
		return false, nil
	}
	if _, err := os.Stat(dst); err == nil {
		// Already quarantined: make sure no uploads copy remains (dedupe,
		// not deletion — the bytes are retained at dst).
		if err := os.Remove(src); err != nil && !os.IsNotExist(err) {
			return false, err
		}
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return false, err
	}
	if err := os.Rename(src, dst); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Restore moves a quarantined file back under the static root. It exists for
// operator recovery; no HTTP un-hide path calls it today.
func (l *Local) Restore(rawURL string) (bool, error) {
	src, dst, ok := l.quarantinePaths(rawURL)
	if !ok {
		return false, nil
	}
	if _, err := os.Stat(src); err == nil {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		return false, err
	}
	if err := os.Rename(dst, src); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// QuarantineWouldMove reports whether Quarantine would move anything, without
// touching the filesystem beyond stats. The sweep dry-run uses it to report
// counts.
func (l *Local) QuarantineWouldMove(rawURL string) bool {
	src, dst, ok := l.quarantinePaths(rawURL)
	if !ok {
		return false
	}
	if _, err := os.Stat(src); err != nil {
		return false
	}
	if _, err := os.Stat(dst); err == nil {
		return false
	}
	return true
}
