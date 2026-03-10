// File: pkg/discovery/multicast.go
package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bethropolis/localgo/pkg/model"
	"go.uber.org/zap"
)

// MulticastDiscovery implements UDP multicast-based device discovery
type MulticastDiscovery struct {
	config         *MulticastConfig
	dto            model.MulticastDto
	devices        map[string]*model.Device
	devicesMutex   sync.RWMutex
	handlers[]func(*model.Device)
	handlersMu     sync.RWMutex
	conn           net.PacketConn
	connMu         sync.Mutex
	closed         atomic.Bool
	httpDiscoverer *HTTPDiscovery
	logger         *zap.SugaredLogger
}

// MulticastConfig contains settings for multicast discovery
type MulticastConfig struct {
	MulticastAddr   string
	Port            int
	AnnounceTimeout time.Duration
	ListenTimeout   time.Duration
}

// DefaultMulticastConfig returns a default configuration
func DefaultMulticastConfig() *MulticastConfig {
	return &MulticastConfig{
		MulticastAddr:   "224.0.0.167:53317",
		Port:            53317,
		AnnounceTimeout: 2 * time.Second,
		ListenTimeout:   5 * time.Second,
	}
}

// NewMulticastDiscovery creates a new multicast discovery instance
func NewMulticastDiscovery(config *MulticastConfig, dto model.MulticastDto, logger *zap.SugaredLogger) *MulticastDiscovery {
	if config == nil {
		config = DefaultMulticastConfig()
	}
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	return &MulticastDiscovery{
		config:  config,
		dto:     dto,
		devices: make(map[string]*model.Device),
		logger:  logger,
	}
}

// AddDeviceHandler adds a handler function that will be called when a device is discovered
func (md *MulticastDiscovery) AddDeviceHandler(handler func(*model.Device)) {
	md.handlersMu.Lock()
	defer md.handlersMu.Unlock()
	md.handlers = append(md.handlers, handler)
}

// StartListening starts listening for multicast announcements
func (md *MulticastDiscovery) StartListening(ctx context.Context) error {
	if md.conn != nil {
		return fmt.Errorf("already listening")
	}

	addr, err := net.ResolveUDPAddr("udp4", md.config.MulticastAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve multicast address: %w", err)
	}

	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		return fmt.Errorf("failed to listen on multicast socket: %w", err)
	}

	conn.SetReadBuffer(2048)
	md.conn = conn

	go md.listenLoop(ctx)

	md.logger.Debugf("Multicast discovery listening on %s", md.config.MulticastAddr)
	return nil
}

// Stop stops the multicast discovery
func (md *MulticastDiscovery) Stop() {
	md.closed.Store(true)
	md.connMu.Lock()
	if md.conn != nil {
		md.conn.Close()
		md.conn = nil
	}
	md.connMu.Unlock()
}

// SendDiscoveryAnnouncement sends a multicast announcement
func (md *MulticastDiscovery) SendDiscoveryAnnouncement() error {
	announcementDto := md.dto
	announcementDto.Announce = true

	data, err := json.Marshal(announcementDto)
	if err != nil {
		return fmt.Errorf("failed to marshal announcement: %w", err)
	}

	addr, err := net.ResolveUDPAddr("udp4", md.config.MulticastAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve multicast address: %w", err)
	}

	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return fmt.Errorf("failed to create UDP connection: %w", err)
	}
	defer conn.Close()

	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to send multicast announcement: %w", err)
	}

	md.logger.Debugf("Sent multicast announcement as %s (fingerprint: %s) to %s",
		md.dto.Alias, getShortFingerprint(md.dto.Fingerprint), md.config.MulticastAddr)
	return nil
}

