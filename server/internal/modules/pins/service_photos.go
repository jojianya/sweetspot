package pins

import (
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"

	"github.com/jojianya/sweetspot247-backend/internal/platform/imaging"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
)

type validatedFile struct {
	data  []byte
	thumb []byte
	ext   string
}

// processPhotos opens, validates, and re-encodes every uploaded photo.
// Failures are domain errors so the handler can map them to statuses:
// ErrPhotoTooLarge and ErrPhotoInvalid are client faults, ErrPhotoUnreadable
// is a server fault.
func processPhotos(files []*multipart.FileHeader) ([]validatedFile, error) {
	validated := make([]validatedFile, 0, len(files))
	for i, fh := range files {
		if fh.Size > maxPhotoSize {
			return nil, ErrPhotoTooLarge
		}

		src, err := fh.Open()
		if err != nil {
			return nil, ErrPhotoUnreadable
		}
		// Cap the read at max+1 so a lied-about FileHeader.Size cannot push
		// an unbounded body into memory; the length check below is authoritative.
		data, err := io.ReadAll(io.LimitReader(src, maxPhotoSize+1))
		src.Close()
		if err != nil {
			return nil, ErrPhotoUnreadable
		}
		if len(data) > maxPhotoSize {
			return nil, ErrPhotoTooLarge
		}

		if err := imaging.Validate(data); err != nil {
			return nil, &photoProblem{msg: fmt.Sprintf("photo %d: %s", i+1, err)}
		}

		proc, err := imaging.Process(data)
		if err != nil {
			return nil, &photoProblem{msg: fmt.Sprintf("photo %d: %s", i+1, err)}
		}

		validated = append(validated, validatedFile{data: proc.Full, thumb: proc.Thumb, ext: "webp"})
	}
	return validated, nil
}

// savePhotos writes the processed photos to storage. On any failure it
// removes every file it already wrote, so a failed create cannot orphan
// files on disk. Cleanup failures are logged for the operator sweep, not
// discarded. A save failure wraps ErrPhotoSave so the handler answers 500.
func savePhotos(store *storage.Local, validated []validatedFile) (photoURLs, thumbURLs []string, err error) {
	type stored struct{ full, thumb string }
	written := make([]stored, 0, len(validated))
	cleanup := func() {
		for _, s := range written {
			if derr := store.Delete(s.full); derr != nil {
				slog.Warn("pin photos cleanup: remove photo", "error", derr.Error(), "url", s.full)
			}
			if derr := store.Delete(s.thumb); derr != nil {
				slog.Warn("pin photos cleanup: remove thumbnail", "error", derr.Error(), "url", s.thumb)
			}
		}
	}

	photoURLs = make([]string, 0, len(validated))
	thumbURLs = make([]string, 0, len(validated))
	for _, vf := range validated {
		full, err := store.Save(vf.data, vf.ext)
		if err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("%w: save full photo: %v", ErrPhotoSave, err)
		}
		thumb, err := store.Save(vf.thumb, vf.ext)
		if err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("%w: save thumbnail: %v", ErrPhotoSave, err)
		}
		written = append(written, stored{full: full, thumb: thumb})
		photoURLs = append(photoURLs, full)
		thumbURLs = append(thumbURLs, thumb)
	}
	return photoURLs, thumbURLs, nil
}
