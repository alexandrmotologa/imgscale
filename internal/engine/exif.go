package engine

import (
	"bytes"
	"encoding/binary"
	"image"
	"io"

	"github.com/disintegration/imaging"
)

// ReadExifOrientation parses JPEG/TIFF EXIF marker to extract the orientation tag (1-8).
func ReadExifOrientation(r io.Reader) int {
	header := make([]byte, 4096)
	n, err := io.ReadAtLeast(r, header, 16)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return 1
	}
	data := header[:n]

	// Check JPEG SOI marker (0xFF, 0xD8)
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}

	pos := 2
	for pos+4 < len(data) {
		if data[pos] != 0xFF {
			break
		}
		marker := data[pos+1]
		length := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))

		// APP1 marker (0xE1) contains EXIF
		if marker == 0xE1 && pos+4+length <= len(data) {
			exifBlock := data[pos+4 : pos+2+length]
			if len(exifBlock) >= 6 && string(exifBlock[:4]) == "Exif" && exifBlock[4] == 0 && exifBlock[5] == 0 {
				return parseTiffOrientation(exifBlock[6:])
			}
		}

		pos += 2 + length
	}

	return 1
}

func parseTiffOrientation(tiff []byte) int {
	if len(tiff) < 8 {
		return 1
	}

	var order binary.ByteOrder
	if string(tiff[:2]) == "II" {
		order = binary.LittleEndian
	} else if string(tiff[:2]) == "MM" {
		order = binary.BigEndian
	} else {
		return 1
	}

	firstIFD := int(order.Uint32(tiff[4:8]))
	if firstIFD < 0 || firstIFD+2 > len(tiff) {
		return 1
	}

	numEntries := int(order.Uint16(tiff[firstIFD : firstIFD+2]))
	entryOffset := firstIFD + 2

	for i := 0; i < numEntries; i++ {
		if entryOffset+12 > len(tiff) {
			break
		}
		tag := order.Uint16(tiff[entryOffset : entryOffset+2])
		if tag == 0x0112 { // Orientation Tag
			val := order.Uint16(tiff[entryOffset+8 : entryOffset+10])
			if val >= 1 && val <= 8 {
				return int(val)
			}
		}
		entryOffset += 12
	}

	return 1
}

// FixOrientation adjusts the image based on EXIF orientation tag.
func FixOrientation(img image.Image, orientation int) image.Image {
	switch orientation {
	case 2:
		return imaging.FlipH(img)
	case 3:
		return imaging.Rotate180(img)
	case 4:
		return imaging.FlipV(img)
	case 5:
		return imaging.Transpose(img)
	case 6:
		return imaging.Rotate270(img) // 90 deg clockwise
	case 7:
		return imaging.Transverse(img)
	case 8:
		return imaging.Rotate90(img) // 270 deg clockwise
	default:
		return img
	}
}

// AutoOrientImage detects EXIF orientation from reader and rotates image into canonical orientation.
func AutoOrientImage(r io.Reader, img image.Image) (image.Image, int) {
	var buf bytes.Buffer
	tr := io.TeeReader(r, &buf)
	orientation := ReadExifOrientation(tr)
	return FixOrientation(img, orientation), orientation
}
