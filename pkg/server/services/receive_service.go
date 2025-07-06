
package services

import (
	"fmt"
	"sync"

	"github.com/bethropolis/localgo/pkg/model"
	"github.com/google/uuid"
)

// ActiveReceiveSession represents an active file receiving session.
type ActiveReceiveSession struct {
	SessionID string
	Sender    model.DeviceInfo
	Files     map[string]ActiveFile
}

// ActiveFile represents a file in an active session.
type ActiveFile struct {
	Dto   model.FileDto
	Token string
}

// ReceiveService manages file receiving sessions.
type ReceiveService struct {
	currentSession *ActiveReceiveSession
	sessionMutex   sync.Mutex
}

// NewReceiveService creates a new ReceiveService.
func NewReceiveService() *ReceiveService {
	return &ReceiveService{}
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
	}

	return s.currentSession, nil
}

// GetSession returns the current active session.
func (s *ReceiveService) GetSession() *ActiveReceiveSession {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()
	return s.currentSession
}

// GetSessionByID returns the session if the ID matches.
func (s *ReceiveService) GetSessionByID(sessionID string) *ActiveReceiveSession {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()
	if s.currentSession != nil && s.currentSession.SessionID == sessionID {
		return s.currentSession
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
