package imaging

import (
	"bytes"
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/h2non/bimg"
)

var exifJPEGMarker = []byte("Exif\x00\x00")
var exifChunkMarker = []byte("EXIF")

// NeedsStrip reports whether raw image bytes carry an EXIF block (JPEG APP1
// or a WebP EXIF chunk). It is a pure byte scan: it never decodes, never
// panics, and never logs image content.
func NeedsStrip(data []byte) bool {
	return bytes.Contains(data, exifJPEGMarker) || bytes.Contains(data, exifChunkMarker)
}

// StripImage re-encodes an image in its own container with all EXIF metadata
// removed, preserving pixels and dimensions. It is idempotent: running it on
// already-stripped bytes yields bytes that NeedsStrip rejects.
func StripImage(data []byte) (out []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			slog.Error("image strip panicked", "panic", fmt.Sprint(r), "stack", string(stack))
			out = nil
			err = fmt.Errorf("could not strip metadata: internal processing failure")
		}
	}()

	imgType := bimg.DetermineImageType(data)
	switch imgType {
	case bimg.JPEG, bimg.PNG, bimg.WEBP:
	default:
		return nil, fmt.Errorf("unsupported image type for stripping")
	}

	out, err = bimg.NewImage(data).Process(bimg.Options{
		Type:          imgType,
		Quality:       webpQuality,
		StripMetadata: true,
	})
	if err != nil {
		return nil, fmt.Errorf("could not strip metadata: %w", err)
	}
	return out, nil
}
