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
	"strconv"
	"sync"
	"time"

	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/discovery"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/network"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// getFilesWithRelativePaths recursively flattens directories while preserving relative structure
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

// SendFiles sends files or directories to a recipient.
func SendFiles(ctx context.Context, cfg *config.Config, filePaths[]string, recipientAlias string, recipientPort int, logger *zap.SugaredLogger) error {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	logger.Infof("Searching for recipient '%s'...", recipientAlias)

	if recipientPort == 0 {
		recipientPort = config.DefaultPort
	}

	var targetDevice *model.Device

	// --- Multicast Discovery (Fast) ---
	logger.Info("Sending multicast announcement...")

	discoverySvcConfig := discovery.DefaultServiceConfig()
	discoverySvcConfig.MulticastConfig.Port = cfg.Port
	discoverySvcConfig.MulticastConfig.MulticastAddr = fmt.Sprintf("%s:%d", cfg.MulticastGroup, cfg.Port)

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

	multicast := discovery.NewMulticastDiscovery(discoverySvcConfig.MulticastConfig, multicastDto, logger)
	httpDiscoverer := discovery.NewHTTPDiscovery(nil, cfg.ToRegisterDto(), nil, logger)
	multicast.SetHTTPDiscoverer(httpDiscoverer)

	discoverySvc := discovery.NewService(discoverySvcConfig, multicast, logger)

	foundChan := make(chan *model.Device, 1)
	discoverySvc.AddDeviceHandler(func(device *model.Device) {
		if device.Alias == recipientAlias {
			select {
			case foundChan <- device:
			default:
			}
		}
	})

	multicastCtx, cancelMulticast := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancelMulticast()

	err := discoverySvc.Start(multicastCtx, cfg.Alias, cfg.Port, cfg.SecurityContext.CertificateHash, cfg.DeviceType, cfg.DeviceModel, cfg.HttpsEnabled)
	if err != nil {
		logger.Warnf("Multicast start failed: %v", err)
	}

	select {
	case device := <-foundChan:
		logger.Infof("Discovered recipient via multicast: %s (%s)", device.Alias, device.IP)
		targetDevice = device
		discoverySvc.Stop()
	case <-multicastCtx.Done():
		logger.Info("Multicast discovery timed out, falling back to HTTP scan...")
		discoverySvc.Stop()
	}

	if targetDevice != nil {
		return sendToDevice(ctx, cfg, targetDevice, filePaths, logger)
	}

	// --- Fallback: HTTP Scan Discovery ---
	logger.Info("Starting HTTP network scan...")

	retryCtx, cancelRetry := context.WithTimeout(ctx, 15*time.Second)
	defer cancelRetry()

	for targetDevice == nil {
		select {
		case <-retryCtx.Done():
			return fmt.Errorf("recipient '%s' not found after multiple attempts", recipientAlias)
		default:
			registerDto := model.RegisterDto{
				Alias:       cfg.Alias,
				Version:     config.ProtocolVersion,
				DeviceModel: cfg.DeviceModel,
				DeviceType:  cfg.DeviceType,
				Fingerprint: cfg.SecurityContext.CertificateHash,
				Port:        cfg.Port,
				Protocol:    model.ProtocolTypeHTTP,
			}

			httpDiscoverer := discovery.NewHTTPDiscovery(nil, registerDto, nil, logger)

			localIPs, err := network.GetLocalIPAddresses()
			if err != nil {
				logger.Warnf("Failed to get local IPs: %v", err)
				time.Sleep(500 * time.Millisecond)
				continue
			}

			var ips[]net.IP
			for _, ip := range localIPs {
				subnetIPs := network.GetSubnetIPs(ip)
				ips = append(ips, subnetIPs...)
			}
			ips = append(ips, net.ParseIP("127.0.0.1"))

			discoverCtx, cancel := context.WithTimeout(retryCtx, 3*time.Second)
			foundDevices, err := httpDiscoverer.ScanNetwork(discoverCtx, ips, recipientPort)
			cancel()

			if err != nil {
				logger.Warnf("HTTP discovery failed: %v", err)
				time.Sleep(500 * time.Millisecond)
				continue
			}

			for _, device := range foundDevices {
				if device.Alias == recipientAlias {
					targetDevice = device
					break
				}
			}

			if targetDevice == nil {
				time.Sleep(500 * time.Millisecond)
			}
		}
	}

	return sendToDevice(ctx, cfg, targetDevice, filePaths, logger)
}

func sendToDevice(ctx context.Context, cfg *config.Config, device *model.Device, filePaths[]string, logger *zap.SugaredLogger) error {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	client := &http.Client{}
	scheme := "http"

	if device.Protocol == model.ProtocolTypeHTTPS {
		scheme = "https"
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client.Transport = tr
	}

	fileMap, err := getFilesWithRelativePaths(filePaths)
	if err != nil {
		return fmt.Errorf("failed to process file paths: %w", err)
	}

	filesDtoMap := make(map[string]model.FileDto)
	filePathMap := make(map[string]string)

	for filePath, remoteName := range fileMap {
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			return fmt.Errorf("failed to get file info for %s: %w", filePath, err)
		}

		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open file for detection %s: %w", filePath, err)
		}

		buffer := make([]byte, 512)
		n, _ := file.Read(buffer)
		file.Close()
		contentType := http.DetectContentType(buffer[:n])

		modTime := fileInfo.ModTime().Format(time.RFC3339)

		fileDto := model.FileDto{
			ID:       uuid.NewString(),
			FileName: remoteName,
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

	url := fmt.Sprintf("%s://%s/api/localsend/v2/prepare-upload", scheme, net.JoinHostPort(device.IP, strconv.Itoa(device.Port)))
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

	for fileID, token := range prepareResponse.Files {
		filePath, exists := filePathMap[fileID]
		if !exists {
			logger.Warnf("Server responded with unknown file ID: %s", fileID)
			continue
		}

		wg.Add(1)
		go func(fID, tkn, fPath string) {
			defer wg.Done()
			logger.Infof("Uploading file: %s", filepath.Base(fPath))
			err := uploadFile(ctx, client, device, fPath, fID, prepareResponse.SessionID, tkn, scheme, logger)
			if err != nil {
				logger.Errorf("Failed to upload file %s: %v", filepath.Base(fPath), err)
				errCh <- fmt.Errorf("failed to upload %s: %w", filepath.Base(fPath), err)
			}
		}(fileID, token, filePath)
	}

	wg.Wait()
	close(errCh)

	var uploadErrors[]error
	for err := range errCh {
		uploadErrors = append(uploadErrors, err)
	}

	if len(uploadErrors) > 0 {
		return fmt.Errorf("encountered %d upload errors, first error: %w", len(uploadErrors), uploadErrors[0])
	}

	logger.Info("All files uploaded successfully!")
	return nil
}

func uploadFile(ctx context.Context, client *http.Client, device *model.Device, filePath, fileID, sessionID, token, scheme string, logger *zap.SugaredLogger) error {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	url := fmt.Sprintf("%s://%s/api/localsend/v2/upload?sessionId=%s&fileId=%s&token=%s", scheme, net.JoinHostPort(device.IP, strconv.Itoa(device.Port)), sessionID, fileID, token)

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

	return nil
}