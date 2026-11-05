package engine

import (
	"image"
	"strings"

	"github.com/disintegration/imaging"
)

// GravityAnchor defines the crop alignment anchor.
type GravityAnchor string

const (
	GravityCenter GravityAnchor = "center"
	GravityNorth  GravityAnchor = "north"
	GravitySouth  GravityAnchor = "south"
	GravityEast   GravityAnchor = "east"
	GravityWest   GravityAnchor = "west"
	GravitySmart  GravityAnchor = "smart"
)

// ParseGravity parses gravity string safely.
func ParseGravity(s string) GravityAnchor {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "smart":
		return GravitySmart
	case "north", "top", "n":
		return GravityNorth
	case "south", "bottom", "s":
		return GravitySouth
	case "east", "right", "e":
		return GravityEast
	case "west", "left", "w":
		return GravityWest
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
	case GravityCenter:
		return imaging.Center
	default:
		return imaging.Center
	}
}

// ApplyCropAndResize transforms an image according to target dimensions, fit mode, gravity, and resampler.
func ApplyCropAndResize(img image.Image, targetW, targetH int, fit FitMode, gravity GravityAnchor, filter ResampleFilter) image.Image {
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
