package engine

import (
	"image"

	"github.com/disintegration/imaging"
)

// ResampleFilter represents the interpolation algorithm.
type ResampleFilter string

const (
	FilterLanczos         ResampleFilter = "lanczos"
	FilterCatmullRom      ResampleFilter = "catmullrom"
	FilterLinear          ResampleFilter = "linear"
	FilterBox             ResampleFilter = "box"
	FilterNearestNeighbor ResampleFilter = "nearest"
)

// ToImagingFilter converts ResampleFilter string to imaging.ResampleFilter.
func (f ResampleFilter) ToImagingFilter() imaging.ResampleFilter {
	switch f {
	case FilterLanczos:
		return imaging.Lanczos
	case FilterCatmullRom:
		return imaging.CatmullRom
	case FilterLinear:
		return imaging.Linear
	case FilterBox:
		return imaging.Box
	case FilterNearestNeighbor:
		return imaging.NearestNeighbor
	default:
		return imaging.Lanczos
	}
}

// FitMode specifies how the image conforms to target dimensions.
type FitMode string

const (
	FitCover   FitMode = "cover"
	FitContain FitMode = "contain"
	FitFill    FitMode = "fill"
	FitInside  FitMode = "inside"
	FitOutside FitMode = "outside"
	FitCrop    FitMode = "crop"
)

// CalculateTargetSize determines output dimensions given input bounds and target parameters.
func CalculateTargetSize(srcW, srcH, reqW, reqH int, fit FitMode, dpr float64) (int, int) {
	if dpr <= 0 {
		dpr = 1.0
	}
	if dpr > 3.0 {
		dpr = 3.0
	}

	targetW := int(float64(reqW) * dpr)
	targetH := int(float64(reqH) * dpr)

	if targetW <= 0 && targetH <= 0 {
		return srcW, srcH
	}

	srcAspect := float64(srcW) / float64(srcH)

	// If one dimension is omitted, calculate it maintaining aspect ratio
	if targetW <= 0 {
		targetW = int(float64(targetH) * srcAspect)
		if targetW < 1 {
			targetW = 1
		}
		return targetW, targetH
	}
	if targetH <= 0 {
		targetH = int(float64(targetW) / srcAspect)
		if targetH < 1 {
			targetH = 1
		}
		return targetW, targetH
	}

	switch fit {
	case FitContain, FitInside:
		targetAspect := float64(targetW) / float64(targetH)
		if fit == FitInside && srcW <= targetW && srcH <= targetH {
			return srcW, srcH
		}
		if srcAspect > targetAspect {
			// Width-constrained
			finalW := targetW
			finalH := int(float64(targetW) / srcAspect)
			if finalH < 1 {
				finalH = 1
			}
			return finalW, finalH
		}
		// Height-constrained
		finalH := targetH
		finalW := int(float64(targetH) * srcAspect)
		if finalW < 1 {
			finalW = 1
		}
		return finalW, finalH

	case FitOutside:
		targetAspect := float64(targetW) / float64(targetH)
		if srcAspect > targetAspect {
			// Height-constrained
			finalH := targetH
			finalW := int(float64(targetH) * srcAspect)
			return finalW, finalH
		}
		// Width-constrained
		finalW := targetW
		finalH := int(float64(targetW) / srcAspect)
		return finalW, finalH

	case FitFill, FitCover, FitCrop:
		return targetW, targetH

	default:
		return targetW, targetH
	}
}

// Resize applies interpolation resizing to the source image.
func Resize(img image.Image, width, height int, filter ResampleFilter) image.Image {
	if width <= 0 || height <= 0 {
		return img
	}
	return imaging.Resize(img, width, height, filter.ToImagingFilter())
}
