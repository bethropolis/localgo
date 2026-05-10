package cmd

import (
	"fmt"
	"crypto/tls"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Run a health check against the local server",
	Long:  `Exits 0 if the server is responding on the configured port, exits 1 otherwise. Useful for container HEALTHCHECK directives.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port := getServePort()
		url := fmt.Sprintf("https://127.0.0.1:%d/api/v2/localhost/info", port)

		tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
		client := &http.Client{Timeout: 3 * time.Second, Transport: tr}
		resp, err := client.Get(url)
		if err != nil {
			return fmt.Errorf("health check failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("health check returned status %d", resp.StatusCode)
		}
		return nil
	},
}

func getServePort() int {
	if port := os.Getenv("LOCALSEND_PORT"); port != "" {
		var p int
		if _, err := fmt.Sscanf(port, "%d", &p); err == nil && p > 0 {
			return p
		}
	}
	return 53317
}
