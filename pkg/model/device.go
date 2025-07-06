package model

import (
	"fmt"
	"net"
	"time"
)

// Device represents a peer device.
type Device struct {
	IP          string     `json:"ip"`
	Version     string     `json:"version"` // LocalSend protocol version
	Port        int        `json:"port"`
	Alias       string     `json:"alias"`
	Protocol    ProtocolType `json:"protocol"`
	
	Fingerprint string     `json:"fingerprint"`
	DeviceModel *string    `json:"deviceModel"` // nullable
	DeviceType  DeviceType `json:"deviceType"`
	Download    bool       `json:"download"` // Whether the device has download server running
	LastSeen    time.Time  `json:"-"`        // Not serialized to JSON
	Available   bool       `json:"-"`        // Not serialized to JSON
}

// NewDevice creates a new Device instance
func NewDevice(info RegisterDto, ip net.IP, detectedPort int, detectedHttps bool) *Device {
	// Use registered port/protocol if provided, otherwise use detected ones
	port := info.Port
	if port <= 0 {
		port = detectedPort
	}
	protocol := ProtocolTypeHTTP
	if detectedHttps {
		protocol = ProtocolTypeHTTPS
	}

	return &Device{
		IP:          ip.String(),
		Version:     info.Version,
		Port:        port,
		Alias:       info.Alias,
		Protocol:    protocol,
		Fingerprint: info.Fingerprint,
		DeviceModel: info.DeviceModel,
		DeviceType:  info.DeviceType,
		Download:    info.Download,
		LastSeen:    time.Now(),
		Available:   true,
	}
}

// UpdateLastSeen updates the last seen timestamp for a device
func (d *Device) UpdateLastSeen() {
	d.LastSeen = time.Now()
	d.Available = true
}

// IsStale checks if a device hasn't been seen recently
func (d *Device) IsStale(staleThreshold time.Duration) bool {
	return time.Since(d.LastSeen) > staleThreshold
}

// ToDebugString returns a string representation suitable for debugging
func (d *Device) ToDebugString() string {
	shortFingerprint := d.Fingerprint
	if len(shortFingerprint) > 8 {
		shortFingerprint = shortFingerprint[:8]
	}

	var deviceModelStr string
	if d.DeviceModel != nil {
		deviceModelStr = *d.DeviceModel
	} else {
		deviceModelStr = "nil"
	}

	return fmt.Sprintf("Device{IP: %s, Protocol: %s, Port: %d, Alias: %s, Fingerprint: %s..., DeviceModel: %s, DeviceType: %s, Download: %t}",
		d.IP, d.Protocol, d.Port, d.Alias, shortFingerprint, deviceModelStr, d.DeviceType, d.Download)
}

// ToInfoDto converts a Device to an InfoDto for API responses
func (d *Device) ToInfoDto() InfoDto {
	return InfoDto{
		Alias:       d.Alias,
		Version:     d.Version,
		DeviceModel: d.DeviceModel,
		DeviceType:  d.DeviceType,
		Fingerprint: d.Fingerprint,
		Download:    d.Download,
	}
}

// FromMulticastDto creates a Device from a MulticastDto and source IP
func FromMulticastDto(dto MulticastDto, ip net.IP) *Device {
	return &Device{
		IP:          ip.String(),
		Version:     dto.Version,
		Port:        dto.Port,
		Alias:       dto.Alias,
		Protocol:    dto.Protocol,
		Fingerprint: dto.Fingerprint,
		DeviceModel: dto.DeviceModel,
		DeviceType:  dto.DeviceType,
		Download:    dto.Download,
		LastSeen:    time.Now(),
		Available:   true,
	}
}
