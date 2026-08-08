package config

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	mathrand "math/rand/v2"

	"github.com/bethropolis/localgo/pkg/crypto"
	"github.com/bethropolis/localgo/pkg/logging"
	"github.com/bethropolis/localgo/pkg/model"
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
	OpenDir           bool                          `json:"-"` // open download directory after transfer
	Concurrency       int                           `json:"-"` // max parallel uploads (0 = use default)
	MulticastInterface string                        `json:"-"` // multicast network interface name
	Private             bool                          `json:"-"` // anonymize device identities
	DiscoveryStrategy   string                        `json:"-"` // discovery strategy: "full" (default) or "fast" (skip subnet scan)
	FileConflictResolve string                        `json:"-"` // conflict resolution: "rename" (default), "overwrite", "skip"
	BindAddress         string                        `json:"-"` // bind to specific interface/IP for listening
	ShareOnce           bool                          `json:"-"` // stop after first download (--once)
	StaticPeers         []string                      `json:"-"` // statically defined peer addresses for Tier 1 cache probe
	TrustedFingerprints []string                      `json:"-"` // fingerprints that bypass the transfer acceptance prompt

	Shell             string `json:"-"` // shell command prefix for exec hooks (default: "sh -c" or "cmd /c")
	ClipboardWriteCmd string `json:"-"` // custom clipboard write command
	ClipboardReadCmd  string `json:"-"` // custom clipboard read command
	CustomTLSCertPath string `json:"-"` // path to custom TLS certificate file
	CustomTLSKeyPath  string `json:"-"` // path to custom TLS private key file
	NotificationCmd   string `json:"-"` // custom notification command
	customFingerprint string `json:"-"` // fingerprint computed from custom TLS cert
}

// SetCustomFingerprint overrides the advertised fingerprint with one computed
// from a user-supplied TLS certificate.
func (c *Config) SetCustomFingerprint(fp string) {
	c.customFingerprint = fp
}

// getSecurityDir determines the best location for the security directory
func getSecurityDir(src *Source) string {
	if envDir := src.GetString("security_dir"); envDir != "" {
		logging.Global().Infof("Using security directory: %s", envDir)
		return envDir
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		logging.Global().Warnf("Could not determine config directory: %v; falling back to current directory", err)
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

func LoadConfig(src *Source, logger *logging.Logger) (*Config, error) {
	if src == nil {
		src = NewSourceFromMap(map[string]any{})
	}
	alias := src.GetString("alias")
	if alias == "" {
		alias = generateDefaultAlias()
	}

	// Use the new security directory resolution
	securityDirPath := getSecurityDir(src)
	securityFilePath := filepath.Join(securityDirPath, DefaultSecurityFile)

	portStr := src.GetString("port")
	port := DefaultPort
	if p, err := strconv.Atoi(portStr); err == nil {
		port = p
	}

	multicastGroup := src.GetString("multicast_group")
	if multicastGroup == "" {
		multicastGroup = DefaultMulticastGroup
	}

	downloadDir := src.GetString("download_dir")
	if downloadDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		downloadDir = filepath.Join(home, "Downloads", "localgo")
	}

	maxBodySizeStr := src.GetString("max_body_size")
	maxBodySize := int64(0)
	if maxBodySizeStr != "" {
		if size, err := strconv.ParseInt(maxBodySizeStr, 10, 64); err == nil {
			maxBodySize = size
		} else {
			logging.Global().Warnf("Invalid LOCALSEND_MAX_BODY_SIZE value: %s, using default", maxBodySizeStr)
		}
	}

	multicastInterface := src.GetString("multicast_interface")

	// Parse LOCALSEND_FORCE_HTTP
	forceHTTP := src.GetString("force_http") == "true" || src.GetString("force_http") == "1"
	HttpsEnabled := !forceHTTP

	securityContext, err := crypto.LoadSecurityContext(securityFilePath, logger)
	if err != nil {
		if os.IsNotExist(err) {
			logging.Global().Infof("Security context not found at %s, generating new one...", securityFilePath)
			securityContext, err = crypto.GenerateSecurityContext(alias, logger)
			if err != nil {
				return nil, fmt.Errorf("failed to generate security context: %w", err)
			}
			if err := os.MkdirAll(securityDirPath, 0700); err != nil {
				logging.Global().Warnf("Could not create security directory '%s': %v", securityDirPath, err)
			}
			if err := crypto.SaveSecurityContext(securityContext, securityFilePath, logger); err != nil {
				logging.Global().Warnf("failed to save newly generated security context to '%s': %v", securityFilePath, err)
			}
		} else {
			return nil, fmt.Errorf("failed to load security context from '%s': %w", securityFilePath, err)
		}
	}

	deviceModel := "GoDevice"
	deviceType := model.DeviceTypeDesktop

	// Parse LOCALSEND_DEVICE_MODEL
	if envDeviceModel := src.GetString("device_model"); envDeviceModel != "" {
		deviceModel = envDeviceModel
	}

	// Parse LOCALSEND_DEVICE_TYPE
	if envDeviceType := src.GetString("device_type"); envDeviceType != "" {
		deviceType = model.DeviceType(envDeviceType)
	}

	autoAccept := src.GetString("auto_accept") == "true" || src.GetString("auto_accept") == "1"
	noClipboard := src.GetString("no_clipboard") == "true" || src.GetString("no_clipboard") == "1"
	quiet := src.GetString("quiet") == "true" || src.GetString("quiet") == "1"

	historyFile := src.GetString("history")

	execHook := src.GetString("exec")

	concurrency := src.GetInt("concurrency")

	shell := src.GetString("shell")
	clipboardWriteCmd := src.GetString("clipboard_write_cmd")
	clipboardReadCmd := src.GetString("clipboard_read_cmd")
	customTLSCertPath := src.GetString("tls_cert")
	customTLSKeyPath := src.GetString("tls_key")
	notificationCmd := src.GetString("notification_cmd")
	discoveryStrategy := src.GetString("discovery_strategy")
	if discoveryStrategy == "" {
		discoveryStrategy = "full"
	}
	fileConflictResolve := src.GetString("file_conflict_resolution")
	if fileConflictResolve == "" {
		fileConflictResolve = "rename"
	}
	bindAddress := src.GetString("bind_address")
	staticPeers := src.GetStringSlice("static_peers")
	trustedFingerprints := src.GetStringSlice("trusted_fingerprints")

	cfg := &Config{
		Alias:              alias,
		Port:               port,
		MulticastGroup:     multicastGroup,
		HttpsEnabled:       HttpsEnabled,
		SecurityContext:    securityContext,
		SecurityPath:       securityFilePath,
		DeviceModel:        &deviceModel,
		DeviceType:         deviceType,
		DownloadDir:        downloadDir,
		AutoAccept:         autoAccept,
		RandomFingerprint:  generateRandomID(64),
		MaxBodySize:        maxBodySize,
		NoClipboard:        noClipboard,
		HistoryFile:        historyFile,
		Quiet:              quiet,
		ExecHook:           execHook,
		Concurrency:        concurrency,
		MulticastInterface: multicastInterface,
		DiscoveryStrategy:  discoveryStrategy,
		FileConflictResolve: fileConflictResolve,
		BindAddress:        bindAddress,
		StaticPeers:        staticPeers,
		TrustedFingerprints: trustedFingerprints,
		Shell:              shell,
		ClipboardWriteCmd:  clipboardWriteCmd,
		ClipboardReadCmd:   clipboardReadCmd,
		CustomTLSCertPath:  customTLSCertPath,
		CustomTLSKeyPath:   customTLSKeyPath,
		NotificationCmd:    notificationCmd,
	}

	return cfg, nil
}

func generateDefaultAlias() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		logging.Global().Infow("Could not get hostname, generating random alias suffix.")
		hostname = "LocalGo"
	}
	return hostname
}

