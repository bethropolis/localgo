// Package model contains the data structures used throughout the LocalGo application
package model

const DefaultPort = 8080

// DeviceType defines the type of the device.
type DeviceType string

const (
	DeviceTypeMobile   DeviceType = "mobile"
	DeviceTypeDesktop  DeviceType = "desktop"
	DeviceTypeWeb      DeviceType = "web"
	DeviceTypeHeadless DeviceType = "headless"
	DeviceTypeServer   DeviceType = "server"
	DeviceTypeLaptop   DeviceType = "laptop"
	DeviceTypeTablet   DeviceType = "tablet"
	DeviceTypeOther    DeviceType = "other"
)

// ProtocolType defines the protocol type.
type ProtocolType string

const (
	ProtocolTypeHTTP  ProtocolType = "http"
	ProtocolTypeHTTPS ProtocolType = "https"
)

// DeviceInfo contains information about the current device
type DeviceInfo struct {
	Alias        string
	Version      string
	DeviceModel  *string
	DeviceType   DeviceType
	Fingerprint  string
	Port         int
	DownloadDir  string
	IP           string
	Download     bool
	HttpsEnabled bool
}

// ToInfoDto converts DeviceInfo to InfoDto
func (d *DeviceInfo) ToInfoDto() InfoDto {
	return InfoDto{
		Alias:       d.Alias,
		Version:     d.Version,
		DeviceModel: d.DeviceModel,
		DeviceType:  d.DeviceType,
		Fingerprint: d.Fingerprint,
		Download:    d.Download,
	}
}

// ToRegisterDto converts DeviceInfo to RegisterDto
func (d *DeviceInfo) ToRegisterDto() RegisterDto {
	protocol := ProtocolTypeHTTP
	if d.HttpsEnabled {
		protocol = ProtocolTypeHTTPS
	}

	return RegisterDto{
		Alias:       d.Alias,
		Version:     d.Version,
		DeviceModel: d.DeviceModel,
		DeviceType:  d.DeviceType,
		Fingerprint: d.Fingerprint,
		Port:        d.Port,
		Protocol:    protocol,
		Download:    d.Download,
	}
}

// ToMulticastDto converts DeviceInfo to MulticastDto
func (d *DeviceInfo) ToMulticastDto(announce bool) MulticastDto {
	protocol := ProtocolTypeHTTP
	if d.HttpsEnabled {
		protocol = ProtocolTypeHTTPS
	}

	return MulticastDto{
		Alias:       d.Alias,
		Version:     d.Version,
		DeviceModel: d.DeviceModel,
		DeviceType:  d.DeviceType,
		Fingerprint: d.Fingerprint,
		Port:        d.Port,
		Protocol:    protocol,
		Download:    d.Download,
		Announce:    announce,
	}
}

// InfoDto represents the response for /info and /register endpoints.
type InfoDto struct {
	Alias       string     `json:"alias"`
	Version     string     `json:"version"`
	DeviceModel *string    `json:"deviceModel"` // nullable
	DeviceType  DeviceType `json:"deviceType"`
	Fingerprint string     `json:"fingerprint"`
	Download    bool       `json:"download"`
}

// RegisterDto represents the request body for /register endpoint (sent by the discoverer).
type RegisterDto struct {
	Alias       string       `json:"alias"`
	Version     string       `json:"version"`
	DeviceModel *string      `json:"deviceModel"` // nullable
	DeviceType  DeviceType   `json:"deviceType"`
	Fingerprint string       `json:"fingerprint"`
	Port        int          `json:"port"`
	Protocol    ProtocolType `json:"protocol"` // "http" or "https"
	Download    bool         `json:"download"`
}

// MulticastDto represents the UDP discovery message.
type MulticastDto struct {
	Alias       string       `json:"alias"`
	Version     string       `json:"version"`
	DeviceModel *string      `json:"deviceModel"` // nullable
	DeviceType  DeviceType   `json:"deviceType"`
	Fingerprint string       `json:"fingerprint"`
	Port        int          `json:"port"`
	Protocol    ProtocolType `json:"protocol"` // http | https
	Download    bool         `json:"download"`
	Announce    bool         `json:"announce"` // True if initial announcement, false if response
}

// PrepareUploadRequestDto is sent to prepare file uploads
type PrepareUploadRequestDto struct {
	Info        InfoDto            `json:"info"`
	Files       map[string]FileDto `json:"files"`
	SendZipped  bool               `json:"sendZipped"`
	ForceBulk   bool               `json:"forceBulk"`
	TargetPath  string             `json:"targetPath"`
	KeepFolders bool               `json:"keepFolders"`
	Token       string             `json:"token,omitempty"`
}

// FileDto contains information about a file being uploaded
type FileDto struct {
	ID       string        `json:"id"`
	FileName string        `json:"fileName"`
	Size     int64         `json:"size"`
	FileType string        `json:"fileType"`
	SHA256   *string       `json:"sha256,omitempty"`   // Use pointer for nullable
	Preview  *string       `json:"preview,omitempty"`  // Use pointer for nullable
	Metadata *FileMetadata `json:"metadata,omitempty"` // Use pointer for nullable
	Legacy   bool          `json:"legacy,omitempty"`   // Added from Dart code
}

// FileMetadata holds optional file metadata (added in v2.1)
type FileMetadata struct {
	Modified *string `json:"modified,omitempty"` // Using string for ISO 8601 format
	Accessed *string `json:"accessed,omitempty"` // Using string for ISO 8601 format
}

// PrepareUploadResponseDto is returned after a successful upload preparation
type PrepareUploadResponseDto struct {
	SessionID string            `json:"sessionId"`
	Files     map[string]string `json:"files"`
	Token     string            `json:"token,omitempty"`
}

// ReceiveRequestResponseDto is returned for download preparations
type ReceiveRequestResponseDto struct {
	Info      InfoDto            `json:"info"` // Added Info field as per protocol spec
	SessionID string             `json:"sessionId"`
	Files     map[string]FileDto `json:"files"` // Changed to use FileDto
}

// StatusDto contains status information for a file
type StatusDto struct {
	Status string `json:"status"`
}
