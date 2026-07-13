package discovery

import (
	"context"
	"time"

	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/model"
)

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



	if err := svc.Start(ctx, multicastDto); err != nil {
		return nil, err
	}
	defer svc.Stop()

	scanCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return svc.Discover(scanCtx, multicastDto)
}
