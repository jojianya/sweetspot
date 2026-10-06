package storage

import (
	"crypto/rand"
	"fmt"
)

func newFileID() ([16]byte, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return b, fmt.Errorf("generating file id: %w", err)
	}
	return b, nil
}
