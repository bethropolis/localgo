package storage

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

// EnsureDirExists creates a directory if it doesn't exist.
func EnsureDirExists(path string) error {
	err := os.MkdirAll(path, 0755) // Use appropriate permissions
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}
	return nil
}

// SaveStreamToFile saves an io.Reader stream to a specified file path.
// It creates necessary directories.
// It reports progress via the onProgress callback (bytes written).
func SaveStreamToFile(stream io.Reader, filePath string, onProgress func(bytesWritten int64)) error {
	dir := filepath.Dir(filePath)
	if err := EnsureDirExists(dir); err != nil {
		return err // Error creating directory
	}

	// Create the destination file
	outFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer outFile.Close()

	// Use io.Copy with a custom writer to track progress
	progressWriter := &ProgressWriter{
		Writer:     outFile,
		OnProgress: onProgress,
	}

	_, err = io.Copy(progressWriter, stream)
	if err != nil {
		// Attempt to remove partially written file on error
		outFile.Close() // Close before removing
		if removeErr := os.Remove(filePath); removeErr != nil {
			log.Printf("Warning: Failed to remove partially written file %s: %v", filePath, removeErr)
		}
		return fmt.Errorf("failed to copy stream to file %s: %w", filePath, err)
	}

	log.Printf("Successfully saved stream to %s", filePath)
	return nil
}

// ProgressWriter is a wrapper around io.Writer that calls a callback on Write.
type ProgressWriter struct {
	Writer       io.Writer
	BytesWritten int64
	OnProgress   func(bytesWritten int64)
}

// Write implements the io.Writer interface.
func (pw *ProgressWriter) Write(p []byte) (n int, err error) {
	n, err = pw.Writer.Write(p)
	pw.BytesWritten += int64(n)
	if pw.OnProgress != nil {
		pw.OnProgress(pw.BytesWritten) // Report progress
	}
	return n, err
}
