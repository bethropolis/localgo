package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/bethropolis/localgo/pkg/model"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

var testLoggerService = zap.NewNop().Sugar()

// MockMulticastDiscovery is a mock implementation of the MulticastDiscovery for testing.

type MockMulticastDiscovery struct {
	startListeningCalled   bool
	sendAnnouncementCalled bool
	dto                    model.MulticastDto
	stopped                bool
}

func (m *MockMulticastDiscovery) AddDeviceHandler(handler func(*model.Device)) {}

func (m *MockMulticastDiscovery) StartListening(ctx context.Context) error {
	m.startListeningCalled = true
	return nil
}

func (m *MockMulticastDiscovery) SendDiscoveryAnnouncement() error {
	m.sendAnnouncementCalled = true
	return nil
}

func (m *MockMulticastDiscovery) Stop() {
	m.stopped = true
}

func (m *MockMulticastDiscovery) SetDto(dto model.MulticastDto) {
	m.dto = dto
}

func TestService_Start(t *testing.T) {
	cfg := DefaultServiceConfig()
	multicast := &MockMulticastDiscovery{}
	service := NewService(cfg, multicast, testLoggerService)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	dto := model.MulticastDto{
		Alias:       "test-alias",
		Version:     "2.1",
		Fingerprint: "test-fingerprint",
		Port:        12345,
		DeviceType:  model.DeviceTypeDesktop,
		Protocol:    model.ProtocolTypeHTTP,
		Announce:    true,
	}
	err := service.Start(ctx, dto)

	assert.NoError(t, err)
	assert.True(t, multicast.startListeningCalled)
	assert.True(t, multicast.sendAnnouncementCalled)
}
