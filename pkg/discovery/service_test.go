
package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/bet/localgo/pkg/model"
	"github.com/stretchr/testify/assert"
)

// MockMulticastDiscovery is a mock implementation of the MulticastDiscovery for testing.

type MockMulticastDiscovery struct {
	startListeningCalled    bool
	sendAnnouncementCalled bool
	dto                     model.MulticastDto
	stopped                 bool
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
	service := NewService(cfg, multicast)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := service.Start(ctx, "test-alias", 12345, "test-fingerprint", model.DeviceTypeDesktop, nil)

	assert.NoError(t, err)
	assert.True(t, multicast.startListeningCalled)
	assert.True(t, multicast.sendAnnouncementCalled)
}