// SendDiscoveryResponse sends a response to a specific address
func (md *MulticastDiscovery) SendDiscoveryResponse(targetAddr *net.UDPAddr, targetDevice *model.Device) error {
	// 1. Try HTTP Response first
	if md.httpDiscoverer != nil && targetDevice != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		scheme := "http"
		if targetDevice.Protocol == model.ProtocolTypeHTTPS {
			scheme = "https"
		}

		_, err := md.httpDiscoverer.RegisterWithDevice(ctx, net.ParseIP(targetDevice.IP), targetDevice.Port, scheme)
		if err == nil {
			md.logger.Debugf("Sent discovery response via HTTP to %s:%d", targetDevice.IP, targetDevice.Port)
			return nil
		}
	}

	// 2. Fallback to UDP
	responseDto := md.dto
	responseDto.Announce = false

	data, err := json.Marshal(responseDto)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	conn, err := net.DialUDP("udp4", nil, targetAddr)
	if err != nil {
		return fmt.Errorf("failed to create UDP connection: %w", err)
	}
	defer conn.Close()

	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to send discovery response: %w", err)
	}

	md.logger.Debugf("Sent discovery response via UDP to %s", targetAddr)
	return nil
}

func (md *MulticastDiscovery) listenLoop(ctx context.Context) {
	buffer := make([]byte, 2048)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if md.closed.Load() {
			return
		}

		md.connMu.Lock()
		conn := md.conn
		md.connMu.Unlock()

		if conn == nil {
			return
		}

		if err := conn.SetReadDeadline(time.Now().Add(md.config.ListenTimeout)); err != nil {
			md.logger.Warnf("Failed to set read deadline: %v", err)
		}

		n, addr, err := conn.ReadFrom(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			if strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			continue
		}

		if err := md.handlePacket(buffer[:n], addr); err != nil {
			md.logger.Warnf("Failed to handle multicast packet: %v", err)
		}
	}
}

func (md *MulticastDiscovery) handlePacket(data[]byte, addr net.Addr) error {
	var dto model.MulticastDto
	if err := json.Unmarshal(data, &dto); err != nil {
		return fmt.Errorf("failed to unmarshal packet: %w", err)
	}

	if dto.Fingerprint == md.dto.Fingerprint {
		return nil
	}

	udpAddr, ok := addr.(*net.UDPAddr)
	if !ok {
		return fmt.Errorf("unexpected address type: %T", addr)
	}

	device := model.FromMulticastDto(dto, udpAddr.IP)

	md.logger.Debugf("Discovered raw device via multicast: %s (%s) at %s:%d",
		device.Alias, getShortFingerprint(device.Fingerprint), device.IP, device.Port)

	md.updateDevice(device)

	if dto.Announce {
		if err := md.SendDiscoveryResponse(udpAddr, device); err != nil {
			md.logger.Warnf("Failed to send discovery response: %v", err)
		}
	}

	return nil
}

func (md *MulticastDiscovery) updateDevice(device *model.Device) {
	md.devicesMutex.Lock()
	key := device.Fingerprint
	existingDevice, exists := md.devices[key]
	if exists {
		existingDevice.UpdateLastSeen()
	} else {
		md.devices[key] = device
	}
	md.devicesMutex.Unlock()

	// Always fire upward to Service so it can handle timestamps properly
	md.handlersMu.RLock()
	handlers := make([]func(*model.Device), len(md.handlers))
	copy(handlers, md.handlers)
	md.handlersMu.RUnlock()

	for _, handler := range handlers {
		go handler(device)
	}
}

func (md *MulticastDiscovery) GetDevices()[]*model.Device {
	md.devicesMutex.RLock()
	defer md.devicesMutex.RUnlock()

	devices := make([]*model.Device, 0, len(md.devices))
	for _, device := range md.devices {
		devices = append(devices, device)
	}

	return devices
}

func (md *MulticastDiscovery) SetDto(dto model.MulticastDto) {
	md.dto = dto
}

func (md *MulticastDiscovery) SetHTTPDiscoverer(hd *HTTPDiscovery) {
	md.httpDiscoverer = hd
}

func getShortFingerprint(fingerprint string) string {
	if len(fingerprint) > 8 {
		return fingerprint[:8] + "..."
	}
	return fingerprint
}