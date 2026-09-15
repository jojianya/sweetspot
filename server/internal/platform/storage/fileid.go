package storage

import (
	"crypto/rand"
)

func newFileID() [16]byte {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return b
}
