# Embedding LocalGo (Library Guide)

LocalGo is structured as a collection of reusable Go packages. You can import `github.com/bethropolis/localgo/pkg/...` to build your own custom LocalSend applications.

## 📦 Key Packages

| Package | Import Path | Purpose |
|---------|-------------|---------|
| `config` | `.../pkg/config` | Configuration structs and defaults |
| `server` | `.../pkg/server` | HTTP/S listener and request handlers |
| `discovery` | `.../pkg/discovery` | Multicast and HTTP discovery engines |
| `send` | `.../pkg/send` | Client-side sending logic |
| `model` | `.../pkg/model` | Shared DTOs (`Device`, `File`, etc.) |

## 🛠 Example: Custom Receiver

This minimal example shows how to start a receiver from your own code.

```go
package main

import (
	"context"
	"log"
	
	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/server"
	"github.com/bethropolis/localgo/pkg/model"
)

func main() {
	// 1. Create Config
	cfg := &config.Config{
		Alias:          "MyCustomApp",
		Port:           53317,
		HttpsEnabled:   true,
		DownloadDir:    "./received_files",
		DeviceType:     model.DeviceTypeMobile, // Identify as mobile
		MulticastGroup: "224.0.0.167",
	}
	
	// Note: You must handle SecurityContext generation manually if not using config.LoadConfig()
	// See pkg/config/config.go for reference.

	// 2. Start Server
	srv := server.NewServer(cfg)
	log.Printf("Starting server on %d...", cfg.Port)
	
	if err := srv.Start(context.Background()); err != nil {
		log.Fatal(err)
	}
}
```

## 🛠 Example: Custom Discovery

Run your own discovery logic to build a device picker UI.

```go
import (
	"context"
	"fmt"
	"time"
	"github.com/bethropolis/localgo/pkg/discovery"
	"github.com/bethropolis/localgo/pkg/model"
)

func DiscoverDevices() {
	// Setup
	cfg := discovery.DefaultServiceConfig()
	dto := model.MulticastDto{
		Alias: "Scanner",
		Port:  53317,
		// ... populate other fields
	}
	
	multicast := discovery.NewMulticastDiscovery(cfg.MulticastConfig, dto)
	service := discovery.NewService(cfg, multicast)

	// Callback
	service.AddDeviceHandler(func(device *model.Device) {
		fmt.Printf("New Device: %s (%s)\n", device.Alias, device.IP)
	})

	// Run for 5 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	service.Discover(ctx, "Scanner", 53317, "fingerprint...", "desktop", nil)
}
```

## 🏗 Best Practices

1.  **Context Management**: Always pass `context.Context` to control lifecycles. LocalGo relies heavily on contexts for cancellation.
2.  **Error Handling**: Check errors from `Start()` and `SendFile()`.
3.  **Concurrency**: The `Server` and `Discovery` services are designed to run in their own goroutines.
