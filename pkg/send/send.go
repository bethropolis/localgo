package send

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/discovery"
	"github.com/bethropolis/localgo/pkg/metadata"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/network"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SendFiles sends files or directories to a recipient.
func SendFiles(ctx context.Context, cfg *config.Config, filePaths []string, recipientAlias string, recipientPort int, logger *zap.SugaredLogger) error {
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
	discoverySvcConfig.MulticastConfig.InterfaceName = cfg.MulticastInterface
	multicastDto := cfg.ToMulticastDto(false)

	multicast := discovery.NewMulticastDiscovery(discoverySvcConfig.MulticastConfig, multicastDto, logger)
	httpDiscoverer := discovery.NewHTTPDiscovery(nil, cfg.ToRegisterDto(), nil, logger)
	multicast.SetHTTPDiscoverer(httpDiscoverer)

	peerCache := discovery.NewPeerCache(logger)
	multicast.SetPeerCache(peerCache)

	discoverySvc := discovery.NewService(discoverySvcConfig, multicast, logger)
	discoverySvc.SetPeerCache(peerCache)

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

	err := discoverySvc.Start(multicastCtx, cfg.ToMulticastDto(false))
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
		if err := verifyDeviceFingerprint(peerCache, targetDevice); err != nil {
			return err
		}
		return SendToDevice(ctx, cfg, targetDevice, filePaths, logger)
	}

	registerDto := cfg.ToRegisterDto()
	httpFallback := discovery.NewHTTPDiscovery(nil, registerDto, nil, logger)

	localIPs, err := network.GetLocalIPAddresses()
	if err != nil {
		return fmt.Errorf("failed to get local IPs: %w", err)
	}

	var ips []net.IP
	for _, ip := range localIPs {
		subnetIPs := network.GetSubnetIPs(ip)
		ips = append(ips, subnetIPs...)
	}
	ips = append(ips, net.ParseIP("127.0.0.1"))

	// Give the scan a proper chunk of time to test all IPs safely
	scanCtx, cancelScan := context.WithTimeout(ctx, 15*time.Second)
	defer cancelScan()

	foundDevices, err := httpFallback.ScanNetwork(scanCtx, ips, recipientPort)
	if err != nil {
		return fmt.Errorf("HTTP discovery failed: %w", err)
	}

	for _, device := range foundDevices {
		if device.Alias == recipientAlias {
			targetDevice = device
			break
		}
	}

	if targetDevice == nil {
		return fmt.Errorf("recipient '%s' not found on network after scan", recipientAlias)
	}

	logger.Infof("Discovered recipient via HTTP Scan: %s (%s)", targetDevice.Alias, targetDevice.IP)

	if err := verifyDeviceFingerprint(peerCache, targetDevice); err != nil {
		return err
	}

	return SendToDevice(ctx, cfg, targetDevice, filePaths, logger)
}

