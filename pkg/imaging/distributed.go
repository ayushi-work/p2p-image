package imaging

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
)

// DistributedProcessingStrategy determines when to use distributed processing
type DistributedProcessingStrategy struct {
	// Size thresholds in pixels (width * height)
	DitherThreshold    int // Above this, use distributed dithering
	BGRemovalThreshold int // Above this, use distributed background removal

	// Overlap in pixels for tile boundaries
	DitherOverlap    int // For error diffusion continuity
	BGRemovalOverlap int // For edge blending
}

// DefaultStrategy returns recommended thresholds
func DefaultStrategy() DistributedProcessingStrategy {
	return DistributedProcessingStrategy{
		DitherThreshold:    2_000_000, // 2MP (e.g., 1600x1250)
		BGRemovalThreshold: 5_000_000, // 5MP (e.g., 2500x2000)
		DitherOverlap:      32,        // pixels
		BGRemovalOverlap:   64,        // pixels
	}
}

// ShouldDistribute determines if image should be processed distributedly
func (s DistributedProcessingStrategy) ShouldDistribute(img image.Image, operation string) bool {
	bounds := img.Bounds()
	pixels := bounds.Dx() * bounds.Dy()

	switch operation {
	case "dither":
		return pixels > s.DitherThreshold
	case "remove_bg":
		return pixels > s.BGRemovalThreshold
	default:
		return false
	}
}

// ImageTile represents a portion of an image with overlap
type ImageTile struct {
	Image   image.Image
	X, Y    int // Position in original image
	Width   int // Width without overlap
	Height  int // Height without overlap
	Overlap int // Overlap size on each edge
	TileID  int // Unique identifier
}

// TileImage divides an image into tiles for distributed processing
func TileImage(img image.Image, numTiles int, overlap int) []ImageTile {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Calculate grid dimensions (try to make tiles roughly square)
	cols := int(math.Ceil(math.Sqrt(float64(numTiles))))
	rows := int(math.Ceil(float64(numTiles) / float64(cols)))

	tileWidth := width / cols
	tileHeight := height / rows

	tiles := make([]ImageTile, 0, numTiles)
	tileID := 0

	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			if tileID >= numTiles {
				break
			}

			// Calculate tile boundaries with overlap
			x := col * tileWidth
			y := row * tileHeight

			// Extend boundaries for overlap
			x1 := max(0, x-overlap)
			y1 := max(0, y-overlap)
			x2 := min(width, x+tileWidth+overlap)
			y2 := min(height, y+tileHeight+overlap)

			// Extract sub-image
			tileRect := image.Rect(
				bounds.Min.X+x1,
				bounds.Min.Y+y1,
				bounds.Min.X+x2,
				bounds.Min.Y+y2,
			)

			tileImg := image.NewRGBA(image.Rect(0, 0, x2-x1, y2-y1))
			draw.Draw(tileImg, tileImg.Bounds(), img, tileRect.Min, draw.Src)

			tiles = append(tiles, ImageTile{
				Image:   tileImg,
				X:       x,
				Y:       y,
				Width:   min(tileWidth, width-x),
				Height:  min(tileHeight, height-y),
				Overlap: overlap,
				TileID:  tileID,
			})

			tileID++
		}
	}

	return tiles
}

// ReassembleTiles combines processed tiles back into a single image
func ReassembleTiles(tiles []ImageTile, originalWidth, originalHeight int, operation string) (image.Image, error) {
	if len(tiles) == 0 {
		return nil, fmt.Errorf("no tiles to reassemble")
	}

	// Create output image
	output := image.NewRGBA(image.Rect(0, 0, originalWidth, originalHeight))

	// Blend tiles with overlap handling
	for _, tile := range tiles {
		blendTile(output, tile, operation)
	}

	return output, nil
}

// blendTile blends a tile into the output image with overlap handling
func blendTile(output *image.RGBA, tile ImageTile, operation string) {
	tileBounds := tile.Image.Bounds()
	overlap := tile.Overlap

	// Process each pixel in the tile
	for ty := 0; ty < tileBounds.Dy(); ty++ {
		for tx := 0; tx < tileBounds.Dx(); tx++ {
			// Calculate position in output image
			outX := tile.X + tx - overlap
			outY := tile.Y + ty - overlap

			// Skip if outside output bounds
			if outX < 0 || outX >= output.Bounds().Dx() || outY < 0 || outY >= output.Bounds().Dy() {
				continue
			}

			// Calculate blend weight based on distance from tile edge
			weight := calculateBlendWeight(tx, ty, tileBounds.Dx(), tileBounds.Dy(), overlap)

			// Get colors
			tileColor := tile.Image.At(tileBounds.Min.X+tx, tileBounds.Min.Y+ty)
			existingColor := output.At(outX, outY)

			// Blend colors
			blended := blendColors(existingColor, tileColor, weight)
			output.Set(outX, outY, blended)
		}
	}
}

// calculateBlendWeight returns weight for blending (0-1)
// Weight is 1.0 in center, fades to 0.5 at edges within overlap region
func calculateBlendWeight(x, y, width, height, overlap int) float64 {
	if overlap == 0 {
		return 1.0
	}

	// Distance from each edge
	distLeft := float64(x)
	distRight := float64(width - x - 1)
	distTop := float64(y)
	distBottom := float64(height - y - 1)

	// Minimum distance to any edge
	minDist := math.Min(math.Min(distLeft, distRight), math.Min(distTop, distBottom))

	// If we're beyond overlap region, full weight
	if minDist >= float64(overlap) {
		return 1.0
	}

	// Linear fade in overlap region: 0.5 at edge, 1.0 at overlap boundary
	return 0.5 + 0.5*(minDist/float64(overlap))
}

// blendColors blends two colors with given weight
func blendColors(c1, c2 color.Color, weight float64) color.Color {
	r1, g1, b1, a1 := c1.RGBA()
	r2, g2, b2, a2 := c2.RGBA()

	// If c1 is transparent (no existing pixel), use c2
	if a1 == 0 {
		return c2
	}

	// Blend
	w1 := 1.0 - weight
	w2 := weight

	r := uint8((float64(r1>>8)*w1 + float64(r2>>8)*w2))
	g := uint8((float64(g1>>8)*w1 + float64(g2>>8)*w2))
	b := uint8((float64(b1>>8)*w1 + float64(b2>>8)*w2))
	a := uint8((float64(a1>>8)*w1 + float64(a2>>8)*w2))

	return color.RGBA{R: r, G: g, B: b, A: a}
}

// DistributedProcessingPlan describes how to distribute work
type DistributedProcessingPlan struct {
	UseDistributed bool
	NumTiles       int
	Overlap        int
	Tiles          []ImageTile
}

// CreateProcessingPlan determines optimal processing strategy
func CreateProcessingPlan(img image.Image, operation string, numPeers int) DistributedProcessingPlan {
	strategy := DefaultStrategy()

	plan := DistributedProcessingPlan{
		UseDistributed: strategy.ShouldDistribute(img, operation),
		NumTiles:       numPeers,
	}

	if !plan.UseDistributed {
		return plan
	}

	// Set overlap based on operation
	switch operation {
	case "dither":
		plan.Overlap = strategy.DitherOverlap
	case "remove_bg":
		plan.Overlap = strategy.BGRemovalOverlap
	default:
		plan.Overlap = 32
	}

	// Create tiles
	plan.Tiles = TileImage(img, numPeers, plan.Overlap)

	return plan
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
