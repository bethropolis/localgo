package storage

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Thread-safe pool of 32KB buffers for small files.
var smallBufferPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 32*1024)
		return &b
	},
}

// Thread-safe pool of 256KB buffers for files over 10MB, reducing syscall overhead on fast LANs.
var largeBufferPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 256*1024)
		return &b
	},
}

// CheckFreeSpace returns the available bytes on the volume containing the specified path.
func CheckFreeSpace(dirPath string) (uint64, error) {
	cleanPath := filepath.Clean(dirPath)
	if err := os.MkdirAll(cleanPath, 0755); err != nil {
		return 0, fmt.Errorf("failed to verify path: %w", err)
	}
	return getAvailableBytes(cleanPath)
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
	return SaveStreamToFileWithMetadata(stream, filePath, 0, nil, nil, nil, onProgress, nil)
}

// SaveStreamToFileWithMetadata saves an io.Reader stream and restores optional timestamps.
// If expectedSha256 is provided, the stream is verified against it after the copy succeeds.
// fileSize is used to select an optimal copy buffer size.
func SaveStreamToFileWithMetadata(stream io.Reader, filePath string, fileSize int64, modified *string, accessed *string, expectedSha256 *string, onProgress func(bytesWritten int64), logger *zap.SugaredLogger) error {
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

	// Select buffer pool based on file size
	pool := &smallBufferPool
	if fileSize > 10*1024*1024 {
		pool = &largeBufferPool
	}
	bufPtr := pool.Get().(*[]byte)
	defer pool.Put(bufPtr)

	// Optional SHA-256 hashing via TeeReader
	var hasher hash.Hash
	var hashingReader io.Reader = stream
	if expectedSha256 != nil && *expectedSha256 != "" {
		hasher = sha256.New()
		hashingReader = io.TeeReader(stream, hasher)
	}

	_, err = io.CopyBuffer(progressWriter, hashingReader, *bufPtr)
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

	// Verify SHA-256 checksum if the sender provided one
	if hasher != nil {
		calculatedHash := hex.EncodeToString(hasher.Sum(nil))
		if calculatedHash != *expectedSha256 {
			if removeErr := os.Remove(filePath); removeErr != nil && logger != nil {
				logger.Warnw("Failed to remove corrupted file", "path", filePath, "error", removeErr)
			}
			return fmt.Errorf("integrity violation: SHA-256 mismatch (got %s, expected %s)", calculatedHash, *expectedSha256)
		}
		if logger != nil {
			logger.Infow("SHA-256 integrity verified", "path", filePath)
		}
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
