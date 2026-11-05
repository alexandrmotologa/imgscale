package engine

import (
	"image"

	"github.com/disintegration/imaging"
)

// FilterOptions holds post-processing visual filter adjustments.
type FilterOptions struct {
	Blur       float64 // 0 to 100
	Sharpen    float64 // 0 to 100
	Grayscale  bool
	Rotate     int     // 0, 90, 180, 270 degrees
	Brightness float64 // -100 to 100 (0 = no change)
	Contrast   float64 // -100 to 100 (0 = no change)
}

// ApplyFilters applies post-processing filters to an image.
func ApplyFilters(img image.Image, opts FilterOptions) image.Image {
	out := img

	// 1. Rotation
	switch opts.Rotate {
	case 90:
		out = imaging.Rotate270(out) // Note: standard clockwise vs counter-clockwise convention
	case 180:
		out = imaging.Rotate180(out)
	case 270:
		out = imaging.Rotate90(out)
	}

	// 2. Grayscale
	if opts.Grayscale {
		out = imaging.Grayscale(out)
	}

	// 3. Brightness adjustment
	if opts.Brightness != 0 {
		// Clamp to [-100, 100]
		b := opts.Brightness
		if b < -100 {
			b = -100
		}
		if b > 100 {
			b = 100
		}
		out = imaging.AdjustBrightness(out, b)
	}

	// 4. Contrast adjustment
	if opts.Contrast != 0 {
		c := opts.Contrast
		if c < -100 {
			c = -100
		}
		if c > 100 {
			c = 100
		}
		out = imaging.AdjustContrast(out, c)
	}

	// 5. Blur
	if opts.Blur > 0 {
		sigma := opts.Blur
		if sigma > 100 {
			sigma = 100
		}
		out = imaging.Blur(out, sigma)
	}

	// 6. Sharpen
	if opts.Sharpen > 0 {
		intensity := opts.Sharpen
		if intensity > 100 {
			intensity = 100
		}
		out = imaging.Sharpen(out, intensity)
	}

	return out
}
