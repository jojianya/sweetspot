package pins

import (
	"fmt"

	"github.com/h2non/bimg"
)

const (
	maxPhotoWidth = 1600
	thumbSize     = 400
	webpQuality   = 80
)

type processedImage struct {
	full  []byte
	thumb []byte
}

func processImage(data []byte) (processedImage, error) {
	full, err := bimg.Resize(data, bimg.Options{
		Width:   maxPhotoWidth,
		Quality: webpQuality,
		Type:    bimg.WEBP,
	})
	if err != nil {
		return processedImage{}, fmt.Errorf("could not process image: %w", err)
	}

	thumb, err := bimg.Resize(full, bimg.Options{
		Width:   thumbSize,
		Height:  thumbSize,
		Crop:    true,
		Quality: webpQuality,
		Type:    bimg.WEBP,
	})
	if err != nil {
		return processedImage{}, fmt.Errorf("could not create thumbnail: %w", err)
	}

	return processedImage{full: full, thumb: thumb}, nil
}