// File: pkg/discovery/multicast.go
package discovery

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"

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
	md.closed.Store(false)

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

func (md *MulticastDiscovery) updateDevice(device *model.Device) {
	md.devicesMutex.Lock()
	md.devices[device.Fingerprint] = device
	md.devicesMutex.Unlock()

	if md.peerCache != nil {
		md.peerCache.Save(device)
	}

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
