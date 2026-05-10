package config

import "github.com/bethropolis/localgo/pkg/model"

// Protocol returns the protocol type based on HttpsEnabled.
func (c *Config) Protocol() model.ProtocolType {
	if c.HttpsEnabled {
		return model.ProtocolTypeHTTPS
	}
	return model.ProtocolTypeHTTP
}

// GetFingerprint returns the appropriate fingerprint (certificate hash if HTTPS, random otherwise).
func (c *Config) GetFingerprint() string {
	if c.HttpsEnabled {
		return c.SecurityContext.CertificateHash
	}
	return c.RandomFingerprint
}

// ToMulticastDto creates a MulticastDto from the current configuration.
func (c *Config) ToMulticastDto(download bool) model.MulticastDto {
	return model.MulticastDto{
		Alias:       c.Alias,
		Version:     ProtocolVersion,
		DeviceModel: c.DeviceModel,
		DeviceType:  c.DeviceType,
		Fingerprint: c.GetFingerprint(),
		Port:        c.Port,
		Protocol:    c.Protocol(),
		Download:    download,
		Announce:    true,
	}
}
