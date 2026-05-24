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
	handlers       []func(*model.Device)
	handlersMu     sync.RWMutex
	conns          []net.PacketConn
	connsMu        sync.Mutex
	closed         atomic.Bool
	httpDiscoverer *HTTPDiscovery
	peerCache      *PeerCache
	logger         *zap.SugaredLogger
}

// MulticastConfig contains settings for multicast discovery
type MulticastConfig struct {
	MulticastAddr   string
	Port            int
	InterfaceName   string
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

// StartListening starts listening for multicast announcements on all suitable interfaces.
// If InterfaceName is set in config, only that interface is used.
func (md *MulticastDiscovery) StartListening(ctx context.Context) error {
	md.connsMu.Lock()
	if len(md.conns) > 0 {
		md.connsMu.Unlock()
		return fmt.Errorf("already listening")
	}
	md.connsMu.Unlock()

	addr, err := net.ResolveUDPAddr("udp4", md.config.MulticastAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve multicast address: %w", err)
	}

	var targetIfaces []net.Interface
	if md.config.InterfaceName != "" {
		iface, err := net.InterfaceByName(md.config.InterfaceName)
		if err != nil {
			return fmt.Errorf("multicast interface '%s' not found: %w", md.config.InterfaceName, err)
		}
		if (iface.Flags & net.FlagUp) == 0 {
			return fmt.Errorf("multicast interface '%s' is down", md.config.InterfaceName)
		}
		if (iface.Flags & net.FlagMulticast) == 0 {
			return fmt.Errorf("multicast interface '%s' does not support multicast", md.config.InterfaceName)
		}
		targetIfaces = append(targetIfaces, *iface)
	} else {
		allIfaces, err := net.Interfaces()
		if err != nil {
			return fmt.Errorf("failed to list network interfaces: %w", err)
		}
		for _, iface := range allIfaces {
			if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagMulticast) == 0 {
				continue
			}
			targetIfaces = append(targetIfaces, iface)
		}
	}

	if len(targetIfaces) == 0 {
		return fmt.Errorf("no suitable multicast interface found")
	}

	for _, iface := range targetIfaces {
		conn, err := net.ListenMulticastUDP("udp4", &iface, addr)
		if err != nil {
			md.logger.Warnf("Failed to listen on interface %s: %v", iface.Name, err)
			continue
		}
		conn.SetReadBuffer(2048)

		md.connsMu.Lock()
		md.conns = append(md.conns, conn)
		md.connsMu.Unlock()

		go md.listenLoop(ctx, conn)

		md.logger.Debugf("Multicast discovery listening on %s (interface: %s)", md.config.MulticastAddr, iface.Name)
	}

	md.connsMu.Lock()
	listening := len(md.conns)
	md.connsMu.Unlock()

	if listening == 0 {
		return fmt.Errorf("failed to listen on any multicast interface")
	}

	return nil
}

// Stop stops the multicast discovery and closes all listeners.
func (md *MulticastDiscovery) Stop() {
	md.closed.Store(true)
	md.connsMu.Lock()
	for _, conn := range md.conns {
		conn.Close()
	}
	md.conns = nil
	md.connsMu.Unlock()
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

	var localAddr *net.UDPAddr
	if md.config.InterfaceName != "" {
		iface, err := net.InterfaceByName(md.config.InterfaceName)
		if err == nil {
			addrs, err := iface.Addrs()
			if err == nil {
				for _, a := range addrs {
					if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.To4() != nil {
						localAddr = &net.UDPAddr{IP: ipnet.IP}
						break
					}
				}
			}
		}
	}

	conn, err := net.DialUDP("udp4", localAddr, addr)
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

func (md *MulticastDiscovery) listenLoop(ctx context.Context, conn net.PacketConn) {
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

func (md *MulticastDiscovery) handlePacket(data []byte, addr net.Addr) error {
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

	if md.peerCache != nil {
		md.peerCache.Save(device)
	}

	// Always fire upward to Service so it can handle timestamps properly
	md.handlersMu.RLock()
	handlers := make([]func(*model.Device), len(md.handlers))
	copy(handlers, md.handlers)
	md.handlersMu.RUnlock()

	for _, handler := range handlers {
		go handler(device)
	}
}

func (md *MulticastDiscovery) GetDevices() []*model.Device {
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

func (md *MulticastDiscovery) SetPeerCache(cache *PeerCache) {
	md.peerCache = cache
}

func getShortFingerprint(fingerprint string) string {
	if len(fingerprint) > 8 {
		return fingerprint[:8] + "..."
	}
	return fingerprint
}
