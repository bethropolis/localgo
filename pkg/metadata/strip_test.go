package metadata

import (
	"os"
	"path/filepath"
	"testing"
)

// minimalJPEG is a valid 1x1 JPEG with APP1 (EXIF) metadata.
func minimalJPEG() []byte {
	// SOI + APP1 (EXIF) + SOS + compressed data + EOI
	exif := make([]byte, 8)
	copy(exif, "Exif\000\000") // EXIF header

	app1Len := uint16(len(exif) + 2) // includes the 2-byte length field
	body := []byte{
		0xFF, 0xD8, // SOI
		0xFF, 0xE1, // APP1 marker
		byte(app1Len >> 8), byte(app1Len & 0xFF), // length big-endian
	}
	body = append(body, exif...)
	body = append(body,
		0xFF, 0xDA, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3F, 0x00, // SOS
		0x62,                         // compressed data
		0xFF, 0xD9,                   // EOI
	)
	return body
}

func TestStripTo_JPEG_RemovesEXIF(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "photo.jpg")
	dest := filepath.Join(dir, "photo_clean.jpg")
	if err := os.WriteFile(src, minimalJPEG(), 0644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	if err := StripTo(src, dest); err != nil {
		t.Fatalf("StripTo: %v", err)
	}

	cleaned, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}

	// Should still be a valid JPEG (SOI + SOS + ... + EOI)
	if len(cleaned) < 4 || cleaned[0] != 0xFF || cleaned[1] != 0xD8 {
		t.Error("missing SOI marker in stripped output")
	}
	if cleaned[len(cleaned)-2] != 0xFF || cleaned[len(cleaned)-1] != 0xD9 {
		t.Error("missing EOI marker in stripped output")
	}

	// Must be smaller than original (APP1 removed)
	if len(cleaned) >= len(minimalJPEG()) {
		t.Errorf("expected stripped file (%d bytes) to be smaller than original (%d bytes)", len(cleaned), len(minimalJPEG()))
	}
}

func TestStrip_OriginalBytesUnchanged(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "photo.jpg")
	origBytes := minimalJPEG()
	if err := os.WriteFile(src, origBytes, 0644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	if err := Strip(src); err != nil {
		t.Fatalf("Strip: %v", err)
	}

	reRead, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("re-read src: %v", err)
	}

	// After Strip, the file should be modified (EXIF removed), but the file
	// should still exist and be valid. Original bytes are not preserved by
	// Strip (it replaces the file), but StripTo preserves the original.
	if len(reRead) >= len(origBytes) {
		t.Errorf("expected stripped file (%d bytes) to be smaller than original (%d bytes)", len(reRead), len(origBytes))
	}
}

func TestStripTo_OriginalUnchanged(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "photo.jpg")
	dest := filepath.Join(dir, "photo_clean.jpg")
	origBytes := minimalJPEG()
	if err := os.WriteFile(src, origBytes, 0644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	if err := StripTo(src, dest); err != nil {
		t.Fatalf("StripTo: %v", err)
	}

	// Original must be byte-identical
	reRead, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("re-read src: %v", err)
	}
	if !bytesEqual(reRead, origBytes) {
		t.Error("original file was modified by StripTo")
	}
}

func TestStripTo_TruncatedJPEG_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "broken.jpg")
	// JPEG with SOI + APP1 but no SOS
	body := []byte{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x08, 0x45, 0x78, 0x69, 0x66, 0x00, 0x00}
	if err := os.WriteFile(src, body, 0644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	dest := filepath.Join(dir, "broken_clean.jpg")
	if err := StripTo(src, dest); err == nil {
		t.Error("expected error for truncated JPEG without SOS, got nil")
	}

	// Dest should not exist
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Error("dest file should not exist after failed strip")
	}
}

func TestStripTo_PNG_eXIf(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "image.png")
	// Minimal PNG with an eXIf chunk
	// PNG signature
	var png []byte
	png = append(png, 0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A) // sig
	// eXIf chunk (length 4, type "eXIf", data "test", CRC)
	exifChunk := buildPNGChunk("eXIf", []byte("test"))
	png = append(png, exifChunk...)
	// IEND chunk
	iend := buildPNGChunk("IEND", nil)
	png = append(png, iend...)

	if err := os.WriteFile(src, png, 0644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	dest := filepath.Join(dir, "clean.png")
	if err := StripTo(src, dest); err != nil {
		t.Fatalf("StripTo: %v", err)
	}

	cleaned, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}

	// Should not contain eXIf
	chunkType := string(cleaned[8:12])
	if chunkType == "eXIf" {
		t.Error("eXIf chunk should have been stripped")
	}

	// Original must be unchanged
	orig, _ := os.ReadFile(src)
	if !bytesEqual(orig, png) {
		t.Error("original file was modified")
	}
}

func TestStripTo_NonImage_ReturnsNil(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "text.txt")
	dest := filepath.Join(dir, "text_out.txt")
	if err := os.WriteFile(src, []byte("hello"), 0644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := StripTo(src, dest); err != nil {
		t.Errorf("expected nil for non-image, got %v", err)
	}
	// Dest should not be created for non-image
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Error("dest should not exist for non-image")
	}
}

func TestStripTo_NonexistentFile_ReturnsError(t *testing.T) {
	err := StripTo("/nonexistent/path.jpg", "/tmp/out.jpg")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func buildPNGChunk(chunkType string, data []byte) []byte {
	length := uint32(len(data))
	var chunk []byte
	chunk = append(chunk, byte(length>>24), byte(length>>16), byte(length>>8), byte(length))
	chunk = append(chunk, []byte(chunkType)...)
	chunk = append(chunk, data...)
	// CRC over chunk type + data (simplified — not validating)
	crc := make([]byte, 4)
	chunk = append(chunk, crc...)
	return chunk
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
