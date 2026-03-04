package services

import (
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
	sessions     map[string]*ActiveReceiveSession
	sessionMutex sync.RWMutex
}

// NewReceiveService creates a new ReceiveService.
func NewReceiveService() *ReceiveService {
	s := &ReceiveService{
		sessions: make(map[string]*ActiveReceiveSession),
	}
	go s.cleanupLoop()
	return s
}

// cleanupLoop periodically checks and expires stale sessions
func (s *ReceiveService) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		s.sessionMutex.Lock()
		for id, session := range s.sessions {
			if time.Since(session.CreatedAt) > 10*time.Minute {
				delete(s.sessions, id)
			}
		}
		s.sessionMutex.Unlock()
	}
}

// CreateSession creates a new receive session.
func (s *ReceiveService) CreateSession(sender model.DeviceInfo, files map[string]model.FileDto) (*ActiveReceiveSession, error) {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	sessionId := uuid.NewString()
	sessionFiles := make(map[string]ActiveFile)
	for fileId, fileDto := range files {
		token := uuid.NewString()
		sessionFiles[fileId] = ActiveFile{
			Dto:   fileDto,
			Token: token,
		}
	}

	session := &ActiveReceiveSession{
		SessionID: sessionId,
		Sender:    sender,
		Files:     sessionFiles,
		CreatedAt: time.Now(),
	}

	s.sessions[sessionId] = session

	return session, nil
}

// GetSession returns a legacy session if one exists (for backward compatibility).
// This is mostly deprecated and GetSessionByID should be used instead.
// Returns the first active session found or nil.
func (s *ReceiveService) GetSession() *ActiveReceiveSession {
	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()

	for _, session := range s.sessions {
		return s.copySession(session)
	}
	return nil
}

// GetSessionByID returns the session if the ID matches.
// Returns a shallow copy of the session to prevent map data races during read.
func (s *ReceiveService) GetSessionByID(sessionID string) *ActiveReceiveSession {
	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()

	if session, ok := s.sessions[sessionID]; ok {
		return s.copySession(session)
	}
	return nil
}

func (s *ReceiveService) copySession(orig *ActiveReceiveSession) *ActiveReceiveSession {
	copySession := &ActiveReceiveSession{
		SessionID: orig.SessionID,
		Sender:    orig.Sender,
		Files:     make(map[string]ActiveFile, len(orig.Files)),
		CreatedAt: orig.CreatedAt,
	}
	for k, v := range orig.Files {
		copySession.Files[k] = v
	}
	return copySession
}

// CloseSession closes a specific session.
func (s *ReceiveService) CloseSession(sessionID string) {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()
	delete(s.sessions, sessionID)
}

// RemoveFileFromSession removes a file from the current session.
func (s *ReceiveService) RemoveFileFromSession(sessionID, fileID string) {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()
	if session, ok := s.sessions[sessionID]; ok {
		delete(session.Files, fileID)
		if len(session.Files) == 0 {
			delete(s.sessions, sessionID)
		}
	}
}
