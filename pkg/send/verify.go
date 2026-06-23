package send

import (
	"fmt"

	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/discovery"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/charmbracelet/huh"
)

func verifyDeviceFingerprint(peerCache *discovery.PeerCache, targetDevice *model.Device) error {
	if targetDevice == nil || targetDevice.Fingerprint == "" {
		return nil
	}

	cachedPeers := peerCache.GetPeers()
	for _, cached := range cachedPeers {
		if cached.Alias == targetDevice.Alias && cached.Fingerprint != targetDevice.Fingerprint {
			cli.PrintWarning("The security fingerprint for '%s' has changed!", targetDevice.Alias)

			var trust bool
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title("Trust this new device fingerprint and update cache?").
						Value(&trust).
						Affirmative("Trust & Save").
						Negative("Abort"),
				),
			).WithTheme(huh.ThemeCharm())

			if err := form.Run(); err != nil || !trust {
				return fmt.Errorf("security verification failed: untrusted certificate hash change")
			}

			peerCache.Save(targetDevice)
			break
		}
	}
	return nil
}
