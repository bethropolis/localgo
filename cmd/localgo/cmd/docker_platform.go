//go:build !linux

package cmd

import (
	"fmt"
	"os"
	"strconv"
)

func setGIDAndUID(pgid, puid int) error {
	return nil
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return def
}

func execBinary(bin string, args []string, env []string) error {
	return fmt.Errorf("exec not supported on this platform")
}

func chownPath(path string, uid, gid int) error {
	return os.Chown(path, uid, gid)
}