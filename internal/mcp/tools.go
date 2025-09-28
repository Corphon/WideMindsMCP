//Thought Expansion Tools(思维扩散工具)

package mcp

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

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

const (
	maxConceptLength        = 200
	maxContextItems         = 20
	maxContextItemLength    = 120
	maxDirectionTitleLength = 120
	maxDirectionDescLength  = 600
	maxKeywordLength        = 50
	maxDirectionKeywords    = 16
	maxGeneratedDirections  = 12
	maxUserIDLength         = 64
	maxSessionIDLength      = 64
)

var allowedDirectionTypes = map[models.DirectionType]struct{}{
	models.Broad:    {},
	models.Deep:     {},
	models.Lateral:  {},
	models.Critical: {},
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

	concept := strings.TrimSpace(getString(params, "concept"))
	if err := validateConcept(concept); err != nil {
		return nil, err
	}

	contextSlice := getStringSlice(params, "context")
	normalizedContext, err := normalizeContext(contextSlice)
	if err != nil {
		return nil, err
	}

	expansionTypeRaw := strings.ToLower(strings.TrimSpace(getString(params, "expansion_type")))
	var expansionType models.DirectionType
	if expansionTypeRaw != "" {
		expansionType = models.DirectionType(expansionTypeRaw)
		if _, ok := allowedDirectionTypes[expansionType]; !ok {
			return nil, validationError("expansion_type is invalid")
		}
	}

	maxDirections := getInt(params, "max_directions", 4)
	if maxDirections <= 0 {
		maxDirections = 4
	}
	if maxDirections > maxGeneratedDirections {
		return nil, validationError("max_directions is too large")
	}

	result, err := t.expander.Expand(&services.ExpansionRequest{
		Concept:       concept,
		Context:       normalizedContext,
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

	sessionID := strings.TrimSpace(getString(params, "session_id"))
	if err := validateSessionID(sessionID); err != nil {
		return nil, err
	}

	directionMap, ok := params["direction"].(map[string]interface{})
	if !ok {
		return nil, validationError("direction payload is required")
	}

	direction, err := buildDirection(directionMap)
	if err != nil {
		return nil, err
	}

	thought, err := t.expander.ExploreDirection(*direction, sessionID)
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

	userID := strings.TrimSpace(getString(params, "user_id"))
	concept := strings.TrimSpace(getString(params, "concept"))
	if err := validateUserID(userID); err != nil {
		return nil, err
	}
	if err := validateConcept(concept); err != nil {
		return nil, err
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

	sessionID := strings.TrimSpace(getString(params, "session_id"))
	if err := validateSessionID(sessionID); err != nil {
		return nil, err
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

func normalizeContext(items []string) ([]string, error) {
	if len(items) > maxContextItems {
		return nil, validationError("context has too many entries")
	}
	normalized := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if utf8.RuneCountInString(trimmed) > maxContextItemLength {
			return nil, validationError("context item is too long")
		}
		normalized = append(normalized, trimmed)
	}
	return normalized, nil
}

func normalizeKeywords(items []string) ([]string, error) {
	cleaned := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if utf8.RuneCountInString(trimmed) > maxKeywordLength {
			return nil, validationError("direction.keywords contains an entry that is too long")
		}
		cleaned = append(cleaned, trimmed)
		if len(cleaned) > maxDirectionKeywords {
			return nil, validationError("direction.keywords has too many entries")
		}
	}
	return cleaned, nil
}

func validateConcept(concept string) error {
	if concept == "" {
		return validationError("concept is required")
	}
	if utf8.RuneCountInString(concept) > maxConceptLength {
		return validationError("concept is too long")
	}
	return nil
}

func validateUserID(userID string) error {
	if userID == "" {
		return nil
	}
	if strings.ContainsAny(userID, " \t\r\n") {
		return validationError("user_id must not contain whitespace")
	}
	if utf8.RuneCountInString(userID) > maxUserIDLength {
		return validationError("user_id is too long")
	}
	return nil
}

func validateSessionID(sessionID string) error {
	if sessionID == "" {
		return validationError("session_id is required")
	}
	if strings.ContainsAny(sessionID, " \t\r\n") {
		return validationError("session_id must not contain whitespace")
	}
	if utf8.RuneCountInString(sessionID) > maxSessionIDLength {
		return validationError("session_id is too long")
	}
	return nil
}

func buildDirection(payload map[string]interface{}) (*models.Direction, error) {
	if payload == nil {
		return nil, validationError("direction payload is required")
	}

	dirTypeStr := strings.ToLower(strings.TrimSpace(getString(payload, "type")))
	if dirTypeStr == "" {
		return nil, validationError("direction.type is required")
	}
	dirType := models.DirectionType(dirTypeStr)
	if _, ok := allowedDirectionTypes[dirType]; !ok {
		return nil, validationError("direction.type is invalid")
	}

	title := strings.TrimSpace(getString(payload, "title"))
	if title == "" {
		return nil, validationError("direction.title is required")
	}
	if utf8.RuneCountInString(title) > maxDirectionTitleLength {
		return nil, validationError("direction.title is too long")
	}

	description := strings.TrimSpace(getString(payload, "description"))
	if utf8.RuneCountInString(description) > maxDirectionDescLength {
		return nil, validationError("direction.description is too long")
	}

	keywords, err := normalizeKeywords(getStringSlice(payload, "keywords"))
	if err != nil {
		return nil, err
	}

	relevance := getFloat(payload, "relevance", 0.5)
	if relevance < 0 || relevance > 1 {
		return nil, validationError("direction.relevance must be between 0 and 1")
	}

	return &models.Direction{
		Type:        dirType,
		Title:       title,
		Description: description,
		Keywords:    keywords,
		Relevance:   relevance,
	}, nil
}

func validationError(msg string) error {
	return fmt.Errorf("%w: %s", appErrors.ErrInvalidRequest, msg)
}
