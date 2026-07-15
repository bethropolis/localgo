package services

import (
	"sync"
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

func TestReceiveService_CreateSession_BlocksConcurrent(t *testing.T) {
	svc := NewReceiveService()

	sender := model.DeviceInfo{
		Alias: "TestSender",
		IP:    "192.168.1.100",
		Port:  53317,
	}

	files := map[string]model.FileDto{
		"file1": {ID: "file1", FileName: "test.txt", Size: 1024},
	}

	first, err := svc.CreateSession(sender, files)
	if err != nil {
		t.Fatalf("First CreateSession should succeed: %v", err)
	}
	if first == nil {
		t.Fatal("Expected non-nil session")
	}

	second, err := svc.CreateSession(sender, files)
	if err == nil {
		t.Error("Expected error for second concurrent session, got nil")
	}
	if second != nil {
		t.Error("Expected nil session for blocked concurrent session")
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

	session, _ := svc.CreateSession(sender, files)
	svc.CloseSession(session.SessionID)

	closed := svc.GetSessionByID(session.SessionID)
	if closed != nil {
		t.Error("Expected nil session after closing")
	}
}

func TestReceiveService_ClaimFile_Success(t *testing.T) {
	svc := NewReceiveService()
	sender := model.DeviceInfo{Alias: "Alice", IP: "192.168.1.10"}
	files := map[string]model.FileDto{
		"f1": {ID: "f1", FileName: "doc.txt", Size: 100},
	}
	session, _ := svc.CreateSession(sender, files)

	dto, gotSender, err := svc.ClaimFile(session.SessionID, "f1", session.Files["f1"].Token, "192.168.1.10")
	if err != nil {
		t.Fatalf("ClaimFile failed: %v", err)
	}
	if dto.FileName != "doc.txt" {
		t.Errorf("expected doc.txt, got %s", dto.FileName)
	}
	if gotSender.Alias != "Alice" {
		t.Errorf("expected Alice, got %s", gotSender.Alias)
	}

	// Second claim should fail
	_, _, err = svc.ClaimFile(session.SessionID, "f1", session.Files["f1"].Token, "192.168.1.10")
	if err != ErrAlreadyUploading {
		t.Errorf("expected ErrAlreadyUploading, got %v", err)
	}
}

func TestReceiveService_ClaimFile_Errors(t *testing.T) {
	svc := NewReceiveService()
	sender := model.DeviceInfo{Alias: "Alice", IP: "192.168.1.10"}
	files := map[string]model.FileDto{
		"f1": {ID: "f1", FileName: "doc.txt", Size: 100},
	}
	session, _ := svc.CreateSession(sender, files)

	tests := []struct {
		name      string
		sessionID string
		fileID    string
		token     string
		senderIP  string
		wantErr   error
	}{
		{"invalid session", "nonexistent", "f1", "x", "192.168.1.10", ErrSessionNotFound},
		{"invalid file", session.SessionID, "bad", "x", "192.168.1.10", ErrInvalidFileToken},
		{"ip mismatch", session.SessionID, "f1", session.Files["f1"].Token, "192.168.1.99", ErrIPMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := svc.ClaimFile(tt.sessionID, tt.fileID, tt.token, tt.senderIP)
			if err != tt.wantErr {
				t.Errorf("got %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestReceiveService_ClaimFile_Concurrent(t *testing.T) {
	svc := NewReceiveService()
	sender := model.DeviceInfo{Alias: "Bob", IP: "10.0.0.1"}
	files := map[string]model.FileDto{
		"f1": {ID: "f1", FileName: "shared.txt", Size: 50},
	}
	session, _ := svc.CreateSession(sender, files)

	// Evaluate args before goroutines to avoid data race
	// on the shared session's Files map.
	sid := session.SessionID
	token := session.Files["f1"].Token

	var wg sync.WaitGroup
	results := make(chan error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := svc.ClaimFile(sid, "f1", token, "10.0.0.1")
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	successCount := 0
	for err := range results {
		if err == nil {
			successCount++
		} else if err != ErrAlreadyUploading {
			t.Errorf("unexpected error: %v", err)
		}
	}
	if successCount != 1 {
		t.Errorf("expected exactly 1 success, got %d", successCount)
	}
}

func TestReceiveService_CompleteFile(t *testing.T) {
	svc := NewReceiveService()
	sender := model.DeviceInfo{Alias: "Alice", IP: "192.168.1.10"}
	files := map[string]model.FileDto{
		"f1": {ID: "f1", FileName: "doc.txt", Size: 100},
	}
	session, _ := svc.CreateSession(sender, files)

	svc.CompleteFile(session.SessionID, "f1")

	// File should be gone
	up := svc.GetSessionByID(session.SessionID)
	if up != nil {
		t.Error("expected session to be removed after completing the last file")
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
