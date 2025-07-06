package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// File represents a file to be shared or received
type File struct {
	ID           string
	Name         string
	Path         string
	Size         int64
	Type         string
	SHA256       string
	LastModified int64
}

// NewFile creates a File instance from a file path
func NewFile(path string) (*File, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	fileType := determineFileType(path)

	file := &File{
		ID:           generateID(path),
		Name:         filepath.Base(path),
		Path:         path,
		Size:         info.Size(),
		Type:         fileType,
		LastModified: info.ModTime().UnixMilli(),
	}

	// Calculate SHA-256 hash asynchronously for large files
	if file.Size < 50*1024*1024 { // Only pre-calculate for files < 50MB
		hash, err := calculateSHA256(path)
		if err == nil {
			file.SHA256 = hash
		}
	}

	return file, nil
}

// ToFileDto converts a File to a FileDto for API responses
func (f *File) ToFileDto() FileDto {
	var sha256Ptr *string
	if f.SHA256 != "" {
		sha256Ptr = &f.SHA256 // Assign address if SHA256 is calculated
	}
	// TODO: Populate Metadata field if needed
	return FileDto{
		ID:       f.ID,
		FileName: f.Name,
		Size:     f.Size,
		FileType: f.Type,
		SHA256:   sha256Ptr, // Use the pointer
		// LastModified removed as it's no longer in FileDto
	}
}

// generateID creates a unique ID for a file based on path and timestamp
func generateID(path string) string {
	timestamp := time.Now().UnixNano()
	data := []byte(fmt.Sprintf("%s-%d", path, timestamp))
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:8]) // Use first 8 bytes for a shorter ID
}

// determineFileType attempts to determine the file type from extension or content
func determineFileType(path string) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp":
		return "image"
	case ".mp4", ".mov", ".avi", ".mkv", ".webm":
		return "video"
	case ".mp3", ".wav", ".ogg", ".flac", ".aac":
		return "audio"
	case ".pdf":
		return "pdf"
	case ".txt", ".md", ".rtf":
		return "text"
	case ".zip", ".tar", ".gz", ".rar", ".7z":
		return "archive"
	case ".apk":
		return "app"
	default:
		return "unknown"
	}
}

// calculateSHA256 calculates the SHA-256 hash of a file
func calculateSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
