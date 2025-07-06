// Package discovery provides network device discovery functionality
package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bet/localgo/pkg/config" // Import config
	"github.com/bet/localgo/pkg/model"
	"github.com/sirupsen/logrus"
)

// Service coordinates different discovery mechanisms
type Service struct {
	config        *ServiceConfig
	multicast     MulticastDiscoverer
	devices       map[string]*model.Device
	devicesMutex  sync.RWMutex
	handlers      []func(*model.Device)
	announceTimer *time.Timer
}

// ServiceConfig contains settings for the discovery service
type ServiceConfig struct {
	MulticastConfig    *MulticastConfig
	AnnounceInterval   time.Duration
	DeviceTimeout      time.Duration
	EnableAnnouncement bool
}

// DefaultServiceConfig returns a default configuration for the discovery service
func DefaultServiceConfig() *ServiceConfig {
	return &ServiceConfig{
		MulticastConfig:    DefaultMulticastConfig(),
		AnnounceInterval:   30 * time.Second, // Send announcement every 30 seconds
		DeviceTimeout:      2 * time.Minute,  // Consider devices offline after 2 minutes
		EnableAnnouncement: true,
	}
}

// NewService creates a new discovery service
func NewService(config *ServiceConfig, multicast MulticastDiscoverer) *Service {
	if config == nil {
		config = DefaultServiceConfig()
	}

	return &Service{
		config:    config,
		devices:   make(map[string]*model.Device),
		multicast: multicast,
	}
}

// Start initializes and starts the discovery service for listening and periodic announcements
func (s *Service) Start(ctx context.Context, alias string, port int, fingerprint string, deviceType model.DeviceType, deviceModel *string) error {
	// Create UDP multicast discovery instance
	// ** Important: Create the DTO needed for multicast here **
	multicastDto := model.MulticastDto{
		Alias:       alias,
		Version:     config.ProtocolVersion, // Use constant from config package
		DeviceModel: deviceModel,
		DeviceType:  deviceType,
		Fingerprint: fingerprint,
		Port:        port,
		Protocol:    model.ProtocolTypeHTTPS, // Assuming HTTPS, make configurable if needed
		Download:    false,                   // Not implementing download server yet
		Announce:    true,                    // Default Announce for DTO, can be overridden
	}

	s.multicast.SetDto(multicastDto)
	s.multicast.AddDeviceHandler(func(device *model.Device) {
		s.updateDevice(device) // Update the service's central device list
	})

	// Start multicast discovery listening
	if err := s.multicast.StartListening(ctx); err != nil {
		// Only return error if it's not already listening (idempotency)
		// This check might be fragile depending on the exact error type from StartListening
		// if !strings.Contains(err.Error(), "already listening") { // Example check
		return fmt.Errorf("failed to start multicast discovery: %w", err)
		// }
		// logrus.Println("Multicast discovery already listening.") // Or log if needed
	}

	// Start periodic announcements if enabled
	if s.config.EnableAnnouncement {
		s.startAnnouncementLoop(ctx)
	}

	// Send initial announcement
	if err := s.multicast.SendDiscoveryAnnouncement(); err != nil {
		logrus.Errorf("Failed to send initial discovery announcement: %v", err)
	}

	return nil
}

// Stop stops the discovery service
func (s *Service) Stop() {
	logrus.Println("Stopping discovery service...")
	// Stop multicast
	if s.multicast != nil {
		s.multicast.Stop()
	}

	// Stop announcement timer
	if s.announceTimer != nil {
		s.announceTimer.Stop()
	}
	logrus.Println("Discovery service stopped.")
}

// Discover performs a discovery scan and returns found devices.
// It sends an announcement and listens for a short duration.
// It requires the service to be configured but not necessarily fully "Start"ed (for listening).
func (s *Service) Discover(ctx context.Context, alias string, port int, fingerprint string, deviceType model.DeviceType, deviceModel *string) ([]*model.Device, error) {
	logrus.Println("Performing one-off discovery scan...")

	// --- Create Multicast DTO for announcement ---
	multicastDto := model.MulticastDto{
		Alias:       alias,
		Version:     config.ProtocolVersion, // Use constant from config package
		DeviceModel: deviceModel,
		DeviceType:  deviceType,
		Fingerprint: fingerprint,
		Port:        port,
		Protocol:    model.ProtocolTypeHTTPS,
		Download:    false,
		Announce:    true, // We are announcing
	}

	s.multicast.SetDto(multicastDto)

	// Send initial announcement
	if err := s.multicast.SendDiscoveryAnnouncement(); err != nil {
		logrus.Errorf("Failed to send initial discovery announcement: %v", err)
	}

	// --- Wait for Responses ---
	// Responses might come via Multicast (handled by the main listening service if Start was called)
	// or via HTTP /register (handled by the HTTP server).
	// We just wait for the context timeout, collecting all devices discovered during that time.

	var lastDevices []*model.Device

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logrus.Infof("Discovery scan finished. %d device(s) found.", len(lastDevices))
			return lastDevices, nil
		case <-ticker.C:
			devices := s.GetDevices()
			lastDevices = devices
			logrus.Debugf("Discovery scan progress: %d device(s) found so far.", len(devices))
		}
	}
}

// GetDevices returns all currently known devices
func (s *Service) GetDevices() []*model.Device {
	s.devicesMutex.RLock()
	defer s.devicesMutex.RUnlock()

	devices := make([]*model.Device, 0, len(s.devices))
	for _, device := range s.devices {
		// Only include non-stale devices
		if !device.IsStale(s.config.DeviceTimeout) {
			devices = append(devices, device)
		}
	}

	return devices
}

// GetDevice returns a specific device by ID
func (s *Service) GetDevice(id string) *model.Device {
	s.devicesMutex.RLock()
	defer s.devicesMutex.RUnlock()

	if device, ok := s.devices[id]; ok {
		// Return only if not stale
		if !device.IsStale(s.config.DeviceTimeout) {
			return device
		}
	}

	return nil
}

// AddDeviceHandler adds a handler for device discovery events
func (s *Service) AddDeviceHandler(handler func(*model.Device)) {
	s.handlers = append(s.handlers, handler)
}

// updateDevice updates the device list with a newly discovered device
func (s *Service) updateDevice(device *model.Device) {
	s.devicesMutex.Lock()
	defer s.devicesMutex.Unlock()

	existingDevice, exists := s.devices[device.Fingerprint]
	if exists {
		// Update last seen timestamp
		existingDevice.UpdateLastSeen()
	} else {
		// Add new device
		s.devices[device.Fingerprint] = device

		// Notify handlers about new device
		for _, handler := range s.handlers {
			go handler(device)
		}
	}
}

// startAnnouncementLoop starts a periodic announcement loop
func (s *Service) startAnnouncementLoop(ctx context.Context) {
	s.announceTimer = time.NewTimer(s.config.AnnounceInterval)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.announceTimer.C:
				if err := s.multicast.SendDiscoveryAnnouncement(); err != nil {
					logrus.Errorf("Failed to send periodic announcement: %v", err)
				}
				s.announceTimer.Reset(s.config.AnnounceInterval)
			}
		}
	}()
}
