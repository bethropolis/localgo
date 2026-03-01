package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/crypto"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/server/services"
	"go.uber.org/zap"
)

var testLogger = zap.NewNop().Sugar()

func TestDiscoveryHandler_InfoHandler(t *testing.T) {
	secCtx := &crypto.StoredSecurityContext{
		CertificateHash: "testfingerprint123",
	}

	cfg := &config.Config{
		Alias:           "TestDevice",
		HttpsEnabled:    true,
		DeviceType:      model.DeviceTypeDesktop,
		DeviceModel:     func() *string { s := "TestModel"; return &s }(),
		SecurityContext: secCtx,
	}

	registrySvc := services.NewRegistryService()
	sendSvc := services.NewSendService()

	handler := NewDiscoveryHandler(cfg, registrySvc, sendSvc, testLogger)

	req := httptest.NewRequest(http.MethodGet, "/info", nil)
	w := httptest.NewRecorder()

	handler.InfoHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var dto model.InfoDto
	err := json.Unmarshal(w.Body.Bytes(), &dto)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if dto.Alias != "TestDevice" {
		t.Errorf("Expected alias 'TestDevice', got '%s'", dto.Alias)
	}

	if dto.Version != config.ProtocolVersion {
		t.Errorf("Expected version '%s', got '%s'", config.ProtocolVersion, dto.Version)
	}

	if dto.Download {
		t.Error("Expected Download to be false when no send session is active")
	}
}

func TestDiscoveryHandler_InfoHandler_WithSendSession(t *testing.T) {
	secCtx := &crypto.StoredSecurityContext{
		CertificateHash: "testfingerprint123",
	}

	cfg := &config.Config{
		Alias:           "TestDevice",
		HttpsEnabled:    true,
		DeviceType:      model.DeviceTypeDesktop,
		SecurityContext: secCtx,
	}

	registrySvc := services.NewRegistryService()
	sendSvc := services.NewSendService()

	files := map[string]model.FileDto{"file1": {ID: "file1", FileName: "test.txt"}}
	sendSvc.CreateSession(files, map[string]string{"file1": "/path/to/test.txt"})

	handler := NewDiscoveryHandler(cfg, registrySvc, sendSvc, testLogger)

	req := httptest.NewRequest(http.MethodGet, "/info", nil)
	w := httptest.NewRecorder()

	handler.InfoHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var dto model.InfoDto
	json.Unmarshal(w.Body.Bytes(), &dto)

	if !dto.Download {
		t.Error("Expected Download to be true when send session is active")
	}
}

func TestDiscoveryHandler_RegisterHandler(t *testing.T) {
	secCtx := &crypto.StoredSecurityContext{
		CertificateHash: "testfingerprint123",
	}

	cfg := &config.Config{
		Alias:           "TestDevice",
		HttpsEnabled:    true,
		DeviceType:      model.DeviceTypeDesktop,
		SecurityContext: secCtx,
	}

	registrySvc := services.NewRegistryService()
	sendSvc := services.NewSendService()

	handler := NewDiscoveryHandler(cfg, registrySvc, sendSvc, testLogger)

	registerDto := model.RegisterDto{
		Alias:       "RemoteDevice",
		Version:     "2.1",
		Fingerprint: "remotefp123",
		Port:        53317,
		Protocol:    model.ProtocolTypeHTTPS,
		DeviceType:  model.DeviceTypeMobile,
	}

	body, _ := json.Marshal(registerDto)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RegisterHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	devices := registrySvc.GetDevices()
	if len(devices) != 1 {
		t.Errorf("Expected 1 registered device, got %d", len(devices))
	}

	if devices[0].Alias != "RemoteDevice" {
		t.Errorf("Expected device alias 'RemoteDevice', got '%s'", devices[0].Alias)
	}
}

func TestDiscoveryHandler_RegisterHandler_MalformedBody(t *testing.T) {
	secCtx := &crypto.StoredSecurityContext{
		CertificateHash: "testfingerprint123",
	}

	cfg := &config.Config{
		Alias:           "TestDevice",
		HttpsEnabled:    true,
		SecurityContext: secCtx,
	}

	registrySvc := services.NewRegistryService()
	sendSvc := services.NewSendService()

	handler := NewDiscoveryHandler(cfg, registrySvc, sendSvc, testLogger)

	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RegisterHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestDiscoveryHandler_RegisterHandler_WrongMethod(t *testing.T) {
	secCtx := &crypto.StoredSecurityContext{
		CertificateHash: "testfingerprint123",
	}

	cfg := &config.Config{
		Alias:           "TestDevice",
		HttpsEnabled:    true,
		SecurityContext: secCtx,
	}

	registrySvc := services.NewRegistryService()
	sendSvc := services.NewSendService()

	handler := NewDiscoveryHandler(cfg, registrySvc, sendSvc, testLogger)

	req := httptest.NewRequest(http.MethodGet, "/register", nil)
	w := httptest.NewRecorder()

	handler.RegisterHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}
