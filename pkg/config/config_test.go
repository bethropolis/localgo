package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bethropolis/localgo/pkg/model"
)

func TestLoadConfig_WithEnvVars(t *testing.T) {
	origEnv := saveEnv()
	defer restoreEnv(origEnv)

	os.Setenv("LOCALSEND_ALIAS", "TestAlias")
	os.Setenv("LOCALSEND_PORT", "53318")
	os.Setenv("LOCALSEND_FORCE_HTTP", "true")
	os.Setenv("LOCALSEND_DEVICE_TYPE", "mobile")
	os.Setenv("LOCALSEND_DEVICE_MODEL", "TestPhone")
	os.Setenv("LOCALSEND_AUTO_ACCEPT", "true")
	os.Setenv("LOCALSEND_MULTICAST_GROUP", "224.0.0.168")
	os.Setenv("LOCALSEND_DOWNLOAD_DIR", "/tmp/test-downloads")

	tmpDir := t.TempDir()
	os.Setenv("LOCALSEND_SECURITY_DIR", tmpDir)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Alias != "TestAlias" {
		t.Errorf("Expected alias 'TestAlias', got '%s'", cfg.Alias)
	}

	if cfg.Port != 53318 {
		t.Errorf("Expected port 53318, got %d", cfg.Port)
	}

	if cfg.HttpsEnabled {
		t.Error("Expected HttpsEnabled to be false when LOCALSEND_FORCE_HTTP=true")
	}

	if cfg.DeviceType != model.DeviceTypeMobile {
		t.Errorf("Expected device type 'mobile', got '%s'", cfg.DeviceType)
	}

	if cfg.DeviceModel == nil || *cfg.DeviceModel != "TestPhone" {
		t.Errorf("Expected device model 'TestPhone', got '%v'", cfg.DeviceModel)
	}

	if !cfg.AutoAccept {
		t.Error("Expected AutoAccept to be true")
	}

	if cfg.MulticastGroup != "224.0.0.168" {
		t.Errorf("Expected multicast group '224.0.0.168', got '%s'", cfg.MulticastGroup)
	}

	if cfg.DownloadDir != "/tmp/test-downloads" {
		t.Errorf("Expected download dir '/tmp/test-downloads', got '%s'", cfg.DownloadDir)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	origEnv := saveEnv()
	defer restoreEnv(origEnv)

	clearEnv()

	tmpDir := t.TempDir()
	os.Setenv("LOCALSEND_SECURITY_DIR", tmpDir)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Alias == "" {
		t.Error("Expected non-empty alias")
	}

	if cfg.Port != DefaultPort {
		t.Errorf("Expected default port %d, got %d", DefaultPort, cfg.Port)
	}

	if !cfg.HttpsEnabled {
		t.Error("Expected HttpsEnabled to be true by default")
	}

	if cfg.MulticastGroup != DefaultMulticastGroup {
		t.Errorf("Expected default multicast group '%s', got '%s'", DefaultMulticastGroup, cfg.MulticastGroup)
	}

	if cfg.SecurityContext == nil {
		t.Error("Expected SecurityContext to be generated")
	}
}

func TestToRegisterDto(t *testing.T) {
	origEnv := saveEnv()
	defer restoreEnv(origEnv)

	tmpDir := t.TempDir()
	os.Setenv("LOCALSEND_SECURITY_DIR", tmpDir)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	dto := cfg.ToRegisterDto()

	if dto.Alias != cfg.Alias {
		t.Errorf("Expected alias '%s', got '%s'", cfg.Alias, dto.Alias)
	}

	if dto.Version != ProtocolVersion {
		t.Errorf("Expected version '%s', got '%s'", ProtocolVersion, dto.Version)
	}

	if dto.DeviceModel != cfg.DeviceModel {
		t.Errorf("Expected device model '%v', got '%v'", cfg.DeviceModel, dto.DeviceModel)
	}

	if dto.DeviceType != cfg.DeviceType {
		t.Errorf("Expected device type '%s', got '%s'", cfg.DeviceType, dto.DeviceType)
	}

	if dto.Fingerprint == "" {
		t.Error("Expected non-empty fingerprint")
	}

	if dto.Port != cfg.Port {
		t.Errorf("Expected port %d, got %d", cfg.Port, dto.Port)
	}
}

func TestToInfoDto(t *testing.T) {
	origEnv := saveEnv()
	defer restoreEnv(origEnv)

	tmpDir := t.TempDir()
	os.Setenv("LOCALSEND_SECURITY_DIR", tmpDir)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	dto := cfg.ToInfoDto()

	if dto.Alias != cfg.Alias {
		t.Errorf("Expected alias '%s', got '%s'", cfg.Alias, dto.Alias)
	}

	if dto.Version != ProtocolVersion {
		t.Errorf("Expected version '%s', got '%s'", ProtocolVersion, dto.Version)
	}

	if dto.Fingerprint == "" {
		t.Error("Expected non-empty fingerprint")
	}
}

func TestGetSecurityDir_EnvOverride(t *testing.T) {
	origEnv := saveEnv()
	defer restoreEnv(origEnv)

	clearEnv()

	tmpDir := t.TempDir()
	os.Setenv("LOCALSEND_SECURITY_DIR", tmpDir)

	dir := getSecurityDir()
	if dir != tmpDir {
		t.Errorf("Expected security dir '%s', got '%s'", tmpDir, dir)
	}
}

func TestGenerateRandomID(t *testing.T) {
	id1 := generateRandomID(16)
	id2 := generateRandomID(16)

	if len(id1) != 16 {
		t.Errorf("Expected ID length 16, got %d", len(id1))
	}

	if id1 == id2 {
		t.Error("Expected different random IDs")
	}
}

func saveEnv() map[string]string {
	env := make(map[string]string)
	envVars := []string{
		"LOCALSEND_ALIAS", "LOCALSEND_PORT", "LOCALSEND_FORCE_HTTP",
		"LOCALSEND_DEVICE_TYPE", "LOCALSEND_DEVICE_MODEL", "LOCALSEND_AUTO_ACCEPT",
		"LOCALSEND_MULTICAST_GROUP", "LOCALSEND_DOWNLOAD_DIR", "LOCALSEND_SECURITY_DIR",
		"XDG_CONFIG_HOME", "HOME", "APPDATA",
	}
	for _, v := range envVars {
		env[v] = os.Getenv(v)
	}
	return env
}

func restoreEnv(origEnv map[string]string) {
	for k, v := range origEnv {
		if v != "" {
			os.Setenv(k, v)
		} else {
			os.Unsetenv(k)
		}
	}
}

func clearEnv() {
	envVars := []string{
		"LOCALSEND_ALIAS", "LOCALSEND_PORT", "LOCALSEND_FORCE_HTTP",
		"LOCALSEND_DEVICE_TYPE", "LOCALSEND_DEVICE_MODEL", "LOCALSEND_AUTO_ACCEPT",
		"LOCALSEND_MULTICAST_GROUP", "LOCALSEND_DOWNLOAD_DIR", "LOCALSEND_SECURITY_DIR",
		"XDG_CONFIG_HOME", "HOME", "APPDATA",
	}
	for _, v := range envVars {
		os.Unsetenv(v)
	}
}

func TestConfig_Constants(t *testing.T) {
	if DefaultPort != 53317 {
		t.Errorf("Expected DefaultPort 53317, got %d", DefaultPort)
	}

	if DefaultMulticastGroup != "224.0.0.167" {
		t.Errorf("Expected DefaultMulticastGroup '224.0.0.167', got '%s'", DefaultMulticastGroup)
	}

	if ProtocolVersion != "2.1" {
		t.Errorf("Expected ProtocolVersion '2.1', got '%s'", ProtocolVersion)
	}
}

var _ = time.Now      // silence unused import
var _ = filepath.Join // silence unused import