func generateRandomID(length int) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	if _, err := rand.Read(result); err != nil {
		// If crypto/rand fails, use math/rand/v2 seeded from crypto/rand
		var seed [32]byte
		rand.Read(seed[:]) // best-effort seed
		r := newRandFromSeed(seed)
		for i := range result {
			result[i] = chars[r.IntN(len(chars))]
		}
		return string(result)
	}
	for i := 0; i < length; i++ {
		result[i] = chars[int(result[i])%len(chars)]
	}
	return string(result)
}

func newRandFromSeed(seed [32]byte) *mathrand.Rand {
	var rngSeed uint64
	for i := 0; i < 8 && i < len(seed); i++ {
		rngSeed |= uint64(seed[i]) << (i * 8)
	}
	return mathrand.New(mathrand.NewPCG(rngSeed, uint64(seed[0])))
}

// ToRegisterDto converts Config to model.RegisterDto for discovery requests
func (c *Config) ToRegisterDto() model.RegisterDto {
	protocol := model.ProtocolTypeHTTP
	fingerprint := c.RandomFingerprint
	if c.HttpsEnabled {
		protocol = model.ProtocolTypeHTTPS
		fingerprint = c.SecurityContext.CertificateHash
	}
	alias := c.Alias
	deviceModel := c.DeviceModel
	deviceType := c.DeviceType
	if c.Private {
		alias = "Anonymous"
		deviceModel = nil
		deviceType = model.DeviceTypeHeadless
	}
	return model.RegisterDto{
		Alias:       alias,
		Version:     ProtocolVersion,
		DeviceModel: deviceModel,
		DeviceType:  deviceType,
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
	alias := c.Alias
	deviceModel := c.DeviceModel
	deviceType := c.DeviceType
	if c.Private {
		alias = "Anonymous"
		deviceModel = nil
		deviceType = model.DeviceTypeHeadless
	}
	return model.InfoDto{
		Alias:       alias,
		Version:     ProtocolVersion,
		DeviceModel: deviceModel,
		DeviceType:  deviceType,
		Fingerprint: fingerprint,
		Download:    true,
	}
}
