package storage_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"WideMindsMCP/internal/models"
	"WideMindsMCP/internal/storage"
)

func TestInMemorySessionStoreLifecycle(t *testing.T) {
	store := storage.NewInMemorySessionStore()
	session := models.NewSession("user", "数据科学")

	if err := store.Save(session); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := store.Get(session.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if loaded.ID != session.ID {
		t.Fatalf("expected session id %s, got %s", session.ID, loaded.ID)
	}

	loaded.AddContext("新的上下文")
	if err := store.Update(loaded); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	sessions, err := store.GetByUserID("user")
	if err != nil {
		t.Fatalf("get by user failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	expired, err := store.GetExpiredSessions(time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("get expired failed: %v", err)
	}
	if len(expired) != 1 {
		t.Fatalf("expected expired session, got %d", len(expired))
	}

	if err := store.Delete(session.ID); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
}

func TestFileSessionStoreIndexPersistence(t *testing.T) {
	dataDir := t.TempDir()
	store := storage.NewFileSessionStore(dataDir)
	session := models.NewSession("persist-user", "思维导图")

	if err := store.Save(session); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	indexPath := filepath.Join(dataDir, "index.json")
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("expected index file, got %v", err)
	}

	store = storage.NewFileSessionStore(dataDir)
	sessions, err := store.GetByUserID("persist-user")
	if err != nil {
		t.Fatalf("get by user failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != session.ID {
		t.Fatalf("expected session id %s, got %s", session.ID, sessions[0].ID)
	}
}

func TestFileSessionStoreIndexCorruptionRecovery(t *testing.T) {
	dataDir := t.TempDir()
	store := storage.NewFileSessionStore(dataDir)
	session := models.NewSession("user", "纠错")

	if err := store.Save(session); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	indexPath := filepath.Join(dataDir, "index.json")
	if err := os.WriteFile(indexPath, []byte("not-json"), 0o644); err != nil {
		t.Fatalf("corrupt index failed: %v", err)
	}

	store = storage.NewFileSessionStore(dataDir)
	sessionsAfter, err := store.GetByUserID("user")
	if err != nil {
		t.Fatalf("get by user failed: %v", err)
	}
	if len(sessionsAfter) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessionsAfter))
	}

	raw, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read index failed: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatalf("index file was not repaired")
	}
}

func TestFileSessionStoreGetExpiredSessions(t *testing.T) {
	dataDir := t.TempDir()
	store := storage.NewFileSessionStore(dataDir)
	now := time.Now().UTC()

	oldSession := models.NewSession("user-old", "历史")
	oldSession.CreatedAt = now.Add(-2 * time.Hour)
	oldSession.UpdatedAt = now.Add(-2 * time.Hour)

	if err := store.Save(oldSession); err != nil {
		t.Fatalf("save old session failed: %v", err)
	}

	recentSession := models.NewSession("user-new", "最新")
	recentSession.CreatedAt = now.Add(-30 * time.Minute)
	recentSession.UpdatedAt = now.Add(-30 * time.Minute)

	if err := store.Save(recentSession); err != nil {
		t.Fatalf("save recent session failed: %v", err)
	}

	cutoff := now.Add(-1 * time.Hour)
	expired, err := store.GetExpiredSessions(cutoff)
	if err != nil {
		t.Fatalf("get expired sessions failed: %v", err)
	}

	if len(expired) != 1 {
		t.Fatalf("expected 1 expired session, got %d", len(expired))
	}
	if expired[0].ID != oldSession.ID {
		t.Fatalf("expected expired session %s, got %s", oldSession.ID, expired[0].ID)
	}
}
