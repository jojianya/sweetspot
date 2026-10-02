package main

// reprocess-exif is a one-off maintenance command that strips EXIF/GPS
// metadata from files already stored in the uploads volume (images written
// before StripMetadata was enabled in the upload pipeline).
//
// Usage:
//   go run ./cmd/reprocess-exif --dir ./uploads --dry-run   # report only
//   go run ./cmd/reprocess-exif --dir ./uploads             # strip in place
//
// Behavior:
//   - Walks --dir non-recursively; only regular files are considered.
//   - Files without an EXIF block are skipped untouched (idempotent: a second
//     run strips nothing).
//   - Files that fail to decode are reported and left alone, never deleted.
//   - Replacement is atomic (temp file + rename) with mode 0644.
//   - Never touches the database: stored URLs keep working because filenames
//     are unchanged. Do NOT run it against live data without a backup;
//     snapshot the uploads volume first (see docs/runbook).
//   - Exits non-zero if any file fails, so scripts can detect partial runs.

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jojianya/sweetspot247-backend/internal/modules/pins/imaging"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "reprocess-exif:", err)
		os.Exit(1)
	}
}

func run() error {
	dir := flag.String("dir", "./uploads", "uploads directory to process")
	dryRun := flag.Bool("dry-run", false, "report files that would be stripped without changing them")
	flag.Parse()

	entries, err := os.ReadDir(*dir)
	if err != nil {
		return fmt.Errorf("read dir %s: %w", *dir, err)
	}

	var scanned, stripped, skipped, failed int
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		path := filepath.Join(*dir, e.Name())
		scanned++
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("reprocess-exif: unreadable file, left alone", "file", e.Name(), "error", err.Error())
			failed++
			continue
		}
		if !imaging.NeedsStrip(data) {
			skipped++
			continue
		}
		if *dryRun {
			slog.Info("reprocess-exif: would strip", "file", e.Name())
			stripped++
			continue
		}
		clean, err := imaging.StripImage(data)
		if err != nil {
			slog.Warn("reprocess-exif: strip failed, left alone", "file", e.Name(), "error", err.Error())
			failed++
			continue
		}
		if err := atomicReplace(path, clean); err != nil {
			slog.Warn("reprocess-exif: replace failed", "file", e.Name(), "error", err.Error())
			failed++
			continue
		}
		slog.Info("reprocess-exif: stripped", "file", e.Name())
		stripped++
	}

	slog.Info("reprocess-exif: done", "scanned", scanned, "stripped", stripped, "skipped", skipped, "failed", failed)
	if failed > 0 {
		return fmt.Errorf("%d file(s) failed", failed)
	}
	return nil
}

func atomicReplace(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".strip-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
