package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/bethropolis/localgo/pkg/discovery"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/network"
	"github.com/stretchr/testify/assert"
)

// waitForDevice waits for a device with the given alias to be discoverable via HTTP.
func waitForDevice(ctx context.Context, alias string, port int, timeout time.Duration) error {
	discoverer := discovery.NewHTTPDiscovery(nil, model.RegisterDto{}, nil) // No need for own device info for scanning
	ticker := time.NewTicker(200 * time.Millisecond)                        // Check more frequently
	defer ticker.Stop()

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout waiting for device %s", alias)
		case <-ticker.C:
			// Check localhost first for testing
			device, err := discoverer.FetchDeviceInfo(timeoutCtx, net.ParseIP("127.0.0.1"), port)
			if err == nil && device.Alias == alias {
				return nil
			}

			// Also check other local IPs
			ips, err := network.GetLocalIPAddresses()
			if err != nil {
				continue
			}
			for _, ip := range ips {
				device, err := discoverer.FetchDeviceInfo(timeoutCtx, ip, port)
				if err == nil && device.Alias == alias {
					return nil
				}
			}
		}
	}
}

func TestSendFile(t *testing.T) {
	// Construct binary name based on OS
	binName := "localgo-cli"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	
	// Create temp dir for binary
	tmpBinDir, err := os.MkdirTemp("", "localgo-bin-")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpBinDir)
	
	binPath := filepath.Join(tmpBinDir, binName)

	// Build the binary first
	// We assume we are running from cmd/localgo-cli directory or root.
	// If running via `go test ./...` from root, WD is the package dir.
	
	wd, _ := os.Getwd()
	
	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	buildCmd.Dir = wd // Build the package in current directory
	
	output, err := buildCmd.CombinedOutput()
	assert.NoError(t, err, "Failed to build localgo-cli binary: %s", string(output))

	// Create a temporary directory for downloads
	tmpDownloadsDir, err := os.MkdirTemp("", "localgo-downloads-")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDownloadsDir)

	// 1. Create a temporary file to send
	tmpfile, err := os.CreateTemp("", "testfile.txt")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	content := []byte("hello, world")
	_, err = tmpfile.Write(content)
	assert.NoError(t, err)
	tmpfile.Close()

	// 2. Start a localgo-cli server in the background
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	serverCmd := exec.CommandContext(ctx, binPath, "serve", "--port", "53317", "--http")
	serverCmd.Env = append(os.Environ(), fmt.Sprintf("LOCALSEND_DOWNLOAD_DIR=%s", tmpDownloadsDir), "LOCALSEND_ALIAS=GoDevice")
	// Clean env to avoid interference? Maybe not needed.
	
	// Separate stdout/stderr for debugging if needed, or pipe to test output
	// serverCmd.Stdout = os.Stdout 
	// serverCmd.Stderr = os.Stderr

	err = serverCmd.Start()
	assert.NoError(t, err)
	
	// Ensure we kill the process
	defer func() {
		if serverCmd.Process != nil {
			_ = serverCmd.Process.Kill()
			_ = serverCmd.Wait()
		}
	}()

	// Wait for the server to be discoverable via HTTP
	err = waitForDevice(ctx, "GoDevice", 53317, 10*time.Second)
	assert.NoError(t, err, "Server did not become discoverable")

	// Give a bit more time for the server to be fully ready
	time.Sleep(1 * time.Second)

	// 3. Use the send command to send the file to the server
	sendCmd := exec.CommandContext(ctx, binPath, "send", "--file", tmpfile.Name(), "--to", "GoDevice", "--port", "53317")
	sendCmd.Env = append(os.Environ(), "LOCALSEND_ALIAS=GoSender")
	
	output, err = sendCmd.CombinedOutput()
	assert.NoError(t, err, "Send command failed: %s", string(output))

	// Allow some time for the file transfer to complete
	time.Sleep(2 * time.Second)

	// 4. Verify that the file is received correctly
	receivedFilePath := filepath.Join(tmpDownloadsDir, filepath.Base(tmpfile.Name()))
	assert.FileExists(t, receivedFilePath)

	receivedContent, err := os.ReadFile(receivedFilePath)
	assert.NoError(t, err)
	assert.Equal(t, content, receivedContent)
}
