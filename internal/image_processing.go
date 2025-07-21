package internal

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"slices"
)


func DetectChessboard(img *image.RGBA) {
	// Convert image to grayscale
	grayscale := grayscale(img)
	grayscaleFile, _ := os.Create("grayscale.png")
	defer grayscaleFile.Close()
	png.Encode(grayscaleFile, grayscale)

	// Perform Canny Edge Detection
	canny := canny(grayscale)
	cannyFile, _ := os.Create("canny.png")
	defer cannyFile.Close()
	png.Encode(cannyFile, canny)

	// Perform Hough Line Transform

	// Determine if line intersections match chessboard
}


func grayscale(img *image.RGBA) *image.Gray16 {
	result := image.NewGray16(img.Rect)

	for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
		for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()

			value := uint16(0.299 * float64(r) + 0.587 * float64(g) + 0.114 * float64(b))
			result.SetGray16(x, y, color.Gray16{ value })
		}
	}
	return result
}


type pixels[T any] struct {
	values []T
	width int
	height int
}
func toRawImage(img *image.Gray16) pixels[float64] {
	result := make([]float64, len(img.Pix) / 2)
	for i := 0; i < len(img.Pix); i += 2 {
		result[i / 2] = float64(uint16(img.Pix[i]) << 8 | uint16(img.Pix[i+1]))
	}
	return pixels[float64]{
		result,
		img.Rect.Dx(),
		img.Rect.Dy(),
	}
}
func toImage(rawImg *pixels[float64]) *image.Gray16 {
	result := image.NewGray16(image.Rectangle{
		image.Point{ 0, 0 },
		image.Point{ rawImg.width, rawImg.height },
	})

	for y := range rawImg.height {
		for x := range rawImg.width {
			index := getIndex(x, y, rawImg.width)

			result.SetGray16(x, y, color.Gray16{ uint16(rawImg.values[index]) })
		}
	}
	return result
}


// Helper function to get the "flat" index from x, y
func getIndex(x int, y int, width int) int {
	return y * width + x
}


type kernel struct {
	values []int
	size int
	invMultiplicand int
}
func applyKernel(img *pixels[float64], kernel *kernel) pixels[float64] {
	result := make([]float64, len(img.values))

	gap := kernel.size / 2

	for y := range img.height {
		for x := range img.width {
			index := getIndex(x, y, img.width)

			// Copy pixel if on edges
			if x < gap || y < gap || x >= (img.width - gap) || y >= (img.height - gap) {
				result[index] = img.values[index]
				continue
			}

			// Compute kernel
			i := 0
			var sum float64 = 0
			for dy := -gap; dy <= gap; dy++ {
				for dx := -gap; dx <= gap; dx++ {
					d_index := getIndex(x + dx, y + dy, img.width)
					sum += float64(kernel.values[i]) * img.values[d_index]
					i++
				}
			}
			result[index] = sum / float64(kernel.invMultiplicand)
		}
	}
	return pixels[float64]{
		result,
		img.width,
		img.height,
	}
}

type gradient struct {
	magnitude float64
	direction float64
}
// Combines horizontal and vertical gradients into single gradient with magnitude and direction
func combineGradients(gradientX *pixels[float64], gradientY *pixels[float64]) pixels[gradient] {
	result := make([]gradient, len(gradientX.values))

	for y := range gradientX.height {
		for x := range gradientX.width {
			index := getIndex(x, y, gradientX.width)

			gradX := gradientX.values[index]
			gradY := gradientY.values[index]

			result[index] = gradient{
				math.Sqrt(gradX * gradX + gradY * gradY),
				math.Atan2(gradY, gradX),
			}
		}
	}
	return pixels[gradient]{
		result,
		gradientX.width,
		gradientX.height,
	}
}


type dir int
const (
    horizontal dir = iota
    antiDiagonal // /
    vertical
    mainDiagonal // \
)
func snapDirection(direction float64) dir {
	// Normalize direction from -pi -> pi to 0 -> 2pi
	if direction < 0 {
		direction += 2 * math.Pi
	}

	// Get nearest multiple of pi/4
	nearest := int(math.Round(direction / (math.Pi / 4)))

	// Only keep top half of directions (0, pi/4, pi/2, 3pi/4)
	nearest %= 4

	return dir(nearest)
}


