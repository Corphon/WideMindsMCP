package services_test

import (
	"testing"
	"time"

	"WideMindsMCP/internal/models"
	"WideMindsMCP/internal/services"
	"WideMindsMCP/internal/storage"
)

func TestSessionManagerCreateAndRetrieve(t *testing.T) {
	store := storage.NewInMemorySessionStore()
	manager := services.NewSessionManager(store)

	session, err := manager.CreateSession("user-42", "人工智能")
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	fetched, err := manager.GetSession(session.ID)
	if err != nil {
		t.Fatalf("get session failed: %v", err)
	}

	if fetched.ID != session.ID {
		t.Fatalf("expected session id %s, got %s", session.ID, fetched.ID)
	}

	thought := models.NewThought("监督学习", session.ID, models.Direction{Type: models.Broad, Title: "学习方法"})
	if err := manager.AddThoughtToSession(session.ID, thought); err != nil {
		t.Fatalf("add thought failed: %v", err)
	}

	fetched, err = manager.GetSession(session.ID)
	if err != nil {
		t.Fatalf("get session failed: %v", err)
	}

	meta := fetched.GetMetadata()
	if meta.TotalThoughts != 2 {
		t.Fatalf("expected total thoughts 2, got %d", meta.TotalThoughts)
	}
}

func TestSessionManagerListSessions(t *testing.T) {
	store := storage.NewInMemorySessionStore()
	manager := services.NewSessionManager(store)

	if _, err := manager.ListSessions(""); err == nil {
		t.Fatalf("expected error when listing sessions without user id")
	}

	first, err := manager.CreateSession("user-1", "First Concept")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	second, err := manager.CreateSession("user-1", "Second Concept")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	secondFetched, err := manager.GetSession(second.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	secondFetched.AddContext("extra")
	if err := manager.UpdateSession(secondFetched); err != nil {
		t.Fatalf("UpdateSession failed: %v", err)
	}

	sessions, err := manager.ListSessions("user-1")
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	if sessions[0].ID != second.ID {
		t.Fatalf("expected most recent session first, got %s", sessions[0].ID)
	}

	if sessions[1].ID != first.ID {
		t.Fatalf("expected first session second, got %s", sessions[1].ID)
	}
}
