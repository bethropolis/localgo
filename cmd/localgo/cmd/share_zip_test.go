package cmd

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestZipDirToTemp(t *testing.T) {
	src := t.TempDir()
	contents := map[string]string{
		"a.txt":            "hello",
		"sub/b.txt":        "world",
		"sub/nested/c.txt": "!",
	}
	for rel, body := range contents {
		p := filepath.Join(src, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}

	zipPath, err := zipDirToTemp(src)
	if err != nil {
		t.Fatalf("zipDirToTemp failed: %v", err)
	}
	t.Cleanup(func() { os.Remove(zipPath) })

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("failed to open produced zip: %v", err)
	}
	defer zr.Close()

	if len(zr.File) != len(contents) {
		t.Fatalf("expected %d entries, got %d", len(contents), len(zr.File))
	}
	for _, f := range zr.File {
		t.Logf("entry: %s", f.Name)
	}
}

func TestZipDirToTempMissingDir(t *testing.T) {
	if _, err := zipDirToTemp(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("expected error for missing directory, got nil")
	}
}

func TestZipArchiveName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"./my-folder", "my-folder.zip"},
		{"/tmp/docs/", "docs.zip"},
		{".", "archive.zip"},
		{"/", "archive.zip"},
	}
	for _, tt := range tests {
		if got := zipArchiveName(tt.in); got != tt.want {
			t.Errorf("zipArchiveName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
