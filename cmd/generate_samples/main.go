package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"github.com/chai2010/webp"
)

func main() {
	outDir := "testdata"
	if err := os.MkdirAll(outDir, 0755); err != nil {
		panic(err)
	}

	// 1. Portrait image (800x1200) with off-center subject to test smart crop
	portrait := image.NewRGBA(image.Rect(0, 0, 800, 1200))
	// Gradient background (studio backdrop)
	for y := 0; y < 1200; y++ {
		for x := 0; x < 800; x++ {
			v := uint8(40 + y*100/1200)
			portrait.SetRGBA(x, y, color.RGBA{R: v, G: v + 10, B: v + 25, A: 255})
		}
	}
	// Draw high-contrast subject (head & shoulders) around (x=480, y=400)
	subjX, subjY := 480, 400
	for y := subjY - 150; y <= subjY+150; y++ {
		for x := subjX - 120; x <= subjX+120; x++ {
			dx := float64(x - subjX)
			dy := float64(y - subjY)
			if (dx*dx)/(120*120)+(dy*dy)/(150*150) <= 1.0 {
				portrait.SetRGBA(x, y, color.RGBA{R: 220, G: 160, B: 130, A: 255})
			}
		}
	}
	// Eyes & high-entropy details
	for y := subjY - 40; y <= subjY - 20; y++ {
		for x := subjX - 60; x <= subjX - 30; x++ {
			portrait.SetRGBA(x, y, color.RGBA{R: 20, G: 20, B: 20, A: 255})
		}
		for x := subjX + 30; x <= subjX + 60; x++ {
			portrait.SetRGBA(x, y, color.RGBA{R: 20, G: 20, B: 20, A: 255})
		}
	}
	savePNG(filepath.Join(outDir, "portrait.png"), portrait)

	// 2. Landscape image (1920x1080)
	landscape := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 0; y < 1080; y++ {
		for x := 0; x < 1920; x++ {
			// Sky gradient
			if y < 650 {
				r := uint8(70 + y*100/650)
				g := uint8(130 + y*80/650)
				b := uint8(220)
				landscape.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
			} else {
				// Mountain/valley ground
				r := uint8(30 + (x%200)*20/200)
				g := uint8(80 + (y-650)*60/430)
				b := uint8(40)
				landscape.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
			}
		}
	}
	// Add mountain peaks
	for x := 0; x < 1920; x++ {
		peakH := int(200.0 * math.Sin(float64(x)*0.005) + 100.0*math.Cos(float64(x)*0.015))
		baseY := 650 - peakH
		for y := baseY; y < 650; y++ {
			landscape.SetRGBA(x, y, color.RGBA{R: 50, G: 65, B: 85, A: 255})
		}
	}
	saveJPEG(filepath.Join(outDir, "landscape.jpg"), landscape, 90)

	// 3. Technical Diagram (1000x1000)
	diagram := image.NewRGBA(image.Rect(0, 0, 1000, 1000))
	draw.Draw(diagram, diagram.Bounds(), &image.Uniform{C: color.RGBA{R: 245, G: 247, B: 250, A: 255}}, image.Point{}, draw.Src)
	// Draw grid
	for i := 0; i < 1000; i += 50 {
		for y := 0; y < 1000; y++ {
			diagram.SetRGBA(i, y, color.RGBA{R: 220, G: 225, B: 235, A: 255})
		}
		for x := 0; x < 1000; x++ {
			diagram.SetRGBA(x, i, color.RGBA{R: 220, G: 225, B: 235, A: 255})
		}
	}
	// Draw boxes & arrows
	drawRect(diagram, 150, 200, 350, 400, color.RGBA{R: 37, G: 99, B: 235, A: 255})
	drawRect(diagram, 550, 200, 750, 400, color.RGBA{R: 16, G: 185, B: 129, A: 255})
	drawRect(diagram, 350, 600, 650, 800, color.RGBA{R: 245, G: 158, B: 11, A: 255})
	savePNG(filepath.Join(outDir, "diagram.png"), diagram)

	// 4. Pattern (800x800) in WebP
	pattern := image.NewRGBA(image.Rect(0, 0, 800, 800))
	for y := 0; y < 800; y++ {
		for x := 0; x < 800; x++ {
			val := math.Sin(float64(x)*0.05)*math.Cos(float64(y)*0.05) + 1.0
			r := uint8(val * 110)
			g := uint8(120 + math.Sin(float64(x+y)*0.03)*100)
			b := uint8(180 + math.Cos(float64(x-y)*0.03)*70)
			pattern.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	saveWebP(filepath.Join(outDir, "pattern.webp"), pattern, 85)

	fmt.Println("Generated sample test images in testdata/")
}

func drawRect(img *image.RGBA, x1, y1, x2, y2 int, c color.RGBA) {
	for y := y1; y <= y2; y++ {
		for x := x1; x <= x2; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func savePNG(path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	_ = png.Encode(f, img)
}

func saveJPEG(path string, img image.Image, quality int) {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	_ = jpeg.Encode(f, img, &jpeg.Options{Quality: quality})
}

func saveWebP(path string, img image.Image, quality int) {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	_ = webp.Encode(f, img, &webp.Options{Lossless: false, Quality: float32(quality)})
}
