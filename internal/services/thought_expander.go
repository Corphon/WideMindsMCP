//Core: Thought Expansion Engine(核心：思维扩散引擎)

package services

import (
	"errors"
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
	Concept       string
	Context       []string
	ExpansionType models.DirectionType
	MaxDirections int
}

type ExpansionResult struct {
	Directions []models.Direction
	Thoughts   []*models.Thought
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
		thoughts, err := te.llmOrchestrator.ExploreDirection(dir, 1)
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

	return te.llmOrchestrator.ExploreDirection(direction, depth)
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

	thoughts, err := te.llmOrchestrator.ExploreDirection(direction, 1)
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
