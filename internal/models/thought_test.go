package models_test

import (
	"testing"

	"WideMindsMCP/internal/models"
)

func TestThoughtAddChildUpdatesPathAndDepth(t *testing.T) {
	direction := models.Direction{Type: models.Broad, Title: "Root"}
	parent := models.NewThought("root concept", "session-1", direction)
	childDirection := models.Direction{Type: models.Deep, Title: "Deep"}
	child := models.NewThought("child concept", "session-1", childDirection)

	parent.AddChild(child)

	if child.ParentID == nil || *child.ParentID != parent.ID {
		t.Fatalf("expected parent ID to be %s, got %v", parent.ID, child.ParentID)
	}

	if child.Depth != parent.Depth+1 {
		t.Fatalf("expected child depth %d, got %d", parent.Depth+1, child.Depth)
	}

	expectedPath := []string{"root concept", "child concept"}
	if got := child.GetPath(); len(got) != len(expectedPath) {
		t.Fatalf("expected path length %d, got %d", len(expectedPath), len(got))
	} else {
		for i := range expectedPath {
			if got[i] != expectedPath[i] {
				t.Fatalf("expected path %v, got %v", expectedPath, got)
			}
		}
	}

	if !child.CreatedAt.After(parent.CreatedAt) && !child.CreatedAt.Equal(parent.CreatedAt) {
		t.Fatalf("child CreatedAt should be set")
	}
}
