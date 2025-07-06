
package discovery

import (
	"context"

	"github.com/bet/localgo/pkg/model"
)

// MulticastDiscoverer is an interface for multicast discovery.
type MulticastDiscoverer interface {
	AddDeviceHandler(handler func(*model.Device))
	StartListening(ctx context.Context) error
	SendDiscoveryAnnouncement() error
	Stop()
	SetDto(dto model.MulticastDto)
}
