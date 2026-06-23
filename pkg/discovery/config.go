package discovery

import "time"

// MulticastConfig contains settings for multicast discovery
type MulticastConfig struct {
	MulticastAddr   string
	Port            int
	InterfaceName   string
	AnnounceTimeout time.Duration
	ListenTimeout   time.Duration
}

// DefaultMulticastConfig returns a default configuration
func DefaultMulticastConfig() *MulticastConfig {
	return &MulticastConfig{
		MulticastAddr:   "224.0.0.167:53317",
		Port:            53317,
		AnnounceTimeout: 2 * time.Second,
		ListenTimeout:   5 * time.Second,
	}
}
