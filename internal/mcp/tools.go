//Thought Expansion Tools(思维扩散工具)

package mcp

import (
	"errors"
	"fmt"

	appErrors "WideMindsMCP/internal/errors"
	"WideMindsMCP/internal/models"
	"WideMindsMCP/internal/services"
)

// 接口
type MCPTool interface {
	Name() string
	Description() string
	Execute(params map[string]interface{}) (interface{}, error)
	Schema() map[string]interface{}
}

// 结构体
type ExpandThoughtTool struct {
	expander *services.ThoughtExpander
}

type ExploreDirectionTool struct {
	expander *services.ThoughtExpander
}

type CreateSessionTool struct {
	manager *services.SessionManager
}

type GetSessionTool struct {
	manager *services.SessionManager
}

// 函数
func NewExpandThoughtTool(expander *services.ThoughtExpander) MCPTool {
	return &ExpandThoughtTool{expander: expander}
}

func NewExploreDirectionTool(expander *services.ThoughtExpander) MCPTool {
	return &ExploreDirectionTool{expander: expander}
}

func NewCreateSessionTool(manager *services.SessionManager) MCPTool {
	return &CreateSessionTool{manager: manager}
}

func NewGetSessionTool(manager *services.SessionManager) MCPTool {
	return &GetSessionTool{manager: manager}
}

// ExpandThoughtTool方法
func (t *ExpandThoughtTool) Name() string {
	return "expand_thought"
}

func (t *ExpandThoughtTool) Description() string {
	return "Generate multiple directions of thought for a given concept"
}

func (t *ExpandThoughtTool) Execute(params map[string]interface{}) (interface{}, error) {
	if t.expander == nil {
		return nil, errors.New("thought expander not available")
	}

	concept := getString(params, "concept")
	if concept == "" {
		return nil, appErrors.ErrInvalidRequest
	}

	context := getStringSlice(params, "context")
	expansionType := models.DirectionType(getString(params, "expansion_type"))
	maxDirections := getInt(params, "max_directions", 4)

	result, err := t.expander.Expand(&services.ExpansionRequest{
		Concept:       concept,
		Context:       context,
		ExpansionType: expansionType,
		MaxDirections: maxDirections,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (t *ExpandThoughtTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"concept":        "string",
		"context":        "array[string]",
		"expansion_type": "enum[broad,deep,lateral,critical]",
		"max_directions": "number",
	}
}

// ExploreDirectionTool方法
func (t *ExploreDirectionTool) Name() string {
	return "explore_direction"
}

func (t *ExploreDirectionTool) Description() string {
	return "Deeply explore a selected direction within an existing session"
}

func (t *ExploreDirectionTool) Execute(params map[string]interface{}) (interface{}, error) {
	if t.expander == nil {
		return nil, errors.New("thought expander not available")
	}

	sessionID := getString(params, "session_id")
	if sessionID == "" {
		return nil, appErrors.ErrInvalidRequest
	}

	directionMap, ok := params["direction"].(map[string]interface{})
	if !ok {
		return nil, appErrors.ErrInvalidRequest
	}

	direction := models.Direction{
		Type:        models.DirectionType(getString(directionMap, "type")),
		Title:       getString(directionMap, "title"),
		Description: getString(directionMap, "description"),
		Keywords:    getStringSlice(directionMap, "keywords"),
		Relevance:   getFloat(directionMap, "relevance", 0.5),
	}

	thought, err := t.expander.ExploreDirection(direction, sessionID)
	if err != nil {
		return nil, err
	}
	return thought, nil
}

func (t *ExploreDirectionTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"session_id": "string",
		"direction": map[string]interface{}{
			"type":        "string",
			"title":       "string",
			"description": "string",
			"keywords":    "array[string]",
			"relevance":   "number",
		},
	}
}

// CreateSessionTool方法
func (t *CreateSessionTool) Name() string {
	return "create_session"
}

func (t *CreateSessionTool) Description() string {
	return "Create a new thought session for a user"
}

func (t *CreateSessionTool) Execute(params map[string]interface{}) (interface{}, error) {
	if t.manager == nil {
		return nil, errors.New("session manager not available")
	}

	userID := getString(params, "user_id")
	concept := getString(params, "concept")
	if concept == "" {
		return nil, appErrors.ErrInvalidRequest
	}

	session, err := t.manager.CreateSession(userID, concept)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (t *CreateSessionTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"user_id": "string",
		"concept": "string",
	}
}

// GetSessionTool方法
func (t *GetSessionTool) Name() string {
	return "get_session"
}

func (t *GetSessionTool) Description() string {
	return "Retrieve an existing session by ID"
}

func (t *GetSessionTool) Execute(params map[string]interface{}) (interface{}, error) {
	if t.manager == nil {
		return nil, errors.New("session manager not available")
	}

	sessionID := getString(params, "session_id")
	if sessionID == "" {
		return nil, appErrors.ErrInvalidRequest
	}

	session, err := t.manager.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (t *GetSessionTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"session_id": "string",
	}
}

func getString(params map[string]interface{}, key string) string {
	if params == nil {
		return ""
	}
	if value, ok := params[key]; ok {
		switch v := value.(type) {
		case string:
			return v
		case fmt.Stringer:
			return v.String()
		}
	}
	return ""
}

func getStringSlice(params map[string]interface{}, key string) []string {
	if params == nil {
		return nil
	}
	value, ok := params[key]
	if !ok {
		return nil
	}

	var result []string
	switch v := value.(type) {
	case []string:
		result = append(result, v...)
	case []interface{}:
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
	}
	return result
}

func getInt(params map[string]interface{}, key string, fallback int) int {
	if params == nil {
		return fallback
	}
	value, ok := params[key]
	if !ok {
		return fallback
	}
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return fallback
}

func getFloat(params map[string]interface{}, key string, fallback float64) float64 {
	if params == nil {
		return fallback
	}
	value, ok := params[key]
	if !ok {
		return fallback
	}
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	}
	return fallback
}
