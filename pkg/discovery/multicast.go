// Package discovery handles device discovery mechanisms
package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/bethropolis/localgo/pkg/model"
	"github.com/sirupsen/logrus"
)

// MulticastDiscovery implements UDP multicast-based device discovery
type MulticastDiscovery struct {
	config       *MulticastConfig
	dto          model.MulticastDto
	devices      map[string]*model.Device
	devicesMutex sync.RWMutex
	handlers     []func(*model.Device)
	conn         net.PacketConn
	closed       bool
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
func NewMulticastDiscovery(config *MulticastConfig, dto model.MulticastDto) *MulticastDiscovery {
	if config == nil {
		config = DefaultMulticastConfig()
	}

	return &MulticastDiscovery{
		config:  config,
		dto:     dto,
		devices: make(map[string]*model.Device),
	}
}

// AddDeviceHandler adds a handler function that will be called when a device is discovered
func (md *MulticastDiscovery) AddDeviceHandler(handler func(*model.Device)) {
	md.handlers = append(md.handlers, handler)
}

// StartListening starts listening for multicast announcements
func (md *MulticastDiscovery) StartListening(ctx context.Context) error {
	if md.conn != nil {
		return fmt.Errorf("already listening")
	}

	// Parse the multicast address
	addr, err := net.ResolveUDPAddr("udp4", md.config.MulticastAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve multicast address: %w", err)
	}

	// Create UDP connection for listening
	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		return fmt.Errorf("failed to listen on multicast socket: %w", err)
	}

	// Set socket options
	conn.SetReadBuffer(2048)
	md.conn = conn

	// Start listening loop
	go md.listenLoop(ctx)

	logrus.Printf("Multicast discovery listening on %s", md.config.MulticastAddr)
	logrus.Debugf("MulticastDiscovery: Listening with DTO: %+v", md.dto)
	return nil
}

// Stop stops the multicast discovery
func (md *MulticastDiscovery) Stop() {
	md.closed = true
	if md.conn != nil {
		md.conn.Close()
		md.conn = nil
	}
}

// SendDiscoveryAnnouncement sends a multicast announcement
func (md *MulticastDiscovery) SendDiscoveryAnnouncement() error {
	// Create a copy of the DTO with announcement flag set
	announcementDto := md.dto
	announcementDto.Announce = true

	// Marshal the DTO to JSON
	data, err := json.Marshal(announcementDto)
	if err != nil {
		return fmt.Errorf("failed to marshal announcement: %w", err)
	}

	// Create a UDP connection
	addr, err := net.ResolveUDPAddr("udp4", md.config.MulticastAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve multicast address: %w", err)
	}

	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return fmt.Errorf("failed to create UDP connection: %w", err)
	}
	defer conn.Close()

	// Send the data
	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to send multicast announcement: %w", err)
	}

	logrus.Printf("Sent multicast announcement as %s (fingerprint: %s) to %s",
		md.dto.Alias, getShortFingerprint(md.dto.Fingerprint), md.config.MulticastAddr)
	logrus.Debugf("MulticastDiscovery: Announcement DTO: %+v", announcementDto)
	return nil
}

// SendDiscoveryResponse sends a response to a specific address
func (md *MulticastDiscovery) SendDiscoveryResponse(targetAddr *net.UDPAddr) error {
	// Create a copy of the DTO with announcement flag unset (response)
	responseDto := md.dto
	responseDto.Announce = false

	// Marshal the DTO to JSON
	data, err := json.Marshal(responseDto)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	// Create a UDP connection
	conn, err := net.DialUDP("udp4", nil, targetAddr)
	if err != nil {
		return fmt.Errorf("failed to create UDP connection: %w", err)
	}
	defer conn.Close()

	// Send the data
	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to send discovery response: %w", err)
	}

	logrus.Printf("Sent discovery response to %s", targetAddr)
	return nil
}

// listenLoop is the main listening loop for multicast messages
func (md *MulticastDiscovery) listenLoop(ctx context.Context) {
	buffer := make([]byte, 2048)

	for {
		// Check if context is done or we're closed
		select {
		case <-ctx.Done():
			return
		default:
			// Continue
		}

		if md.closed || md.conn == nil {
			return
		}

		// Set read deadline for periodic context checking
		if err := md.conn.SetReadDeadline(time.Now().Add(md.config.ListenTimeout)); err != nil {
			logrus.Printf("Failed to set read deadline: %v", err)
		}

		// Read incoming packet
		n, addr, err := md.conn.ReadFrom(buffer)
		if err != nil {
			// Handle timeout (not a real error)
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}

			// Handle closed connection
			if strings.Contains(err.Error(), "use of closed network connection") {
				return
			}

			logrus.Printf("Error reading from multicast: %v", err)
			continue
		}

		// Process the received data
		logrus.Debugf("MulticastDiscovery: Received %d bytes from %v", n, addr)
		if err := md.handlePacket(buffer[:n], addr); err != nil {
			logrus.Printf("Failed to handle multicast packet: %v", err)
		}
	}
}

// handlePacket processes a received UDP packet
func (md *MulticastDiscovery) handlePacket(data []byte, addr net.Addr) error {
	// Parse the JSON data
	var dto model.MulticastDto
	if err := json.Unmarshal(data, &dto); err != nil {
		return fmt.Errorf("failed to unmarshal packet: %w", err)
	}

	// Skip our own announcements
	if dto.Fingerprint == md.dto.Fingerprint {
		logrus.Debugf("MulticastDiscovery: Ignoring self-announcement (fingerprint: %s)", dto.Fingerprint)
		return nil
	}

	// Get the sender's IP
	udpAddr, ok := addr.(*net.UDPAddr)
	if !ok {
		return fmt.Errorf("unexpected address type: %T", addr)
	}

	// Create a device from the DTO
	device := model.FromMulticastDto(dto, udpAddr.IP)

	logrus.Printf("Discovered device via multicast: %s (%s) at %s:%d",
		device.Alias, getShortFingerprint(device.Fingerprint), device.IP, device.Port)
	logrus.Debugf("MulticastDiscovery: Device DTO: %+v", dto)

	// Update our device map
	md.updateDevice(device)

	// If this is an announcement (not a response), send a response
	if dto.Announce {
		if err := md.SendDiscoveryResponse(udpAddr); err != nil {
			logrus.Printf("Failed to send discovery response: %v", err)
		}
	}

	return nil
}

// updateDevice adds or updates a device in the device map
func (md *MulticastDiscovery) updateDevice(device *model.Device) {
	md.devicesMutex.Lock()
	defer md.devicesMutex.Unlock()

	key := device.Fingerprint

	existingDevice, exists := md.devices[key]
	if exists {
		// Update existing device
		existingDevice.UpdateLastSeen()
	} else {
		// Add new device
		md.devices[key] = device

		// Notify all handlers about the new device
		for _, handler := range md.handlers {
			go handler(device)
		}
	}
}

// GetDevices returns all discovered devices
func (md *MulticastDiscovery) GetDevices() []*model.Device {
	md.devicesMutex.RLock()
	defer md.devicesMutex.RUnlock()

	devices := make([]*model.Device, 0, len(md.devices))
	for _, device := range md.devices {
		devices = append(devices, device)
	}

	return devices
}

// SetDto sets the DTO for the multicast discovery
func (md *MulticastDiscovery) SetDto(dto model.MulticastDto) {
	md.dto = dto
}

// getShortFingerprint returns a short version of the fingerprint for logging.
func getShortFingerprint(fingerprint string) string {
	if len(fingerprint) > 8 {
		return fingerprint[:8] + "..."
	}
	return fingerprint
}
