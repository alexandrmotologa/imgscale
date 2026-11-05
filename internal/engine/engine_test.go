package engine

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// createSyntheticImage creates a test image with a high-contrast focal box in the lower-right quadrant.
func createSyntheticImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	// Smooth base background
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := uint8(200 + (x * 40 / w))
			img.SetRGBA(x, y, color.RGBA{R: c, G: c, B: c, A: 255})
		}
	}

	// High entropy focal feature (checkerboard square in bottom right)
	boxX := (w * 3) / 4
	boxY := (h * 3) / 4
	boxSize := w / 8
	if boxSize < 10 {
		boxSize = 10
	}

	for y := boxY; y < boxY+boxSize && y < h; y++ {
		for x := boxX; x < boxX+boxSize && x < w; x++ {
			if (x/4+y/4)%2 == 0 {
				img.SetRGBA(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
			} else {
				img.SetRGBA(x, y, color.RGBA{R: 0, G: 255, B: 255, A: 255})
			}
		}
	}

	return img
}

func encodeTestPNG(img image.Image) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func TestResizeAndCropModes(t *testing.T) {
	src := createSyntheticImage(800, 600)
	raw := encodeTestPNG(src)

	tests := []struct {
		name         string
		opts         TransformOptions
		expectWidth  int
		expectHeight int
		expectFormat ImageFormat
	}{
		{
			name: "Cover Resize",
			opts: TransformOptions{
				Width:   400,
				Height:  300,
				Fit:     FitCover,
				Gravity: GravityCenter,
				Format:  FormatJPEG,
			},
			expectWidth:  400,
			expectHeight: 300,
			expectFormat: FormatJPEG,
		},
		{
			name: "Contain Resize (Aspect 4:3 in 1:1)",
			opts: TransformOptions{
				Width:  400,
				Height: 400,
				Fit:    FitContain,
				Format: FormatPNG,
			},
			expectWidth:  400,
			expectHeight: 300,
			expectFormat: FormatPNG,
		},
		{
			name: "WebP Transcoding with Smart Crop",
			opts: TransformOptions{
				Width:   200,
				Height:  200,
				Fit:     FitCover,
				Gravity: GravitySmart,
				Format:  FormatWebP,
			},
			expectWidth:  200,
			expectHeight: 200,
			expectFormat: FormatWebP,
		},
		{
			name: "DPR Scaling 2x",
			opts: TransformOptions{
				Width:  150,
				Height: 100,
				Fit:    FitFill,
				DPR:    2.0,
				Format: FormatJPEG,
			},
			expectWidth:  300,
			expectHeight: 200,
			expectFormat: FormatJPEG,
		},
		{
			name: "Grayscale and Rotation",
			opts: TransformOptions{
				Width:   200,
				Height:  100,
				Fit:     FitCover,
				Format:  FormatJPEG,
				Filters: FilterOptions{Grayscale: true, Rotate: 90},
			},
			expectWidth:  100,
			expectHeight: 200,
			expectFormat: FormatJPEG,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := Transform(bytes.NewReader(raw), tt.opts)
			if err != nil {
				t.Fatalf("Transform failed: %v", err)
			}

			if res.OutputWidth != tt.expectWidth {
				t.Errorf("Expected width %d, got %d", tt.expectWidth, res.OutputWidth)
			}
			if res.OutputHeight != tt.expectHeight {
				t.Errorf("Expected height %d, got %d", tt.expectHeight, res.OutputHeight)
			}
			if res.Format != tt.expectFormat {
				t.Errorf("Expected format %s, got %s", tt.expectFormat, res.Format)
			}
			if len(res.Data) == 0 {
				t.Error("Expected output data, got empty slice")
			}
		})
	}
}

func TestSmartCropFocalPoint(t *testing.T) {
	// 800x400 image with feature in right side
	src := createSyntheticImage(800, 400)
	cropRect := FindSmartCrop(src, 400, 400)

	// Since the high-entropy feature is at x=600, the smart crop window should be pulled towards the right
	if cropRect.Min.X < 200 {
		t.Errorf("Expected smart crop to anchor towards right high-entropy feature, but Min.X was %d", cropRect.Min.X)
	}
	if cropRect.Dx() != 400 || cropRect.Dy() != 400 {
		t.Errorf("Expected 400x400 crop dimensions, got %dx%d", cropRect.Dx(), cropRect.Dy())
	}
}

