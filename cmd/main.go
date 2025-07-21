package main

import (
	"image"

	"github.com/stevesajeev1/chessvision/internal"
)


func main() {
	var images []*image.RGBA

	for {
		// Get screenshots
		images = images[:0]
		internal.GetScreenshots(&images)

		for _, image := range images {
			// Detect if a chessboard is on screen
			internal.DetectChessboard(image)

			// Use Object Detection to identify pieces and locations

			// Get evaluation of position from Stockfish

			// Display eval bar on screen
		}
		break
	}
}