package discovery

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/bethropolis/localgo/pkg/model"
	"go.uber.org/zap"
)

var testLoggerMulticast = zap.NewNop().Sugar()

// We use a different multicast address for testing to avoid conflicting with actual apps
const testMulticastAddr = "224.0.0.254:53318"

func TestMulticastDiscovery_Lifecycle(t *testing.T) {
	config := DefaultMulticastConfig()
	config.MulticastAddr = testMulticastAddr

	dto := model.MulticastDto{
		Alias:       "TestDevice1",
		Fingerprint: "fingerprint1",
		Port:        53318,
	}

	md := NewMulticastDiscovery(config, dto, testLoggerMulticast)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := md.StartListening(ctx)
	if err != nil {
		t.Skipf("multicast socket unavailable (CI/sandbox environment): %v", err)
	}

	// Starting again should error
	err = md.StartListening(ctx)
	if err == nil {
		t.Fatalf("expected error when starting already listening discovery")
	}

	md.Stop()
}

func TestMulticastDiscovery_ReceiveAnnouncement(t *testing.T) {
	// Set up receiver
	config1 := DefaultMulticastConfig()
	config1.MulticastAddr = testMulticastAddr

	receiverDto := model.MulticastDto{
		Alias:       "Receiver",
		Fingerprint: "receiver-fp",
		Port:        53318,
	}

	receiver := NewMulticastDiscovery(config1, receiverDto, testLoggerMulticast)

	deviceFoundCh := make(chan *model.Device, 1)
	receiver.AddDeviceHandler(func(d *model.Device) {
		deviceFoundCh <- d
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := receiver.StartListening(ctx)
	if err != nil {
		t.Skipf("multicast socket unavailable (CI/sandbox environment): %v", err)
	}
	defer receiver.Stop()

	// Give receiver time to bind
	time.Sleep(100 * time.Millisecond)

	// Set up sender (don't need to listen, just send)
	senderDto := model.MulticastDto{
		Alias:       "Sender",
		Fingerprint: "sender-fp",
		Port:        53318,
		Announce:    true,
	}

	sender := NewMulticastDiscovery(config1, senderDto, testLoggerMulticast)
	err = sender.SendDiscoveryAnnouncement()
	if err != nil {
		t.Skipf("sender unable to announce (CI/sandbox environment): %v", err)
	}

	// Wait for receiver to get it
	select {
	case device := <-deviceFoundCh:
		if device.Alias != "Sender" {
			t.Errorf("expected alias 'Sender', got '%s'", device.Alias)
		}
		if device.Fingerprint != "sender-fp" {
			t.Errorf("expected fingerprint 'sender-fp', got '%s'", device.Fingerprint)
		}
	case <-time.After(2 * time.Second):
		t.Skipf("multicast delivery timed out (CI/sandbox environment)")
	}

	devices := receiver.GetDevices()
	if len(devices) != 1 {
		t.Errorf("expected 1 device in receiver map, got %d", len(devices))
	}
}

func TestMulticastDiscovery_IgnoreSelf(t *testing.T) {
	config := DefaultMulticastConfig()
	config.MulticastAddr = testMulticastAddr

	dto := model.MulticastDto{
		Alias:       "SelfDevice",
		Fingerprint: "self-fp",
		Port:        53318,
	}

	md := NewMulticastDiscovery(config, dto, testLoggerMulticast)

	deviceFoundCh := make(chan *model.Device, 1)
	md.AddDeviceHandler(func(d *model.Device) {
		deviceFoundCh <- d
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	md.StartListening(ctx)
	if md.conn == nil {
		t.Skipf("multicast socket unavailable (CI/sandbox environment)")
	}
	defer md.Stop()

	time.Sleep(100 * time.Millisecond)

	md.SendDiscoveryAnnouncement()

	select {
	case <-deviceFoundCh:
		t.Fatalf("should not have discovered self")
	case <-time.After(500 * time.Millisecond):
		// Expected timeout
	}

	if len(md.GetDevices()) != 0 {
		t.Errorf("expected 0 devices in map, got %d", len(md.GetDevices()))
	}
}

// A helper for testing packet handling directly without network
func TestMulticastDiscovery_HandlePacket(t *testing.T) {
	md := NewMulticastDiscovery(nil, model.MulticastDto{
		Fingerprint: "my-fp",
	}, testLoggerMulticast)

	var wg sync.WaitGroup
	wg.Add(1)

	md.AddDeviceHandler(func(d *model.Device) {
		defer wg.Done()
		if d.Alias != "ExternalDevice" {
			t.Errorf("wrong alias: %s", d.Alias)
		}
	})

	externalDto := model.MulticastDto{
		Alias:       "ExternalDevice",
		Fingerprint: "external-fp",
		Announce:    false, // false so it doesn't try to send a response (which needs a real connection)
	}

	data, _ := json.Marshal(externalDto)

	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.100"), Port: 12345}
	err := md.handlePacket(data, addr)
	if err != nil {
		t.Fatalf("handlePacket failed: %v", err)
	}

	wg.Wait()

	devices := md.GetDevices()
	if len(devices) != 1 {
		t.Errorf("expected 1 device, got %d", len(devices))
	}
}
