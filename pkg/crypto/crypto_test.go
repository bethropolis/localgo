package crypto

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

var testLogger = zap.NewNop().Sugar()

func TestGenerateSecurityContext(t *testing.T) {
	ctx, err := GenerateSecurityContext("test-device", testLogger)
	if err != nil {
		t.Fatalf("GenerateSecurityContext failed: %v", err)
	}

	if ctx.PrivateKey == "" {
		t.Error("PrivateKey should not be empty")
	}

	if ctx.Certificate == "" {
		t.Error("Certificate should not be empty")
	}

	if ctx.CertificateHash == "" {
		t.Error("CertificateHash should not be empty")
	}

	if len(ctx.CertificateHash) != 64 {
		t.Errorf("CertificateHash should be 64 chars (SHA256 hex), got %d", len(ctx.CertificateHash))
	}
}

func TestSaveAndLoadSecurityContext(t *testing.T) {
	origCtx, err := GenerateSecurityContext("test-device", testLogger)
	if err != nil {
		t.Fatalf("GenerateSecurityContext failed: %v", err)
	}

	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "security.json")

	err = SaveSecurityContext(origCtx, tmpPath, testLogger)
	if err != nil {
		t.Fatalf("SaveSecurityContext failed: %v", err)
	}

	loadedCtx, err := LoadSecurityContext(tmpPath, testLogger)
	if err != nil {
		t.Fatalf("LoadSecurityContext failed: %v", err)
	}

	if loadedCtx.PrivateKey != origCtx.PrivateKey {
		t.Error("PrivateKey mismatch after save/load")
	}

	if loadedCtx.Certificate != origCtx.Certificate {
		t.Error("Certificate mismatch after save/load")
	}

	if loadedCtx.CertificateHash != origCtx.CertificateHash {
		t.Error("CertificateHash mismatch after save/load")
	}
}

func TestLoadSecurityContextNotExist(t *testing.T) {
	_, err := LoadSecurityContext("/nonexistent/path/to/security.json", testLogger)
	if err == nil {
		t.Error("LoadSecurityContext should fail for nonexistent file")
	}
}

func TestSaveSecurityContextInvalidPath(t *testing.T) {
	ctx := &StoredSecurityContext{
		PrivateKey:      "test",
		Certificate:     "test",
		CertificateHash: "test",
	}

	err := SaveSecurityContext(ctx, "/invalid/path/that/does/not/exist/security.json", testLogger)
	if err == nil {
		t.Error("SaveSecurityContext should fail for invalid path")
	}
}

func TestSecurityContextFingerprintFormat(t *testing.T) {
	ctx, err := GenerateSecurityContext("test", testLogger)
	if err != nil {
		t.Fatalf("GenerateSecurityContext failed: %v", err)
	}

	for _, c := range ctx.CertificateHash {
		if c < '0' || (c > '9' && c < 'a') || c > 'f' {
			t.Errorf("CertificateHash contains non-hex character: %c", c)
		}
	}
}

func TestGenerateSecurityContextDifferentAliases(t *testing.T) {
	ctx1, err := GenerateSecurityContext("device1", testLogger)
	if err != nil {
		t.Fatalf("GenerateSecurityContext failed: %v", err)
	}

	ctx2, err := GenerateSecurityContext("device2", testLogger)
	if err != nil {
		t.Fatalf("GenerateSecurityContext failed: %v", err)
	}

	if ctx1.CertificateHash == ctx2.CertificateHash {
		t.Error("Different aliases should produce different certificate hashes")
	}
}

func TestStoredSecurityContextJSON(t *testing.T) {
	ctx := &StoredSecurityContext{
		PrivateKey:      "test-key",
		Certificate:     "test-cert",
		CertificateHash: "abc123",
	}

	tmpFile := filepath.Join(t.TempDir(), "test.json")

	file, err := os.Create(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(ctx); err != nil {
		t.Fatalf("Failed to encode: %v", err)
	}

	file.Close()

	loaded := &StoredSecurityContext{}
	file, err = os.Open(tmpFile)
	if err != nil {
		t.Fatalf("Failed to open temp file: %v", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(loaded); err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if loaded.PrivateKey != ctx.PrivateKey {
		t.Error("PrivateKey mismatch")
	}
}
