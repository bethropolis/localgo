package metadata

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Strip strips metadata (EXIF, text chunks) from image files using a
// temp-file + rename strategy so the original is never overwritten in place.
// Supported formats: JPEG, PNG. Returns nil for non-image files.
func Strip(path string) error {
	tmp, err := stripToTemp(path)
	if err != nil {
		return err
	}
	if tmp == "" {
		return nil
	}
	defer os.Remove(tmp)
	return os.Rename(tmp, path)
}

// StripTo writes a stripped copy of the source image to destPath.
// Both paths may be the same (caller should use Strip for that).
// Returns nil for non-image files without error.
func StripTo(srcPath, destPath string) error {
	srcIsImage, err := IsImageFile(srcPath)
	if err != nil || !srcIsImage {
		return err
	}

	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("strip: open: %w", err)
	}
	defer f.Close()

	sig := make([]byte, 8)
	if _, err := io.ReadFull(f, sig); err != nil {
		return fmt.Errorf("strip: read sig: %w", err)
	}
	f.Close()

	switch {
	case isJPEG(sig):
		return stripJPEGTo(srcPath, destPath)
	case isPNG(sig):
		return stripPNGTo(srcPath, destPath)
	}
	return nil
}

// stripToTemp strips metadata to a temp file in the same directory.
// Returns empty string if the file is not a supported image type.
func stripToTemp(path string) (string, error) {
	srcIsImage, err := IsImageFile(path)
	if err != nil || !srcIsImage {
		return "", err
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".localgo-strip-*")
	if err != nil {
		return "", fmt.Errorf("strip: temp: %w", err)
	}
	tmpPath := tmp.Name()
	tmp.Close()
	os.Remove(tmpPath)

	if err := StripTo(path, tmpPath); err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	return tmpPath, nil
}

// IsImageFile returns true if the file at path has a JPEG or PNG magic signature.
func IsImageFile(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("strip: open: %w", err)
	}
	defer f.Close()

	sig := make([]byte, 8)
	if _, err := io.ReadFull(f, sig); err != nil {
		return false, nil
	}
	return isJPEG(sig) || isPNG(sig), nil
}

func isJPEG(sig []byte) bool {
	return len(sig) >= 2 && sig[0] == 0xFF && sig[1] == 0xD8
}

func isPNG(sig []byte) bool {
	pngSig := []byte{137, 80, 78, 71, 13, 10, 26, 10}
	return bytes.Equal(sig, pngSig)
}

// stripJPEGTo removes APP1 (EXIF) and APP13 (Photoshop/IPTC) markers.
func stripJPEGTo(srcPath, destPath string) error {
	f, err := os.Open(srcPath)
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
	if !isJPEG(data) {
		return nil
	}

	var out bytes.Buffer
	out.Write(data[:2]) // SOI

	pos := 2
	sosSeen := false
	for pos+1 < len(data) {
		if data[pos] != 0xFF {
			break
		}

		marker := data[pos+1]

		if marker == 0xDA {
			out.Write(data[pos:])
			sosSeen = true
			break
		}

		if marker == 0xD9 || marker == 0x00 || marker == 0x01 {
			if pos+2 > len(data) {
				break
			}
			out.Write(data[pos : pos+2])
			pos += 2
			if marker == 0xD9 {
				break
			}
			continue
		}

		if pos+3 >= len(data) {
			break
		}
		segLen := int(binary.BigEndian.Uint16(data[pos+2:pos+4])) + 2
		if pos+segLen > len(data) {
			break
		}

		if marker != 0xE1 && marker != 0xED {
			out.Write(data[pos : pos+segLen])
		}

		pos += segLen
	}

	if !sosSeen {
		return fmt.Errorf("strip: no SOS marker found in JPEG")
	}

	return writeAtomic(destPath, out.Bytes())
}

// stripPNGTo removes tEXt, zTXt, iTXt, and eXIf metadata chunks.
func stripPNGTo(srcPath, destPath string) error {
	f, err := os.Open(srcPath)
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
	if len(data) < 8 || !isPNG(data[:8]) {
		return nil
	}

	var out bytes.Buffer
	out.Write(data[:8]) // signature

	pos := 8
	iendSeen := false
	for pos+4 <= len(data) {
		chunkLen := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		if pos+12+chunkLen > len(data) {
			break
		}
		chunkType := string(data[pos+4 : pos+8])

		if chunkType == "tEXt" || chunkType == "zTXt" || chunkType == "iTXt" || chunkType == "eXIf" {
			pos += 12 + chunkLen
			continue
		}

		if chunkType == "IEND" {
			out.Write(data[pos : pos+12+chunkLen])
			iendSeen = true
			break
		}

		out.Write(data[pos : pos+12+chunkLen])
		pos += 12 + chunkLen
	}

	if !iendSeen {
		return fmt.Errorf("strip: no IEND chunk found in PNG")
	}

	return writeAtomic(destPath, out.Bytes())
}

// writeAtomic writes data to path via a temp file and rename.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".localgo-write-*")
	if err != nil {
		return fmt.Errorf("strip: temp: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("strip: write: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("strip: sync: %w", err)
	}
	tmp.Close()

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("strip: rename: %w", err)
	}
	return nil
}
