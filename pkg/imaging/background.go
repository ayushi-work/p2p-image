package imaging

import (
	"image"
	"image/color"
	"math"
)

// BackgroundRemovalOptions contains parameters for background removal
type BackgroundRemovalOptions struct {
	Algorithm    string  // "grabcut" or "simple"
	Threshold    float64 // for simple thresholding (0-1)
	Iterations   int     // for iterative algorithms
	ColorSigma   float64 // color similarity sigma for edge-preserving smoothing
	SmoothRadius int     // radius for alpha smoothing
	Feather      float64 // additional softening (0-1)
}

// RemoveBackground removes background from an image
// Returns RGBA image with alpha channel
func RemoveBackground(img image.Image, opts BackgroundRemovalOptions) (image.Image, error) {
	if opts.Algorithm == "" {
		opts.Algorithm = "simple"
	}
	if opts.Threshold == 0 {
		opts.Threshold = 0.5
	}
	if opts.Iterations == 0 {
		opts.Iterations = 5
	}
	if opts.ColorSigma == 0 {
		opts.ColorSigma = 0.12
	}
	if opts.SmoothRadius == 0 {
		opts.SmoothRadius = 3
	}
	if opts.Feather < 0 {
		opts.Feather = 0
	}

	// For production, this would use GrabCut or ML-based segmentation
	// Here we implement a simplified edge-aware background removal
	// that demonstrates the concept while maintaining edge quality

	switch opts.Algorithm {
	case "grabcut":
		return applyGrabCut(img, opts)
	default:
		return applySimpleRemoval(img, opts)
	}
}

// applySimpleRemoval uses color-based segmentation
// This is a simplified version - production would use more sophisticated algorithms
func applySimpleRemoval(img image.Image, opts BackgroundRemovalOptions) (image.Image, error) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	output := image.NewRGBA(bounds)

	// Estimate background by sampling border pixels
	bgColor := estimateBorderBackgroundColor(img)

	// Create alpha mask with edge-aware processing
	alphaMask := make([][]float64, height)
	for y := 0; y < height; y++ {
		alphaMask[y] = make([]float64, width)
	}

	// Calculate initial alpha based on color distance
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := img.At(bounds.Min.X+x, bounds.Min.Y+y)
			distance := colorDistance(c, bgColor)

			// Map distance to alpha using smoothstep for softer edges
			t := clamp(distance/opts.Threshold, 0, 1)
			// smoothstep: 3t^2 - 2t^3
			smooth := t * t * (3 - 2*t)
			alphaMask[y][x] = smooth
		}
	}

	// Apply edge-preserving smoothing to alpha mask using color similarity
	alphaMask = smoothAlphaMaskWithImage(alphaMask, img, width, height, opts.SmoothRadius, opts.ColorSigma)

	// Generate output with alpha channel
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			alpha := uint8(alphaMask[y][x] * 255)

			output.Set(bounds.Min.X+x, bounds.Min.Y+y, color.RGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
				A: alpha,
			})
		}
	}

	return output, nil
}

// applyGrabCut implements simplified GrabCut-like algorithm
// Production version would use proper graph-cut optimization
func applyGrabCut(img image.Image, opts BackgroundRemovalOptions) (image.Image, error) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	output := image.NewRGBA(bounds)

	// Initialize trimap: 0=background, 1=unknown, 2=foreground
	// Assume center region is foreground, edges are background
	trimap := make([][]int, height)
	for y := 0; y < height; y++ {
		trimap[y] = make([]int, width)
		for x := 0; x < width; x++ {
			// Simple heuristic: center 60% is likely foreground
			centerX := float64(width) / 2
			centerY := float64(height) / 2
			distX := math.Abs(float64(x)-centerX) / centerX
			distY := math.Abs(float64(y)-centerY) / centerY

			if distX < 0.6 && distY < 0.6 {
				trimap[y][x] = 2 // foreground
			} else if distX > 0.9 || distY > 0.9 {
				trimap[y][x] = 0 // background
			} else {
				trimap[y][x] = 1 // unknown
			}
		}
	}

	// Iterative refinement
	for iter := 0; iter < opts.Iterations; iter++ {
		trimap = refineTrimap(img, trimap, width, height, estimateBorderBackgroundColor(img))
	}

	// Convert trimap to alpha mask
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()

			var alpha uint8
			switch trimap[y][x] {
			case 0:
				alpha = 0 // background
			case 2:
				alpha = 255 // foreground
			default:
				alpha = 128 // unknown (semi-transparent)
			}

			output.Set(bounds.Min.X+x, bounds.Min.Y+y, color.RGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
				A: alpha,
			})
		}
	}

	return output, nil
}

// estimateBackgroundColor samples corner pixels
func estimateBackgroundColor(img image.Image) color.Color {
	bounds := img.Bounds()

	// Sample corners
	tl := img.At(bounds.Min.X, bounds.Min.Y)
	tr := img.At(bounds.Max.X-1, bounds.Min.Y)
	bl := img.At(bounds.Min.X, bounds.Max.Y-1)
	br := img.At(bounds.Max.X-1, bounds.Max.Y-1)

	// Average corner colors
	r1, g1, b1, _ := tl.RGBA()
	r2, g2, b2, _ := tr.RGBA()
	r3, g3, b3, _ := bl.RGBA()
	r4, g4, b4, _ := br.RGBA()

	avgR := (r1 + r2 + r3 + r4) / 4
	avgG := (g1 + g2 + g3 + g4) / 4
	avgB := (b1 + b2 + b3 + b4) / 4

	return color.RGBA{
		R: uint8(avgR >> 8),
		G: uint8(avgG >> 8),
		B: uint8(avgB >> 8),
		A: 255,
	}
}

