package storage

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ErrUnsupportedContentType = errors.New("unsupported content type")

type Local struct {
	dir  string
	base string
	// quarantineDir holds files moved out of dir when their pin is hidden.
	// It must sit outside the static root so /uploads/<file> 404s after
	// the move; restores are a plain move back.
	quarantineDir string
}

func NewLocal(dir, base string) *Local {
	return &Local{dir: dir, base: strings.TrimSuffix(base, "/"), quarantineDir: dir + "-quarantine"}
}

// NewLocalWithQuarantine pins the quarantine directory explicitly (production
// wiring from QUARANTINE_DIR). An empty quarantineDir falls back to the
// sibling default used by NewLocal.
func NewLocalWithQuarantine(dir, base, quarantineDir string) *Local {
	if strings.TrimSpace(quarantineDir) == "" {
		return NewLocal(dir, base)
	}
	return &Local{dir: dir, base: strings.TrimSuffix(base, "/"), quarantineDir: quarantineDir}
}

// QuarantineDir reports where hidden pins' files are moved.
func (l *Local) QuarantineDir() string {
	return l.quarantineDir
}

func (l *Local) Save(data []byte, ext string) (string, error) {
	if err := os.MkdirAll(l.dir, 0o755); err != nil {
		return "", err
	}

	id, err := newFileID()
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s.%s", hex.EncodeToString(id[:]), ext)
	path := filepath.Join(l.dir, name)

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/uploads/%s", l.base, name), nil
}

// Delete removes the file for a URL previously returned by Save. It is a
// best-effort cleanup: only the basename is used (so any base-URL prefix is
// safe) and missing files are not an error.
func (l *Local) Delete(url string) error {
	name := filepath.Base(url)
	if name == "." || name == "/" || name == "" {
		return nil
	}
	// Prevent path traversal: reject names containing path separators
	// or parent-directory references after cleaning.
	clean := filepath.Clean(name)
	if clean != name || strings.Contains(name, "..") {
		return nil
	}
	err := os.Remove(filepath.Join(l.dir, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (l *Local) Dir() string {
	return l.dir
}
