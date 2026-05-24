package discovery

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/bethropolis/localgo/pkg/model"
	"go.uber.org/zap"
)

const (
	peerCacheMaxEntries = 50
	peerCacheFileName   = "peers.json"
)

type PeerCacheEntry struct {
	Alias       string    `json:"alias"`
	IP          string    `json:"ip"`
	Port        int       `json:"port"`
	Fingerprint string    `json:"fingerprint"`
	Protocol    string    `json:"protocol"`
	LastSeen    time.Time `json:"last_seen"`
}

type PeerCache struct {
	mu       sync.Mutex
	filePath string
	logger   *zap.SugaredLogger
}

func NewPeerCache(logger *zap.SugaredLogger) *PeerCache {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = "."
	}
	cacheDir = filepath.Join(cacheDir, "localgo")

	_ = os.MkdirAll(cacheDir, 0755)

	return &PeerCache{
		filePath: filepath.Join(cacheDir, peerCacheFileName),
		logger:   logger,
	}
}

func (pc *PeerCache) Load() []PeerCacheEntry {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	data, err := os.ReadFile(pc.filePath)
	if err != nil {
		if !os.IsNotExist(err) && pc.logger != nil {
			pc.logger.Warnw("Failed to read peer cache file", "path", pc.filePath, "error", err)
		}
		return nil
	}

	var entries []PeerCacheEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		if pc.logger != nil {
			pc.logger.Warnw("Failed to parse peer cache file", "path", pc.filePath, "error", err)
		}
		return nil
	}

	return entries
}

func (pc *PeerCache) Save(device *model.Device) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	entry := PeerCacheEntry{
		Alias:       device.Alias,
		IP:          device.IP,
		Port:        device.Port,
		Fingerprint: device.Fingerprint,
		Protocol:    string(device.Protocol),
		LastSeen:    time.Now(),
	}

	existing := pc.loadUnsafe()

	// Deduplicate by fingerprint
	found := false
	for i, e := range existing {
		if e.Fingerprint == entry.Fingerprint {
			existing[i] = entry
			found = true
			break
		}
	}
	if !found {
		existing = append(existing, entry)
	}

	// Sort by LastSeen descending, keep newest
	sort.Slice(existing, func(i, j int) bool {
		return existing[i].LastSeen.After(existing[j].LastSeen)
	})

	if len(existing) > peerCacheMaxEntries {
		existing = existing[:peerCacheMaxEntries]
	}

	data, err := json.Marshal(existing)
	if err != nil {
		if pc.logger != nil {
			pc.logger.Warnw("Failed to marshal peer cache", "error", err)
		}
		return
	}

	if err := os.WriteFile(pc.filePath, data, 0644); err != nil {
		if pc.logger != nil {
			pc.logger.Warnw("Failed to write peer cache file", "path", pc.filePath, "error", err)
		}
	}
}

func (pc *PeerCache) loadUnsafe() []PeerCacheEntry {
	data, err := os.ReadFile(pc.filePath)
	if err != nil {
		return nil
	}
	var entries []PeerCacheEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}
	return entries
}

func (pc *PeerCache) ToDevices() []*model.Device {
	entries := pc.Load()
	devices := make([]*model.Device, 0, len(entries))
	for _, e := range entries {
		devices = append(devices, &model.Device{
			IP:          e.IP,
			Port:        e.Port,
			Alias:       e.Alias,
			Protocol:    model.ProtocolType(e.Protocol),
			Fingerprint: e.Fingerprint,
			LastSeen:    e.LastSeen,
		})
	}
	return devices
}

// ProbeCached pings each cached peer with a GET /api/localsend/v2/info request
// and calls onFound for each peer that responds.
func ProbeCached(ctx context.Context, cache *PeerCache, onFound func(*model.Device), logger *zap.SugaredLogger) {
	if cache == nil {
		return
	}

	entries := cache.Load()
	if len(entries) == 0 {
		return
	}

	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	var wg sync.WaitGroup
	for _, entry := range entries {
		wg.Add(1)
		go func(e PeerCacheEntry) {
			defer wg.Done()

			scheme := "https"
			if e.Protocol == string(model.ProtocolTypeHTTP) {
				scheme = "http"
			}

			url := scheme + "://" + net.JoinHostPort(e.IP, strconv.Itoa(e.Port)) + "/api/localsend/v2/info"
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return
			}

			resp, err := client.Do(req)
			if err != nil {
				return
			}
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				device := &model.Device{
					IP:          e.IP,
					Port:        e.Port,
					Alias:       e.Alias,
					Protocol:    model.ProtocolType(e.Protocol),
					Fingerprint: e.Fingerprint,
					LastSeen:    time.Now(),
				}
				if logger != nil {
					logger.Debugf("Cached peer %s (%s:%d) responded", e.Alias, e.IP, e.Port)
				}
				onFound(device)
			}
		}(entry)
	}
	wg.Wait()
}

