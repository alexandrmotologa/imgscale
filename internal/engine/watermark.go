package engine

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/disintegration/imaging"
)

// WatermarkOptions defines overlay placement and alpha blending settings.
type WatermarkOptions struct {
	Watermark image.Image
	Gravity   GravityAnchor
	Opacity   float64 // 0.0 to 1.0
	Scale     float64 // 1 to 100 percentage of base image width
	Padding   int     // Distance from boundary in pixels
}

// ApplyWatermark composites a watermark image onto the base image.
func ApplyWatermark(base image.Image, opts WatermarkOptions) image.Image {
	if opts.Watermark == nil {
		return base
	}

	baseBounds := base.Bounds()
	baseW := baseBounds.Dx()
	baseH := baseBounds.Dy()

	// Default scale: 20% of base width
	scale := opts.Scale
	if scale <= 0 {
		scale = 20.0
	}
	if scale > 100 {
		scale = 100.0
	}

	targetWmW := int(float64(baseW) * (scale / 100.0))
	if targetWmW < 10 {
		targetWmW = 10
	}

	// Resize watermark preserving aspect ratio
	wm := imaging.Resize(opts.Watermark, targetWmW, 0, imaging.Lanczos)
	wmBounds := wm.Bounds()
	wmW := wmBounds.Dx()
	wmH := wmBounds.Dy()

	padding := opts.Padding
	if padding < 0 {
		padding = 16
	}

	// Calculate target (X, Y) top-left placement
	var posX, posY int

	switch opts.Gravity {
	case GravityNorthWest:
		posX = padding
		posY = padding
	case GravityNorth:
		posX = (baseW - wmW) / 2
		posY = padding
	case GravityNorthEast:
		posX = baseW - wmW - padding
		posY = padding
	case GravityWest:
		posX = padding
		posY = (baseH - wmH) / 2
	case GravityCenter:
		posX = (baseW - wmW) / 2
		posY = (baseH - wmH) / 2
	case GravityEast:
		posX = baseW - wmW - padding
		posY = (baseH - wmH) / 2
	case GravitySouthWest:
		posX = padding
		posY = baseH - wmH - padding
	case GravitySouth:
		posX = (baseW - wmW) / 2
		posY = baseH - wmH - padding
	case GravitySouthEast, "":
		// Default to SouthEast (bottom right corner)
		posX = baseW - wmW - padding
		posY = baseH - wmH - padding
	default:
		posX = baseW - wmW - padding
		posY = baseH - wmH - padding
	}

	// Clamp positions to stay inside image
	if posX < 0 {
		posX = 0
	}
	if posY < 0 {
		posY = 0
	}

	// Alpha opacity factor
	opacity := opts.Opacity
	if opacity <= 0 {
		opacity = 0.85
	}
	if opacity > 1.0 {
		opacity = 1.0
	}

	// Composite onto RGBA canvas
	canvas := image.NewRGBA(baseBounds)
	draw.Draw(canvas, baseBounds, base, baseBounds.Min, draw.Src)

	// Apply alpha-adjusted watermark
	for y := 0; y < wmH; y++ {
		targetY := baseBounds.Min.Y + posY + y
		if targetY >= baseBounds.Max.Y {
			break
		}

		for x := 0; x < wmW; x++ {
			targetX := baseBounds.Min.X + posX + x
			if targetX >= baseBounds.Max.X {
				break
			}

			wmC := color.NRGBAModel.Convert(wm.At(wmBounds.Min.X+x, wmBounds.Min.Y+y)).(color.NRGBA)
			if wmC.A == 0 {
				continue
			}

			// Modulate alpha by opacity
			alpha := float64(wmC.A) / 255.0 * opacity

			baseC := canvas.RGBAAt(targetX, targetY)

			// Standard "over" alpha blending
			r := uint8(float64(wmC.R)*alpha + float64(baseC.R)*(1.0-alpha))
			g := uint8(float64(wmC.G)*alpha + float64(baseC.G)*(1.0-alpha))
			b := uint8(float64(wmC.B)*alpha + float64(baseC.B)*(1.0-alpha))

			canvas.SetRGBA(targetX, targetY, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	return canvas
}
