package discovery

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/crypto"
	"github.com/bethropolis/localgo/pkg/logging"
	"github.com/bethropolis/localgo/pkg/model"
)

const testSelfFingerprint = "self-fingerprint"

// collector gathers the devices a listener reports.
type collector struct {
	mu      sync.Mutex
	devices []*model.Device
}

func (c *collector) add(d *model.Device) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.devices = append(c.devices, d)
}

func (c *collector) all() []*model.Device {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]*model.Device(nil), c.devices...)
}

func registerBody(t *testing.T, fingerprint string) []byte {
	t.Helper()
	body, err := json.Marshal(model.RegisterDto{
		Alias:       "Phone",
		Version:     "2.1",
		DeviceType:  model.DeviceTypeMobile,
		Fingerprint: fingerprint,
		Port:        53317,
		Protocol:    model.ProtocolTypeHTTPS,
	})
	require.NoError(t, err)
	return body
}

func newTestListener(t *testing.T, cert *tls.Certificate) (*RegisterListener, *collector) {
	t.Helper()
	got := &collector{}
	l, err := StartRegisterListener(RegisterListenerConfig{
		Info:        model.InfoDto{Alias: "Me", Version: "2.1", Fingerprint: testSelfFingerprint},
		BindAddress: "127.0.0.1",
		TLSCert:     cert,
		OnDevice:    got.add,
	})
	require.NoError(t, err)
	t.Cleanup(l.Close)
	return l, got
}

// doRequest sends a request to the listener and returns the status code and body.
func doRequest(t *testing.T, method, url string, body []byte) (int, []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// The listener uses a self-signed certificate, like every LocalSend device.
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, respBody
}

