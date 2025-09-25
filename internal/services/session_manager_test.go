package services_test

import (
	"testing"

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
