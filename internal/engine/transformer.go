package engine

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"time"
)

// TransformOptions encapsulates all transformation requests.
type TransformOptions struct {
	Width       int
	Height      int
	Fit         FitMode
	Gravity     GravityAnchor
	FocalPointX float64 // 0.0 to 1.0 (relative X anchor)
	FocalPointY float64 // 0.0 to 1.0 (relative Y anchor)
	Filter      ResampleFilter
	Format      ImageFormat
	Quality     int
	DPR         float64
	Filters     FilterOptions
	LQIP        string // "datauri", "json", "blur"
	Watermark   WatermarkOptions
	AutoOrient  bool
	Debug       string // "entropy", "meta"
	Accept      string // HTTP Accept header for content negotiation
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

	// Capture buffer for EXIF inspection if auto orientation requested
	var rawBuf bytes.Buffer
	tr := io.TeeReader(r, &rawBuf)

	// 1. Decode source image
	srcImg, origFormat, err := DecodeImage(tr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode source image: %w", err)
	}

	// Apply EXIF Auto-Orientation if requested or by default
	if opts.AutoOrient {
		orientation := ReadExifOrientation(bytes.NewReader(rawBuf.Bytes()))
		srcImg = FixOrientation(srcImg, orientation)
	}

	srcBounds := srcImg.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	// 2. Handle Low-Quality Image Placeholder (LQIP) mode
	if opts.LQIP != "" {
		lqipData, contentType, err := GenerateLQIP(srcImg, opts.LQIP)
		if err != nil {
			return nil, fmt.Errorf("failed to generate LQIP: %w", err)
		}
		return &TransformResult{
			Data:         lqipData,
			Format:       FormatWebP,
			ContentType:  contentType,
			SrcWidth:     srcW,
			SrcHeight:    srcH,
			OutputWidth:  24,
			OutputHeight: 24,
			Duration:     time.Since(start),
			ByteSize:     len(lqipData),
		}, nil
	}

	// 3. Handle debug=entropy visual saliency inspection
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

	// 4. Negotiate target format
	finalFormat := NegotiateFormat(opts.Format, opts.Accept)
	if finalFormat == FormatAuto {
		finalFormat = origFormat
		if finalFormat == FormatAuto {
			finalFormat = FormatJPEG
		}
	}

	// 5. Calculate target dimensions accounting for DPR
	dpr := opts.DPR
	if dpr <= 0 {
		dpr = 1.0
	}
	targetW := int(float64(opts.Width) * dpr)
	targetH := int(float64(opts.Height) * dpr)

	// 6. Apply resizing and cropping
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

		transformed = ApplyCropAndResize(srcImg, targetW, targetH, fit, gravity, opts.FocalPointX, opts.FocalPointY, filter)
	} else {
		transformed = srcImg
	}

	// 7. Apply post-processing filters (rotation, blur, sharpen, grayscale, brightness, contrast)
	transformed = ApplyFilters(transformed, opts.Filters)

	// 8. Apply watermark overlay if supplied
	if opts.Watermark.Watermark != nil {
		transformed = ApplyWatermark(transformed, opts.Watermark)
	}

	// 9. Encode output to target format
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