func TestRegisterListener_ReportsRegisteringDevice(t *testing.T) {
	for _, path := range []string{"/api/localsend/v1/register", "/api/localsend/v2/register"} {
		t.Run(path, func(t *testing.T) {
			l, got := newTestListener(t, nil)
			assert.Equal(t, model.ProtocolTypeHTTP, l.Protocol)

			status, body := doRequest(t, http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d%s", l.Port, path), registerBody(t, "peer-fingerprint"))
			require.Equal(t, http.StatusOK, status)

			var info model.InfoDto
			require.NoError(t, json.Unmarshal(body, &info))
			assert.Equal(t, "Me", info.Alias)
			assert.Equal(t, testSelfFingerprint, info.Fingerprint)
			assert.Equal(t, l.Port, info.Port, "the answer must describe the port peers actually reached")
			assert.Equal(t, l.Protocol, info.Protocol)

			devices := got.all()
			require.Len(t, devices, 1)
			assert.Equal(t, "Phone", devices[0].Alias)
			assert.Equal(t, "peer-fingerprint", devices[0].Fingerprint)
			assert.Equal(t, "127.0.0.1", devices[0].IP)
			assert.Equal(t, 53317, devices[0].Port)
			assert.Equal(t, model.ProtocolTypeHTTPS, devices[0].Protocol)
		})
	}
}

func TestRegisterListener_ServesHTTPSWhenGivenACertificate(t *testing.T) {
	secCtx, err := crypto.GenerateSecurityContext("test", nil)
	require.NoError(t, err)
	cert, err := tls.X509KeyPair([]byte(secCtx.Certificate), []byte(secCtx.PrivateKey))
	require.NoError(t, err)

	l, got := newTestListener(t, &cert)
	assert.Equal(t, model.ProtocolTypeHTTPS, l.Protocol)

	status, _ := doRequest(t, http.MethodPost, fmt.Sprintf("https://127.0.0.1:%d/api/localsend/v2/register", l.Port), registerBody(t, "peer-fingerprint"))
	require.Equal(t, http.StatusOK, status)
	assert.Len(t, got.all(), 1)
}

func TestRegisterListener_RejectsBadRequests(t *testing.T) {
	l, got := newTestListener(t, nil)
	url := fmt.Sprintf("http://127.0.0.1:%d/api/localsend/v2/register", l.Port)

	t.Run("own fingerprint", func(t *testing.T) {
		status, _ := doRequest(t, http.MethodPost, url, registerBody(t, testSelfFingerprint))
		assert.Equal(t, http.StatusPreconditionFailed, status)
	})

	t.Run("malformed body", func(t *testing.T) {
		status, _ := doRequest(t, http.MethodPost, url, []byte("{not json"))
		assert.Equal(t, http.StatusBadRequest, status)
	})

	t.Run("oversized body", func(t *testing.T) {
		status, _ := doRequest(t, http.MethodPost, url, bytes.Repeat([]byte(" "), maxRegisterBody+1))
		assert.Equal(t, http.StatusBadRequest, status)
	})

	t.Run("wrong method", func(t *testing.T) {
		status, _ := doRequest(t, http.MethodGet, url, nil)
		assert.Equal(t, http.StatusMethodNotAllowed, status)
	})

	assert.Empty(t, got.all(), "no rejected request may reach OnDevice")
}

func TestRegisterListener_PreferredPort(t *testing.T) {
	// Reserve a free port, release it and ask for it: it must be used.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	free := probe.Addr().(*net.TCPAddr).Port
	require.NoError(t, probe.Close())

	l, err := StartRegisterListener(RegisterListenerConfig{BindAddress: "127.0.0.1", PreferredPort: free})
	require.NoError(t, err)
	defer l.Close()
	assert.Equal(t, free, l.Port)
	assert.NoError(t, l.PreferredPortErr)

	// A second listener asking for the same port must fall back, and say why.
	other, err := StartRegisterListener(RegisterListenerConfig{BindAddress: "127.0.0.1", PreferredPort: free})
	require.NoError(t, err)
	defer other.Close()
	assert.NotEqual(t, free, other.Port)
	assert.NotZero(t, other.Port)
	assert.Error(t, other.PreferredPortErr, "callers need to know a firewall may block the fallback port")
}

func TestStartRegisterListenerForConfig_RefusesCustomTLS(t *testing.T) {
	cfg := &config.Config{CustomTLSCertPath: "cert.pem", CustomTLSKeyPath: "key.pem"}

	l, err := StartRegisterListenerForConfig(cfg, nil)

	assert.ErrorIs(t, err, ErrCustomTLSUnsupported)
	assert.Nil(t, l)
}

func TestStartRegisterListenerForConfig_DoesNotAdvertiseDownload(t *testing.T) {
	cfg := &config.Config{Alias: "Me", BindAddress: "127.0.0.1", RandomFingerprint: testSelfFingerprint}

	l, err := StartRegisterListenerForConfig(cfg, nil)
	require.NoError(t, err)
	defer l.Close()
	assert.Equal(t, model.ProtocolTypeHTTP, l.Protocol)

	status, body := doRequest(t, http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/api/localsend/v2/register", l.Port), registerBody(t, "peer-fingerprint"))
	require.Equal(t, http.StatusOK, status)

	var info model.InfoDto
	require.NoError(t, json.Unmarshal(body, &info))
	assert.Equal(t, "Me", info.Alias)
	assert.False(t, info.Download, "a discovery run does not serve downloads")
}

func TestService_AddDevice(t *testing.T) {
	cache := &PeerCache{
		filePath: filepath.Join(t.TempDir(), "peers.json"),
		peers:    make(map[string]*model.Device),
		logger:   logging.NewQuiet(),
	}
	svc := NewService(DefaultServiceConfig(), &MockMulticastDiscovery{}, testLoggerService)
	svc.SetPeerCache(cache)
	device := &model.Device{Alias: "Phone", IP: "192.168.1.50", Fingerprint: "peer-fingerprint", LastSeen: time.Now()}

	svc.AddDevice(device)

	assert.Equal(t, device, svc.GetDevice("peer-fingerprint"))
	assert.NotNil(t, cache.GetByFingerprint("peer-fingerprint"), "the device must also reach the peer cache")
}

func TestService_AddDevice_IgnoresUnkeyableDevices(t *testing.T) {
	cache := &PeerCache{
		filePath: filepath.Join(t.TempDir(), "peers.json"),
		peers:    make(map[string]*model.Device),
		logger:   logging.NewQuiet(),
	}
	svc := NewService(DefaultServiceConfig(), &MockMulticastDiscovery{}, testLoggerService)
	svc.SetPeerCache(cache)

	svc.AddDevice(nil)
	svc.AddDevice(&model.Device{Alias: "NoFingerprint", IP: "192.168.1.51", LastSeen: time.Now()})

	assert.Empty(t, svc.GetDevices(), "a device without a fingerprint cannot be keyed")
	assert.Empty(t, cache.GetPeers(), "and must not poison the peer cache")
}
