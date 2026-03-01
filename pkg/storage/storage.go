package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
)

// EnsureDirExists creates a directory if it doesn't exist.
func EnsureDirExists(path string) error {
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}
	return nil
}

// SaveStreamToFile saves an io.Reader stream to a specified file path.
// It creates necessary directories.
// It reports progress via the onProgress callback (bytes written).
func SaveStreamToFile(stream io.Reader, filePath string, onProgress func(bytesWritten int64)) error {
	return SaveStreamToFileWithMetadata(stream, filePath, nil, nil, onProgress, nil)
}

// SaveStreamToFileWithMetadata saves an io.Reader stream and restores optional timestamps.
func SaveStreamToFileWithMetadata(stream io.Reader, filePath string, modified *string, accessed *string, onProgress func(bytesWritten int64), logger *zap.SugaredLogger) error {
	dir := filepath.Dir(filePath)
	if err := EnsureDirExists(dir); err != nil {
		return err
	}

	outFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer outFile.Close()

	progressWriter := &ProgressWriter{
		Writer:     outFile,
		OnProgress: onProgress,
	}

	_, err = io.Copy(progressWriter, stream)
	if err != nil {
		outFile.Close()
		if removeErr := os.Remove(filePath); removeErr != nil {
			if logger != nil {
				logger.Warnw("Failed to remove partially written file", "path", filePath, "error", removeErr)
			}
		}
		return fmt.Errorf("failed to copy stream to file %s: %w", filePath, err)
	}

	outFile.Close()

	if modified != nil || accessed != nil {
		mtime := time.Now()
		atime := time.Now()

		if modified != nil {
			if t, err := time.Parse(time.RFC3339, *modified); err == nil {
				mtime = t
			} else {
				if logger != nil {
					logger.Warnw("Failed to parse modified time", "time", *modified, "error", err)
				}
			}
		}

		if accessed != nil {
			if t, err := time.Parse(time.RFC3339, *accessed); err == nil {
				atime = t
			} else if modified != nil {
				atime = mtime
			} else {
				if logger != nil {
					logger.Warnw("Failed to parse accessed time", "time", *accessed, "error", err)
				}
			}
		}

		if err := os.Chtimes(filePath, atime, mtime); err != nil {
			if logger != nil {
				logger.Warnw("Failed to apply timestamps", "path", filePath, "error", err)
			}
		}
	}

	if logger != nil {
		logger.Infow("Successfully saved stream", "path", filePath)
	}
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
		pw.OnProgress(pw.BytesWritten)
	}
	return n, err
}
