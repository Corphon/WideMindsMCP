package storage_test

import (
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
