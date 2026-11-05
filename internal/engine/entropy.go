package engine

import (
	"image"
	"image/color"
	"math"

	"github.com/disintegration/imaging"
)

// SaliencyPoint represents the center of highest visual energy.
type SaliencyPoint struct {
	X int
	Y int
}

// FindSmartCrop analyzes image gradients using Sobel kernels and returns the optimal crop bounds.
func FindSmartCrop(img image.Image, targetWidth, targetHeight int) image.Rectangle {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	if targetWidth <= 0 || targetHeight <= 0 || targetWidth >= srcW && targetHeight >= srcH {
		return bounds
	}

	// Aspect ratio constraint
	targetAspect := float64(targetWidth) / float64(targetHeight)
	srcAspect := float64(srcW) / float64(srcH)

	var cropW, cropH int
	if srcAspect > targetAspect {
		// Image is wider than target aspect ratio: crop horizontally
		cropH = srcH
		cropW = int(float64(srcH) * targetAspect)
		if cropW > srcW {
			cropW = srcW
		}
	} else {
		// Image is taller than target aspect ratio: crop vertically
		cropW = srcW
		cropH = int(float64(srcW) / targetAspect)
		if cropH > srcH {
			cropH = srcH
		}
	}

	if cropW == srcW && cropH == srcH {
		return bounds
	}

	// Downsample to max dimension 256 for fast saliency computation (< 2ms)
	scale := 1.0
	analysisImg := img
	maxDim := 256
	if srcW > maxDim || srcH > maxDim {
		if srcW > srcH {
			scale = float64(maxDim) / float64(srcW)
			analysisImg = imaging.Resize(img, maxDim, 0, imaging.Box)
		} else {
			scale = float64(maxDim) / float64(srcH)
			analysisImg = imaging.Resize(img, 0, maxDim, imaging.Box)
		}
	}

	aBounds := analysisImg.Bounds()
	aW := aBounds.Dx()
	aH := aBounds.Dy()

	aCropW := int(float64(cropW) * scale)
	aCropH := int(float64(cropH) * scale)
	if aCropW < 1 {
		aCropW = 1
	}
	if aCropH < 1 {
		aCropH = 1
	}
	if aCropW > aW {
		aCropW = aW
	}
	if aCropH > aH {
		aCropH = aH
	}

	// 1. Compute grayscale luminance and Sobel edge magnitude
	gray := imaging.Grayscale(analysisImg)
	sat := computeSobelIntegral(gray, aW, aH)

	// 2. Sliding window search for rectangle maximizing entropy sum
	bestScore := -1.0
	bestX := 0
	bestY := 0

	stepX := (aW - aCropW) / 16
	if stepX < 1 {
		stepX = 1
	}
	stepY := (aH - aCropH) / 16
	if stepY < 1 {
		stepY = 1
	}

	for y := 0; y <= aH-aCropH; y += stepY {
		for x := 0; x <= aW-aCropW; x += stepX {
			score := querySAT(sat, aW, aH, x, y, x+aCropW-1, y+aCropH-1)

			// Mild center bias to avoid edge cling on low-contrast images
			centerX := float64(x) + float64(aCropW)/2.0
			centerY := float64(y) + float64(aCropH)/2.0
			distX := (centerX - float64(aW)/2.0) / float64(aW)
			distY := (centerY - float64(aH)/2.0) / float64(aH)
			centerDist := math.Sqrt(distX*distX + distY*distY)
			centerWeight := 1.0 - 0.15*centerDist

			weightedScore := score * centerWeight
			if weightedScore > bestScore {
				bestScore = weightedScore
				bestX = x
				bestY = y
			}
		}
	}

	// Project scaled coordinates back to original image space
	finalX := int(float64(bestX) / scale)
	finalY := int(float64(bestY) / scale)

	if finalX+cropW > srcW {
		finalX = srcW - cropW
	}
	if finalY+cropH > srcH {
		finalY = srcH - cropH
	}
	if finalX < 0 {
		finalX = 0
	}
	if finalY < 0 {
		finalY = 0
	}

	return image.Rect(bounds.Min.X+finalX, bounds.Min.Y+finalY, bounds.Min.X+finalX+cropW, bounds.Min.Y+finalY+cropH)
}

