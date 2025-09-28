package services

import (
	"strings"
	"testing"

	"WideMindsMCP/internal/models"
)

func TestBuildSessionExplorationContext(t *testing.T) {
	session := &models.Session{
		ID:      "session-test",
		Context: []string{" background: robotics ", ""},
	}

	rootDirection := models.Direction{Title: "Root", Description: "Initial concept"}
	session.RootThought = models.NewThought("AI strategy", session.ID, rootDirection)

	childDirection := models.Direction{Title: "Risk assessment"}
	child := models.NewThought("Ethical risks", session.ID, childDirection)
	session.RootThought.AddChild(child)

	targetDirection := models.Direction{Title: "Energy Storage", Description: "Focus on battery lifecycles", Keywords: []string{"batteries", "supply chain"}}

	ctx := buildSessionExplorationContext(session, targetDirection)

	assertContains(t, ctx, "background: robotics")
	assertContains(t, ctx, "history: root -> AI strategy")
	assertContains(t, ctx, "history: AI strategy -> Ethical risks")
	assertContains(t, ctx, "goal: deepen Energy Storage")
	assertContains(t, ctx, "keywords: batteries, supply chain")
}

func TestExploreDirectionIncludesContextSummary(t *testing.T) {
	orchestrator := NewLLMOrchestrator("", "", "")

	direction := models.Direction{
		Title:       "Systems thinking",
		Description: "Explore interconnected impacts",
	}

	context := []string{"background: robotics", "history: AI strategy -> Ethical risks", "preference: concise"}

	thoughts, err := orchestrator.ExploreDirection(direction, 2, context)
	if err != nil {
		t.Fatalf("ExploreDirection returned error: %v", err)
	}

	if len(thoughts) != 2 {
		t.Fatalf("expected 2 thoughts, got %d", len(thoughts))
	}

	for _, thought := range thoughts {
		if thought == nil {
			t.Fatalf("thought should not be nil")
		}
		if thought.Content == "" {
			t.Fatalf("thought content should not be empty")
		}
		if !containsSubstring(thought.Content, "context: background: robotics | history: AI strategy -> Ethical risks | preference: concise") {
			t.Fatalf("expected context summary in thought content, got %q", thought.Content)
		}
	}
}

func assertContains(t *testing.T, list []string, expected string) {
	t.Helper()
	for _, entry := range list {
		if entry == expected {
			return
		}
	}
	t.Fatalf("expected list to contain %q, got %v", expected, list)
}

func containsSubstring(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
