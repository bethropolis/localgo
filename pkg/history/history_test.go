package history_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bethropolis/localgo/pkg/history"
)

func TestHistoryLogger(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "history.jsonl")

	logger, err := history.NewLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	entry1 := history.Entry{
		SenderAlias: "Alice",
		SenderIP:    "192.168.1.5",
		FileName:    "file1.txt",
		FilePath:    "/tmp/file1.txt",
		FileSize:    1234,
		FileType:    "text/plain",
		Status:      history.StatusReceived,
	}

	if err := logger.Log(entry1); err != nil {
		t.Fatalf("failed to log entry: %v", err)
	}

	logger.Close()

	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("failed to open log file: %v", err)
	}
	defer f.Close()

	var readEntry history.Entry
	if err := json.NewDecoder(f).Decode(&readEntry); err != nil {
		t.Fatalf("failed to decode JSON from file: %v", err)
	}

	if readEntry.SenderAlias != "Alice" || readEntry.Status != history.StatusReceived {
		t.Errorf("read entry does not match written entry: %+v", readEntry)
	}
	if readEntry.Timestamp.IsZero() {
		t.Errorf("expected timestamp to be auto-populated")
	}
}

func TestDefaultPath(t *testing.T) {
	// filepath.FromSlash converts forward slashes to the OS path separator.
	// This makes the expected strings correct on both Unix (no-op) and
	// Windows (converts / to \).

	t.Run("XDG_DATA_HOME", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", filepath.FromSlash("/custom/xdg"))
		t.Setenv("HOME", "")

		got := history.DefaultPath()
		want := filepath.Join(filepath.FromSlash("/custom/xdg"), "localgo", "history.jsonl")
		if got != want {
			t.Errorf("XDG path\n got:  %s\n want: %s", got, want)
		}
	})

	t.Run("HOME fallback", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		t.Setenv("HOME", filepath.FromSlash("/custom/home"))

		got := history.DefaultPath()
		want := filepath.Join(filepath.FromSlash("/custom/home"), ".local", "share", "localgo", "history.jsonl")
		if got != want {
			t.Errorf("HOME path\n got:  %s\n want: %s", got, want)
		}
	})
}