// computeSobelIntegral builds a Summed Area Table (integral image) of Sobel gradients.
func computeSobelIntegral(gray *image.NRGBA, w, h int) []float64 {
	mag := make([]float64, w*h)

	for y := 1; y < h-1; y++ {
		rowPrev := (y - 1) * gray.Stride
		rowCurr := y * gray.Stride
		rowNext := (y + 1) * gray.Stride

		for x := 1; x < w-1; x++ {
			p00 := float64(gray.Pix[rowPrev+(x-1)*4])
			p01 := float64(gray.Pix[rowPrev+x*4])
			p02 := float64(gray.Pix[rowPrev+(x+1)*4])
			p10 := float64(gray.Pix[rowCurr+(x-1)*4])
			p12 := float64(gray.Pix[rowCurr+(x+1)*4])
			p20 := float64(gray.Pix[rowNext+(x-1)*4])
			p21 := float64(gray.Pix[rowNext+x*4])
			p22 := float64(gray.Pix[rowNext+(x+1)*4])

			// Sobel kernels
			gx := (p02 + 2*p12 + p22) - (p00 + 2*p10 + p20)
			gy := (p20 + 2*p21 + p22) - (p00 + 2*p01 + p02)

			mag[y*w+x] = math.Abs(gx) + math.Abs(gy)
		}
	}

	// Build Summed Area Table
	sat := make([]float64, w*h)
	for y := 0; y < h; y++ {
		rowSum := 0.0
		for x := 0; x < w; x++ {
			rowSum += mag[y*w+x]
			if y == 0 {
				sat[y*w+x] = rowSum
			} else {
				sat[y*w+x] = sat[(y-1)*w+x] + rowSum
			}
		}
	}
	return sat
}

// querySAT returns the sum of values within rectangle [x1, y1] to [x2, y2] in O(1) time.
func querySAT(sat []float64, w, h, x1, y1, x2, y2 int) float64 {
	if x1 < 0 {
		x1 = 0
	}
	if y1 < 0 {
		y1 = 0
	}
	if x2 >= w {
		x2 = w - 1
	}
	if y2 >= h {
		y2 = h - 1
	}

	d := sat[y2*w+x2]
	b := 0.0
	if y1 > 0 {
		b = sat[(y1-1)*w+x2]
	}
	c := 0.0
	if x1 > 0 {
		c = sat[y2*w+(x1-1)]
	}
	a := 0.0
	if x1 > 0 && y1 > 0 {
		a = sat[(y1-1)*w+(x1-1)]
	}

	return d - b - c + a
}

// GenerateEntropyHeatmap builds an RGB visualization with edge magnitudes and highlighted crop frame.
func GenerateEntropyHeatmap(img image.Image, cropRect image.Rectangle) image.Image {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	gray := imaging.Grayscale(img)
	out := image.NewRGBA(bounds)

	// Draw Sobel heatmap in cyan/blue-to-yellow spectrum
	for y := 1; y < h-1; y++ {
		rowPrev := (y - 1) * gray.Stride
		rowCurr := y * gray.Stride
		rowNext := (y + 1) * gray.Stride

		for x := 1; x < w-1; x++ {
			p00 := float64(gray.Pix[rowPrev+(x-1)*4])
			p01 := float64(gray.Pix[rowPrev+x*4])
			p02 := float64(gray.Pix[rowPrev+(x+1)*4])
			p10 := float64(gray.Pix[rowCurr+(x-1)*4])
			p12 := float64(gray.Pix[rowCurr+(x+1)*4])
			p20 := float64(gray.Pix[rowNext+(x-1)*4])
			p21 := float64(gray.Pix[rowNext+x*4])
			p22 := float64(gray.Pix[rowNext+(x+1)*4])

			gx := (p02 + 2*p12 + p22) - (p00 + 2*p10 + p20)
			gy := (p20 + 2*p21 + p22) - (p00 + 2*p01 + p02)
			magnitude := math.Sqrt(gx*gx + gy*gy)

			// Normalize magnitude to 0-255
			val := uint8(math.Min(255, magnitude*1.2))

			// Turbo/heat color mapping: dark purple/blue -> cyan -> yellow
			r := uint8(math.Min(255, float64(val)*1.2))
			g := uint8(math.Min(255, float64(val)*0.9))
			b := uint8(255 - val/2)

			out.SetRGBA(bounds.Min.X+x, bounds.Min.Y+y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	// Draw crop rectangle border with neon green line (2px thickness)
	green := color.RGBA{R: 0, G: 255, B: 128, A: 255}
	borderThickness := 3

	for t := 0; t < borderThickness; t++ {
		// Top and bottom borders
		for x := cropRect.Min.X; x <= cropRect.Max.X; x++ {
			out.Set(x, cropRect.Min.Y+t, green)
			out.Set(x, cropRect.Max.Y-1-t, green)
		}
		// Left and right borders
		for y := cropRect.Min.Y; y <= cropRect.Max.Y; y++ {
			out.Set(cropRect.Min.X+t, y, green)
			out.Set(cropRect.Max.X-1-t, y, green)
		}
	}

	return out
}
