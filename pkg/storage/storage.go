package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Thread-safe pool of 32KB buffers for stream copies, reducing GC churn.
var copyBufferPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 32*1024)
		return &b
	},
}

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

	// Retrieve a pre-allocated copy buffer from the pool
	bufPtr := copyBufferPool.Get().(*[]byte)
	defer copyBufferPool.Put(bufPtr)

	_, err = io.CopyBuffer(progressWriter, stream, *bufPtr)
	if err != nil {
		outFile.Close()
		if removeErr := os.Remove(filePath); removeErr != nil {
			if logger != nil {
				logger.Warnw("Failed to remove partially written file", "path", filePath, "error", removeErr)
			}
		}
		return fmt.Errorf("failed to copy stream to file %s: %w", filePath, err)
	}

	if closeErr := outFile.Close(); closeErr != nil {
		if removeErr := os.Remove(filePath); removeErr != nil && logger != nil {
			logger.Warnw("Failed to remove incomplete file after close error", "path", filePath, "error", removeErr)
		}
		return fmt.Errorf("failed to close and flush file %s: %w", filePath, closeErr)
	}

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

// ResolveDuplicateFilename finds an available filename by appending numbers if the file exists.
func ResolveDuplicateFilename(dir, baseName string) string {
	ext := filepath.Ext(baseName)
	nameWithoutExt := strings.TrimSuffix(baseName, ext)

	candidate := filepath.Join(dir, baseName)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}

	for i := 1; i <= 999; i++ {
		newName := fmt.Sprintf("%s (%d)%s", nameWithoutExt, i, ext)
		candidate = filepath.Join(dir, newName)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}

	// Fallback to avoid silent overwrite if (1) through (999) are all taken
	randomBytes := make([]byte, 3)
	rand.Read(randomBytes)
	newName := fmt.Sprintf("%s_%s%s", nameWithoutExt, hex.EncodeToString(randomBytes), ext)
	return filepath.Join(dir, newName)
}