func TestEntropyHeatmapGeneration(t *testing.T) {
	src := createSyntheticImage(300, 200)
	raw := encodeTestPNG(src)

	res, err := Transform(bytes.NewReader(raw), TransformOptions{
		Width:  150,
		Height: 150,
		Debug:  "entropy",
	})
	if err != nil {
		t.Fatalf("Entropy debug transform failed: %v", err)
	}

	if res.Format != FormatJPEG {
		t.Errorf("Expected JPEG heatmap, got %s", res.Format)
	}
	if len(res.Data) == 0 {
		t.Error("Expected heatmap data, got empty")
	}
}

func TestContentNegotiation(t *testing.T) {
	// When client accepts webp
	fmt1 := NegotiateFormat(FormatAuto, "text/html,image/avif,image/webp,*/*;q=0.8")
	if fmt1 != FormatWebP {
		t.Errorf("Expected FormatWebP, got %s", fmt1)
	}

	// When client only accepts traditional formats
	fmt2 := NegotiateFormat(FormatAuto, "text/html,image/jpeg,image/png")
	if fmt2 != FormatJPEG {
		t.Errorf("Expected FormatJPEG, got %s", fmt2)
	}

	// Explicit override ignores Accept header
	fmt3 := NegotiateFormat(FormatPNG, "image/webp")
	if fmt3 != FormatPNG {
		t.Errorf("Expected explicit PNG, got %s", fmt3)
	}
}

func TestFocalPointCrop(t *testing.T) {
	rectTopLeft := CalculateFocalCrop(1000, 1000, 400, 400, 0.05, 0.05)
	if rectTopLeft.Min.X != 0 || rectTopLeft.Min.Y != 0 {
		t.Errorf("Expected focal crop at (0, 0), got (%d, %d)", rectTopLeft.Min.X, rectTopLeft.Min.Y)
	}

	rectBottomRight := CalculateFocalCrop(1000, 1000, 400, 400, 0.95, 0.95)
	if rectBottomRight.Max.X != 1000 || rectBottomRight.Max.Y != 1000 {
		t.Errorf("Expected focal crop at (600, 600) to (1000, 1000), got max (%d, %d)", rectBottomRight.Max.X, rectBottomRight.Max.Y)
	}
}

func TestLQIP(t *testing.T) {
	src := createSyntheticImage(400, 300)
	raw := encodeTestPNG(src)

	// 1. DataURI mode
	resDataURI, err := Transform(bytes.NewReader(raw), TransformOptions{LQIP: "datauri"})
	if err != nil {
		t.Fatalf("LQIP datauri failed: %v", err)
	}
	if !bytes.HasPrefix(resDataURI.Data, []byte("data:image/webp;base64,")) {
		t.Errorf("Expected data URI prefix, got %s", string(resDataURI.Data[:30]))
	}

	// 2. JSON mode
	resJSON, err := Transform(bytes.NewReader(raw), TransformOptions{LQIP: "json"})
	if err != nil {
		t.Fatalf("LQIP json failed: %v", err)
	}
	if resJSON.ContentType != "application/json" {
		t.Errorf("Expected application/json, got %s", resJSON.ContentType)
	}

	// 3. Dominant Color calculation
	domColor := ComputeDominantColor(src)
	if len(domColor) != 7 || domColor[0] != '#' {
		t.Errorf("Invalid dominant color format: %s", domColor)
	}
}

func TestWatermark(t *testing.T) {
	base := createSyntheticImage(500, 500)
	wm := createSyntheticImage(100, 100)

	composite := ApplyWatermark(base, WatermarkOptions{
		Watermark: wm,
		Gravity:   GravitySouthEast,
		Opacity:   0.6,
		Scale:     20,
		Padding:   10,
	})

	if composite.Bounds().Dx() != 500 || composite.Bounds().Dy() != 500 {
		t.Errorf("Expected 500x500 composite bounds")
	}
}

func TestExifFixOrientation(t *testing.T) {
	src := createSyntheticImage(400, 200)

	// Orientation 6 = rotate 90 deg clockwise (so width and height swap)
	rotated := FixOrientation(src, 6)
	if rotated.Bounds().Dx() != 200 || rotated.Bounds().Dy() != 400 {
		t.Errorf("Expected 200x400 after 90 deg rotation, got %dx%d", rotated.Bounds().Dx(), rotated.Bounds().Dy())
	}
}
