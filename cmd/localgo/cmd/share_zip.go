package cmd

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func zipDirToTemp(dir string) (string, error) {
	baseName := filepath.Base(dir)
	if baseName == "." || baseName == "/" {
		baseName = "archive"
	}
	zipFile, err := os.CreateTemp("", "localgo-"+baseName+"-*.zip")
	if err != nil {
		return "", fmt.Errorf("failed to create temp zip: %w", err)
	}
	zipPathName := zipFile.Name()
	zipWriter := zip.NewWriter(zipFile)

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			rel = info.Name()
		}
		rel = filepath.ToSlash(rel)

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = rel
		header.Method = zip.Deflate

		w, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		err = func() error {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(w, f)
			return err
		}()
		return err
	})
	if err != nil {
		zipWriter.Close()
		zipFile.Close()
		os.Remove(zipPathName)
		return "", err
	}

	if err := zipWriter.Close(); err != nil {
		zipFile.Close()
		os.Remove(zipPathName)
		return "", err
	}

	if err := zipFile.Close(); err != nil {
		os.Remove(zipPathName)
		return "", err
	}

	return zipPathName, nil
}
