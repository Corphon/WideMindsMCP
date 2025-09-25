//Thought Session(思维会话)

package models

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

// 结构体
type Session struct {
	ID          string
	UserID      string
	RootThought *Thought
	Context     []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	IsActive    bool
}

type SessionMetadata struct {
	TotalThoughts int
	MaxDepth      int
	Directions    []string
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
