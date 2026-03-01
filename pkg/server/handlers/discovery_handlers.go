// Package handlers contains HTTP handlers for the LocalGo server
package handlers

import (
	"encoding/json"
	"net"
	"net/http"

	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/httputil"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/server/services"
	"go.uber.org/zap"
)

// DiscoveryHandler handles /info and /register requests.
type DiscoveryHandler struct {
	config          *config.Config
	registryService *services.RegistryService
	sendService     *services.SendService
	logger          *zap.SugaredLogger
}

// NewDiscoveryHandler creates a new DiscoveryHandler.
func NewDiscoveryHandler(cfg *config.Config, registryService *services.RegistryService, sendService *services.SendService, logger *zap.SugaredLogger) *DiscoveryHandler {
	return &DiscoveryHandler{
		config:          cfg,
		registryService: registryService,
		sendService:     sendService,
		logger:          logger,
	}
}

// InfoHandler handles GET /info requests (v1 & v2 are identical here).
func (h *DiscoveryHandler) InfoHandler(w http.ResponseWriter, r *http.Request) {
	if h.config.SecurityContext == nil {
		h.logger.Info("Error: Security context not available for /info")
		httputil.RespondError(w, http.StatusInternalServerError, "Internal Server Error: Security context missing")
		return
	}

	senderFingerprint := r.URL.Query().Get("fingerprint")
	if senderFingerprint != "" && senderFingerprint == h.config.SecurityContext.CertificateHash {
		h.logger.Info("Received /info request from self, ignoring.")
		httputil.RespondError(w, http.StatusPreconditionFailed, "Self-discovered")
		return
	}

	downloadCapable := h.sendService.GetSession() != nil // True if we have an active send session

	dto := model.InfoDto{
		Alias:       h.config.Alias,
		Version:     config.ProtocolVersion,
		DeviceModel: h.config.DeviceModel,
		DeviceType:  h.config.DeviceType,
		Fingerprint: h.config.SecurityContext.CertificateHash,
		Download:    downloadCapable,
	}

	h.logger.Infof("Responding to /info request from %s", r.RemoteAddr)
	httputil.RespondJSON(w, http.StatusOK, dto)
}

// RegisterHandler handles POST /register requests (v1 & v2 are identical here).
func (h *DiscoveryHandler) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if h.config.SecurityContext == nil {
		h.logger.Info("Error: Security context not available for /register")
		httputil.RespondError(w, http.StatusInternalServerError, "Internal Server Error: Security context missing")
		return
	}

	if r.Method != http.MethodPost {
		httputil.RespondError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}

	var requestDto model.RegisterDto
	err := json.NewDecoder(r.Body).Decode(&requestDto)
	if err != nil {
		h.logger.Infof("Error decoding /register request from %s: %v", r.RemoteAddr, err)
		httputil.RespondError(w, http.StatusBadRequest, "Request body malformed")
		return
	}
	defer r.Body.Close()

	if requestDto.Fingerprint == h.config.SecurityContext.CertificateHash {
		h.logger.Info("Received /register request from self, ignoring.")
		httputil.RespondError(w, http.StatusPreconditionFailed, "Self-discovered")
		return
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}

	device := model.NewDevice(requestDto, net.ParseIP(ip), requestDto.Port, requestDto.Protocol == model.ProtocolTypeHTTPS)
	h.registryService.RegisterDevice(device)

	h.logger.Infof("Received /register request from %s: Alias=%s, Fingerprint=%.8s...", r.RemoteAddr, requestDto.Alias, requestDto.Fingerprint)

	downloadCapable := h.sendService.GetSession() != nil

	responseDto := model.InfoDto{
		Alias:       h.config.Alias,
		Version:     config.ProtocolVersion,
		DeviceModel: h.config.DeviceModel,
		DeviceType:  h.config.DeviceType,
		Fingerprint: h.config.SecurityContext.CertificateHash,
		Download:    downloadCapable,
	}

	httputil.RespondJSON(w, http.StatusOK, responseDto)
}
