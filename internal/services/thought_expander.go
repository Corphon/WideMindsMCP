//Core: Thought Expansion Engine(核心：思维扩散引擎)

package services

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	appErrors "WideMindsMCP/internal/errors"
	"WideMindsMCP/internal/models"
)

// 结构体
type ThoughtExpander struct {
	llmOrchestrator *LLMOrchestrator
	sessionManager  *SessionManager
}

type ExpansionRequest struct {
	Concept       string               `json:"concept"`
	Context       []string             `json:"context"`
	ExpansionType models.DirectionType `json:"expansionType"`
	MaxDirections int                  `json:"maxDirections"`
}

type ExpansionResult struct {
	Directions []models.Direction `json:"directions"`
	Thoughts   []*models.Thought  `json:"thoughts"`
}

// 函数
func NewThoughtExpander(llm *LLMOrchestrator, sm *SessionManager) *ThoughtExpander {
	return &ThoughtExpander{
		llmOrchestrator: llm,
		sessionManager:  sm,
	}
}

// 方法
func (te *ThoughtExpander) Expand(req *ExpansionRequest) (*ExpansionResult, error) {
	if te == nil {
		return nil, errors.New("thought expander is not initialized")
	}
	if req == nil {
		return nil, appErrors.ErrInvalidRequest
	}
	if req.Concept == "" {
		return nil, appErrors.ErrInvalidRequest
	}

	directions, err := te.GenerateDirections(req.Concept, req.Context)
	if err != nil {
		return nil, err
	}

	filtered := make([]models.Direction, 0, len(directions))
	for _, dir := range directions {
		if req.ExpansionType != "" && dir.Type != req.ExpansionType {
			continue
		}
		filtered = append(filtered, dir)
	}
	if len(filtered) == 0 {
		filtered = directions
	}

	if req.MaxDirections > 0 && len(filtered) > req.MaxDirections {
		filtered = filtered[:req.MaxDirections]
	}

	previewThoughts := make([]*models.Thought, 0, len(filtered))
	for _, dir := range filtered {
		previewCtx := buildExplorationInput(req.Context, dir)
		thoughts, err := te.llmOrchestrator.ExploreDirection(dir, 1, previewCtx)
		if err != nil {
			return nil, err
		}
		if len(thoughts) > 0 {
			previewThoughts = append(previewThoughts, thoughts[0])
		}
	}

	return &ExpansionResult{
		Directions: filtered,
		Thoughts:   previewThoughts,
	}, nil
}

func (te *ThoughtExpander) DeepDive(direction models.Direction, depth int) ([]*models.Thought, error) {
	if te == nil {
		return nil, errors.New("thought expander is not initialized")
	}
	if depth <= 0 {
		depth = 1
	}

	return te.llmOrchestrator.ExploreDirection(direction, depth, nil)
}

func (te *ThoughtExpander) GenerateDirections(concept string, context []string) ([]models.Direction, error) {
	if te == nil {
		return nil, errors.New("thought expander is not initialized")
	}
	return te.llmOrchestrator.GenerateThoughtDirections(concept, context)
}

func (te *ThoughtExpander) ExploreDirection(direction models.Direction, sessionID string) (*models.Thought, error) {
	if te == nil {
		return nil, errors.New("thought expander is not initialized")
	}
	if sessionID == "" {
		return nil, appErrors.ErrInvalidRequest
	}

	session, err := te.sessionManager.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	explorationCtx := buildSessionExplorationContext(session, direction)
	thoughts, err := te.llmOrchestrator.ExploreDirection(direction, 1, explorationCtx)
	if err != nil {
		return nil, err
	}
	if len(thoughts) == 0 {
		return nil, errors.New("no thoughts generated for direction")
	}

	thought := thoughts[0]
	thought.SessionID = session.ID

	parent := session.RootThought
	if thought.ParentID != nil {
		tree := session.GetThoughtTree()
		if existing, ok := tree[*thought.ParentID]; ok {
			parent = existing
		}
	}

	if parent == nil {
		session.RootThought = thought
	} else {
		parent.AddChild(thought)
	}

	session.UpdatedAt = time.Now().UTC()
	if err := te.sessionManager.UpdateSession(session); err != nil {
		return nil, err
	}

	return thought, nil
}

func buildExplorationInput(base []string, direction models.Direction) []string {
	entries := make([]string, 0, len(base)+4)
	for _, item := range base {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			entries = append(entries, trimmed)
		}
	}

	if title := strings.TrimSpace(direction.Title); title != "" {
		entries = append(entries, fmt.Sprintf("goal: deepen %s", title))
	}

	if desc := strings.TrimSpace(direction.Description); desc != "" {
		entries = append(entries, fmt.Sprintf("background: %s", desc))
	}

	if len(direction.Keywords) > 0 {
		keywords := make([]string, 0, len(direction.Keywords))
		for _, keyword := range direction.Keywords {
			keyword = strings.TrimSpace(keyword)
			if keyword != "" {
				keywords = append(keywords, keyword)
			}
		}
		if len(keywords) > 0 {
			entries = append(entries, fmt.Sprintf("keywords: %s", strings.Join(keywords, ", ")))
		}
	}

	return entries
}

func buildSessionExplorationContext(session *models.Session, direction models.Direction) []string {
	if session == nil {
		return buildExplorationInput(nil, direction)
	}

	base := make([]string, 0, len(session.Context)+6)
	for _, entry := range session.Context {
		trimmed := strings.TrimSpace(entry)
		if trimmed != "" {
			base = append(base, trimmed)
		}
	}

	if session.RootThought != nil {
		rootContent := strings.TrimSpace(session.RootThought.Content)
		if rootContent != "" {
			base = append(base, fmt.Sprintf("history: root -> %s", rootContent))
		}
		base = append(base, collectThoughtPathHints(session.RootThought, 4)...)
	}

	return buildExplorationInput(base, direction)
}

func collectThoughtPathHints(root *models.Thought, limit int) []string {
	if root == nil || limit <= 0 {
		return nil
	}

	queue := []*models.Thought{root}
	nodes := make([]*models.Thought, 0, 16)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current == nil {
			continue
		}
		nodes = append(nodes, current)
		for _, child := range current.Children {
			if child != nil {
				queue = append(queue, child)
			}
		}
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Depth == nodes[j].Depth {
			return strings.Compare(nodes[i].Content, nodes[j].Content) < 0
		}
		return nodes[i].Depth > nodes[j].Depth
	})

	hints := make([]string, 0, limit)
	seen := map[string]struct{}{}
	for _, node := range nodes {
		if len(hints) >= limit {
			break
		}
		path := node.GetPath()
		if len(path) == 0 {
			continue
		}
		joined := strings.Join(path, " -> ")
		if _, ok := seen[joined]; ok {
			continue
		}
		seen[joined] = struct{}{}
		hints = append(hints, fmt.Sprintf("history: %s", joined))
	}

	return hints
}
