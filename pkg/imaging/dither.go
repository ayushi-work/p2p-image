package imaging

import (
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
)

// DitherOptions contains parameters for dithering
type DitherOptions struct {
	Algorithm string // "floyd-steinberg" or "atkinson"
	Colors    int    // number of colors in palette (default: 2 for B&W)
}

// Dither applies Floyd-Steinberg or Atkinson dithering to an image
// Preserves original resolution and exports as PNG for maximum quality
func Dither(img image.Image, opts DitherOptions) (image.Image, error) {
	if opts.Algorithm == "" {
		opts.Algorithm = "floyd-steinberg"
	}
	if opts.Colors == 0 {
		opts.Colors = 2 // Black and white by default
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Create output image
	output := image.NewRGBA(bounds)

	// Convert to grayscale working buffer with error accumulation
	// Use float64 for precision during error diffusion
	gray := make([][]float64, height)
	for y := 0; y < height; y++ {
		gray[y] = make([]float64, width)
		for x := 0; x < width; x++ {
			r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			// Convert to grayscale using standard luminance formula
			// Work in linear space for better quality
			gray[y][x] = 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
		}
	}

	// Apply dithering algorithm
	switch opts.Algorithm {
	case "atkinson":
		applyAtkinson(gray, width, height, opts.Colors)
	default: // floyd-steinberg
		applyFloydSteinberg(gray, width, height, opts.Colors)
	}

	// Convert back to image
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			val := uint8(clamp(gray[y][x]/65535.0*255.0, 0, 255))
			output.Set(bounds.Min.X+x, bounds.Min.Y+y, color.RGBA{val, val, val, 255})
		}
	}

	return output, nil
}

// applyFloydSteinberg applies Floyd-Steinberg error diffusion
// Error distribution:
//
//	X   7/16
//
// 3/16 5/16 1/16
func applyFloydSteinberg(gray [][]float64, width, height, colors int) {
	levels := float64(colors - 1)
	maxVal := 65535.0

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			oldPixel := gray[y][x]

			// Quantize to nearest level
			newPixel := math.Round(oldPixel/maxVal*levels) * (maxVal / levels)
			gray[y][x] = newPixel

			// Calculate error
			err := oldPixel - newPixel

			// Distribute error to neighbors
			if x+1 < width {
				gray[y][x+1] += err * 7.0 / 16.0
			}
			if y+1 < height {
				if x > 0 {
					gray[y+1][x-1] += err * 3.0 / 16.0
				}
				gray[y+1][x] += err * 5.0 / 16.0
				if x+1 < width {
					gray[y+1][x+1] += err * 1.0 / 16.0
				}
			}
		}
	}
}

// applyAtkinson applies Atkinson dithering
// Error distribution (1/8 each):
//
//	X   1   1
//
// 1   1   1
//
//	1
func applyAtkinson(gray [][]float64, width, height, colors int) {
	levels := float64(colors - 1)
	maxVal := 65535.0

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			oldPixel := gray[y][x]

			// Quantize
			newPixel := math.Round(oldPixel/maxVal*levels) * (maxVal / levels)
			gray[y][x] = newPixel

			// Calculate error
			err := oldPixel - newPixel

			// Distribute error (Atkinson uses 1/8, only 6/8 distributed)
			factor := 1.0 / 8.0

			if x+1 < width {
				gray[y][x+1] += err * factor
			}
			if x+2 < width {
				gray[y][x+2] += err * factor
			}
			if y+1 < height {
				if x > 0 {
					gray[y+1][x-1] += err * factor
				}
				gray[y+1][x] += err * factor
				if x+1 < width {
					gray[y+1][x+1] += err * factor
				}
			}
			if y+2 < height {
				gray[y+2][x] += err * factor
			}
		}
	}
}

// EncodePNG encodes image as PNG with maximum quality
func EncodePNG(w io.Writer, img image.Image) error {
	encoder := png.Encoder{
		CompressionLevel: png.BestCompression,
	}
	return encoder.Encode(w, img)
}

func clamp(val, min, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}