func thresholdLowerBound(gradients *pixels[gradient]) pixels[float64] {
	result := make([]float64, len(gradients.values))

	for y := range gradients.height {
		for x := range gradients.width {
			index := getIndex(x, y, gradients.width)
			grad := gradients.values[index]

			// Copy pixel if on edges
			if x < 1 || y < 1 || x >= (gradients.width - 1) || y >= (gradients.height - 1) {
				result[index] = grad.magnitude
				continue
			}

			// Snap to gradient direction
			snappedDirection := snapDirection(grad.direction)

			// Compare magnitude with other values in gradient direction
			var firstIndex int
			var secondIndex int
			switch snappedDirection {
			case horizontal:
				firstIndex = getIndex(x - 1, y, gradients.width)
				secondIndex = getIndex(x + 1, y, gradients.width)
			case antiDiagonal:
				firstIndex = getIndex(x + 1, y - 1, gradients.width)
				secondIndex = getIndex(x - 1, y + 1, gradients.width)
			case vertical:
				firstIndex = getIndex(x, y - 1, gradients.width)
				secondIndex = getIndex(x, y + 1, gradients.width)
			case mainDiagonal:
				firstIndex = getIndex(x - 1, y - 1, gradients.width)
				secondIndex = getIndex(x + 1, y + 1, gradients.width)
			}

			// Magnitude must be greatest to not be suppressed
			if grad.magnitude > gradients.values[firstIndex].magnitude && grad.magnitude > gradients.values[secondIndex].magnitude {
				result[index] = grad.magnitude
			} else {
				result[index] = 0
			}
		}
	}

	return pixels[float64]{
		result,
		gradients.width,
		gradients.height,
	}
}


type threshold struct {
	low float64
	high float64
}
func getThreshold(img *pixels[float64]) threshold {
	// Calculate the median intensity
	values := make([]float64, len(img.values))
	copy(values, img.values)
	slices.Sort(values)

	mid := len(values) / 2
	var median float64
	if len(values) % 2 == 0 {
		median = (values[mid] + values[mid + 1]) / 2
	} else {
		median = values[mid]
	}

	// Set low threshold to 2/3 * median and high threshold to 4/3 * median
	return threshold{
		2.0/3.0 * median,
		4.0/3.0 * median,
	}
}


func applyThreshold(img *pixels[float64], bounds *threshold) pixels[float64] {
	result := make([]float64, len(img.values))

	// Suppress pixels below lower threshold
	for index, value := range img.values {
		if value < bounds.low {
			result[index] = 0
		} else {
			result[index] = value
		}
	}

	return pixels[float64]{
		result,
		img.width,
		img.height,
	}
}


func floodfill(x int, y int, img *pixels[float64], bounds *threshold, keep *[]bool) {
	if x < 0 || x >= img.width || y < 0 || y >= img.height {
		return
	}

	index := getIndex(x, y, img.width)
	if (*keep)[index] {
		return
	}

	// Keep neighboring weak pixel
	if img.values[index] >= bounds.low && img.values[index] <= bounds.high {
		(*keep)[index] = true
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				floodfill(x + dx, y + dy, img, bounds, keep);
			}
		}
	}
}


func applyHysteresis(img *pixels[float64], bounds *threshold) pixels[float64] {
	// Perform blob analysis
	keep := make([]bool, len(img.values))
	for y := range img.height {
		for x := range img.width {
			index := getIndex(x, y, img.width)

			// Found a strong pixel, keep its neighboring weak pixels
			if img.values[index] > bounds.high {
				keep[index] = true
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						floodfill(x + dx, y + dy, img, bounds, &keep);
					}
				}
			}
		}
	}

	// Keep pixels marked by blob analysis
	result := make([]float64, len(img.values))
	for index, value := range img.values {
		if keep[index] {
			result[index] = value
		} else {
			result[index] = 0
		}
	}

	return pixels[float64]{
		result,
		img.width,
		img.height,
	}
}


func canny(img *image.Gray16) *image.Gray16 {
	// Get pixel values directly
	rawImg := toRawImage(img)

	// Apply Gaussian filter
	GAUSSIAN_KERNEL := kernel{
		[]int{
			2, 4, 5, 4, 2,
			4, 9, 12, 9, 4,
			5, 12, 15, 12, 5,
			4, 9, 12, 9, 4,
			2, 4, 5, 4, 2,
		},
		5,
		159,
	}

	gaussian := applyKernel(&rawImg, &GAUSSIAN_KERNEL)

	// Calculate horizontal and vertical gradients using Sobel filter
	SOBEL_KERNEL_X := kernel{
		[]int {
			-1, 0, 1,
			-2, 0, 2,
			-1, 0, 1,
		},
		3,
		1,
	}
	SOBEL_KERNEL_Y := kernel{
		[]int {
			-1, -2, -1,
			0, 0, 0,
			1, 2, 1,
		},
		3,
		1,
	}

	sobelX := applyKernel(&gaussian, &SOBEL_KERNEL_X)
	sobelY := applyKernel(&gaussian, &SOBEL_KERNEL_Y)

	// Combine into single gradient with magnitude and direction
	gradients := combineGradients(&sobelX, &sobelY)

	// Perform edge thinning using lower bound cut-off suppression
	thinnedGradients := thresholdLowerBound(&gradients)

	// Perform double thresholding
	// Calculate threshold
	bounds := getThreshold(&thinnedGradients)
	// Suppress values less than low threshold
	thresholded := applyThreshold(&thinnedGradients, &bounds)

	// Perform hysteresis
	result := applyHysteresis(&thresholded, &bounds)
	return toImage(&result)
}