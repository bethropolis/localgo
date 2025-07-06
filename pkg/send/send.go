package send

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/discovery"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/network"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// SendFile sends a file to a recipient.
func SendFile(ctx context.Context, cfg *config.Config, filePath string, recipientAlias string, recipientPort int) error {
	logrus.Infof("Searching for recipient '%s'...", recipientAlias)

	// Use default port if not specified
	if recipientPort == 0 {
		recipientPort = config.DefaultPort
	}

	var targetDevice *model.Device
	// Retry discovery for a few seconds
	retryCtx, cancelRetry := context.WithTimeout(ctx, 15*time.Second)
	defer cancelRetry()

	for targetDevice == nil {
		select {
		case <-retryCtx.Done():
			return fmt.Errorf("recipient '%s' not found after multiple attempts", recipientAlias)
		default:
			// Use HTTP discovery with explicit port and IP scanning
			registerDto := model.RegisterDto{
				Alias:       cfg.Alias,
				Version:     config.ProtocolVersion,
				DeviceModel: cfg.DeviceModel,
				DeviceType:  cfg.DeviceType,
				Fingerprint: cfg.SecurityContext.CertificateHash,
				Port:        cfg.Port,
				Protocol:    model.ProtocolTypeHTTP, // Use HTTP for discovery
			}

			httpDiscoverer := discovery.NewHTTPDiscovery(nil, registerDto, nil)

			// Get local IPs for scanning
			ips, err := network.GetLocalIPAddresses()
			if err != nil {
				logrus.Warnf("Failed to get local IPs: %v", err)
				time.Sleep(500 * time.Millisecond)
				continue
			}

			// Add localhost for testing
			ips = append(ips, net.ParseIP("127.0.0.1"))

			discoverCtx, cancel := context.WithTimeout(retryCtx, 3*time.Second)
			foundDevices, err := httpDiscoverer.ScanNetwork(discoverCtx, ips, recipientPort)
			cancel()

			if err != nil {
				logrus.Warnf("HTTP discovery failed: %v", err)
				time.Sleep(500 * time.Millisecond)
				continue
			}

			logrus.Infof("Found %d devices during scan attempt.", len(foundDevices))

			// Find the target device
			for _, device := range foundDevices {
				logrus.Infof("Checking device: %s", device.ToDebugString())
				if device.Alias == recipientAlias {
					targetDevice = device
					break
				}
			}

			if targetDevice == nil {
				logrus.Infof("Recipient '%s' not found in this scan attempt. Retrying...", recipientAlias)
				time.Sleep(500 * time.Millisecond)
			}
		}
	}

	logrus.Infof("Found recipient: %s", targetDevice.ToDebugString())
	return sendToDevice(ctx, targetDevice, filePath)
}

func sendToDevice(ctx context.Context, device *model.Device, filePath string) error {
	client := &http.Client{}
	scheme := "http"

	// Configure client and scheme based on discovered device protocol
	if device.Protocol == model.ProtocolTypeHTTPS {
		scheme = "https"
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client.Transport = tr
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	fileDto := model.FileDto{
		ID:       uuid.NewString(),
		FileName: filepath.Base(filePath),
		Size:     fileInfo.Size(),
		FileType: http.DetectContentType([]byte{}), // This is not ideal, but we'll fix it later
	}

	prepareDto := model.PrepareUploadRequestDto{
		Info: model.InfoDto{
			Alias:       device.Alias,
			Version:     device.Version,
			DeviceModel: device.DeviceModel,
			DeviceType:  device.DeviceType,
			Fingerprint: device.Fingerprint,
			Download:    device.Download,
		},
		Files: map[string]model.FileDto{
			fileDto.ID: fileDto,
		},
	}

	jsonData, err := json.Marshal(prepareDto)
	if err != nil {
		return fmt.Errorf("failed to marshal prepare dto: %w", err)
	}

	url := fmt.Sprintf("%s://%s:%d/api/localsend/v2/prepare-upload", scheme, device.IP, device.Port)
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send prepare request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("prepare request failed with status: %s", resp.Status)
	}

	var prepareResponse model.PrepareUploadResponseDto
	if err := json.NewDecoder(resp.Body).Decode(&prepareResponse); err != nil {
		return fmt.Errorf("failed to decode prepare response: %w", err)
	}

	return uploadFile(ctx, client, device, filePath, fileDto.ID, prepareResponse.SessionID, prepareResponse.Files[fileDto.ID], scheme)
}

func uploadFile(ctx context.Context, client *http.Client, device *model.Device, filePath, fileID, sessionID, token, scheme string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	url := fmt.Sprintf("%s://%s:%d/api/localsend/v2/upload?sessionId=%s&fileId=%s&token=%s", scheme, device.IP, device.Port, sessionID, fileID, token)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, file)
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send upload request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload request failed with status: %s", resp.Status)
	}

	logrus.Info("File sent successfully!")
	return nil
}
