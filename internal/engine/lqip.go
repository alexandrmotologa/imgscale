package engine

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"

	"github.com/disintegration/imaging"
)

// LQIPResult encapsulates placeholder metadata and base64 payload.
type LQIPResult struct {
	DataURI       string `json:"data_uri"`
	DominantColor string `json:"dominant_color"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	ByteSize      int    `json:"byte_size"`
}

// ComputeDominantColor calculates the average representative hex color of an image.
func ComputeDominantColor(img image.Image) string {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w <= 0 || h <= 0 {
		return "#000000"
	}

	stepX := w / 32
	if stepX < 1 {
		stepX = 1
	}
	stepY := h / 32
	if stepY < 1 {
		stepY = 1
	}

	var sumR, sumG, sumB, count uint64

	for y := bounds.Min.Y; y < bounds.Max.Y; y += stepY {
		for x := bounds.Min.X; x < bounds.Max.X; x += stepX {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if c.A > 32 { // Ignore fully transparent pixels
				sumR += uint64(c.R)
				sumG += uint64(c.G)
				sumB += uint64(c.B)
				count++
			}
		}
	}

	if count == 0 {
		return "#000000"
	}

	avgR := uint8(sumR / count)
	avgG := uint8(sumG / count)
	avgB := uint8(sumB / count)

	return fmt.Sprintf("#%02x%02x%02x", avgR, avgG, avgB)
}

// GenerateLQIP builds a low-quality blurred placeholder image or data-URI.
func GenerateLQIP(img image.Image, mode string) ([]byte, string, error) {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	targetW := 24
	targetH := 24
	if srcW > srcH {
		targetH = int(float64(targetW) * float64(srcH) / float64(srcW))
		if targetH < 1 {
			targetH = 1
		}
	} else if srcH > 0 {
		targetW = int(float64(targetH) * float64(srcW) / float64(srcH))
		if targetW < 1 {
			targetW = 1
		}
	}

	// 1. Downscale to micro thumbnail
	thumb := imaging.Resize(img, targetW, targetH, imaging.Box)

	// 2. Apply gentle Gaussian blur to smooth pixel blocks
	blurred := imaging.Blur(thumb, 1.2)

	// 3. Encode to WebP
	encoded, err := EncodeToBytes(blurred, FormatWebP, 45)
	if err != nil {
		return nil, "", fmt.Errorf("failed to encode LQIP webp: %w", err)
	}

	dominantColor := ComputeDominantColor(img)
	dataURI := "data:image/webp;base64," + base64.StdEncoding.EncodeToString(encoded)

	switch mode {
	case "json":
		res := LQIPResult{
			DataURI:       dataURI,
			DominantColor: dominantColor,
			Width:         srcW,
			Height:        srcH,
			ByteSize:      len(encoded),
		}
		data, err := json.Marshal(res)
		if err != nil {
			return nil, "", err
		}
		return data, "application/json", nil

	case "datauri":
		return []byte(dataURI), "text/plain; charset=utf-8", nil

	case "blur", "raw":
		return encoded, "image/webp", nil

	default:
		// Default to JSON
		res := LQIPResult{
			DataURI:       dataURI,
			DominantColor: dominantColor,
			Width:         srcW,
			Height:        srcH,
			ByteSize:      len(encoded),
		}
		data, _ := json.Marshal(res)
		return data, "application/json", nil
	}
}
