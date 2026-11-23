package discovery

import (
	"context"
	"time"

	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/model"
)

// DiscoverDevices performs tiered discovery: cache probe, then multicast.
// The subnet scan fallback is omitted here — the caller (interactive send)
// already falls back to a subnet scan if no devices are found.
func DiscoverDevices(ctx context.Context, serviceCfg *ServiceConfig, appCfg *config.Config, httpsEnabled bool) ([]*model.Device, error) {
	if serviceCfg == nil {
		serviceCfg = DefaultServiceConfig()
	}

	multicastDto := appCfg.ToMulticastDto(false)

	multicast := NewMulticastDiscovery(serviceCfg.MulticastConfig, multicastDto, nil)

	peerCache := NewPeerCache(nil)
	multicast.SetPeerCache(peerCache)

	svc := NewService(serviceCfg, multicast, nil)
	svc.SetPeerCache(peerCache)

	// LocalSend clients answer an announcement with an HTTP /register request, not over UDP. Without a listener
	// for it they stay invisible. If it cannot start, discovery continues with the multicast listener only.
	if listener, err := StartRegisterListenerForConfig(appCfg, svc.AddDevice); err == nil {
		defer listener.Close()
		multicastDto.Port = listener.Port
		multicastDto.Protocol = listener.Protocol
	}

	if err := svc.Start(ctx, multicastDto); err != nil {
		return nil, err
	}
	defer svc.Stop()

	scanCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return svc.Discover(scanCtx, multicastDto)
}
