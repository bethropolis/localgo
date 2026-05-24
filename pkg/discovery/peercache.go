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
	"go.uber.org/zap"
)

// PeerCache persists discovered peers to disk and provides thread-safe access.
type PeerCache struct {
	mu       sync.RWMutex
	filePath string
	peers    map[string]*model.Device
	logger   *zap.SugaredLogger
}

// NewPeerCache creates or loads a peer cache from the XDG cache directory.
func NewPeerCache(logger *zap.SugaredLogger) *PeerCache {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	path := filepath.Join(cacheDir, "localgo", "peers.json")

	pc := &PeerCache{
		filePath: path,
		peers:    make(map[string]*model.Device),
		logger:   logger,
	}
	pc.load()
	return pc
}

// Save adds or updates a peer and persists atomically.
func (pc *PeerCache) Save(device *model.Device) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.peers[device.Fingerprint] = device
	if err := pc.persist(); err != nil {
		pc.logger.Warnf("Failed to persist peer cache: %v", err)
	}
}

// GetPeers returns a snapshot of all cached peers.
func (pc *PeerCache) GetPeers() []*model.Device {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	list := make([]*model.Device, 0, len(pc.peers))
	for _, d := range pc.peers {
		list = append(list, d)
	}
	return list
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

	staleThreshold := 30 * 24 * time.Hour
	now := time.Now()
	evictedCount := 0

	for _, d := range list {
		// Evict peers not seen in the last 30 days
		if !d.LastSeen.IsZero() && now.Sub(d.LastSeen) > staleThreshold {
			evictedCount++
			continue
		}
		pc.peers[d.Fingerprint] = d
	}

	if evictedCount > 0 {
		pc.logger.Debugf("Evicted %d stale peer(s) from the local cache (older than 30 days)", evictedCount)
		// Persist the cleaned cache back to disk in the background
		go func() {
			pc.mu.Lock()
			defer pc.mu.Unlock()
			_ = pc.persist()
		}()
	}
}

// persist writes the in-memory map to disk atomically via a temp file + rename.
// Must be called with mu held.
func (pc *PeerCache) persist() error {
	list := make([]*model.Device, 0, len(pc.peers))
	for _, d := range pc.peers {
		list = append(list, d)
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
func ProbeCached(ctx context.Context, cache *PeerCache, onFound func(*model.Device), logger *zap.SugaredLogger) {
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
				d.LastSeen = now
				if logger != nil {
					logger.Debugf("Cached peer %s (%s:%d) responded", d.Alias, d.IP, d.Port)
				}
				onFound(d)
			}
		}(device)
	}
	wg.Wait()
}
