package engine

import (
	"image"
	"strings"

	"github.com/disintegration/imaging"
)

// GravityAnchor defines the crop alignment anchor.
type GravityAnchor string

const (
	GravityCenter     GravityAnchor = "center"
	GravityNorth      GravityAnchor = "north"
	GravitySouth      GravityAnchor = "south"
	GravityEast       GravityAnchor = "east"
	GravityWest       GravityAnchor = "west"
	GravityNorthWest  GravityAnchor = "northwest"
	GravityNorthEast  GravityAnchor = "northeast"
	GravitySouthWest  GravityAnchor = "southwest"
	GravitySouthEast  GravityAnchor = "southeast"
	GravitySmart      GravityAnchor = "smart"
	GravityFocalPoint GravityAnchor = "fp"
)

// ParseGravity parses gravity string safely.
func ParseGravity(s string) GravityAnchor {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "smart":
		return GravitySmart
	case "fp", "focal", "focalpoint":
		return GravityFocalPoint
	case "north", "top", "n":
		return GravityNorth
	case "south", "bottom", "s":
		return GravitySouth
	case "east", "right", "e":
		return GravityEast
	case "west", "left", "w":
		return GravityWest
	case "northwest", "nw", "topleft":
		return GravityNorthWest
	case "northeast", "ne", "topright":
		return GravityNorthEast
	case "southwest", "sw", "bottomleft":
		return GravitySouthWest
	case "southeast", "se", "bottomright":
		return GravitySouthEast
	case "center", "c", "":
		return GravityCenter
	default:
		return GravityCenter
	}
}

// ToImagingAnchor maps GravityAnchor to imaging.Anchor.
func (g GravityAnchor) ToImagingAnchor() imaging.Anchor {
	switch g {
	case GravityNorth:
		return imaging.Top
	case GravitySouth:
		return imaging.Bottom
	case GravityEast:
		return imaging.Right
	case GravityWest:
		return imaging.Left
	case GravityNorthWest:
		return imaging.TopLeft
	case GravityNorthEast:
		return imaging.TopRight
	case GravitySouthWest:
		return imaging.BottomLeft
	case GravitySouthEast:
		return imaging.BottomRight
	case GravityCenter:
		return imaging.Center
	default:
		return imaging.Center
	}
}

// CalculateFocalCrop calculates crop boundary centered around normalized focal coordinates (0.0 to 1.0).
func CalculateFocalCrop(srcW, srcH, targetW, targetH int, fpX, fpY float64) image.Rectangle {
	if fpX < 0 {
		fpX = 0
	}
	if fpX > 1.0 {
		fpX = 1.0
	}
	if fpY < 0 {
		fpY = 0
	}
	if fpY > 1.0 {
		fpY = 1.0
	}

	targetAspect := float64(targetW) / float64(targetH)
	srcAspect := float64(srcW) / float64(srcH)

	var cropW, cropH int
	if srcAspect > targetAspect {
		cropH = srcH
		cropW = int(float64(srcH) * targetAspect)
		if cropW > srcW {
			cropW = srcW
		}
	} else {
		cropW = srcW
		cropH = int(float64(srcW) / targetAspect)
		if cropH > srcH {
			cropH = srcH
		}
	}

	// Focal center in pixel coordinates
	focalPixX := int(fpX * float64(srcW))
	focalPixY := int(fpY * float64(srcH))

	// Top-left candidate
	x0 := focalPixX - cropW/2
	y0 := focalPixY - cropH/2

	// Clamp to image edges
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x0+cropW > srcW {
		x0 = srcW - cropW
	}
	if y0+cropH > srcH {
		y0 = srcH - cropH
	}

	return image.Rect(x0, y0, x0+cropW, y0+cropH)
}

// ApplyCropAndResize transforms an image according to target dimensions, fit mode, gravity, focal point, and resampler.
func ApplyCropAndResize(img image.Image, targetW, targetH int, fit FitMode, gravity GravityAnchor, fpX, fpY float64, filter ResampleFilter) image.Image {
	if targetW <= 0 && targetH <= 0 {
		return img
	}

	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	switch fit {
	case FitCover:
		if targetW <= 0 {
			targetW = srcW
		}
		if targetH <= 0 {
			targetH = srcH
		}

		if gravity == GravitySmart {
			cropRect := FindSmartCrop(img, targetW, targetH)
			cropped := imaging.Crop(img, cropRect)
			return imaging.Resize(cropped, targetW, targetH, filter.ToImagingFilter())
		}
		if gravity == GravityFocalPoint {
			cropRect := CalculateFocalCrop(srcW, srcH, targetW, targetH, fpX, fpY)
			cropped := imaging.Crop(img, cropRect)
			return imaging.Resize(cropped, targetW, targetH, filter.ToImagingFilter())
		}
		return imaging.Fill(img, targetW, targetH, gravity.ToImagingAnchor(), filter.ToImagingFilter())

	case FitCrop:
		if targetW <= 0 {
			targetW = srcW
		}
		if targetH <= 0 {
			targetH = srcH
		}

		if gravity == GravitySmart {
			cropRect := FindSmartCrop(img, targetW, targetH)
			return imaging.Crop(img, cropRect)
		}
		if gravity == GravityFocalPoint {
			cropRect := CalculateFocalCrop(srcW, srcH, targetW, targetH, fpX, fpY)
			return imaging.Crop(img, cropRect)
		}
		return imaging.CropAnchor(img, targetW, targetH, gravity.ToImagingAnchor())

	case FitContain:
		w, h := CalculateTargetSize(srcW, srcH, targetW, targetH, FitContain, 1.0)
		return imaging.Resize(img, w, h, filter.ToImagingFilter())

	case FitInside:
		w, h := CalculateTargetSize(srcW, srcH, targetW, targetH, FitInside, 1.0)
		return imaging.Resize(img, w, h, filter.ToImagingFilter())

	case FitOutside:
		w, h := CalculateTargetSize(srcW, srcH, targetW, targetH, FitOutside, 1.0)
		return imaging.Resize(img, w, h, filter.ToImagingFilter())

	case FitFill:
		w := targetW
		h := targetH
		if w <= 0 {
			w = srcW
		}
		if h <= 0 {
			h = srcH
		}
		return imaging.Resize(img, w, h, filter.ToImagingFilter())

	default:
		return imaging.Fill(img, targetW, targetH, gravity.ToImagingAnchor(), filter.ToImagingFilter())
	}
}
