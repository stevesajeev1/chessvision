package internal

import (
	"image"

	"github.com/kbinani/screenshot"
)


func GetScreenshots(images *[]*image.RGBA) {
	n := screenshot.NumActiveDisplays()
	for i := range n {
		bounds := screenshot.GetDisplayBounds(i)
		img, err := screenshot.CaptureRect(bounds)
		if err != nil {
			panic(err)
		}
		*images = append(*images, img)
	}
}