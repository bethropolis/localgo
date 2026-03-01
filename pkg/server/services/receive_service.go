package services

import (
	"fmt"
	"sync"
	"time"

	"github.com/bethropolis/localgo/pkg/model"
	"github.com/google/uuid"
)

// ActiveReceiveSession represents an active file receiving session.
type ActiveReceiveSession struct {
	SessionID string
	Sender    model.DeviceInfo
	Files     map[string]ActiveFile
	CreatedAt time.Time
}

// ActiveFile represents a file in an active session.
type ActiveFile struct {
	Dto   model.FileDto
	Token string
}

// ReceiveService manages file receiving sessions.
type ReceiveService struct {
	currentSession *ActiveReceiveSession
	sessionMutex   sync.RWMutex
}

// NewReceiveService creates a new ReceiveService.
func NewReceiveService() *ReceiveService {
	s := &ReceiveService{}
	go s.cleanupLoop()
	return s
}

// cleanupLoop periodically checks and expires stale sessions
func (s *ReceiveService) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		s.sessionMutex.Lock()
		if s.currentSession != nil && time.Since(s.currentSession.CreatedAt) > 10*time.Minute {
			s.currentSession = nil
		}
		s.sessionMutex.Unlock()
	}
}

// CreateSession creates a new receive session.
func (s *ReceiveService) CreateSession(sender model.DeviceInfo, files map[string]model.FileDto) (*ActiveReceiveSession, error) {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	if s.currentSession != nil {
		return nil, fmt.Errorf("session already active")
	}

	sessionId := uuid.NewString()
	sessionFiles := make(map[string]ActiveFile)
	for fileId, fileDto := range files {
		token := uuid.NewString()
		sessionFiles[fileId] = ActiveFile{
			Dto:   fileDto,
			Token: token,
		}
	}

	s.currentSession = &ActiveReceiveSession{
		SessionID: sessionId,
		Sender:    sender,
		Files:     sessionFiles,
		CreatedAt: time.Now(),
	}

	return s.currentSession, nil
}

// GetSession returns the current active session.
// Returns a shallow copy of the session to prevent map data races during read.
func (s *ReceiveService) GetSession() *ActiveReceiveSession {
	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()
	if s.currentSession == nil {
		return nil
	}

	// Return a copy to avoid data races on the Files map
	copySession := &ActiveReceiveSession{
		SessionID: s.currentSession.SessionID,
		Sender:    s.currentSession.Sender,
		Files:     make(map[string]ActiveFile, len(s.currentSession.Files)),
		CreatedAt: s.currentSession.CreatedAt,
	}
	for k, v := range s.currentSession.Files {
		copySession.Files[k] = v
	}
	return copySession
}

// GetSessionByID returns the session if the ID matches.
// Returns a shallow copy of the session to prevent map data races during read.
func (s *ReceiveService) GetSessionByID(sessionID string) *ActiveReceiveSession {
	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()
	if s.currentSession != nil && s.currentSession.SessionID == sessionID {
		// Return a copy to avoid data races on the Files map
		copySession := &ActiveReceiveSession{
			SessionID: s.currentSession.SessionID,
			Sender:    s.currentSession.Sender,
			Files:     make(map[string]ActiveFile, len(s.currentSession.Files)),
			CreatedAt: s.currentSession.CreatedAt,
		}
		for k, v := range s.currentSession.Files {
			copySession.Files[k] = v
		}
		return copySession
	}
	return nil
}

// CloseSession closes the current session.
func (s *ReceiveService) CloseSession() {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()
	s.currentSession = nil
}

// RemoveFileFromSession removes a file from the current session.
func (s *ReceiveService) RemoveFileFromSession(sessionID, fileID string) {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()
	if s.currentSession != nil && s.currentSession.SessionID == sessionID {
		delete(s.currentSession.Files, fileID)
		if len(s.currentSession.Files) == 0 {
			s.currentSession = nil
		}
	}
}
