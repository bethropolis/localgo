package send

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/network"
	"github.com/charmbracelet/huh"
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
		if err := verifyDeviceFingerprint(peerCache, targetDevice); err != nil {
			return err
		}
		return sendToDevice(ctx, cfg, targetDevice, filePaths, logger)
	}

	registerDto := model.RegisterDto{
		Alias:       cfg.Alias,
		Version:     config.ProtocolVersion,
		DeviceModel: cfg.DeviceModel,
		DeviceType:  cfg.DeviceType,
		Fingerprint: cfg.SecurityContext.CertificateHash,
		Port:        cfg.Port,
		Protocol:    model.ProtocolTypeHTTP,
	}

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

	return sendToDevice(ctx, cfg, targetDevice, filePaths, logger)
}

// verifyDeviceFingerprint checks if a cached fingerprint differs from the target's
// and prompts the user to trust the updated fingerprint before proceeding.
func verifyDeviceFingerprint(peerCache *discovery.PeerCache, targetDevice *model.Device) error {
	if targetDevice == nil || targetDevice.Fingerprint == "" {
		return nil
	}

	cachedPeers := peerCache.GetPeers()
	for _, cached := range cachedPeers {
		if cached.Alias == targetDevice.Alias && cached.Fingerprint != targetDevice.Fingerprint {
			cli.PrintWarning("The security fingerprint for '%s' has changed!", targetDevice.Alias)

			var trust bool
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title("Trust this new device fingerprint and update cache?").
						Value(&trust).
						Affirmative("Trust & Save").
						Negative("Abort"),
				),
			).WithTheme(huh.ThemeCharm())

			if err := form.Run(); err != nil || !trust {
				return fmt.Errorf("security verification failed: untrusted certificate hash change")
			}

			peerCache.Save(targetDevice)
			break
		}
	}
	return nil
}

func sendToDevice(ctx context.Context, cfg *config.Config, device *model.Device, filePaths []string, logger *zap.SugaredLogger) error {
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
		defer tr.CloseIdleConnections()
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

		// If this is a temporary clipboard file, sanitize display name to text_transfer.txt
		if strings.HasPrefix(filepath.Base(filePath), "localgo-clip-") {
			remoteName = "text_transfer.txt"
			contentType = "text/plain"
		}

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

func uploadFile(ctx context.Context, client *http.Client, device *model.Device, filePath, fileID, sessionID, token, scheme string, trackProgress func(int64), logger *zap.SugaredLogger) error {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	url := fmt.Sprintf("%s://%s/api/localsend/v2/upload?sessionId=%s&fileId=%s&token=%s", scheme, net.JoinHostPort(device.IP, strconv.Itoa(device.Port)), sessionID, fileID, token)

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file stats: %w", err)
	}

	var body io.ReadCloser = file
	if trackProgress != nil {
		bar := &progressBar{current: 0, track: trackProgress}
		body = &progressTracker{Reader: file, bar: bar}
	}

	// Wrap with idle timeout: cancel request if no data flows for 15s
	uploadCtx, cancel := context.WithCancel(ctx)
	body = NewIdleTimeoutReader(body, 15*time.Second, cancel)

	req, err := http.NewRequestWithContext(uploadCtx, http.MethodPost, url, body)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create upload request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = stat.Size()

	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return fmt.Errorf("upload stalled: no data transmitted for 15s")
		}
		return fmt.Errorf("failed to send upload request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload request failed with status: %s", resp.Status)
	}

	return nil
}

// IdleTimeoutReader wraps an io.ReadCloser and cancels the context if no data
// is read within the configured idle duration.
type IdleTimeoutReader struct {
	r           io.ReadCloser
	idleTimeout time.Duration
	timer       *time.Timer
	cancel      func()
}

func NewIdleTimeoutReader(r io.ReadCloser, timeout time.Duration, cancel func()) *IdleTimeoutReader {
	tr := &IdleTimeoutReader{
		r:           r,
		idleTimeout: timeout,
		cancel:      cancel,
	}
	tr.timer = time.AfterFunc(timeout, func() {
		tr.cancel()
	})
	return tr
}

func (tr *IdleTimeoutReader) Read(p []byte) (int, error) {
	tr.timer.Reset(tr.idleTimeout)
	n, err := tr.r.Read(p)
	if err != nil {
		tr.timer.Stop()
	}
	return n, err
}

func (tr *IdleTimeoutReader) Close() error {
	tr.timer.Stop()
	return tr.r.Close()
}

type progressBar struct {
	current int64
	track   func(int64)
}

type progressTracker struct {
	io.Reader
	bar *progressBar
}

func (pt *progressTracker) Read(p []byte) (int, error) {
	n, err := pt.Reader.Read(p)
	if n > 0 && pt.bar != nil {
		pt.bar.current += int64(n)
		pt.bar.track(pt.bar.current)
	}
	return n, err
}

func (pt *progressTracker) Close() error {
	if f, ok := pt.Reader.(*os.File); ok {
		return f.Close()
	}
	return nil
}
