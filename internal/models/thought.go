//Thought Node(思维节点)

package models

import (
	"time"

	"github.com/google/uuid"
)

// 结构体
type Thought struct {
	ID        string
	Content   string
	ParentID  *string
	SessionID string
	Direction Direction
	Depth     int
	CreatedAt time.Time
	Children  []*Thought
	Path      []string
	parent    *Thought
}

// 方法
func NewThought(content, sessionID string, direction Direction) *Thought {
	now := time.Now().UTC()
	thought := &Thought{
		ID:        uuid.NewString(),
		Content:   content,
		SessionID: sessionID,
		Direction: direction,
		Depth:     0,
		CreatedAt: now,
		Children:  make([]*Thought, 0),
		Path:      []string{content},
	}
	return thought
}

func (t *Thought) AddChild(child *Thought) {
	if t == nil || child == nil {
		return
	}

	child.ParentID = &t.ID
	child.parent = t
	child.Depth = t.Depth + 1
	if len(t.Path) > 0 {
		child.Path = append(append([]string{}, t.Path...), child.Content)
	} else {
		child.Path = []string{child.Content}
	}
	if child.CreatedAt.IsZero() {
		child.CreatedAt = time.Now().UTC()
	}

	t.Children = append(t.Children, child)
}

func (t *Thought) GetPath() []string {
	if t == nil {
		return nil
	}

	if len(t.Path) > 0 {
		return append([]string(nil), t.Path...)
	}

	path := []string{t.Content}
	current := t.parent
	for current != nil {
		path = append([]string{current.Content}, path...)
		current = current.parent
	}

	return path
}

func (t *Thought) IsRoot() bool {
	if t == nil {
		return false
	}
	return t.ParentID == nil
}
