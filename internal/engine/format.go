package engine

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"strings"

	"github.com/chai2010/webp"
	_ "golang.org/x/image/webp"
)

// ImageFormat represents a supported output or input format.
type ImageFormat string

const (
	FormatAuto ImageFormat = "auto"
	FormatWebP ImageFormat = "webp"
	FormatJPEG ImageFormat = "jpeg"
	FormatPNG  ImageFormat = "png"
	FormatGIF  ImageFormat = "gif"
)

// ParseFormat parses format string safely.
func ParseFormat(s string) ImageFormat {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "webp":
		return FormatWebP
	case "jpeg", "jpg":
		return FormatJPEG
	case "png":
		return FormatPNG
	case "gif":
		return FormatGIF
	case "auto", "":
		return FormatAuto
	default:
		return FormatAuto
	}
}

// ContentType returns the standard MIME type for an ImageFormat.
func (f ImageFormat) ContentType() string {
	switch f {
	case FormatWebP:
		return "image/webp"
	case FormatJPEG:
		return "image/jpeg"
	case FormatPNG:
		return "image/png"
	case FormatGIF:
		return "image/gif"
	default:
		return "image/jpeg"
	}
}

// Extension returns typical file extension without dot.
func (f ImageFormat) Extension() string {
	switch f {
	case FormatWebP:
		return "webp"
	case FormatJPEG:
		return "jpg"
	case FormatPNG:
		return "png"
	case FormatGIF:
		return "gif"
	default:
		return "jpg"
	}
}

// NegotiateFormat resolves "auto" against the HTTP Accept header.
func NegotiateFormat(reqFormat ImageFormat, acceptHeader string) ImageFormat {
	if reqFormat != FormatAuto && reqFormat != "" {
		return reqFormat
	}

	acceptLower := strings.ToLower(acceptHeader)
	if strings.Contains(acceptLower, "image/webp") {
		return FormatWebP
	}

	return FormatJPEG
}

// DetectFormat inspects the first few bytes of image data to determine its format.
func DetectFormat(r io.Reader) (ImageFormat, io.Reader, error) {
	header := make([]byte, 512)
	n, err := io.ReadAtLeast(r, header, 16)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", nil, err
	}

	combined := io.MultiReader(bytes.NewReader(header[:n]), r)
	data := header[:n]

	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}) {
		return FormatPNG, combined, nil
	}
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return FormatJPEG, combined, nil
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return FormatWebP, combined, nil
	}
	if len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a") {
		return FormatGIF, combined, nil
	}

	return FormatJPEG, combined, nil
}

// DecodeImage decodes an image from an io.Reader, supporting JPEG, PNG, GIF, and WebP.
func DecodeImage(r io.Reader) (image.Image, ImageFormat, error) {
	detectedFmt, combinedReader, err := DetectFormat(r)
	if err != nil {
		return nil, "", fmt.Errorf("failed to detect image format: %w", err)
	}

	img, formatStr, err := image.Decode(combinedReader)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode image: %w", err)
	}

	parsedFmt := ParseFormat(formatStr)
	if parsedFmt == FormatAuto {
		parsedFmt = detectedFmt
	}

	return img, parsedFmt, nil
}

// EncodeImage writes the image to an io.Writer in the specified format and quality.
func EncodeImage(w io.Writer, img image.Image, format ImageFormat, quality int) error {
	if quality <= 0 {
		quality = 80
	}
	if quality > 100 {
		quality = 100
	}

	switch format {
	case FormatWebP:
		return webp.Encode(w, img, &webp.Options{
			Lossless: false,
			Quality:  float32(quality),
		})

	case FormatJPEG:
		return jpeg.Encode(w, img, &jpeg.Options{
			Quality: quality,
		})

	case FormatPNG:
		return png.Encode(w, img)

	case FormatGIF:
		return gif.Encode(w, img, &gif.Options{
			NumColors: 256,
		})

	default:
		return errors.New("unsupported image format for encoding: " + string(format))
	}
}

// EncodeToBytes encodes the image in-memory and returns the resulting byte slice.
func EncodeToBytes(img image.Image, format ImageFormat, quality int) ([]byte, error) {
	var buf bytes.Buffer
	if err := EncodeImage(&buf, img, format, quality); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
