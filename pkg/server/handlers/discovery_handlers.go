// Package handlers contains HTTP handlers for the LocalGo server
package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/bet/localgo/pkg/config"
	"github.com/bet/localgo/pkg/httputil"
	"github.com/bet/localgo/pkg/model"
)

// DiscoveryHandler handles /info and /register requests.
type DiscoveryHandler struct {
	config *config.Config
}

// NewDiscoveryHandler creates a new DiscoveryHandler.
func NewDiscoveryHandler(cfg *config.Config) *DiscoveryHandler {
	return &DiscoveryHandler{config: cfg}
}

// InfoHandler handles GET /info requests (v1 & v2 are identical here).
func (h *DiscoveryHandler) InfoHandler(w http.ResponseWriter, r *http.Request) {
	if h.config.SecurityContext == nil {
		log.Println("Error: Security context not available for /info")
		httputil.RespondError(w, http.StatusInternalServerError, "Internal Server Error: Security context missing")
		return
	}

	senderFingerprint := r.URL.Query().Get("fingerprint")
	if senderFingerprint != "" && senderFingerprint == h.config.SecurityContext.CertificateHash {
		log.Println("Received /info request from self, ignoring.")
		httputil.RespondError(w, http.StatusPreconditionFailed, "Self-discovered")
		return
	}

	downloadCapable := false // TODO: update in Phase 3 (Web Share)

	dto := model.InfoDto{
		Alias:       h.config.Alias,
		Version:     config.ProtocolVersion,
		DeviceModel: h.config.DeviceModel,
		DeviceType:  h.config.DeviceType,
		Fingerprint: h.config.SecurityContext.CertificateHash,
		Download:    downloadCapable,
	}

	log.Printf("Responding to /info request from %s", r.RemoteAddr)
	httputil.RespondJSON(w, http.StatusOK, dto)
}

// RegisterHandler handles POST /register requests (v1 & v2 are identical here).
func (h *DiscoveryHandler) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if h.config.SecurityContext == nil {
		log.Println("Error: Security context not available for /register")
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
		log.Printf("Error decoding /register request from %s: %v", r.RemoteAddr, err)
		httputil.RespondError(w, http.StatusBadRequest, "Request body malformed")
		return
	}
	defer r.Body.Close()

	if requestDto.Fingerprint == h.config.SecurityContext.CertificateHash {
		log.Println("Received /register request from self, ignoring.")
		httputil.RespondError(w, http.StatusPreconditionFailed, "Self-discovered")
		return
	}

	// TODO: Implement device registration logic using DiscoveryService (Phase 1)
	log.Printf("Received /register request from %s: Alias=%s, Fingerprint=%.8s...", r.RemoteAddr, requestDto.Alias, requestDto.Fingerprint)

	downloadCapable := false // TODO: update in Phase 3

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
