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
	if c.customFingerprint != "" {
		return c.customFingerprint
	}
	if c.HttpsEnabled {
		return c.SecurityContext.CertificateHash
	}
	return c.RandomFingerprint
}

// ToMulticastDto creates a MulticastDto from the current configuration.
func (c *Config) ToMulticastDto(download bool) model.MulticastDto {
	alias := c.Alias
	deviceModel := c.DeviceModel
	deviceType := c.DeviceType
	if c.Private {
		alias = "Anonymous"
		deviceModel = nil
		deviceType = model.DeviceTypeHeadless
	}
	return model.MulticastDto{
		Alias:       alias,
		Version:     ProtocolVersion,
		DeviceModel: deviceModel,
		DeviceType:  deviceType,
		Fingerprint: c.GetFingerprint(),
		Port:        c.Port,
		Protocol:    c.Protocol(),
		Download:    download,
		Announce:    true,
	}
}
