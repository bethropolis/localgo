package discovery

import (
	"context"
	"time"

	"github.com/bethropolis/localgo/pkg/model"
)

func DiscoverDevices(ctx context.Context, cfg *ServiceConfig, alias string, port int, fingerprint string, deviceModel *string, httpsEnabled bool) ([]*model.Device, error) {
	if cfg == nil {
		cfg = DefaultServiceConfig()
	}

	protocol := model.ProtocolTypeHTTP
	if httpsEnabled {
		protocol = model.ProtocolTypeHTTPS
	}

	multicastDto := model.MulticastDto{
		Alias:       alias,
		Version:     "2.1",
		DeviceModel: deviceModel,
		DeviceType:  model.DeviceTypeDesktop,
		Fingerprint: fingerprint,
		Port:        port,
		Protocol:    protocol,
		Download:    false,
		Announce:    true,
	}

	multicast := NewMulticastDiscovery(cfg.MulticastConfig, multicastDto, nil)

	peerCache := NewPeerCache(nil)
	multicast.SetPeerCache(peerCache)

	svc := NewService(cfg, multicast, nil)
	svc.SetPeerCache(peerCache)

	if err := svc.Start(ctx, alias, port, fingerprint, model.DeviceTypeDesktop, deviceModel, httpsEnabled); err != nil {
		return nil, err
	}

	scanCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return svc.Discover(scanCtx, alias, port, fingerprint, model.DeviceTypeDesktop, deviceModel, httpsEnabled, false)
}
