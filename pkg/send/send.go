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
	"sync"
	"time"

	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/discovery"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/network"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// SendFile sends a file to a recipient.
func SendFiles(ctx context.Context, cfg *config.Config, filePaths []string, recipientAlias string, recipientPort int) error {
	logrus.Infof("Searching for recipient '%s'...", recipientAlias)

	// Use default port if not specified
	if recipientPort == 0 {
		recipientPort = config.DefaultPort
	}

	var targetDevice *model.Device

	// --- Multicast Discovery (Fast) ---
	logrus.Info("Sending multicast announcement...")

	// Create multicast discovery DTO
	discoverySvcConfig := discovery.DefaultServiceConfig()
	discoverySvcConfig.MulticastConfig.Port = cfg.Port
	discoverySvcConfig.MulticastConfig.MulticastAddr = fmt.Sprintf("%s:%d", cfg.MulticastGroup, cfg.Port)

	// Set correct protocol for multicast
	protocol_type := model.ProtocolTypeHTTP
	if cfg.HttpsEnabled {
		protocol_type = model.ProtocolTypeHTTPS
	}

	multicastDto := model.MulticastDto{
		Alias:       cfg.Alias,
		Version:     config.ProtocolVersion,
		DeviceModel: cfg.DeviceModel,
		DeviceType:  cfg.DeviceType,
		Fingerprint: cfg.SecurityContext.CertificateHash,
		Port:        cfg.Port,
		Protocol:    protocol_type,
		Download:    false,
		Announce:    true,
	}

	multicast := discovery.NewMulticastDiscovery(discoverySvcConfig.MulticastConfig, multicastDto)
	httpDiscoverer := discovery.NewHTTPDiscovery(nil, cfg.ToRegisterDto(), nil)
	multicast.SetHTTPDiscoverer(httpDiscoverer)

	discoverySvc := discovery.NewService(discoverySvcConfig, multicast)

	// Channel to catch the found device
	foundChan := make(chan *model.Device, 1)
	discoverySvc.AddDeviceHandler(func(device *model.Device) {
		if device.Alias == recipientAlias {
			select {
			case foundChan <- device:
			default:
			}
		}
	})

	// Start listening and announcing
	// Use a short timeout for multicast-only phase (e.g., 1.5 seconds)
	multicastCtx, cancelMulticast := context.WithTimeout(ctx, 1500*time.Millisecond)

	go func() {
		defer cancelMulticast()
		err := discoverySvc.Start(multicastCtx, cfg.Alias, cfg.Port, cfg.SecurityContext.CertificateHash, cfg.DeviceType, cfg.DeviceModel, cfg.HttpsEnabled)
		if err != nil {
			logrus.Warnf("Multicast start failed: %v", err)
		}
	}()

	// Wait for multicast result
	select {
	case device := <-foundChan:
		logrus.Infof("Discovered recipient via multicast: %s (%s)", device.Alias, device.IP)
		targetDevice = device
		discoverySvc.Stop()
	case <-multicastCtx.Done():
		logrus.Info("Multicast discovery timed out, falling back to HTTP scan...")
		discoverySvc.Stop()
	}

	if targetDevice != nil {
		return sendToDevice(ctx, cfg, targetDevice, filePaths)
	}

	// --- Fallback: HTTP Scan Discovery ---
	logrus.Info("Starting HTTP network scan...")

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
			localIPs, err := network.GetLocalIPAddresses()
			if err != nil {
				logrus.Warnf("Failed to get local IPs: %v", err)
				time.Sleep(500 * time.Millisecond)
				continue
			}

			var ips []net.IP
			for _, ip := range localIPs {
				subnetIPs := network.GetSubnetIPs(ip)
				ips = append(ips, subnetIPs...)
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
	return sendToDevice(ctx, cfg, targetDevice, filePaths)
}

func sendToDevice(ctx context.Context, cfg *config.Config, device *model.Device, filePaths []string) error {
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

	filesDtoMap := make(map[string]model.FileDto)
	filePathMap := make(map[string]string) // fileId to original path

	for _, filePath := range filePaths {
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			return fmt.Errorf("failed to get file info for %s: %w", filePath, err)
		}

		// Detect file type
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open file for detection %s: %w", filePath, err)
		}

		// Read first 512 bytes for content type detection
		buffer := make([]byte, 512)
		n, _ := file.Read(buffer)
		contentType := http.DetectContentType(buffer[:n])
		file.Close()

		modTime := fileInfo.ModTime().Format(time.RFC3339)

		fileDto := model.FileDto{
			ID:       uuid.NewString(),
			FileName: filepath.Base(filePath),
			Size:     fileInfo.Size(),
			FileType: contentType,
			Metadata: &model.FileMetadata{
				Modified: &modTime,
			},
		}

		filesDtoMap[fileDto.ID] = fileDto
		filePathMap[fileDto.ID] = filePath
	}

	prepareDto := model.PrepareUploadRequestDto{
		Info: model.InfoDto{
			Alias:       cfg.Alias,
			Version:     config.ProtocolVersion,
			DeviceModel: cfg.DeviceModel,
			DeviceType:  cfg.DeviceType,
			Fingerprint: cfg.SecurityContext.CertificateHash,
			Download:    true,
		},
		Files: filesDtoMap,
	}

	jsonData, err := json.Marshal(prepareDto)
	if err != nil {
		return fmt.Errorf("failed to marshal prepare dto: %w", err)
	}

	url := fmt.Sprintf("%s://%s:%d/api/localsend/v2/prepare-upload", scheme, device.IP, device.Port)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create prepare request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
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

	var wg sync.WaitGroup
	errCh := make(chan error, len(prepareResponse.Files))

	// Iterate and upload each file returned in prepareResponse.Files
	for fileID, token := range prepareResponse.Files {
		filePath, exists := filePathMap[fileID]
		if !exists {
			logrus.Warnf("Server responded with unknown file ID: %s", fileID)
			continue
		}

		wg.Add(1)
		go func(fID, tkn, fPath string) {
			defer wg.Done()
			logrus.Infof("Uploading file: %s", filepath.Base(fPath))
			err := uploadFile(ctx, client, device, fPath, fID, prepareResponse.SessionID, tkn, scheme)
			if err != nil {
				logrus.Errorf("Failed to upload file %s: %v", filepath.Base(fPath), err)
				errCh <- fmt.Errorf("failed to upload %s: %w", filepath.Base(fPath), err)
			}
		}(fileID, token, filePath)
	}

	wg.Wait()
	close(errCh)

	var uploadErrors []error
	for err := range errCh {
		uploadErrors = append(uploadErrors, err)
	}

	if len(uploadErrors) > 0 {
		return fmt.Errorf("encountered %d upload errors, first error: %w", len(uploadErrors), uploadErrors[0])
	}

	logrus.Info("All files uploaded successfully!")
	return nil
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
