package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"
)

var dockerStartCmd = &cobra.Command{
	Use:   "docker-start",
	Short: "Set up permissions and drop privileges before running serve",
	Long: `Intended to be used as the container ENTRYPOINT.

It reads PUID and PGID from the environment (defaulting to 1000),
chowns /app/downloads and /app/config to that UID/GID, drops privileges
via syscall.Setuid/setgid, and then execs "localgo serve".

On non-Linux platforms or when already running as a non-root user, it
execs serve directly without any permission changes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if os.Getuid() != 0 {
			return execServe(args)
		}

		if runtime.GOOS != "linux" {
			return execServe(args)
		}

		puid := getEnvInt("PUID", 1000)
		pgid := getEnvInt("PGID", 1000)

		dirs := []string{"/app/downloads", "/app/config"}
		for _, dir := range dirs {
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "docker-start: failed to create %s: %v\n", dir, err)
				return err
			}
			if err := os.Chown(dir, puid, pgid); err != nil {
				fmt.Fprintf(os.Stderr, "docker-start: chown %s: %v\n", dir, err)
				return err
			}
		}

		if err := syscall.Setgid(pgid); err != nil {
			fmt.Fprintf(os.Stderr, "docker-start: Setgid(%d): %v\n", pgid, err)
			return err
		}
		if err := syscall.Setuid(puid); err != nil {
			fmt.Fprintf(os.Stderr, "docker-start: Setuid(%d): %v\n", puid, err)
			return err
		}

		fmt.Printf("docker-start: running as UID %d / GID %d\n", puid, pgid)
		return execServe(args)
	},
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return def
}

func execServe(args []string) error {
	serveArgv := []string{"localgo", "serve"}
	if len(args) > 0 {
		serveArgv = append([]string{"localgo"}, args...)
	}

	bin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine binary path: %w", err)
	}

	env := os.Environ()
	return syscall.Exec(bin, serveArgv, env)
}

func init() {
	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(dockerStartCmd)
}

func resolveBinaryPath() string {
	exe, err := os.Executable()
	if err == nil {
		return exe
	}
	if path, err := os.Readlink("/proc/self/exe"); err == nil {
		return path
	}
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		full := filepath.Join(p, "localgo")
		if info, err := os.Stat(full); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return full
		}
	}
	return "localgo"
}
