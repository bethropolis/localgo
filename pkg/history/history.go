// Package history provides an append-only JSONL transfer history log.
package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// Status values for a history entry.
const (
	StatusReceived   = "received"
	StatusClipboard  = "clipboard"
	StatusFailed     = "failed"
	DisabledSentinel = "off"
)

// Entry represents a single file transfer event.
type Entry struct {
	Timestamp   time.Time `json:"timestamp"`
	SenderAlias string    `json:"sender_alias"`
	SenderIP    string    `json:"sender_ip"`
	FileName    string    `json:"file_name"`
	FilePath    string    `json:"file_path"`
	FileSize    int64     `json:"file_size"`
	FileType    string    `json:"file_type"`
	Status      string    `json:"status"`
}

// Logger writes transfer history entries to an append-only JSONL file.
type Logger struct {
	mu   sync.Mutex
	file *os.File
	enc  *json.Encoder
}

// NewLogger opens (or creates) the JSONL history file at path.
// The directory is created if it does not exist.
func NewLogger(path string) (*Logger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("history: create directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("history: open file: %w", err)
	}
	enc := json.NewEncoder(f)
	return &Logger{file: f, enc: enc}, nil
}

// Log appends one entry to the JSONL file. It is safe for concurrent use.
func (l *Logger) Log(e Entry) error {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.enc.Encode(e); err != nil {
		return fmt.Errorf("history: encode entry: %w", err)
	}
	return nil
}

// Close closes the underlying file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}

// DefaultPath returns the platform-appropriate default history file path.
//
// Resolution order:
//  1. $XDG_DATA_HOME/localgo/history.jsonl  (all platforms)
//  2. $HOME/.local/share/localgo/history.jsonl  (Unix)
//  3. %APPDATA%\localgo\history.jsonl  (Windows)
//  4. localgo-history.jsonl  (current directory, last resort)
func DefaultPath() string {
	if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
		return filepath.Join(dataHome, "localgo", "history.jsonl")
	}
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".local", "share", "localgo", "history.jsonl")
	}
	// Windows: HOME is usually unset; APPDATA points to the roaming profile.
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "localgo", "history.jsonl")
		}
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			return filepath.Join(userProfile, "AppData", "Roaming", "localgo", "history.jsonl")
		}
	}
	return "localgo-history.jsonl"
}