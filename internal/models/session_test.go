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

func TestSessionApplyThoughtUpdate(t *testing.T) {
	session := models.NewSession("user", "Root")
	child := models.NewThought("Child", session.ID, models.Direction{Type: models.Deep, Title: "Initial"})
	grandchild := models.NewThought("Grand", session.ID, models.Direction{Type: models.Lateral, Title: "Branch"})
	child.AddChild(grandchild)
	session.RootThought.AddChild(child)

	updatedTitle := "Refined"
	newContent := "Updated child"
	update := &models.ThoughtUpdate{Content: &newContent, Direction: &models.Direction{Type: models.Deep, Title: updatedTitle}}

	updated, err := session.ApplyThoughtUpdate(child.ID, update)
	if err != nil {
		t.Fatalf("ApplyThoughtUpdate returned error: %v", err)
	}

	if updated.Content != newContent {
		t.Fatalf("expected content %q, got %q", newContent, updated.Content)
	}

	if updated.Direction.Title != updatedTitle {
		t.Fatalf("expected direction title %q, got %q", updatedTitle, updated.Direction.Title)
	}

	if len(updated.Path) != 2 || updated.Path[1] != newContent {
		t.Fatalf("expected updated path to include new content, got %#v", updated.Path)
	}

	if grandchild.Depth != 2 {
		t.Fatalf("expected grandchild depth 2, got %d", grandchild.Depth)
	}

	if len(grandchild.Path) != 3 || grandchild.Path[1] != newContent {
		t.Fatalf("expected grandchild path to reflect updated ancestor, got %#v", grandchild.Path)
	}
}

func TestSessionRemoveThought(t *testing.T) {
	session := models.NewSession("user", "Root")
	child := models.NewThought("Child", session.ID, models.Direction{Type: models.Deep, Title: "Initial"})
	session.RootThought.AddChild(child)

	if err := session.RemoveThought(child.ID); err != nil {
		t.Fatalf("RemoveThought returned error: %v", err)
	}

	if len(session.RootThought.Children) != 0 {
		t.Fatalf("expected root children to be empty after removal")
	}

	if err := session.RemoveThought(session.RootThought.ID); err != nil {
		t.Fatalf("RemoveThought on root returned error: %v", err)
	}

	if session.RootThought != nil {
		t.Fatalf("expected root thought to be nil after root removal")
	}
}
