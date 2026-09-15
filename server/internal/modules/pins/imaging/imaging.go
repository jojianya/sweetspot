package imaging

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/h2non/bimg"
)

const (
	maxPhotoWidth = 1600
	thumbSize     = 400
	webpQuality   = 80
)

type Result struct {
	Full  []byte
	Thumb []byte
}

func Validate(data []byte) error {
	switch http.DetectContentType(data) {
	case "image/jpeg", "image/png":
		return nil
	default:
		return errors.New("only jpg and png images are allowed")
	}
}

func Process(data []byte) (Result, error) {
	full, err := bimg.Resize(data, bimg.Options{
		Width:   maxPhotoWidth,
		Quality: webpQuality,
		Type:    bimg.WEBP,
	})
	if err != nil {
		return Result{}, fmt.Errorf("could not process image: %w", err)
	}

	thumb, err := bimg.Resize(full, bimg.Options{
		Width:   thumbSize,
		Height:  thumbSize,
		Crop:    true,
		Quality: webpQuality,
		Type:    bimg.WEBP,
	})
	if err != nil {
		return Result{}, fmt.Errorf("could not create thumbnail: %w", err)
	}

	return Result{Full: full, Thumb: thumb}, nil
}
