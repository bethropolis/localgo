package discovery

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/logging"
)

// MaxCachedPeers is the maximum number of peers to keep in cache.
const MaxCachedPeers = 50

// StaleThreshold is how long without contact before a peer is evicted.
const StaleThreshold = 14 * 24 * time.Hour

// PeerCache persists discovered peers to disk and provides thread-safe access.
type PeerCache struct {
	mu       sync.RWMutex
	filePath string
	peers    map[string]*model.Device
	order    []string // LRU order (most recent at end)
	logger   *logging.Logger
}

// NewPeerCache creates or loads a peer cache from the XDG cache directory.
func NewPeerCache(logger *logging.Logger) *PeerCache {
	if logger == nil {
		logger = logging.NewQuiet()
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	path := filepath.Join(cacheDir, "localgo", "peers.json")

	pc := &PeerCache{
		filePath: path,
		peers:    make(map[string]*model.Device),
		order:    make([]string, 0, MaxCachedPeers),
		logger:   logger,
	}
	pc.load()
	return pc
}

// Save adds or updates a peer, updates LRU order, evicts stale/over-limit entries, and persists.
func (pc *PeerCache) Save(device *model.Device) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	now := time.Now()
	device.SetLastSeen(now)

	if _, exists := pc.peers[device.Fingerprint]; !exists {
		pc.order = append(pc.order, device.Fingerprint)
	}
	pc.peers[device.Fingerprint] = device

	// Evict stale peers first
	staleCutoff := now.Add(-StaleThreshold)
	var fresh []string
	for _, fp := range pc.order {
		d, ok := pc.peers[fp]
		if !ok || (!d.GetLastSeen().IsZero() && d.GetLastSeen().Before(staleCutoff)) {
			delete(pc.peers, fp)
			continue
		}
		fresh = append(fresh, fp)
	}
	pc.order = fresh

	// LRU evict oldest entries if over cap
	for len(pc.order) > MaxCachedPeers {
		fp := pc.order[0]
		pc.order = pc.order[1:]
		delete(pc.peers, fp)
	}

	if err := pc.persist(); err != nil {
		pc.logger.Warnf("Failed to persist peer cache: %v", err)
	}
}

// touchLRU moves the given fingerprint to the end (most recently used).
func (pc *PeerCache) touchLRU(fingerprint string) {
	for i, fp := range pc.order {
		if fp == fingerprint {
			pc.order = append(pc.order[:i], pc.order[i+1:]...)
			pc.order = append(pc.order, fp)
			break
		}
	}
}

// GetPeers returns a snapshot of all cached peers (most recently seen first).
func (pc *PeerCache) GetPeers() []*model.Device {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	list := make([]*model.Device, 0, len(pc.order))
	for i := len(pc.order) - 1; i >= 0; i-- {
		if d, ok := pc.peers[pc.order[i]]; ok {
			list = append(list, d)
		}
	}
	return list
}

// GetByFingerprint returns a cached peer by fingerprint.
func (pc *PeerCache) GetByFingerprint(fp string) *model.Device {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.peers[fp]
}

// load reads peers.json into the in-memory map. Must be called with mu held.
func (pc *PeerCache) load() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	data, err := os.ReadFile(pc.filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			pc.logger.Warnf("Failed to read peer cache file: %v", err)
		}
		return
	}

	var list []*model.Device
	if err := json.Unmarshal(data, &list); err != nil {
		pc.logger.Warnf("Failed to unmarshal peer cache: %v", err)
		return
	}

	now := time.Now()
	staleCutoff := now.Add(-StaleThreshold)
	evictedCount := 0

	for _, d := range list {
		if !d.GetLastSeen().IsZero() && d.GetLastSeen().Before(staleCutoff) {
			evictedCount++
			continue
		}
		if len(pc.order) >= MaxCachedPeers {
			evictedCount++
			continue
		}
		pc.order = append(pc.order, d.Fingerprint)
		pc.peers[d.Fingerprint] = d
	}

	if evictedCount > 0 {
		pc.logger.Debugf("Evicted %d stale/over-limit peer(s) from the local cache (older than 14 days or >%d entries)", evictedCount, MaxCachedPeers)
		go func() {
			pc.mu.Lock()
			defer pc.mu.Unlock()
			_ = pc.persist()
		}()
	}
}

// cachedPeer represents a peer with its LRU ordering for serialization.
type cachedPeer struct {
	Device *model.Device `json:"device"`
}

// persist writes the in-memory cache to disk atomically via a temp file + rename.
// Must be called with mu held.
func (pc *PeerCache) persist() error {
	list := make([]*model.Device, 0, len(pc.order))
	for _, fp := range pc.order {
		if d, ok := pc.peers[fp]; ok {
			list = append(list, d)
		}
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(pc.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	tempFile, err := os.CreateTemp(dir, "peers-*.tmp")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	removeTemp := true
	defer func() {
		tempFile.Close()
		if removeTemp {
			os.Remove(tempPath)
		}
	}()

	if _, err := tempFile.Write(data); err != nil {
		return err
	}
	if err := tempFile.Sync(); err != nil {
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}

	if err := os.Rename(tempPath, pc.filePath); err != nil {
		return err
	}
	removeTemp = false
	return nil
}

// ProbeCached pings each cached peer with GET /api/localsend/v2/info
// and calls onFound for every peer that responds.
func ProbeCached(ctx context.Context, cache *PeerCache, onFound func(*model.Device), logger *logging.Logger) {
	if cache == nil {
		return
	}

	peers := cache.GetPeers()
	if len(peers) == 0 {
		return
	}

	client := &http.Client{
		Timeout: 2 * time.Second,
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client.Transport = tr
	defer tr.CloseIdleConnections()

	var wg sync.WaitGroup
	for _, device := range peers {
		wg.Add(1)
		go func(d *model.Device) {
			defer wg.Done()

			scheme := "http"
			if d.Protocol == model.ProtocolTypeHTTPS {
				scheme = "https"
			}

			url := scheme + "://" + net.JoinHostPort(d.IP, strconv.Itoa(d.Port)) + "/api/localsend/v2/info"
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
				now := time.Now()
			d.SetLastSeen(now)
			cache.Save(d) // persist updated LastSeen to disk
			if logger != nil {
				logger.Debugf("Cached peer %s (%s:%d) responded", d.Alias, d.IP, d.Port)
			}
			onFound(d)
			}
		}(device)
	}
	wg.Wait()
}
