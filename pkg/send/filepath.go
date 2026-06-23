package send

import (
	"os"
	"path/filepath"
)

func getFilesWithRelativePaths(paths []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, p := range paths {
		p = filepath.Clean(p)
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			baseDir := filepath.Dir(p)
			err = filepath.Walk(p, func(path string, fInfo os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !fInfo.IsDir() {
					rel, err := filepath.Rel(baseDir, path)
					if err == nil {
						result[path] = filepath.ToSlash(rel)
					} else {
						result[path] = filepath.Base(path)
					}
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
		} else {
			result[p] = filepath.Base(p)
		}
	}
	return result, nil
}
