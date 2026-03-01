package services

import (
	"testing"

	"github.com/bethropolis/localgo/pkg/model"
)

func TestSendService_CreateSession(t *testing.T) {
	svc := NewSendService()

	files := map[string]model.FileDto{
		"file1": {
			ID:       "file1",
			FileName: "test.txt",
			Size:     1024,
		},
	}

	filePaths := map[string]string{
		"file1": "/path/to/test.txt",
	}

	session, err := svc.CreateSession(files, filePaths)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session.SessionID == "" {
		t.Error("Expected non-empty session ID")
	}

	if len(session.Files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(session.Files))
	}

	if len(session.FilePaths) != 1 {
		t.Errorf("Expected 1 file path, got %d", len(session.FilePaths))
	}
}

func TestSendService_CreateSession_AlreadyActive(t *testing.T) {
	svc := NewSendService()

	files := map[string]model.FileDto{"file1": {ID: "file1", FileName: "test.txt"}}
	filePaths := map[string]string{"file1": "/path/to/test.txt"}

	_, err := svc.CreateSession(files, filePaths)
	if err != nil {
		t.Fatalf("First CreateSession failed: %v", err)
	}

	_, err = svc.CreateSession(files, filePaths)
	if err == nil {
		t.Error("Expected error when creating session while one is already active")
	}
}

func TestSendService_GetSession(t *testing.T) {
	svc := NewSendService()

	session := svc.GetSession()
	if session != nil {
		t.Error("Expected nil session initially")
	}

	files := map[string]model.FileDto{"file1": {ID: "file1", FileName: "test.txt"}}
	filePaths := map[string]string{"file1": "/path/to/test.txt"}

	createdSession, _ := svc.CreateSession(files, filePaths)
	retrievedSession := svc.GetSession()

	if retrievedSession.SessionID != createdSession.SessionID {
		t.Errorf("Expected session ID %s, got %s", createdSession.SessionID, retrievedSession.SessionID)
	}
}

func TestSendService_GetSessionByID(t *testing.T) {
	svc := NewSendService()

	files := map[string]model.FileDto{"file1": {ID: "file1", FileName: "test.txt"}}
	filePaths := map[string]string{"file1": "/path/to/test.txt"}

	createdSession, _ := svc.CreateSession(files, filePaths)

	retrievedSession := svc.GetSessionByID(createdSession.SessionID)
	if retrievedSession == nil {
		t.Fatal("Expected to find session by ID")
	}

	notFoundSession := svc.GetSessionByID("nonexistent")
	if notFoundSession != nil {
		t.Error("Expected nil for nonexistent session ID")
	}
}

func TestSendService_CloseSession(t *testing.T) {
	svc := NewSendService()

	files := map[string]model.FileDto{"file1": {ID: "file1", FileName: "test.txt"}}
	filePaths := map[string]string{"file1": "/path/to/test.txt"}

	svc.CreateSession(files, filePaths)
	svc.CloseSession()

	session := svc.GetSession()
	if session != nil {
		t.Error("Expected nil session after closing")
	}
}
