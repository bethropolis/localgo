package services

import (
	"sync"
	"time"

	"github.com/bethropolis/localgo/pkg/model"
)

// RegistryService manages a list of known peer devices.
type RegistryService struct {
	devices      map[string]*model.Device
	devicesMutex sync.RWMutex
}

// NewRegistryService creates a new RegistryService.
func NewRegistryService() *RegistryService {
	return &RegistryService{
		devices: make(map[string]*model.Device),
	}
}

// RegisterDevice adds or updates a device in the registry.
func (s *RegistryService) RegisterDevice(device *model.Device) {
	s.devicesMutex.Lock()
	defer s.devicesMutex.Unlock()
	s.devices[device.Fingerprint] = device
}

// GetDevices returns a list of all registered devices.
func (s *RegistryService) GetDevices() []*model.Device {
	s.devicesMutex.RLock()
	defer s.devicesMutex.RUnlock()

	devices := make([]*model.Device, 0, len(s.devices))
	for _, dev := range s.devices {
		devices = append(devices, dev)
	}
	return devices
}

// CleanupStaleDevices removes devices that haven't been seen recently.
func (s *RegistryService) CleanupStaleDevices(staleThreshold time.Duration) {
	s.devicesMutex.Lock()
	defer s.devicesMutex.Unlock()

	for fingerprint, dev := range s.devices {
		if dev.IsStale(staleThreshold) {
			delete(s.devices, fingerprint)
		}
	}
}
