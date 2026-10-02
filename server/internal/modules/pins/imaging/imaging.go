package imaging

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

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

// resizeFn is the bimg entry point used by Process and Avatar. It is a
// package-level variable (not a parameter) so tests can substitute a
// panicking stub without changing production call sites; production code
// always uses bimg.Resize.
var resizeFn = bimg.Resize

func Process(data []byte) (result Result, err error) {
	processSem <- struct{}{}
	defer func() {
		<-processSem
		if r := recover(); r != nil {
			stack := debug.Stack()
			slog.Error("image processing panicked", "op", "process", "panic", fmt.Sprint(r), "stack", string(stack))
			result = Result{}
			err = fmt.Errorf("could not process image: internal processing failure")
		}
	}()
	full, err := resizeFn(data, bimg.Options{
		Width:   maxPhotoWidth,
		Quality: webpQuality,
		Type:    bimg.WEBP,
	})
	if err != nil {
		return Result{}, fmt.Errorf("could not process image: %w", err)
	}

	thumb, err := resizeFn(full, bimg.Options{
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
func Avatar(data []byte) (img []byte, err error) {
	processSem <- struct{}{}
	defer func() {
		<-processSem
		if r := recover(); r != nil {
			stack := debug.Stack()
			slog.Error("image processing panicked", "op", "avatar", "panic", fmt.Sprint(r), "stack", string(stack))
			img = nil
			err = fmt.Errorf("could not process avatar: internal processing failure")
		}
	}()
	img, err = resizeFn(data, bimg.Options{
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
