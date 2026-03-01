package services

import (
	"testing"

	"github.com/bethropolis/localgo/pkg/model"
)

func TestReceiveService_CreateSession(t *testing.T) {
	svc := NewReceiveService()

	sender := model.DeviceInfo{
		Alias:       "TestSender",
		IP:          "192.168.1.100",
		Port:        53317,
		Fingerprint: "abc123",
	}

	files := map[string]model.FileDto{
		"file1": {
			ID:       "file1",
			FileName: "test.txt",
			Size:     1024,
		},
	}

	session, err := svc.CreateSession(sender, files)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session.SessionID == "" {
		t.Error("Expected non-empty session ID")
	}

	if len(session.Files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(session.Files))
	}

	for fileID, file := range session.Files {
		if file.Token == "" {
			t.Errorf("Expected non-empty token for file %s", fileID)
		}
	}
}

func TestReceiveService_CreateSession_AlreadyActive(t *testing.T) {
	svc := NewReceiveService()

	sender := model.DeviceInfo{
		Alias: "TestSender",
		IP:    "192.168.1.100",
		Port:  53317,
	}

	files := map[string]model.FileDto{
		"file1": {ID: "file1", FileName: "test.txt", Size: 1024},
	}

	_, err := svc.CreateSession(sender, files)
	if err != nil {
		t.Fatalf("First CreateSession failed: %v", err)
	}

	_, err = svc.CreateSession(sender, files)
	if err == nil {
		t.Error("Expected error when creating session while one is already active")
	}
}

func TestReceiveService_GetSession(t *testing.T) {
	svc := NewReceiveService()

	session := svc.GetSession()
	if session != nil {
		t.Error("Expected nil session initially")
	}

	sender := model.DeviceInfo{Alias: "TestSender"}
	files := map[string]model.FileDto{"file1": {ID: "file1", FileName: "test.txt"}}

	createdSession, _ := svc.CreateSession(sender, files)
	retrievedSession := svc.GetSession()

	if retrievedSession.SessionID != createdSession.SessionID {
		t.Errorf("Expected session ID %s, got %s", createdSession.SessionID, retrievedSession.SessionID)
	}
}

func TestReceiveService_GetSessionByID(t *testing.T) {
	svc := NewReceiveService()

	sender := model.DeviceInfo{Alias: "TestSender"}
	files := map[string]model.FileDto{"file1": {ID: "file1", FileName: "test.txt"}}

	createdSession, _ := svc.CreateSession(sender, files)

	retrievedSession := svc.GetSessionByID(createdSession.SessionID)
	if retrievedSession == nil {
		t.Fatal("Expected to find session by ID")
	}

	notFoundSession := svc.GetSessionByID("nonexistent")
	if notFoundSession != nil {
		t.Error("Expected nil for nonexistent session ID")
	}
}

func TestReceiveService_CloseSession(t *testing.T) {
	svc := NewReceiveService()

	sender := model.DeviceInfo{Alias: "TestSender"}
	files := map[string]model.FileDto{"file1": {ID: "file1", FileName: "test.txt"}}

	svc.CreateSession(sender, files)
	svc.CloseSession()

	session := svc.GetSession()
	if session != nil {
		t.Error("Expected nil session after closing")
	}
}

func TestReceiveService_RemoveFileFromSession(t *testing.T) {
	svc := NewReceiveService()

	sender := model.DeviceInfo{Alias: "TestSender"}
	files := map[string]model.FileDto{
		"file1": {ID: "file1", FileName: "test1.txt"},
		"file2": {ID: "file2", FileName: "test2.txt"},
	}

	session, _ := svc.CreateSession(sender, files)
	svc.RemoveFileFromSession(session.SessionID, "file1")

	updatedSession := svc.GetSession()
	if len(updatedSession.Files) != 1 {
		t.Errorf("Expected 1 file remaining, got %d", len(updatedSession.Files))
	}

	svc.RemoveFileFromSession(session.SessionID, "file2")
	updatedSession = svc.GetSession()
	if updatedSession != nil {
		t.Error("Expected session to be nil after removing all files")
	}
}
