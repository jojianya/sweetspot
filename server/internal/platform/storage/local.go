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
}

func NewLocal(dir, base string) *Local {
	return &Local{dir: dir, base: strings.TrimSuffix(base, "/")}
}

func (l *Local) Save(data []byte, ext string) (string, error) {
	if err := os.MkdirAll(l.dir, 0o755); err != nil {
		return "", err
	}

	id := newFileID()
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
	err := os.Remove(filepath.Join(l.dir, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (l *Local) Dir() string {
	return l.dir
}
