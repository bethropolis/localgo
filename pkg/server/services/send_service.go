package services

import (
	"fmt"
	"sync"

	"github.com/bethropolis/localgo/pkg/model"
	"github.com/google/uuid"
)

// ActiveSendSession represents an active file sending session.
type ActiveSendSession struct {
	SessionID string
	Files     map[string]model.FileDto
	FilePaths map[string]string // Maps fileID to local absolute path
}

// SendService manages file sending sessions.
type SendService struct {
	currentSession *ActiveSendSession
	sessionMutex   sync.RWMutex
}

// NewSendService creates a new SendService.
func NewSendService() *SendService {
	return &SendService{}
}

// CreateSession creates a new send session.
func (s *SendService) CreateSession(files map[string]model.FileDto, filePaths map[string]string) (*ActiveSendSession, error) {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	if s.currentSession != nil {
		return nil, fmt.Errorf("session already active")
	}

	sessionId := uuid.NewString()
	s.currentSession = &ActiveSendSession{
		SessionID: sessionId,
		Files:     files,
		FilePaths: filePaths,
	}

	return s.currentSession, nil
}

// GetSession returns the current active session.
// Returns a shallow copy to prevent map races
func (s *SendService) GetSession() *ActiveSendSession {
	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()

	if s.currentSession == nil {
		return nil
	}

	copySession := &ActiveSendSession{
		SessionID: s.currentSession.SessionID,
		Files:     make(map[string]model.FileDto, len(s.currentSession.Files)),
		FilePaths: make(map[string]string, len(s.currentSession.FilePaths)),
	}
	for k, v := range s.currentSession.Files {
		copySession.Files[k] = v
	}
	for k, v := range s.currentSession.FilePaths {
		copySession.FilePaths[k] = v
	}
	return copySession
}

// GetSessionByID returns the session if the ID matches.
// Returns a shallow copy to prevent map races
func (s *SendService) GetSessionByID(sessionID string) *ActiveSendSession {
	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()

	if s.currentSession != nil && s.currentSession.SessionID == sessionID {
		copySession := &ActiveSendSession{
			SessionID: s.currentSession.SessionID,
			Files:     make(map[string]model.FileDto, len(s.currentSession.Files)),
			FilePaths: make(map[string]string, len(s.currentSession.FilePaths)),
		}
		for k, v := range s.currentSession.Files {
			copySession.Files[k] = v
		}
		for k, v := range s.currentSession.FilePaths {
			copySession.FilePaths[k] = v
		}
		return copySession
	}
	return nil
}

// CloseSession closes the current session.
func (s *SendService) CloseSession() {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()
	s.currentSession = nil
}
