package storage

import (
	"go.uber.org/zap"
	"os"
	"strings"
	"testing"
	"time"
)

var testLogger = zap.NewNop().Sugar()

func TestEnsureDirExists(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := tmpDir + "/subdir"

	err := EnsureDirExists(subDir)
	if err != nil {
		t.Fatalf("EnsureDirExists failed: %v", err)
	}

	info, err := os.Stat(subDir)
	if err != nil {
		t.Fatalf("Failed to stat directory: %v", err)
	}

	if !info.IsDir() {
		t.Error("Expected directory to exist")
	}
}

func TestEnsureDirExists_ExistingDir(t *testing.T) {
	tmpDir := t.TempDir()

	err := EnsureDirExists(tmpDir)
	if err != nil {
		t.Fatalf("EnsureDirExists failed: %v", err)
	}
}

func TestSaveStreamToFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := tmpDir + "/test.txt"
	content := "Hello, World!"

	err := SaveStreamToFile(strings.NewReader(content), filePath, nil)
	if err != nil {
		t.Fatalf("SaveStreamToFile failed: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(data) != content {
		t.Errorf("Expected content '%s', got '%s'", content, string(data))
	}
}

func TestSaveStreamToFileWithMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := tmpDir + "/test.txt"
	content := "Hello, World!"

	modTime := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	accTime := time.Now().Add(-12 * time.Hour).Format(time.RFC3339)

	err := SaveStreamToFileWithMetadata(
		strings.NewReader(content),
		filePath,
		&modTime,
		&accTime,
		nil,
		testLogger,
	)
	if err != nil {
		t.Fatalf("SaveStreamToFileWithMetadata failed: %v", err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	expectedModTime, _ := time.Parse(time.RFC3339, modTime)
	if !info.ModTime().Equal(expectedModTime) {
		t.Errorf("Expected mod time %v, got %v", expectedModTime, info.ModTime())
	}
}

func TestSaveStreamToFile_ProgressCallback(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := tmpDir + "/test.txt"
	content := "Hello, World!"

	var bytesWritten int64
	progressFn := func(bw int64) {
		bytesWritten = bw
	}

	err := SaveStreamToFile(strings.NewReader(content), filePath, progressFn)
	if err != nil {
		t.Fatalf("SaveStreamToFile failed: %v", err)
	}

	if bytesWritten != int64(len(content)) {
		t.Errorf("Expected bytesWritten %d, got %d", len(content), bytesWritten)
	}
}

func TestSaveStreamToFile_NestedDir(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := tmpDir + "/nested/dir/test.txt"
	content := "Nested content"

	err := SaveStreamToFile(strings.NewReader(content), filePath, nil)
	if err != nil {
		t.Fatalf("SaveStreamToFile failed: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(data) != content {
		t.Errorf("Expected content '%s', got '%s'", content, string(data))
	}
}

func TestSaveStreamToFile_InvalidTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := tmpDir + "/test.txt"
	content := "Test content"

	invalidTime := "not-a-timestamp"

	err := SaveStreamToFileWithMetadata(
		strings.NewReader(content),
		filePath,
		&invalidTime,
		nil,
		nil,
		testLogger,
	)
	if err != nil {
		t.Fatalf("SaveStreamToFileWithMetadata failed: %v", err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	now := time.Now()
	if now.Sub(info.ModTime()) > time.Minute {
		t.Error("Expected timestamp to fallback to current time for invalid input")
	}
}
