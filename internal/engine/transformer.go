package engine

import (
	"fmt"
	"image"
	"io"
	"time"
)

// TransformOptions encapsulates all transformation requests.
type TransformOptions struct {
	Width    int
	Height   int
	Fit      FitMode
	Gravity  GravityAnchor
	Filter   ResampleFilter
	Format   ImageFormat
	Quality  int
	DPR      float64
	Filters  FilterOptions
	Debug    string // "entropy", "meta"
	Accept   string // HTTP Accept header for content negotiation
}

// TransformResult holds the output data and operational telemetry.
type TransformResult struct {
	Data         []byte
	Format       ImageFormat
	ContentType  string
	SrcWidth     int
	SrcHeight    int
	OutputWidth  int
	OutputHeight int
	Duration     time.Duration
	ByteSize     int
}

// Transform coordinates the entire decoding, transformation, filter, and encoding pipeline.
func Transform(r io.Reader, opts TransformOptions) (*TransformResult, error) {
	start := time.Now()

	// 1. Decode source image
	srcImg, origFormat, err := DecodeImage(r)
	if err != nil {
		return nil, fmt.Errorf("failed to decode source image: %w", err)
	}

	srcBounds := srcImg.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	// 2. Handle debug=entropy visual saliency inspection
	if opts.Debug == "entropy" {
		targetW := opts.Width
		targetH := opts.Height
		if targetW <= 0 {
			targetW = 400
		}
		if targetH <= 0 {
			targetH = 400
		}
		cropRect := FindSmartCrop(srcImg, targetW, targetH)
		heatmap := GenerateEntropyHeatmap(srcImg, cropRect)

		encoded, err := EncodeToBytes(heatmap, FormatJPEG, 85)
		if err != nil {
			return nil, fmt.Errorf("failed to encode entropy heatmap: %w", err)
		}

		return &TransformResult{
			Data:         encoded,
			Format:       FormatJPEG,
			ContentType:  FormatJPEG.ContentType(),
			SrcWidth:     srcW,
			SrcHeight:    srcH,
			OutputWidth:  srcW,
			OutputHeight: srcH,
			Duration:     time.Since(start),
			ByteSize:     len(encoded),
		}, nil
	}

	// 3. Negotiate target format
	finalFormat := NegotiateFormat(opts.Format, opts.Accept)
	if finalFormat == FormatAuto {
		finalFormat = origFormat
		if finalFormat == FormatAuto {
			finalFormat = FormatJPEG
		}
	}

	// 4. Calculate target dimensions accounting for DPR
	dpr := opts.DPR
	if dpr <= 0 {
		dpr = 1.0
	}
	targetW := int(float64(opts.Width) * dpr)
	targetH := int(float64(opts.Height) * dpr)

	// 5. Apply resizing and cropping
	var transformed image.Image
	if targetW > 0 || targetH > 0 {
		filter := opts.Filter
		if filter == "" {
			filter = FilterLanczos
		}
		fit := opts.Fit
		if fit == "" {
			fit = FitCover
		}
		gravity := opts.Gravity
		if gravity == "" {
			gravity = GravityCenter
		}

		transformed = ApplyCropAndResize(srcImg, targetW, targetH, fit, gravity, filter)
	} else {
		transformed = srcImg
	}

	// 6. Apply post-processing filters (rotation, blur, sharpen, grayscale)
	transformed = ApplyFilters(transformed, opts.Filters)

	// 7. Encode output to target format
	quality := opts.Quality
	if quality <= 0 {
		quality = 80
	}

	encoded, err := EncodeToBytes(transformed, finalFormat, quality)
	if err != nil {
		return nil, fmt.Errorf("failed to encode output image: %w", err)
	}

	outBounds := transformed.Bounds()

	return &TransformResult{
		Data:         encoded,
		Format:       finalFormat,
		ContentType:  finalFormat.ContentType(),
		SrcWidth:     srcW,
		SrcHeight:    srcH,
		OutputWidth:  outBounds.Dx(),
		OutputHeight: outBounds.Dy(),
		Duration:     time.Since(start),
		ByteSize:     len(encoded),
	}, nil
}
