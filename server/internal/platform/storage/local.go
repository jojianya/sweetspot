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

func (l *Local) Dir() string {
	return l.dir
}
