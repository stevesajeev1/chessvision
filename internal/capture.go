package internal

import (
	"image"

	"github.com/kbinani/screenshot"
)

func GetScreenshots() []*image.RGBA {
	var images []*image.RGBA

	n := screenshot.NumActiveDisplays()
	for i := range n {
		bounds := screenshot.GetDisplayBounds(i)
		image, err := screenshot.CaptureRect(bounds)
		if err != nil {
			panic(err)
		}
		images = append(images, image)
	}
	return images
}