func SendToDevice(ctx context.Context, cfg *config.Config, device *model.Device, filePaths []string, logger *zap.SugaredLogger) error {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	client := &http.Client{}
	scheme := "http"

	if device.Protocol == "" {
		addr := net.JoinHostPort(device.IP, strconv.Itoa(device.Port))
		dialer := &net.Dialer{Timeout: 2 * time.Second}
		conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{InsecureSkipVerify: true})
		if err == nil {
			conn.Close()
			device.Protocol = model.ProtocolTypeHTTPS
		}
	}

	if device.Protocol == model.ProtocolTypeHTTPS {
		scheme = "https"
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
		}
		if device.Fingerprint != "" {
			expectedFingerprint := device.Fingerprint
			tlsConfig.VerifyConnection = func(state tls.ConnectionState) error {
				if len(state.PeerCertificates) == 0 {
					return fmt.Errorf("no peer certificates presented")
				}
				cert := state.PeerCertificates[0]
				hash := sha256.Sum256(cert.Raw)
				actual := hex.EncodeToString(hash[:])
				if !strings.EqualFold(actual, expectedFingerprint) {
					return fmt.Errorf("TLS certificate fingerprint mismatch: expected %s, got %s", expectedFingerprint, actual)
				}
				return nil
			}
		}
		tr := &http.Transport{
			TLSClientConfig: tlsConfig,
		}
		client.Transport = tr
		defer tr.CloseIdleConnections()
	}

	fileMap, err := getFilesWithRelativePaths(filePaths)
	if err != nil {
		return fmt.Errorf("failed to process file paths: %w", err)
	}

	// Strip EXIF/metadata from image files in private mode
	if cfg.Private {
		for filePath := range fileMap {
			if err := metadata.Strip(filePath); err != nil {
				logger.Warnf("Failed to strip metadata from %s: %v", filePath, err)
			}
		}
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

		if cfg.Private {
			remoteName = anonymizeFileName(contentType)
		}

		// If this is a temporary clipboard file, sanitize display name to text_transfer.txt
		if strings.HasPrefix(filepath.Base(filePath), "localgo-clip-") {
			remoteName = "text_transfer.txt"
			contentType = "text/plain"
		}

		modTime := fileInfo.ModTime().Format(time.RFC3339)

		var metadataPtr *model.FileMetadata
		if !cfg.Private {
			metadataPtr = &model.FileMetadata{Modified: &modTime}
		}

		fileDto := model.FileDto{
			ID:       uuid.NewString(),
			FileName: remoteName,
			Size:     fileInfo.Size(),
			FileType: contentType,
			Metadata: metadataPtr,
		}

		filesDtoMap[fileDto.ID] = fileDto
		filePathMap[fileDto.ID] = filePath
	}

	infoAlias := cfg.Alias
	infoDeviceModel := cfg.DeviceModel
	infoDeviceType := cfg.DeviceType
	if cfg.Private {
		infoAlias = "Anonymous"
		infoDeviceModel = nil
		infoDeviceType = model.DeviceTypeHeadless
	}

	fingerprint := cfg.RandomFingerprint
	if cfg.HttpsEnabled {
		fingerprint = cfg.SecurityContext.CertificateHash
	}

	prepareDto := model.PrepareUploadRequestDto{
		Info: model.InfoDto{
			Alias:       infoAlias,
			Version:     config.ProtocolVersion,
			DeviceModel: infoDeviceModel,
			DeviceType:  infoDeviceType,
			Fingerprint: fingerprint,
			Port:        device.Port,
			Protocol:    model.ProtocolType(scheme),
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

	mp := cli.NewMultiProgress(int64(len(prepareResponse.Files)))

	var wg sync.WaitGroup
	errCh := make(chan error, len(prepareResponse.Files))

	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 4
	}
	sem := make(chan struct{}, concurrency)

	for fileID, token := range prepareResponse.Files {
		filePath, exists := filePathMap[fileID]
		if !exists {
			logger.Warnf("Server responded with unknown file ID: %s", fileID)
			continue
		}

		var fileSize int64
		if fi, err := os.Stat(filePath); err == nil {
			fileSize = fi.Size()
		}
		trackProgress := mp.AddBar(filepath.Base(filePath), fileSize)

		wg.Add(1)
		go func(fID, tkn, fPath string, track func(int64)) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			logger.Infof("Uploading file: %s", filepath.Base(fPath))
			err := uploadFile(ctx, client, device, fPath, fID, prepareResponse.SessionID, tkn, scheme, track, logger)
			if err != nil {
				logger.Errorf("Failed to upload file %s: %v", filepath.Base(fPath), err)
				errCh <- fmt.Errorf("failed to upload %s: %w", filepath.Base(fPath), err)
			}
		}(fileID, token, filePath, trackProgress)
	}

	wg.Wait()
	mp.ForceComplete()
	mp.Wait()
	close(errCh)

	var uploadErrors []error
	for err := range errCh {
		uploadErrors = append(uploadErrors, err)
	}

	if len(uploadErrors) > 0 {
		return fmt.Errorf("encountered %d upload errors, first error: %w", len(uploadErrors), uploadErrors[0])
	}

	logger.Info("All files uploaded successfully!")
	return nil
}


