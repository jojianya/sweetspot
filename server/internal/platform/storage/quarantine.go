package storage

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// quarantineTempPrefix marks interrupted-copy temp files inside the quarantine
// directory. Temp names never equal a final name, so a crash mid-copy cannot
// leave a partial file where the sweep or a later quarantine would mistake it
// for a real quarantined file; the sweep cleans them.
const quarantineTempPrefix = ".qtmp-"

// renameFn and copyFileFn are package variables (not parameters) so tests can
// simulate a cross-volume rename (EXDEV) and a failed copy without touching
// production call sites.
var renameFn = os.Rename
var copyFileFn = copyFileSynced

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
	if filepath.IsAbs(rel) {
		return "", "", false
	}
	if filepath.IsAbs(v) && !strings.HasPrefix(v, "/uploads/") {
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
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return false, err
	}
	if err := renameFn(src, dst); err == nil {
		// Quarantined bytes are sensitive: lock the moved file down even
		// though the source arrived world-readable.
		if err := os.Chmod(dst, 0o600); err != nil {
			return false, err
		}
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else if !isEXDEV(err) {
		return false, err
	}
	// Cross-volume (uploads and quarantine on separate mounts): copy then
	// remove, verifying before touching the source.
	return l.quarantineCopy(src, dst)
}

func isEXDEV(err error) bool {
	return errors.Is(err, syscall.EXDEV)
}

// quarantineCopy falls back to copy+verify+remove when rename reports EXDEV.
// The copy lands under a temp name in the destination dir, is fsynced, is
// atomically renamed within the destination, has its size verified, and only
// then is the source removed. Any failure removes the temp file and leaves
// the source intact.
func (l *Local) quarantineCopy(src, dst string) (bool, error) {
	st, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), quarantineTempPrefix+"*")
	if err != nil {
		return false, err
	}
	tmpName := tmp.Name()
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return false, err
	}
	if err := copyFileFn(src, tmpName, st.Mode()); err != nil {
		os.Remove(tmpName)
		return false, err
	}
	if err := renameFn(tmpName, dst); err != nil {
		os.Remove(tmpName)
		return false, err
	}
	if err := fsyncDir(filepath.Dir(dst)); err != nil {
		return false, err
	}
	fin, err := os.Stat(dst)
	if err != nil {
		return false, err
	}
	if fin.Size() != st.Size() {
		os.Remove(dst)
		return false, fmt.Errorf("quarantine copy size mismatch for %q", filepath.Base(dst))
	}
	if err := os.Remove(src); err != nil {
		// Copy verified; the retry path dedupes via the dst-exists branch.
		return false, err
	}
	return true, nil
}

// copyFileSynced copies src to the existing temp path with owner-only
// permissions and fsyncs before returning. Quarantined bytes are sensitive,
// so the copy is always 0600 regardless of the source mode.
func copyFileSynced(src, tmp string, _ os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	// CreateTemp pins 0600; enforce it explicitly so the mode never depends
	// on the source or the process umask.
	if err := out.Chmod(0o600); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func fsyncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// EnsureQuarantineDir creates the quarantine directory owner-only and
// tightens a pre-existing one to 0700 (volumes created before the 0700
// default keep their old bits otherwise). It returns any failure so boot can
// treat an unusable quarantine as a startup failure instead of silently
// leaving hidden pins public.
func (l *Local) EnsureQuarantineDir() error {
	if err := os.MkdirAll(l.quarantineDir, 0o700); err != nil {
		return err
	}
	return os.Chmod(l.quarantineDir, 0o700)
}

// VerifyQuarantineWritable proves the quarantine directory can actually take
// files: ensure (mkdir + tighten to 0700) plus a write probe (create, sync,
// remove a temp file). Boot treats any failure as a startup failure so a
// broken setup can never silently leave hidden pins public.
func (l *Local) VerifyQuarantineWritable() error {
	if err := l.EnsureQuarantineDir(); err != nil {
		return err
	}
	f, err := os.CreateTemp(l.quarantineDir, ".wprobe-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return nil
}

// CleanStaleTemps removes interrupted-copy temp files from the quarantine
// directory. Final-name files are never touched. The sweep calls this in real
// mode; dry-run leaves the filesystem alone.
func (l *Local) CleanStaleTemps() (int, error) {
	entries, err := os.ReadDir(l.quarantineDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), quarantineTempPrefix) {
			continue
		}
		if err := os.Remove(filepath.Join(l.quarantineDir, e.Name())); err != nil && !os.IsNotExist(err) {
			return n, err
		}
		n++
	}
	return n, nil
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
