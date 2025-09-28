//Thought Session(思维会话)

package models

import (
	"fmt"
	"sort"
	"strings"
	"time"

	appErrors "WideMindsMCP/internal/errors"

	"github.com/google/uuid"
)

// 结构体
type Session struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userId"`
	RootThought *Thought  `json:"rootThought,omitempty"`
	Context     []string  `json:"context,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	IsActive    bool      `json:"isActive"`
}

func (s *Session) FindThought(thoughtID string) (*Thought, *Thought) {
	if s == nil || s.RootThought == nil || strings.TrimSpace(thoughtID) == "" {
		return nil, nil
	}

	queue := []*Thought{s.RootThought}
	parentMap := map[string]*Thought{s.RootThought.ID: nil}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == nil {
			continue
		}
		if current.ID == thoughtID {
			return current, parentMap[current.ID]
		}
		for _, child := range current.Children {
			if child == nil {
				continue
			}
			parentMap[child.ID] = current
			queue = append(queue, child)
		}
	}
	return nil, nil
}

func (s *Session) NormalizeTree() {
	if s == nil || s.RootThought == nil {
		return
	}

	s.RootThought.ParentID = nil
	s.RootThought.parent = nil
	s.RootThought.Depth = 0
	s.RootThought.Path = []string{s.RootThought.Content}

	queue := []*Thought{s.RootThought}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == nil {
			continue
		}

		for _, child := range current.Children {
			if child == nil {
				continue
			}
			child.parent = current
			child.ParentID = &current.ID
			child.Depth = current.Depth + 1
			if len(current.Path) > 0 {
				child.Path = append(append([]string{}, current.Path...), child.Content)
			} else {
				child.Path = []string{child.Content}
			}
			queue = append(queue, child)
		}
	}
}

func (s *Session) ApplyThoughtUpdate(thoughtID string, update *ThoughtUpdate) (*Thought, error) {
	if s == nil || strings.TrimSpace(thoughtID) == "" || update == nil {
		return nil, appErrors.ErrInvalidRequest
	}

	target, _ := s.FindThought(thoughtID)
	if target == nil {
		return nil, fmt.Errorf("%w: %s", appErrors.ErrThoughtNotFound, thoughtID)
	}

	if update.Content != nil {
		target.Content = strings.TrimSpace(*update.Content)
	}
	if update.Direction != nil {
		target.Direction = *update.Direction
	}

	s.NormalizeTree()
	s.UpdatedAt = time.Now().UTC()

	return target, nil
}

func (s *Session) RemoveThought(thoughtID string) error {
	if s == nil || strings.TrimSpace(thoughtID) == "" {
		return appErrors.ErrInvalidRequest
	}

	if s.RootThought == nil {
		return fmt.Errorf("%w: %s", appErrors.ErrThoughtNotFound, thoughtID)
	}

	if s.RootThought.ID == thoughtID {
		s.RootThought = nil
		s.UpdatedAt = time.Now().UTC()
		return nil
	}

	_, parent := s.FindThought(thoughtID)
	if parent == nil {
		return fmt.Errorf("%w: %s", appErrors.ErrThoughtNotFound, thoughtID)
	}

	if !parent.RemoveChildByID(thoughtID) {
		return fmt.Errorf("%w: %s", appErrors.ErrThoughtNotFound, thoughtID)
	}

	s.NormalizeTree()
	s.UpdatedAt = time.Now().UTC()
	return nil
}

type SessionMetadata struct {
	TotalThoughts int      `json:"totalThoughts"`
	MaxDepth      int      `json:"maxDepth"`
	Directions    []string `json:"directions"`
}

// 方法
func NewSession(userID, initialConcept string) *Session {
	sessionID := uuid.NewString()
	now := time.Now().UTC()
	direction := Direction{
		Type:        Broad,
		Title:       "Root",
		Description: "Initial concept",
	}

	rootThought := NewThought(initialConcept, sessionID, direction)

	return &Session{
		ID:          sessionID,
		UserID:      userID,
		RootThought: rootThought,
		Context:     []string{initialConcept},
		CreatedAt:   now,
		UpdatedAt:   now,
		IsActive:    true,
	}
}

func (s *Session) AddContext(context string) {
	if s == nil || context == "" {
		return
	}

	s.Context = append(s.Context, context)
	s.UpdatedAt = time.Now().UTC()
}

func (s *Session) GetMetadata() *SessionMetadata {
	if s == nil || s.RootThought == nil {
		return &SessionMetadata{}
	}

	total := 0
	maxDepth := 0
	directionSet := map[string]struct{}{}

	queue := []*Thought{s.RootThought}
	for len(queue) > 0 {
		thought := queue[0]
		queue = queue[1:]

		total++
		if thought.Depth > maxDepth {
			maxDepth = thought.Depth
		}
		key := thought.Direction.Title
		if key == "" {
			key = string(thought.Direction.Type)
		}
		if key != "" {
			directionSet[key] = struct{}{}
		}

		for _, child := range thought.Children {
			if child != nil {
				queue = append(queue, child)
			}
		}
	}

	directions := make([]string, 0, len(directionSet))
	for key := range directionSet {
		directions = append(directions, key)
	}
	sort.Strings(directions)

	return &SessionMetadata{
		TotalThoughts: total,
		MaxDepth:      maxDepth,
		Directions:    directions,
	}
}

func (s *Session) Close() {
	if s == nil {
		return
	}

	s.IsActive = false
	s.UpdatedAt = time.Now().UTC()
}

func (s *Session) GetThoughtTree() map[string]*Thought {
	if s == nil || s.RootThought == nil {
		return map[string]*Thought{}
	}

	tree := make(map[string]*Thought)
	queue := []*Thought{s.RootThought}
	for len(queue) > 0 {
		thought := queue[0]
		queue = queue[1:]

		tree[thought.ID] = thought
		for _, child := range thought.Children {
			if child != nil {
				queue = append(queue, child)
			}
		}
	}

	return tree
}
