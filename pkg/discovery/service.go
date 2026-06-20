// File: pkg/discovery/service.go
package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bethropolis/localgo/pkg/model"
	"go.uber.org/zap"
)

// Service coordinates different discovery mechanisms
type Service struct {
	config        *ServiceConfig
	multicast     MulticastDiscoverer
	devices       map[string]*model.Device
	devicesMutex  sync.RWMutex
	handlers      []func(*model.Device)
	handlersMutex sync.RWMutex
	announceTimer *time.Timer
	peerCache     *PeerCache
	stopCh        chan struct{}
	stopOnce      sync.Once
	logger        *zap.SugaredLogger
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
		AnnounceInterval:   30 * time.Second,
		DeviceTimeout:      2 * time.Minute,
		EnableAnnouncement: true,
	}
}

// NewService creates a new discovery service
func NewService(config *ServiceConfig, multicast MulticastDiscoverer, logger *zap.SugaredLogger) *Service {
	if config == nil {
		config = DefaultServiceConfig()
	}
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	s := &Service{
		config:    config,
		devices:   make(map[string]*model.Device),
		multicast: multicast,
		stopCh:    make(chan struct{}),
		logger:    logger,
	}

	// ALWAYS ensure multicast propagates raw events upward to the Service state
	if s.multicast != nil {
		s.multicast.AddDeviceHandler(func(device *model.Device) {
			s.updateDevice(device)
		})
	}

	return s
}

// SetPeerCache sets the persistent peer cache for both service and multicast.
func (s *Service) SetPeerCache(cache *PeerCache) {
	s.peerCache = cache
	if mc, ok := s.multicast.(interface{ SetPeerCache(*PeerCache) }); ok {
		mc.SetPeerCache(cache)
	}
}

// Start initializes and starts the discovery service for listening and periodic announcements
func (s *Service) Start(ctx context.Context, dto model.MulticastDto) error {
	s.multicast.SetDto(dto)

	if err := s.multicast.StartListening(ctx); err != nil {
		return fmt.Errorf("failed to start multicast discovery: %w", err)
	}

	if s.config.EnableAnnouncement {
		s.startAnnouncementLoop(ctx)
	}

	if err := s.multicast.SendDiscoveryAnnouncement(); err != nil {
		s.logger.Errorf("Failed to send initial discovery announcement: %v", err)
	}

	// Probe cached peers in the background
	if s.peerCache != nil {
		probeCtx, cancelProbe := context.WithTimeout(ctx, 10*time.Second)
		go func() {
			defer cancelProbe()
			ProbeCached(probeCtx, s.peerCache, func(device *model.Device) {
				s.updateDevice(device)
			}, s.logger)
		}()
	}

	return nil
}

// Stop stops the discovery service
func (s *Service) Stop() {
	s.logger.Debugf("Stopping discovery service...")
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	if s.multicast != nil {
		s.multicast.Stop()
	}

	if s.announceTimer != nil {
		s.announceTimer.Stop()
	}
	s.logger.Debugf("Discovery service stopped.")
}

// Discover performs a discovery scan and returns found devices.
func (s *Service) Discover(ctx context.Context, dto model.MulticastDto) ([]*model.Device, error) {
	s.logger.Debugf("Performing one-off discovery scan...")

	s.multicast.SetDto(dto)

	// MUST be listening to receive multicast responses
	if err := s.multicast.StartListening(ctx); err != nil {
		s.logger.Debugf("Failed to start multicast listener (responses may be missed): %v", err)
	}

	if err := s.multicast.SendDiscoveryAnnouncement(); err != nil {
		s.logger.Errorf("Failed to send initial discovery announcement: %v", err)
	}

	var lastDevices []*model.Device

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			lastDevices = s.GetDevices() // Perform one absolute final read to guarantee capture
			s.logger.Debugf("Discovery scan finished. %d device(s) found.", len(lastDevices))
			return lastDevices, nil
		case <-ticker.C:
			devices := s.GetDevices()
			lastDevices = devices
		}
	}
}

// GetDevices returns all currently known devices
func (s *Service) GetDevices() []*model.Device {
	s.devicesMutex.RLock()
	defer s.devicesMutex.RUnlock()

	devices := make([]*model.Device, 0, len(s.devices))
	for _, device := range s.devices {
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
		if !device.IsStale(s.config.DeviceTimeout) {
			return device
		}
	}

	return nil
}

// AddDeviceHandler adds a handler for device discovery events
func (s *Service) AddDeviceHandler(handler func(*model.Device)) {
	s.handlersMutex.Lock()
	defer s.handlersMutex.Unlock()
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

		// Notify Service-level handlers about the new device ONLY ONCE (debounces the bursts)
		s.handlersMutex.RLock()
		handlers := make([]func(*model.Device), len(s.handlers))
		copy(handlers, s.handlers)
		s.handlersMutex.RUnlock()

		for _, handler := range handlers {
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
				s.announceTimer.Stop()
				return
			case <-s.stopCh:
				s.announceTimer.Stop()
				return
			case <-s.announceTimer.C:
				if err := s.multicast.SendDiscoveryAnnouncement(); err != nil {
					s.logger.Errorf("Failed to send periodic announcement: %v", err)
				}
				s.announceTimer.Reset(s.config.AnnounceInterval)
			}
		}
	}()
}
