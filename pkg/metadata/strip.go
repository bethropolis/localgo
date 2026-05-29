package metadata

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Strip strips metadata (EXIF, text chunks) from image files in place
// by writing a stripped copy to a temp file and replacing the original.
// Supported formats: JPEG, PNG.
func Strip(path string) error {
	ext := filepath.Ext(path)
	switch ext {
	case ".jpg", ".jpeg":
		return stripJPEG(path)
	case ".png":
		return stripPNG(path)
	}
	return nil
}

// stripJPEG removes APP1 (EXIF) and APP13 (Photoshop/IPTC) markers.
func stripJPEG(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("strip: open: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, f); err != nil {
		return fmt.Errorf("strip: read: %w", err)
	}
	f.Close()

	data := buf.Bytes()

	// Must start with SOI marker 0xFFD8
	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 {
		return nil // not a valid JPEG
	}

	var out bytes.Buffer
	out.Write(data[:2]) // SOI

	pos := 2
	for pos < len(data) {
		if data[pos] != 0xFF {
			break
		}

		marker := data[pos+1]

		// SOS (Start of Scan) — everything after is compressed data, keep as-is
		if marker == 0xDA {
			out.Write(data[pos:])
			break
		}

		// Markers without length: SOI (0xD8), EOI (0xD9), TEM (0x01)
		if marker == 0xD9 || marker == 0x00 || marker == 0x01 {
			out.Write(data[pos : pos+2])
			pos += 2
			if marker == 0xD9 {
				break
			}
			continue
		}

		// All other markers have a 2-byte length (big-endian, includes itself)
		if pos+3 >= len(data) {
			break
		}
		segLen := int(binary.BigEndian.Uint16(data[pos+2:pos+4])) + 2

		if pos+segLen > len(data) {
			break
		}

		// Skip APP1 (EXIF, 0xFFE1) and APP13 (Photoshop/IPTC, 0xFFED)
		if marker != 0xE1 && marker != 0xED {
			out.Write(data[pos : pos+segLen])
		}

		pos += segLen
	}

	return os.WriteFile(path, out.Bytes(), 0644)
}

// stripPNG removes tEXt, zTXt, and iTXt metadata chunks.
func stripPNG(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("strip: open: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, f); err != nil {
		return fmt.Errorf("strip: read: %w", err)
	}
	f.Close()

	data := buf.Bytes()

	// Must be a valid PNG: 8-byte signature
	pngSig := []byte{137, 80, 78, 71, 13, 10, 26, 10}
	if len(data) < 8 || !bytes.Equal(data[:8], pngSig) {
		return nil
	}

	var out bytes.Buffer
	out.Write(data[:8]) // signature

	pos := 8
	for pos+4 <= len(data) {
		chunkLen := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		if pos+12+chunkLen > len(data) {
			break
		}
		chunkType := string(data[pos+4 : pos+8])

		// Skip text chunks
		if chunkType == "tEXt" || chunkType == "zTXt" || chunkType == "iTXt" {
			pos += 12 + chunkLen
			continue
		}

		// IEND — end of image
		if chunkType == "IEND" {
			out.Write(data[pos : pos+12+chunkLen])
			break
		}

		out.Write(data[pos : pos+12+chunkLen])
		pos += 12 + chunkLen
	}

	return os.WriteFile(path, out.Bytes(), 0644)
}
