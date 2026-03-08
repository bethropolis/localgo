// File: pkg/discovery/http_discovery.go
package discovery

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/network"
	"go.uber.org/zap"
)

type HTTPDiscoveryConfig struct {
	RequestTimeout time.Duration
}

func DefaultHTTPDiscoveryConfig() *HTTPDiscoveryConfig {
	return &HTTPDiscoveryConfig{
		RequestTimeout: 2 * time.Second,
	}
}

type HTTPDiscovery struct {
	config        *HTTPDiscoveryConfig
	dto           model.RegisterDto
	client        *http.Client
	deviceHandler func(*model.Device)
	logger        *zap.SugaredLogger
}

func NewHTTPDiscovery(config *HTTPDiscoveryConfig, dto model.RegisterDto, handler func(*model.Device), logger *zap.SugaredLogger) *HTTPDiscovery {
	if config == nil {
		config = DefaultHTTPDiscoveryConfig()
	}
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	client := &http.Client{
		Timeout: config.RequestTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	return &HTTPDiscovery{
		config:        config,
		dto:           dto,
		client:        client,
		deviceHandler: handler,
		logger:        logger,
	}
}

func (hd *HTTPDiscovery) fetchDeviceInfo(ctx context.Context, ip net.IP, port int, scheme string) (*model.Device, error) {
	url := fmt.Sprintf("%s://%s/api/localsend/v2/info", scheme, net.JoinHostPort(ip.String(), strconv.Itoa(port)))

	hd.logger.Debugf("HTTPDiscovery: Fetching device info from URL: %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := hd.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var infoDto model.InfoDto
	if err := json.Unmarshal(body, &infoDto); err != nil {
		return nil, fmt.Errorf("failed to parse response body: %w", err)
	}

	return &model.Device{
		IP:          ip.String(),
		Version:     infoDto.Version,
		Protocol:    model.ProtocolType(scheme),
		Port:        port,
		Alias:       infoDto.Alias,
		Fingerprint: infoDto.Fingerprint,
		DeviceModel: infoDto.DeviceModel,
		DeviceType:  infoDto.DeviceType,
		Download:    infoDto.Download,
		LastSeen:    time.Now(),
		Available:   true,
	}, nil
}

func (hd *HTTPDiscovery) FetchDeviceInfo(ctx context.Context, ip net.IP, port int) (*model.Device, error) {
	device, err := hd.fetchDeviceInfo(ctx, ip, port, "https")
	if err != nil {
		device, err = hd.fetchDeviceInfo(ctx, ip, port, "http")
	}
	return device, err
}

func (hd *HTTPDiscovery) RegisterWithDevice(ctx context.Context, ip net.IP, port int, scheme string) (*model.Device, error) {
	jsonData, err := json.Marshal(hd.dto)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	if scheme == "" {
		scheme = "http"
	}
	url := fmt.Sprintf("%s://%s/api/localsend/v2/register", scheme, net.JoinHostPort(ip.String(), strconv.Itoa(port)))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := hd.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var infoDto model.InfoDto
	if err := json.Unmarshal(body, &infoDto); err != nil {
		return nil, fmt.Errorf("failed to parse response body: %w", err)
	}

	return &model.Device{
		IP:          ip.String(),
		Version:     infoDto.Version,
		Protocol:    model.ProtocolType(scheme),
		Port:        port,
		Alias:       infoDto.Alias,
		Fingerprint: infoDto.Fingerprint,
		DeviceModel: infoDto.DeviceModel,
		DeviceType:  infoDto.DeviceType,
		Download:    infoDto.Download,
		LastSeen:    time.Now(),
		Available:   true,
	}, nil
}

func (hd *HTTPDiscovery) ScanNetwork(ctx context.Context, ips[]net.IP, port int) ([]*model.Device, error) {
	var devices[]*model.Device
	var wg sync.WaitGroup
	deviceChan := make(chan *model.Device, len(ips))

	hd.logger.Debugf("Scanning %d IPs on port %d", len(ips), port)

	for _, ip := range ips {
		wg.Add(1)
		go func(ip net.IP) {
			defer wg.Done()

			device, err := hd.fetchDeviceInfo(ctx, ip, port, "https")
			if err != nil {
				device, err = hd.fetchDeviceInfo(ctx, ip, port, "http")
				if err != nil {
					return
				}
			}

			deviceChan <- device
		}(ip)
	}

	wg.Wait()
	close(deviceChan)

	for device := range deviceChan {
		devices = append(devices, device)
	}

	return devices, nil
}

func (hd *HTTPDiscovery) ScanLocalNetwork(ctx context.Context, port int) ([]*model.Device, error) {
	localIPs, err := getLocalNetworkIPs()
	if err != nil {
		return nil, fmt.Errorf("could not get local ip addresses to scan: %w", err)
	}
	localIPs = append(localIPs, net.ParseIP("127.0.0.1"))
	return hd.ScanNetwork(ctx, localIPs, port)
}

func getLocalNetworkIPs() ([]net.IP, error) {
	return network.GetLocalIPAddresses()
}