package services

import (
	"testing"
	"time"

	"github.com/bethropolis/localgo/pkg/model"
)

func TestRegistryService_RegisterDevice(t *testing.T) {
	svc := NewRegistryService()

	device := &model.Device{
		IP:          "192.168.1.100",
		Port:        53317,
		Alias:       "TestDevice",
		Fingerprint: "abc123",
		DeviceType:  model.DeviceTypeDesktop,
	}

	svc.RegisterDevice(device)

	devices := svc.GetDevices()
	if len(devices) != 1 {
		t.Errorf("Expected 1 device, got %d", len(devices))
	}

	if devices[0].Alias != "TestDevice" {
		t.Errorf("Expected alias 'TestDevice', got '%s'", devices[0].Alias)
	}
}

func TestRegistryService_RegisterDevice_UpdatesExisting(t *testing.T) {
	svc := NewRegistryService()

	device1 := &model.Device{
		IP:          "192.168.1.100",
		Port:        53317,
		Alias:       "TestDevice",
		Fingerprint: "abc123",
		DeviceType:  model.DeviceTypeDesktop,
	}

	device2 := &model.Device{
		IP:          "192.168.1.101",
		Port:        53318,
		Alias:       "UpdatedDevice",
		Fingerprint: "abc123",
		DeviceType:  model.DeviceTypeMobile,
	}

	svc.RegisterDevice(device1)
	svc.RegisterDevice(device2)

	devices := svc.GetDevices()
	if len(devices) != 1 {
		t.Errorf("Expected 1 device (updated), got %d", len(devices))
	}

	if devices[0].Alias != "UpdatedDevice" {
		t.Errorf("Expected alias 'UpdatedDevice', got '%s'", devices[0].Alias)
	}
}

func TestRegistryService_GetDevices(t *testing.T) {
	svc := NewRegistryService()

	devices := svc.GetDevices()
	if len(devices) != 0 {
		t.Errorf("Expected 0 devices initially, got %d", len(devices))
	}

	svc.RegisterDevice(&model.Device{
		IP:          "192.168.1.100",
		Fingerprint: "abc123",
		Alias:       "Device1",
	})

	svc.RegisterDevice(&model.Device{
		IP:          "192.168.1.101",
		Fingerprint: "def456",
		Alias:       "Device2",
	})

	devices = svc.GetDevices()
	if len(devices) != 2 {
		t.Errorf("Expected 2 devices, got %d", len(devices))
	}
}

func TestRegistryService_CleanupStaleDevices(t *testing.T) {
	svc := NewRegistryService()

	staleDevice := &model.Device{
		IP:          "192.168.1.100",
		Fingerprint: "abc123",
		Alias:       "StaleDevice",
		LastSeen:    time.Now().Add(-2 * time.Hour),
	}

	freshDevice := &model.Device{
		IP:          "192.168.1.101",
		Fingerprint: "def456",
		Alias:       "FreshDevice",
		LastSeen:    time.Now(),
	}

	svc.RegisterDevice(staleDevice)
	svc.RegisterDevice(freshDevice)

	svc.CleanupStaleDevices(1 * time.Hour)

	devices := svc.GetDevices()
	if len(devices) != 1 {
		t.Errorf("Expected 1 device after cleanup, got %d", len(devices))
	}

	if devices[0].Alias != "FreshDevice" {
		t.Errorf("Expected 'FreshDevice' to remain, got '%s'", devices[0].Alias)
	}
}

func TestRegistryService_EmptyCleanup(t *testing.T) {
	svc := NewRegistryService()

	svc.RegisterDevice(&model.Device{
		IP:          "192.168.1.100",
		Fingerprint: "abc123",
		Alias:       "Device",
		LastSeen:    time.Now(),
	})

	svc.CleanupStaleDevices(1 * time.Hour)

	devices := svc.GetDevices()
	if len(devices) != 1 {
		t.Errorf("Expected 1 device, got %d", len(devices))
	}
}
