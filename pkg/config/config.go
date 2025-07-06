package config

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/bethropolis/localgo/pkg/crypto"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/sirupsen/logrus"
)

const (
	DefaultPort           = 53317
	DefaultMulticastGroup = "224.0.0.167"
	ProtocolVersion       = "2.1"
	DefaultSecurityDir    = ".localgo_security"
	DefaultSecurityFile   = "context.json"
)

type Config struct {
	Alias           string                        `json:"alias"`
	Port            int                           `json:"port"`
	HttpsEnabled    bool                          `json:"https_enabled"`
	MulticastGroup  string                        `json:"multicast_group"`
	DeviceModel     *string                       `json:"deviceModel"`
	DeviceType      model.DeviceType              `json:"deviceType"`
	SecurityContext *crypto.StoredSecurityContext `json:"-"`
	SecurityPath    string                        `json:"-"`
	PIN             string                        `json:"-"`
	DownloadDir     string                        `json:"-"`
}

func LoadConfig() (*Config, error) {
	alias := os.Getenv("LOCALSEND_ALIAS")
	if alias == "" {
		alias = generateDefaultAlias()
	}

	exePath, err := os.Executable()
	if err != nil {
		logrus.Warnf("Could not get executable path, using current directory for security file: %v", err)
		exePath = "."
	}
	securityDirPath := filepath.Join(filepath.Dir(exePath), DefaultSecurityDir)
	securityFilePath := filepath.Join(securityDirPath, DefaultSecurityFile)

	portStr := os.Getenv("LOCALSEND_PORT")
	port := DefaultPort
	if p, err := strconv.Atoi(portStr); err == nil {
		port = p
	}

	multicastGroup := os.Getenv("LOCALSEND_MULTICAST_GROUP")
	if multicastGroup == "" {
		multicastGroup = DefaultMulticastGroup
	}

	downloadDir := os.Getenv("LOCALSEND_DOWNLOAD_DIR")
	if downloadDir == "" {
		downloadDir = "./downloads"
	}

	securityContext, err := crypto.LoadSecurityContext(securityFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			logrus.Infof("Security context not found at %s, generating new one...", securityFilePath)
			securityContext, err = crypto.GenerateSecurityContext(alias)
			if err != nil {
				return nil, fmt.Errorf("failed to generate security context: %w", err)
			}
			if err := os.MkdirAll(securityDirPath, 0700); err != nil {
				logrus.Warnf("Could not create security directory '%s': %v", securityDirPath, err)
			}
			if err := crypto.SaveSecurityContext(securityContext, securityFilePath); err != nil {
				logrus.Warnf("failed to save newly generated security context to '%s': %v", securityFilePath, err)
			}
		} else {
			return nil, fmt.Errorf("failed to load security context from '%s': %w", securityFilePath, err)
		}
	}

	deviceModel := "GoDevice"
	deviceType := model.DeviceTypeDesktop

	cfg := &Config{
		Alias:           alias,
		Port:            port,
		MulticastGroup:  multicastGroup,
		HttpsEnabled:    true,
		SecurityContext: securityContext,
		SecurityPath:    securityFilePath,
		DeviceModel:     &deviceModel,
		DeviceType:      deviceType,
		DownloadDir:     downloadDir,
	}

	return cfg, nil
}

func generateDefaultAlias() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		logrus.Info("Could not get hostname, generating random alias suffix.")
		hostname = "LocalGo"
	}
	return hostname
}

func generateRandomID(length int) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	if _, err := rand.Read(result); err != nil {
		timeBasedID := strconv.FormatInt(time.Now().UnixNano(), 10)
		if len(timeBasedID) >= length {
			return timeBasedID[:length]
		}
		for i := 0; i < length; i++ {
			result[i] = chars[time.Now().UnixNano()%int64(len(chars))]
		}
		return string(result)
	}
	for i := 0; i < length; i++ {
		result[i] = chars[int(result[i])%len(chars)]
	}
	return string(result)
}

// ToRegisterDto converts Config to model.RegisterDto for discovery requests
func (c *Config) ToRegisterDto() model.RegisterDto {
	protocol := model.ProtocolTypeHTTP
	if c.HttpsEnabled {
		protocol = model.ProtocolTypeHTTPS
	}
	return model.RegisterDto{
		Alias:       c.Alias,
		Version:     ProtocolVersion, // Use constant from this package
		DeviceModel: c.DeviceModel,
		DeviceType:  c.DeviceType,
		Fingerprint: c.SecurityContext.CertificateHash,
		Port:        c.Port,
		Protocol:    protocol,
		Download:    false, // Update later when download server is implemented
	}
}

// ToInfoDto converts Config to model.InfoDto for discovery requests
func (c *Config) ToInfoDto() model.InfoDto {
	return model.InfoDto{
		Alias:       c.Alias,
		Version:     ProtocolVersion,
		DeviceModel: c.DeviceModel,
		DeviceType:  c.DeviceType,
		Fingerprint: c.SecurityContext.CertificateHash,
		Download:    false, // Update later when download server is implemented
	}
}
