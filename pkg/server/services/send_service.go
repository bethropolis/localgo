
package services

import (
	"fmt"
	"sync"

	"github.com/bet/localgo/pkg/model"
	"github.com/google/uuid"
)

// ActiveSendSession represents an active file sending session.
type ActiveSendSession struct {
	SessionID string
	Files     map[string]model.FileDto
}

// SendService manages file sending sessions.
type SendService struct {
	currentSession *ActiveSendSession
	sessionMutex   sync.Mutex
}

// NewSendService creates a new SendService.
func NewSendService() *SendService {
	return &SendService{}
}

// CreateSession creates a new send session.
func (s *SendService) CreateSession(files map[string]model.FileDto) (*ActiveSendSession, error) {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	if s.currentSession != nil {
		return nil, fmt.Errorf("session already active")
	}

	sessionId := uuid.NewString()
	s.currentSession = &ActiveSendSession{
		SessionID: sessionId,
		Files:     files,
	}

	return s.currentSession, nil
}

// GetSession returns the current active session.
func (s *SendService) GetSession() *ActiveSendSession {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()
	return s.currentSession
}

// GetSessionByID returns the session if the ID matches.
func (s *SendService) GetSessionByID(sessionID string) *ActiveSendSession {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()
	if s.currentSession != nil && s.currentSession.SessionID == sessionID {
		return s.currentSession
	}
	return nil
}

// CloseSession closes the current session.
func (s *SendService) CloseSession() {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()
	s.currentSession = nil
}
