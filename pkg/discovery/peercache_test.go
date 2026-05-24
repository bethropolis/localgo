package discovery

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bethropolis/localgo/pkg/model"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestPeerCache_SaveAndGetPeers(t *testing.T) {
	tempDir := t.TempDir()
	cachePath := filepath.Join(tempDir, "peers.json")

	pc := &PeerCache{
		filePath: cachePath,
		peers:    make(map[string]*model.Device),
		logger:   zap.NewNop().Sugar(),
	}

	device := &model.Device{
		Alias:       "TestTarget",
		IP:          "192.168.1.200",
		Port:        53317,
		Protocol:    model.ProtocolTypeHTTPS,
		Fingerprint: "fingerprint123",
		LastSeen:    time.Now(),
	}

	pc.Save(device)

	// Load into a fresh cache to verify persistence
	pc2 := &PeerCache{
		filePath: cachePath,
		peers:    make(map[string]*model.Device),
		logger:   zap.NewNop().Sugar(),
	}
	pc2.load()

	peers := pc2.GetPeers()
	assert.Len(t, peers, 1)
	assert.Equal(t, "TestTarget", peers[0].Alias)
	assert.Equal(t, "192.168.1.200", peers[0].IP)
	assert.Equal(t, 53317, peers[0].Port)
	assert.Equal(t, model.ProtocolTypeHTTPS, peers[0].Protocol)
}

func TestPeerCache_UpdateExisting(t *testing.T) {
	tempDir := t.TempDir()
	cachePath := filepath.Join(tempDir, "peers.json")

	pc := &PeerCache{
		filePath: cachePath,
		peers:    make(map[string]*model.Device),
		logger:   zap.NewNop().Sugar(),
	}

	device := &model.Device{
		Alias:       "OldName",
		IP:          "192.168.1.200",
		Port:        53317,
		Protocol:    model.ProtocolTypeHTTPS,
		Fingerprint: "fp1",
		LastSeen:    time.Now(),
	}
	pc.Save(device)

	updated := &model.Device{
		Alias:       "NewName",
		IP:          "192.168.1.200",
		Port:        53317,
		Protocol:    model.ProtocolTypeHTTPS,
		Fingerprint: "fp1",
		LastSeen:    time.Now(),
	}
	pc.Save(updated)

	peers := pc.GetPeers()
	assert.Len(t, peers, 1)
	assert.Equal(t, "NewName", peers[0].Alias)
}

func TestPeerCache_LoadCorruptedFile(t *testing.T) {
	tempDir := t.TempDir()
	cachePath := filepath.Join(tempDir, "peers.json")

	err := os.WriteFile(cachePath, []byte("invalid-json"), 0644)
	assert.NoError(t, err)

	pc := &PeerCache{
		filePath: cachePath,
		peers:    make(map[string]*model.Device),
		logger:   zap.NewNop().Sugar(),
	}

	pc.load()
	assert.Empty(t, pc.GetPeers())
}

func TestPeerCache_LoadMissingFile(t *testing.T) {
	tempDir := t.TempDir()
	cachePath := filepath.Join(tempDir, "nonexistent.json")

	pc := &PeerCache{
		filePath: cachePath,
		peers:    make(map[string]*model.Device),
		logger:   zap.NewNop().Sugar(),
	}

	// Should not panic or error
	pc.load()
	assert.Empty(t, pc.GetPeers())
}

func TestPeerCache_ConcurrentSave(t *testing.T) {
	tempDir := t.TempDir()
	cachePath := filepath.Join(tempDir, "peers.json")

	pc := &PeerCache{
		filePath: cachePath,
		peers:    make(map[string]*model.Device),
		logger:   zap.NewNop().Sugar(),
	}

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			pc.Save(&model.Device{
				Alias:       "A",
				Fingerprint: "fp-a",
				LastSeen:    time.Now(),
			})
		}
		done <- struct{}{}
	}()
	go func() {
		for i := 0; i < 100; i++ {
			pc.Save(&model.Device{
				Alias:       "B",
				Fingerprint: "fp-b",
				LastSeen:    time.Now(),
			})
		}
		done <- struct{}{}
	}()

	<-done
	<-done

	peers := pc.GetPeers()
	assert.Len(t, peers, 2)
}
