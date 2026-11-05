package engine

import (
	"bytes"
	"testing"
)

func BenchmarkLanczos3Resize(b *testing.B) {
	src := createSyntheticImage(1920, 1080)
	raw := encodeTestPNG(src)
	opts := TransformOptions{
		Width:   800,
		Height:  600,
		Fit:     FitCover,
		Filter:  FilterLanczos,
		Format:  FormatJPEG,
		Quality: 80,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Transform(bytes.NewReader(raw), opts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSmartCropEntropy(b *testing.B) {
	src := createSyntheticImage(1920, 1080)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FindSmartCrop(src, 800, 600)
	}
}

func BenchmarkWebPEncoding(b *testing.B) {
	src := createSyntheticImage(800, 600)
	raw := encodeTestPNG(src)
	opts := TransformOptions{
		Width:   400,
		Height:  300,
		Fit:     FitCover,
		Format:  FormatWebP,
		Quality: 80,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Transform(bytes.NewReader(raw), opts)
		if err != nil {
			b.Fatal(err)
		}
	}
}
