package models_test

import (
	"testing"

	"WideMindsMCP/internal/models"
)

func TestSessionMetadata(t *testing.T) {
	session := models.NewSession("user-1", "Machine Learning")
	if session.RootThought == nil {
		t.Fatalf("expected root thought to be created")
	}

	direction := models.Direction{Type: models.Deep, Title: "数学基础"}
	child := models.NewThought("线性代数", session.ID, direction)
	session.RootThought.AddChild(child)

	meta := session.GetMetadata()
	if meta.TotalThoughts != 2 {
		t.Fatalf("expected 2 thoughts, got %d", meta.TotalThoughts)
	}

	if meta.MaxDepth != 1 {
		t.Fatalf("expected max depth 1, got %d", meta.MaxDepth)
	}

	found := false
	for _, dir := range meta.Directions {
		if dir == direction.Title {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected directions to include %s", direction.Title)
	}

	session.Close()
	if session.IsActive {
		t.Fatalf("expected session to be inactive after Close")
	}
}
