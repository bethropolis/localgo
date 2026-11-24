package discovery

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/httputil"
	"github.com/bethropolis/localgo/pkg/logging"
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
	// PreferredPortErr reports why PreferredPort was not used, so callers can warn that a firewall which only
	// allows that port will drop the answers. Nil when the preferred port was used, or none was requested.
	PreferredPortErr error

	server *http.Server
}

// StartRegisterListener listens on PreferredPort, or on a free TCP port if that is taken, and serves /register
// until Close is called.
func StartRegisterListener(cfg RegisterListenerConfig) (*RegisterListener, error) {
	l := &RegisterListener{}

	var ln net.Listener
	var err error
	if cfg.PreferredPort > 0 {
		ln, err = net.Listen("tcp", net.JoinHostPort(cfg.BindAddress, strconv.Itoa(cfg.PreferredPort)))
		if err != nil {
			// Typically the port is held by a running server; a free port still works on the same network.
			l.PreferredPortErr = err
		}
	}
	if ln == nil {
		ln, err = net.Listen("tcp", net.JoinHostPort(cfg.BindAddress, "0"))
		if err != nil {
			return nil, fmt.Errorf("failed to listen for register requests: %w", err)
		}
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

	// The info we answer with has to describe this listener: when the preferred port was taken, peers can only
	// reach us on the fallback port, so that is the one they must be told about.
	cfg.Info.Port = tcpAddr.Port
	cfg.Info.Protocol = protocol

	mux := http.NewServeMux()
	mux.HandleFunc(registerRouteV1, cfg.handleRegister)
	mux.HandleFunc(registerRouteV2, cfg.handleRegister)

	l.Port = tcpAddr.Port
	l.Protocol = protocol
	l.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: registerHeaderWait,
		// Handshake failures are routine here: a peer that gives up mid-handshake
		// (a slow Wi-Fi link is enough) is nothing the user can act on.
		ErrorLog: log.New(handshakeLogWriter{}, "", 0),
	}
	go func() { _ = l.server.Serve(ln) }()

	return l, nil
}

// handshakeLogWriter sends net/http's TLS handshake complaints to the debug log.
type handshakeLogWriter struct{}

func (handshakeLogWriter) Write(p []byte) (int, error) {
	logging.Global().Debugf("%s", strings.TrimRight(string(p), "\n"))
	return len(p), nil
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
	defer r.Body.Close()

	var dto model.RegisterDto
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRegisterBody)).Decode(&dto); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Request body malformed")
		return
	}

	if dto.Fingerprint == cfg.Info.Fingerprint {
		httputil.RespondError(w, http.StatusPreconditionFailed, "Self-discovered")
		return
	}

	if cfg.OnDevice != nil {
		cfg.OnDevice(model.NewDevice(dto, httputil.RemoteIP(r), dto.Port, dto.Protocol == model.ProtocolTypeHTTPS))
	}

	httputil.RespondJSON(w, http.StatusOK, cfg.Info)
}
