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
	"go.uber.org/zap"
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
	MaxBodySize       int64                         `json:"-"`
	NoClipboard       bool                          `json:"-"` // skip clipboard; save text as a file instead
	HistoryFile       string                        `json:"-"` // path to transfer history jsonl file
	Quiet             bool                          `json:"-"` // quiet mode - minimal output
	ExecHook          string                        `json:"-"` // shell command to run after receiving file
}

// getSecurityDir determines the best location for the security directory
func getSecurityDir() string {
    if envDir := os.Getenv("LOCALSEND_SECURITY_DIR"); envDir != "" {
        zap.S().Infof("Using security directory: %s", envDir)
        return envDir
    }

    configDir, err := os.UserConfigDir()
    if err != nil {
        zap.S().Warnf("Could not determine config directory: %v; falling back to current directory", err)
        return DefaultSecurityDir
    }

    return filepath.Join(configDir, "localgo", ".security")
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

func LoadConfig(logger *zap.SugaredLogger) (*Config, error) {
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
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		downloadDir = filepath.Join(home, "Downloads", "localgo")
	}

	maxBodySizeStr := os.Getenv("LOCALSEND_MAX_BODY_SIZE")
	maxBodySize := int64(0)
	if maxBodySizeStr != "" {
		if size, err := strconv.ParseInt(maxBodySizeStr, 10, 64); err == nil {
			maxBodySize = size
		} else {
			zap.S().Warnf("Invalid LOCALSEND_MAX_BODY_SIZE value: %s, using default", maxBodySizeStr)
		}
	}

	// Parse LOCALSEND_FORCE_HTTP
	forceHTTP := os.Getenv("LOCALSEND_FORCE_HTTP") == "true" || os.Getenv("LOCALSEND_FORCE_HTTP") == "1"
	HttpsEnabled := !forceHTTP

	securityContext, err := crypto.LoadSecurityContext(securityFilePath, logger)
	if err != nil {
		if os.IsNotExist(err) {
			zap.S().Infof("Security context not found at %s, generating new one...", securityFilePath)
			securityContext, err = crypto.GenerateSecurityContext(alias, logger)
			if err != nil {
				return nil, fmt.Errorf("failed to generate security context: %w", err)
			}
			if err := os.MkdirAll(securityDirPath, 0700); err != nil {
				zap.S().Warnf("Could not create security directory '%s': %v", securityDirPath, err)
			}
			if err := crypto.SaveSecurityContext(securityContext, securityFilePath, logger); err != nil {
				zap.S().Warnf("failed to save newly generated security context to '%s': %v", securityFilePath, err)
			}
		} else {
			return nil, fmt.Errorf("failed to load security context from '%s': %w", securityFilePath, err)
		}
	}

	deviceModel := "GoDevice"
	deviceType := model.DeviceTypeDesktop

	// Parse LOCALSEND_DEVICE_MODEL
	if envDeviceModel := os.Getenv("LOCALSEND_DEVICE_MODEL"); envDeviceModel != "" {
		deviceModel = envDeviceModel
	}

	// Parse LOCALSEND_DEVICE_TYPE
	if envDeviceType := os.Getenv("LOCALSEND_DEVICE_TYPE"); envDeviceType != "" {
		deviceType = model.DeviceType(envDeviceType)
	}

	autoAccept := os.Getenv("LOCALSEND_AUTO_ACCEPT") == "true" || os.Getenv("LOCALSEND_AUTO_ACCEPT") == "1"
	noClipboard := os.Getenv("LOCALSEND_NO_CLIPBOARD") == "true" || os.Getenv("LOCALSEND_NO_CLIPBOARD") == "1"
	quiet := os.Getenv("LOCALSEND_QUIET") == "true" || os.Getenv("LOCALSEND_QUIET") == "1"

	historyFile := os.Getenv("LOCALSEND_HISTORY")

	execHook := os.Getenv("LOCALSEND_EXEC")

	cfg := &Config{
		Alias:             alias,
		Port:              port,
		MulticastGroup:    multicastGroup,
		HttpsEnabled:      HttpsEnabled,
		SecurityContext:   securityContext,
		SecurityPath:      securityFilePath,
		DeviceModel:       &deviceModel,
		DeviceType:        deviceType,
		DownloadDir:       downloadDir,
		AutoAccept:        autoAccept,
		RandomFingerprint: generateRandomID(64),
		MaxBodySize:       maxBodySize,
		NoClipboard:       noClipboard,
		HistoryFile:       historyFile,
		Quiet:             quiet,
		ExecHook:          execHook,
	}

	return cfg, nil
}

func generateDefaultAlias() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		zap.S().Infow("Could not get hostname, generating random alias suffix.")
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
		Download:    true,
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
		Download:    true,
	}
}
