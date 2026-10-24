// File: pkg/discovery/tlsclient_test.go
package discovery

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bethropolis/localgo/pkg/crypto"
	"github.com/bethropolis/localgo/pkg/logging"
)

func TestClientTLSConfigPresentsCertificate(t *testing.T) {
	log := logging.NewQuiet()
	clientCtx, err := crypto.GenerateSecurityContext("client", log)
	if err != nil {
		t.Fatalf("GenerateSecurityContext failed: %v", err)
	}
	serverCtx, err := crypto.GenerateSecurityContext("server", log)
	if err != nil {
		t.Fatalf("GenerateSecurityContext failed: %v", err)
	}
	serverCert, err := tls.X509KeyPair([]byte(serverCtx.Certificate), []byte(serverCtx.PrivateKey))
	if err != nil {
		t.Fatalf("X509KeyPair failed: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM([]byte(clientCtx.Certificate))

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool}
	srv.StartTLS()
	defer srv.Close()

	get := func() error {
		client := &http.Client{Transport: &http.Transport{TLSClientConfig: clientTLSConfig()}}
		resp, err := client.Get(srv.URL)
		if err == nil {
			resp.Body.Close()
		}
		return err
	}

	SetClientCertificate(nil)
	if err := get(); err == nil {
		t.Fatal("expected failure when no client certificate is configured")
	}

	fn, err := clientCtx.ClientCertificateFunc()
	if err != nil {
		t.Fatalf("ClientCertificateFunc failed: %v", err)
	}
	SetClientCertificate(fn)
	defer SetClientCertificate(nil)
	if err := get(); err != nil {
		t.Fatalf("request with client certificate failed: %v", err)
	}
}
