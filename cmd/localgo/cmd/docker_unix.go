//go:build linux

package cmd

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
)

func dropPrivileges(pgid, puid int) error {
	if err := syscall.Setgid(pgid); err != nil {
		fmt.Fprintf(os.Stderr, "docker-start: Setgid(%d): %v\n", pgid, err)
		return err
	}
	if err := syscall.Setuid(puid); err != nil {
		fmt.Fprintf(os.Stderr, "docker-start: Setuid(%d): %v\n", puid, err)
		return err
	}
	return nil
}

func setGIDAndUID(pgid, puid int) error {
	return dropPrivileges(pgid, puid)
}

func execBinary(bin string, args []string, env []string) error {
	return syscall.Exec(bin, args, env)
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return def
}