// estimateBorderBackgroundColor samples border pixels and returns their average color
func estimateBorderBackgroundColor(img image.Image) color.Color {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	var rSum, gSum, bSum uint64
	var count uint64

	stepX := 1
	stepY := 1
	if width > 200 {
		stepX = width / 100
	}
	if height > 200 {
		stepY = height / 100
	}

	// Sample top and bottom rows and left/right columns
	for x := bounds.Min.X; x < bounds.Max.X; x += stepX {
		r, g, b, _ := img.At(x, bounds.Min.Y).RGBA()
		rSum += uint64(r)
		gSum += uint64(g)
		bSum += uint64(b)
		count++

		r, g, b, _ = img.At(x, bounds.Max.Y-1).RGBA()
		rSum += uint64(r)
		gSum += uint64(g)
		bSum += uint64(b)
		count++
	}
	for y := bounds.Min.Y; y < bounds.Max.Y; y += stepY {
		r, g, b, _ := img.At(bounds.Min.X, y).RGBA()
		rSum += uint64(r)
		gSum += uint64(g)
		bSum += uint64(b)
		count++

		r, g, b, _ = img.At(bounds.Max.X-1, y).RGBA()
		rSum += uint64(r)
		gSum += uint64(g)
		bSum += uint64(b)
		count++
	}

	if count == 0 {
		return estimateBackgroundColor(img)
	}

	avgR := rSum / count
	avgG := gSum / count
	avgB := bSum / count

	return color.RGBA{
		R: uint8(avgR >> 8),
		G: uint8(avgG >> 8),
		B: uint8(avgB >> 8),
		A: 255,
	}
}

// colorDistance calculates Euclidean distance in RGB space
func colorDistance(c1, c2 color.Color) float64 {
	r1, g1, b1, _ := c1.RGBA()
	r2, g2, b2, _ := c2.RGBA()

	dr := float64(r1) - float64(r2)
	dg := float64(g1) - float64(g2)
	db := float64(b1) - float64(b2)

	return math.Sqrt(dr*dr+dg*dg+db*db) / 65535.0
}

// smoothAlphaMask applies bilateral-like filtering to preserve edges
// smoothAlphaMaskWithImage performs edge-aware smoothing using color similarity
func smoothAlphaMaskWithImage(mask [][]float64, img image.Image, width, height, radius int, colorSigma float64) [][]float64 {
	result := make([][]float64, height)
	for y := 0; y < height; y++ {
		result[y] = make([]float64, width)
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			sum := 0.0
			count := 0.0
			centerVal := mask[y][x]
			centerColor := img.At(x+img.Bounds().Min.X, y+img.Bounds().Min.Y)

			for dy := -radius; dy <= radius; dy++ {
				for dx := -radius; dx <= radius; dx++ {
					ny := y + dy
					nx := x + dx

					if ny >= 0 && ny < height && nx >= 0 && nx < width {
						val := mask[ny][nx]
						neighborColor := img.At(nx+img.Bounds().Min.X, ny+img.Bounds().Min.Y)
						// weight by color similarity and mask similarity
						colorW := math.Exp(-colorDistance(centerColor, neighborColor) / colorSigma)
						maskW := math.Exp(-math.Abs(val-centerVal) * 8.0)
						weight := colorW * maskW
						sum += val * weight
						count += weight
					}
				}
			}

			if count > 0 {
				result[y][x] = sum / count
			} else {
				result[y][x] = mask[y][x]
			}
		}
	}

	return result
}

// refineTrimap refines the trimap using neighbor information
func refineTrimap(img image.Image, trimap [][]int, width, height int, bgColor color.Color) [][]int {
	bounds := img.Bounds()
	result := make([][]int, height)
	for y := 0; y < height; y++ {
		result[y] = make([]int, width)
		copy(result[y], trimap[y])
	}

	// Refine unknown pixels based on neighbors
	for y := 1; y < height-1; y++ {
		for x := 1; x < width-1; x++ {
			if trimap[y][x] != 1 {
				continue // only refine unknown pixels
			}

			// Count foreground and background neighbors
			fgCount := 0
			bgCount := 0

			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					neighbor := trimap[y+dy][x+dx]
					if neighbor == 2 {
						fgCount++
					} else if neighbor == 0 {
						bgCount++
					}
				}
			}

			// Decide based on majority
			if fgCount > bgCount*2 {
				result[y][x] = 2
			} else if bgCount > fgCount*2 {
				result[y][x] = 0
			}

			// Also consider color similarity to background
			centerColor := img.At(bounds.Min.X+x, bounds.Min.Y+y)
			if colorDistance(centerColor, bgColor) < 0.18 {
				result[y][x] = 0
			}
		}
	}

	return result
}

// smoothstep helper (keeps mapping smooth)
func smoothstep(x float64) float64 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 1
	}
	return x * x * (3 - 2*x)
}
