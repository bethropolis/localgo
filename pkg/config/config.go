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
	Alias             string                        `json:"alias"`
	Port              int                           `json:"port"`
	HttpsEnabled      bool                          `json:"https_enabled"`
	MulticastGroup    string                        `json:"multicast_group"`
	DeviceModel       *string                       `json:"deviceModel"`
	DeviceType        model.DeviceType              `json:"deviceType"`
	SecurityContext   *crypto.StoredSecurityContext `json:"-"`
	SecurityPath      string                        `json:"-"`
	PIN               string                        `json:"-"`
	DownloadDir       string                        `json:"-"`
	AutoAccept        bool                          `json:"-"`
	RandomFingerprint string                        `json:"-"`
}

// getSecurityDir determines the best location for the security directory
// Priority order:
// 1. XDG_CONFIG_HOME/localgo/.security (or platform equivalent)
// 2. HOME/.localgo/.security
// 3. Current directory .localgo_security (fallback for compatibility)
func getSecurityDir() string {
	// Check for explicit override via environment variable
	if envDir := os.Getenv("LOCALSEND_SECURITY_DIR"); envDir != "" {
		logrus.Infof("Using security directory from LOCALSEND_SECURITY_DIR: %s", envDir)
		return envDir
	}

	// Try XDG_CONFIG_HOME first (Linux/Unix standard)
	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		dir := filepath.Join(configHome, "localgo", ".security")
		if testDirWritable(dir) {
			logrus.Debugf("Using XDG config directory for security: %s", dir)
			return dir
		}
	}

	// Try HOME/.config/localgo/.security (XDG default when XDG_CONFIG_HOME not set)
	if home := os.Getenv("HOME"); home != "" {
		dir := filepath.Join(home, ".config", "localgo", ".security")
		if testDirWritable(dir) {
			logrus.Debugf("Using HOME/.config directory for security: %s", dir)
			return dir
		}
	}

	// Try APPDATA on Windows
	if appData := os.Getenv("APPDATA"); appData != "" {
		dir := filepath.Join(appData, "localgo", ".security")
		if testDirWritable(dir) {
			logrus.Debugf("Using APPDATA directory for security: %s", dir)
			return dir
		}
	}

	// Fallback: Try HOME/.localgo/.security
	if home := os.Getenv("HOME"); home != "" {
		dir := filepath.Join(home, ".localgo", ".security")
		if testDirWritable(dir) {
			logrus.Debugf("Using HOME/.localgo directory for security: %s", dir)
			return dir
		}
	}

	// Final fallback: Current directory (for compatibility with older versions)
	exePath, err := os.Executable()
	if err != nil {
		logrus.Warnf("Could not get executable path, using current directory for security: %v", err)
		exePath = "."
	}
	dir := filepath.Join(filepath.Dir(exePath), DefaultSecurityDir)
	logrus.Warnf("Using fallback security directory (current/executable dir): %s", dir)

	// Check if old location exists and provide migration hint
	if _, err := os.Stat(dir); err == nil {
		logrus.Infof("Found existing security directory at %s", dir)
		logrus.Infof("Consider moving to ~/.config/localgo/.security for better XDG compliance")
	}

	return dir
}

// testDirWritable tests if a directory is writable (creates it if needed)
// Returns true if the directory exists or can be created and is writable
func testDirWritable(dir string) bool {
	// Check if directory exists
	if info, err := os.Stat(dir); err == nil {
		// Directory exists, check if it's actually a directory
		if !info.IsDir() {
			return false
		}
		// Test write permission by attempting to create a temp file
		testFile := filepath.Join(dir, ".write_test")
		f, err := os.Create(testFile)
		if err != nil {
			return false
		}
		f.Close()
		os.Remove(testFile)
		return true
	}

	// Directory doesn't exist, try to create it
	if err := os.MkdirAll(dir, 0700); err != nil {
		return false
	}
	return true
}

func LoadConfig() (*Config, error) {
	alias := os.Getenv("LOCALSEND_ALIAS")
	if alias == "" {
		alias = generateDefaultAlias()
	}

	// Use the new security directory resolution
	securityDirPath := getSecurityDir()
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

	autoAccept := os.Getenv("LOCALSEND_AUTO_ACCEPT") == "true" || os.Getenv("LOCALSEND_AUTO_ACCEPT") == "1"

	cfg := &Config{
		Alias:             alias,
		Port:              port,
		MulticastGroup:    multicastGroup,
		HttpsEnabled:      true,
		SecurityContext:   securityContext,
		SecurityPath:      securityFilePath,
		DeviceModel:       &deviceModel,
		DeviceType:        deviceType,
		DownloadDir:       downloadDir,
		AutoAccept:        autoAccept,
		RandomFingerprint: generateRandomID(64),
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
	fingerprint := c.RandomFingerprint
	if c.HttpsEnabled {
		protocol = model.ProtocolTypeHTTPS
		fingerprint = c.SecurityContext.CertificateHash
	}
	return model.RegisterDto{
		Alias:       c.Alias,
		Version:     ProtocolVersion, // Use constant from this package
		DeviceModel: c.DeviceModel,
		DeviceType:  c.DeviceType,
		Fingerprint: fingerprint,
		Port:        c.Port,
		Protocol:    protocol,
		Download:    false, // Update later when download server is implemented
	}
}

// ToInfoDto converts Config to model.InfoDto for discovery requests
func (c *Config) ToInfoDto() model.InfoDto {
	fingerprint := c.RandomFingerprint
	if c.HttpsEnabled {
		fingerprint = c.SecurityContext.CertificateHash
	}
	return model.InfoDto{
		Alias:       c.Alias,
		Version:     ProtocolVersion,
		DeviceModel: c.DeviceModel,
		DeviceType:  c.DeviceType,
		Fingerprint: fingerprint,
		Download:    false, // Update later when download server is implemented
	}
}
