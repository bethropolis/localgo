package model_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bethropolis/localgo/pkg/model"
)

func TestNewFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.txt")
	content := "hello world"

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	file, err := model.NewFile(filePath)
	if err != nil {
		t.Fatalf("NewFile returned error: %v", err)
	}

	if file.Name != "test.txt" {
		t.Errorf("expected Name 'test.txt', got '%s'", file.Name)
	}

	if file.Size != int64(len(content)) {
		t.Errorf("expected Size %d, got %d", len(content), file.Size)
	}

	if file.Path != filePath {
		t.Errorf("expected Path '%s', got '%s'", filePath, file.Path)
	}

	if file.Type != "text/plain; charset=utf-8" {
		t.Errorf("expected Type 'text/plain; charset=utf-8', got '%s'", file.Type)
	}

	expectedHash := sha256.Sum256([]byte(content))
	expectedHashHex := hex.EncodeToString(expectedHash[:])

	if file.SHA256 != expectedHashHex {
		t.Errorf("expected SHA256 '%s', got '%s'", expectedHashHex, file.SHA256)
	}

	if file.LastModified == 0 {
		t.Errorf("expected LastModified to be non-zero")
	}
}

func TestNewFile_NonExistent(t *testing.T) {
	_, err := model.NewFile("/non/existent/file.txt")
	if err == nil {
		t.Errorf("expected error for non-existent file, got nil")
	}
}

func TestToFileDto(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.md")
	content := "# markdown"
	os.WriteFile(filePath, []byte(content), 0644)

	file, _ := model.NewFile(filePath)
	dto := file.ToFileDto()

	if dto.ID != file.ID {
		t.Errorf("expected ID '%s', got '%s'", file.ID, dto.ID)
	}

	if dto.FileName != "test.md" {
		t.Errorf("expected FileName 'test.md', got '%s'", dto.FileName)
	}

	if dto.Size != int64(len(content)) {
		t.Errorf("expected Size %d, got %d", len(content), dto.Size)
	}

	if dto.FileType != "text/markdown; charset=utf-8" && dto.FileType != "text/markdown" {
		t.Errorf("expected FileType 'text/markdown' or 'text/markdown; charset=utf-8', got '%s'", dto.FileType)
	}

	if dto.SHA256 == nil || *dto.SHA256 != file.SHA256 {
		t.Errorf("expected SHA256 %v, got %v", file.SHA256, dto.SHA256)
	}

	if dto.Metadata == nil {
		t.Fatalf("expected Metadata to not be nil")
	}

	if dto.Metadata.Modified == nil {
		t.Errorf("expected Metadata.Modified to not be nil")
	} else {
		// Verify it parses back to the correct time roughly
		parsedTime, err := time.Parse(time.RFC3339, *dto.Metadata.Modified)
		if err != nil {
			t.Errorf("failed to parse modified time %s: %v", *dto.Metadata.Modified, err)
		} else {
			// Convert back from UnixMilli to check
			expectedTime := time.Unix(0, file.LastModified).Truncate(time.Second) // RFC3339 precision issues might happen
			if !parsedTime.Truncate(time.Second).Equal(expectedTime) {
				t.Errorf("expected parsed time %v, got %v", expectedTime, parsedTime)
			}
		}
	}
}

func TestDetermineFileType_Fallback(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		ext      string
		expected string
	}{
		{".unknown", "application/octet-stream"},
		{".apk", "application/vnd.android.package-archive"},
	}

	for _, tt := range tests {
		filePath := filepath.Join(tempDir, "test"+tt.ext)
		os.WriteFile(filePath, []byte("data"), 0644)

		file, _ := model.NewFile(filePath)
		if file.Type != tt.expected {
			t.Errorf("expected %s for %s, got %s", tt.expected, tt.ext, file.Type)
		}
	}
}
