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
	Algorithm string  // "floyd-steinberg" or "atkinson"
	Colors    int     // number of colors in palette (default: 2 for B&W)
	Strength  float64 // blend strength of dithered result (0-1)
	Gamma     float64 // gamma correction to apply before/after dithering
	Color     bool    // if true, perform per-channel color dithering
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
	if opts.Strength == 0 {
		opts.Strength = 1.0
	}
	if opts.Gamma == 0 {
		opts.Gamma = 1.0
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Create output image
	output := image.NewRGBA(bounds)

	maxVal := 65535.0

	if opts.Color {
		// Per-channel buffers
		rbuf := make([][]float64, height)
		gbuf := make([][]float64, height)
		bbuf := make([][]float64, height)
		for y := 0; y < height; y++ {
			rbuf[y] = make([]float64, width)
			gbuf[y] = make([]float64, width)
			bbuf[y] = make([]float64, width)
			for x := 0; x < width; x++ {
				r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
				// apply gamma correction
				rbuf[y][x] = math.Pow(float64(r)/maxVal, opts.Gamma) * maxVal
				gbuf[y][x] = math.Pow(float64(g)/maxVal, opts.Gamma) * maxVal
				bbuf[y][x] = math.Pow(float64(b)/maxVal, opts.Gamma) * maxVal
			}
		}

		// Apply chosen algorithm per buffer
		switch opts.Algorithm {
		case "atkinson":
			applyAtkinsonOnBuffer(rbuf, width, height, opts.Colors)
			applyAtkinsonOnBuffer(gbuf, width, height, opts.Colors)
			applyAtkinsonOnBuffer(bbuf, width, height, opts.Colors)
		default:
			applyFloydOnBuffer(rbuf, width, height, opts.Colors)
			applyFloydOnBuffer(gbuf, width, height, opts.Colors)
			applyFloydOnBuffer(bbuf, width, height, opts.Colors)
		}

		// Compose output with blending by Strength
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				or, og, ob, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
				dr := uint8(clamp(rbuf[y][x]/maxVal*255.0, 0, 255))
				dg := uint8(clamp(gbuf[y][x]/maxVal*255.0, 0, 255))
				db := uint8(clamp(bbuf[y][x]/maxVal*255.0, 0, 255))

				finalR := uint8((1-opts.Strength)*float64(or>>8) + opts.Strength*float64(dr))
				finalG := uint8((1-opts.Strength)*float64(og>>8) + opts.Strength*float64(dg))
				finalB := uint8((1-opts.Strength)*float64(ob>>8) + opts.Strength*float64(db))

				output.Set(bounds.Min.X+x, bounds.Min.Y+y, color.RGBA{finalR, finalG, finalB, 255})
			}
		}

	} else {
		// Grayscale path (original behavior) with gamma and strength
		gray := make([][]float64, height)
		for y := 0; y < height; y++ {
			gray[y] = make([]float64, width)
			for x := 0; x < width; x++ {
				r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
				lum := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
				gray[y][x] = math.Pow(lum/maxVal, opts.Gamma) * maxVal
			}
		}

		switch opts.Algorithm {
		case "atkinson":
			applyAtkinson(gray, width, height, opts.Colors)
		default:
			applyFloydSteinberg(gray, width, height, opts.Colors)
		}

		// Compose output blending with original grayscale based on Strength
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
				origGray := 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(b>>8)
				ditherVal := clamp(gray[y][x]/maxVal*255.0, 0, 255)
				final := uint8((1-opts.Strength)*origGray + opts.Strength*ditherVal)
				output.Set(bounds.Min.X+x, bounds.Min.Y+y, color.RGBA{final, final, final, 255})
			}
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

// applyFloydOnBuffer applies Floyd-Steinberg error diffusion to a single-channel buffer
func applyFloydOnBuffer(buf [][]float64, width, height, colors int) {
	levels := float64(colors - 1)
	maxVal := 65535.0

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			oldPixel := buf[y][x]

			newPixel := math.Round(oldPixel/maxVal*levels) * (maxVal / levels)
			buf[y][x] = newPixel

			err := oldPixel - newPixel

			if x+1 < width {
				buf[y][x+1] += err * 7.0 / 16.0
			}
			if y+1 < height {
				if x > 0 {
					buf[y+1][x-1] += err * 3.0 / 16.0
				}
				buf[y+1][x] += err * 5.0 / 16.0
				if x+1 < width {
					buf[y+1][x+1] += err * 1.0 / 16.0
				}
			}
		}
	}
}

// applyAtkinsonOnBuffer applies Atkinson dithering to a single-channel buffer
func applyAtkinsonOnBuffer(buf [][]float64, width, height, colors int) {
	levels := float64(colors - 1)
	maxVal := 65535.0

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			oldPixel := buf[y][x]

			newPixel := math.Round(oldPixel/maxVal*levels) * (maxVal / levels)
			buf[y][x] = newPixel

			err := oldPixel - newPixel
			factor := 1.0 / 8.0

			if x+1 < width {
				buf[y][x+1] += err * factor
			}
			if x+2 < width {
				buf[y][x+2] += err * factor
			}
			if y+1 < height {
				if x > 0 {
					buf[y+1][x-1] += err * factor
				}
				buf[y+1][x] += err * factor
				if x+1 < width {
					buf[y+1][x+1] += err * factor
				}
			}
			if y+2 < height {
				buf[y+2][x] += err * factor
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
