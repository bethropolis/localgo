package services

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/google/uuid"
)

// FileTransferState tracks the lifecycle of a file within a receive session.
type FileTransferState int

const (
	FilePending   FileTransferState = iota // initial state after session creation
	FileUploading                           // claim acquired, upload in progress
	FileDone                                // upload completed or failed
)

var (
	ErrSessionNotFound  = errors.New("invalid session")
	ErrIPMismatch       = errors.New("ip mismatch")
	ErrInvalidFileToken = errors.New("invalid file or token")
	ErrAlreadyUploading = errors.New("already uploading")
	ErrAlreadyCompleted = errors.New("already completed")
)

// ActiveReceiveSession represents an active file receiving session.
type ActiveReceiveSession struct {
	SessionID string
	Sender    model.DeviceInfo
	Files     map[string]ActiveFile
	CreatedAt time.Time
	Progress  *cli.MultiProgress
}

// ActiveFile represents a file in an active session.
type ActiveFile struct {
	Dto   model.FileDto
	Token string
	State FileTransferState
}

// ReceiveService manages file receiving sessions.
type ReceiveService struct {
	sessions     map[string]*ActiveReceiveSession
	sessionMutex sync.RWMutex
	stopCh       chan struct{}
	closeOnce    sync.Once
}

// NewReceiveService creates a new ReceiveService.
func NewReceiveService() *ReceiveService {
	s := &ReceiveService{
		sessions: make(map[string]*ActiveReceiveSession),
		stopCh:   make(chan struct{}),
	}
	go s.cleanupLoop()
	return s
}

// Close stops the cleanup loop and releases resources.
func (s *ReceiveService) Close() {
	s.closeOnce.Do(func() {
		close(s.stopCh)
	})
}

// cleanupLoop periodically checks and expires stale sessions
func (s *ReceiveService) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.sessionMutex.Lock()
			for id, session := range s.sessions {
				if time.Since(session.CreatedAt) > 10*time.Minute {
					if session.Progress != nil {
						session.Progress.ForceComplete()
						go session.Progress.Wait()
					}
					delete(s.sessions, id)
				}
			}
			s.sessionMutex.Unlock()
		}
	}
}

// CreateSession creates a new receive session.
// Returns an error if another session is already active (409 Blocked by another session).
func (s *ReceiveService) CreateSession(sender model.DeviceInfo, files map[string]model.FileDto) (*ActiveReceiveSession, error) {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	if len(s.sessions) > 0 {
		return nil, fmt.Errorf("another session is already active")
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

	session := &ActiveReceiveSession{
		SessionID: sessionId,
		Sender:    sender,
		Files:     sessionFiles,
		CreatedAt: time.Now(),
		Progress:  cli.NewMultiProgress(int64(len(files))),
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
		Progress:  orig.Progress,
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
	if session, ok := s.sessions[sessionID]; ok {
		if session.Progress != nil {
			session.Progress.ForceComplete()
			session.Progress.Wait()
		}
		delete(s.sessions, sessionID)
	}
}

// ClaimFile atomically validates session, sender IP, file ID, and token,
// then marks the file as uploading. Returns the file DTO and sender info.
// Returns ErrAlreadyUploading / ErrAlreadyCompleted for duplicate requests.
// Caller must call CompleteFile or FailFile after the upload finishes.
func (s *ReceiveService) ClaimFile(sessionID, fileID, token, senderIP string) (model.FileDto, model.DeviceInfo, error) {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return model.FileDto{}, model.DeviceInfo{}, ErrSessionNotFound
	}
	if senderIP != session.Sender.IP {
		return model.FileDto{}, model.DeviceInfo{}, ErrIPMismatch
	}
	file, ok := session.Files[fileID]
	if !ok || file.Token != token {
		return model.FileDto{}, model.DeviceInfo{}, ErrInvalidFileToken
	}
	switch file.State {
	case FileUploading:
		return model.FileDto{}, model.DeviceInfo{}, ErrAlreadyUploading
	case FileDone:
		return model.FileDto{}, model.DeviceInfo{}, ErrAlreadyCompleted
	}
	file.State = FileUploading
	session.Files[fileID] = file
	return file.Dto, session.Sender, nil
}

// CompleteFile removes the file from the session after a successful upload.
// If no files remain, the session is cleaned up and the progress bar completes.
func (s *ReceiveService) CompleteFile(sessionID, fileID string) {
	s.sessionMutex.Lock()
	session, ok := s.sessions[sessionID]
	if !ok {
		s.sessionMutex.Unlock()
		return
	}
	delete(session.Files, fileID)
	sessionEmpty := len(session.Files) == 0
	if sessionEmpty {
		delete(s.sessions, sessionID)
	}
	s.sessionMutex.Unlock()

	if sessionEmpty && session.Progress != nil {
		session.Progress.ForceComplete()
		go session.Progress.Wait()
	}
}

// FailFile resets the file state back to pending so the sender can retry.
func (s *ReceiveService) FailFile(sessionID, fileID string) {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return
	}
	file, ok := session.Files[fileID]
	if !ok {
		return
	}
	file.State = FilePending
	session.Files[fileID] = file
}

// GetSessionProgress returns the MultiProgress for a session (or nil).
// The Progress pointer is assigned at session creation and never mutated,
// so this is safe to read under RLock.
func (s *ReceiveService) GetSessionProgress(sessionID string) *cli.MultiProgress {
	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()
	if session, ok := s.sessions[sessionID]; ok {
		return session.Progress
	}
	return nil
}

// CloseAllSessions force-completes progress bars and removes all active sessions.
func (s *ReceiveService) CloseAllSessions() {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	for id, session := range s.sessions {
		if session.Progress != nil {
			session.Progress.ForceComplete()
			go session.Progress.Wait()
		}
		delete(s.sessions, id)
	}
}

// RemoveFileFromSession removes a file from the current session.
func (s *ReceiveService) RemoveFileFromSession(sessionID, fileID string) {
	s.sessionMutex.Lock()
	session, ok := s.sessions[sessionID]
	if !ok {
		s.sessionMutex.Unlock()
		return
	}
	delete(session.Files, fileID)
	sessionEmpty := len(session.Files) == 0
	if sessionEmpty {
		delete(s.sessions, sessionID)
	}
	s.sessionMutex.Unlock()

	// Gracefully stop the progress bar rendering goroutine when the session ends
	if sessionEmpty && session.Progress != nil {
		session.Progress.ForceComplete()
		go session.Progress.Wait()
	}
}
