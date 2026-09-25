package imaging

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/h2non/bimg"
)

const (
	maxPhotoWidth        = 1600
	thumbSize            = 400
	avatarSize           = 256
	webpQuality          = 80
	maxPhotoDim          = 8000
	maxConcurrentProcess = 2
)

// processSem bounds the number of concurrent libvips operations so a burst of
// uploads cannot exhaust memory during decode/resize.
var processSem = make(chan struct{}, maxConcurrentProcess)

type Result struct {
	Full  []byte
	Thumb []byte
}

func Validate(data []byte) error {
	switch http.DetectContentType(data) {
	case "image/jpeg", "image/png":
	default:
		return errors.New("only jpg and png images are allowed")
	}

	meta, err := bimg.Metadata(data)
	if err != nil {
		return errors.New("could not read image metadata")
	}
	if meta.Size.Width > maxPhotoDim || meta.Size.Height > maxPhotoDim {
		return fmt.Errorf("image dimensions exceed %dx%d", maxPhotoDim, maxPhotoDim)
	}
	return nil
}

func Process(data []byte) (Result, error) {
	processSem <- struct{}{}
	defer func() {
		recover()
		<-processSem
	}()
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

// Avatar center-crops and re-encodes an uploaded image into a square webp at
// avatarSize. Runs under the same semaphore as Process.
func Avatar(data []byte) ([]byte, error) {
	processSem <- struct{}{}
	defer func() {
		recover()
		<-processSem
	}()
	img, err := bimg.Resize(data, bimg.Options{
		Width:   avatarSize,
		Height:  avatarSize,
		Crop:    true,
		Quality: webpQuality,
		Type:    bimg.WEBP,
	})
	if err != nil {
		return nil, fmt.Errorf("could not process avatar: %w", err)
	}
	return img, nil
}
