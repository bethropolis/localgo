// File: pkg/discovery/tlsclient.go
package discovery

import (
	"crypto/tls"
	"sync/atomic"
)

// clientCertFunc is the callback used to present a TLS client certificate
// when probing other devices over HTTPS.
var clientCertFunc atomic.Pointer[func(*tls.CertificateRequestInfo) (*tls.Certificate, error)]

// SetClientCertificate sets the callback used to present our certificate as a
// TLS client certificate when contacting other devices (see
// crypto.StoredSecurityContext.ClientCertificateFunc). LocalSend apps speaking
// protocol v2.2 request a client certificate and drop the handshake without
// one, which makes them invisible to HTTP-based discovery. Passing nil
// disables it.
func SetClientCertificate(fn func(*tls.CertificateRequestInfo) (*tls.Certificate, error)) {
	if fn == nil {
		clientCertFunc.Store(nil)
		return
	}
	clientCertFunc.Store(&fn)
}

// clientTLSConfig returns the TLS config for outgoing discovery requests.
// Peers use self-signed certificates, so they are not verified here.
func clientTLSConfig() *tls.Config {
	cfg := &tls.Config{InsecureSkipVerify: true}
	if fn := clientCertFunc.Load(); fn != nil {
		cfg.GetClientCertificate = *fn
	}
	return cfg
}
