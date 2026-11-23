package discovery

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/httputil"
	"github.com/bethropolis/localgo/pkg/model"
)

const (
	registerRouteV1    = "POST /api/localsend/v1/register"
	registerRouteV2    = "POST /api/localsend/v2/register"
	maxRegisterBody    = 64 << 10
	registerHeaderWait = 5 * time.Second
)

// ErrCustomTLSUnsupported is returned when a custom TLS certificate is configured: its fingerprint is only
// computed by the server, so a listener started here could not announce a matching one.
var ErrCustomTLSUnsupported = errors.New("custom TLS certificates are not supported by the register listener")

// RegisterListenerConfig describes a RegisterListener.
type RegisterListenerConfig struct {
	// Info is the body returned to every client that registers.
	Info model.InfoDto
	// BindAddress is the address to listen on. Empty listens on all interfaces.
	BindAddress string
	// PreferredPort is tried first, so a firewall that only opens the LocalSend port keeps working. When it is zero
	// or taken (for example by a running server) a free port is used instead.
	PreferredPort int
	// TLSCert makes the listener serve HTTPS. Nil serves plain HTTP.
	TLSCert *tls.Certificate
	// OnDevice is called for every device that registers.
	OnDevice func(*model.Device)
}

// RegisterListener receives the HTTP /register requests that LocalSend clients (protocol v2.2) send in answer to a
// multicast announcement. Those clients never answer over UDP, so a one-off discovery without a listener like this
// one cannot see them.
type RegisterListener struct {
	// Port is the TCP port to put in the multicast announcement.
	Port int
	// Protocol is the scheme to put in the multicast announcement.
	Protocol model.ProtocolType

	server *http.Server
}

// StartRegisterListener listens on PreferredPort, or on a free TCP port if that is taken, and serves /register
// until Close is called.
func StartRegisterListener(cfg RegisterListenerConfig) (*RegisterListener, error) {
	var ln net.Listener
	var err error
	if cfg.PreferredPort > 0 {
		ln, err = net.Listen("tcp", net.JoinHostPort(cfg.BindAddress, fmt.Sprint(cfg.PreferredPort)))
	}
	if ln == nil {
		ln, err = net.Listen("tcp", net.JoinHostPort(cfg.BindAddress, "0"))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to listen for register requests: %w", err)
	}
	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		_ = ln.Close()
		return nil, fmt.Errorf("unexpected listener address type: %T", ln.Addr())
	}

	protocol := model.ProtocolTypeHTTP
	if cfg.TLSCert != nil {
		protocol = model.ProtocolTypeHTTPS
		ln = tls.NewListener(ln, &tls.Config{
			Certificates: []tls.Certificate{*cfg.TLSCert},
			MinVersion:   tls.VersionTLS12,
		})
	}

	mux := http.NewServeMux()
	mux.HandleFunc(registerRouteV1, cfg.handleRegister)
	mux.HandleFunc(registerRouteV2, cfg.handleRegister)

	l := &RegisterListener{
		Port:     tcpAddr.Port,
		Protocol: protocol,
		server:   &http.Server{Handler: mux, ReadHeaderTimeout: registerHeaderWait},
	}
	go func() { _ = l.server.Serve(ln) }()

	return l, nil
}

// StartRegisterListenerForConfig starts a RegisterListener that answers as the device described by appCfg.
func StartRegisterListenerForConfig(appCfg *config.Config, onDevice func(*model.Device)) (*RegisterListener, error) {
	if appCfg.CustomTLSCertPath != "" && appCfg.CustomTLSKeyPath != "" {
		return nil, ErrCustomTLSUnsupported
	}
	if appCfg.HttpsEnabled && appCfg.SecurityContext == nil {
		return nil, errors.New("security context not available")
	}

	info := appCfg.ToInfoDto()
	// A discovery run does not serve downloads.
	info.Download = false

	cfg := RegisterListenerConfig{
		Info:          info,
		BindAddress:   appCfg.BindAddress,
		PreferredPort: appCfg.Port,
		OnDevice:      onDevice,
	}

	if appCfg.HttpsEnabled {
		cert, err := tls.X509KeyPair([]byte(appCfg.SecurityContext.Certificate), []byte(appCfg.SecurityContext.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS key pair: %w", err)
		}
		cfg.TLSCert = &cert
	}

	return StartRegisterListener(cfg)
}

// Close stops the listener.
func (l *RegisterListener) Close() {
	_ = l.server.Close()
}

func (cfg RegisterListenerConfig) handleRegister(w http.ResponseWriter, r *http.Request) {
	var dto model.RegisterDto
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRegisterBody)).Decode(&dto); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Request body malformed")
		return
	}

	if dto.Fingerprint == cfg.Info.Fingerprint {
		httputil.RespondError(w, http.StatusPreconditionFailed, "Self-discovered")
		return
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}

	if cfg.OnDevice != nil {
		cfg.OnDevice(model.NewDevice(dto, net.ParseIP(ip), dto.Port, dto.Protocol == model.ProtocolTypeHTTPS))
	}

	httputil.RespondJSON(w, http.StatusOK, cfg.Info)
}
