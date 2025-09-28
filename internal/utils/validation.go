package utils

import (
	"fmt"
	"strings"
	"unicode/utf8"

	appErrors "WideMindsMCP/internal/errors"
	"WideMindsMCP/internal/models"
)

const (
	MaxConceptLength        = 200
	MaxUserIDLength         = 64
	MaxSessionIDLength      = 64
	MaxContextItems         = 20
	MaxContextItemLength    = 120
	MaxDirectionTitleLength = 120
	MaxDirectionDescLength  = 600
	MaxKeywordLength        = 50
	MaxDirectionKeywords    = 16
	MaxThoughtContentLength = 400
)

var allowedDirectionTypes = map[models.DirectionType]struct{}{
	models.Broad:    {},
	models.Deep:     {},
	models.Lateral:  {},
	models.Critical: {},
}

// ValidationError wraps a message with ErrInvalidRequest for consistent reporting.
func ValidationError(message string) error {
	return fmt.Errorf("%w: %s", appErrors.ErrInvalidRequest, message)
}

// ParseDirectionType normalizes the input direction type and ensures it is supported.
func ParseDirectionType(value string) (models.DirectionType, error) {
	normalized := models.DirectionType(strings.ToLower(strings.TrimSpace(value)))
	if normalized == "" {
		return "", ValidationError("direction.type is required")
	}
	if _, ok := allowedDirectionTypes[normalized]; !ok {
		return "", ValidationError("direction.type is invalid")
	}
	return normalized, nil
}

// IsAllowedDirectionType reports whether the given type is supported.
func IsAllowedDirectionType(value models.DirectionType) bool {
	_, ok := allowedDirectionTypes[models.DirectionType(strings.ToLower(strings.TrimSpace(string(value))))]
	return ok
}

// ValidateConcept ensures the concept string is present and within limits.
func ValidateConcept(concept string) error {
	if strings.TrimSpace(concept) == "" {
		return ValidationError("concept is required")
	}
	if utf8.RuneCountInString(concept) > MaxConceptLength {
		return ValidationError("concept is too long")
	}
	return nil
}

// ValidateUserID enforces formatting and length constraints on user identifiers.
func ValidateUserID(userID string) error {
	if userID == "" {
		return nil
	}
	if strings.ContainsAny(userID, " \t\r\n") {
		return ValidationError("user_id must not contain whitespace")
	}
	if utf8.RuneCountInString(userID) > MaxUserIDLength {
		return ValidationError("user_id is too long")
	}
	return nil
}

// ValidateSessionID ensures the session identifier meets formatting expectations.
func ValidateSessionID(sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return ValidationError("session_id is required")
	}
	if strings.ContainsAny(sessionID, " \t\r\n") {
		return ValidationError("session_id must not contain whitespace")
	}
	if utf8.RuneCountInString(sessionID) > MaxSessionIDLength {
		return ValidationError("session_id is too long")
	}
	return nil
}

// NormalizeContext trims entries, removes empties, and enforces maximum counts/lengths.
func NormalizeContext(items []string) ([]string, error) {
	if len(items) > MaxContextItems {
		return nil, ValidationError("context has too many entries")
	}
	normalized := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if utf8.RuneCountInString(trimmed) > MaxContextItemLength {
			return nil, ValidationError("context item is too long")
		}
		normalized = append(normalized, trimmed)
	}
	return normalized, nil
}

// NormalizeKeywords enforces keyword limits and returns a cleaned slice.
func NormalizeKeywords(items []string) ([]string, error) {
	cleaned := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if utf8.RuneCountInString(trimmed) > MaxKeywordLength {
			return nil, ValidationError("direction.keywords contains an entry that is too long")
		}
		cleaned = append(cleaned, trimmed)
		if len(cleaned) > MaxDirectionKeywords {
			return nil, ValidationError("direction.keywords has too many entries")
		}
	}
	return cleaned, nil
}

// ValidateDirection normalizes and validates the provided direction.
func ValidateDirection(direction *models.Direction) error {
	if direction == nil {
		return ValidationError("direction is required")
	}
	parsedType, err := ParseDirectionType(string(direction.Type))
	if err != nil {
		return err
	}
	direction.Type = parsedType

	direction.Title = strings.TrimSpace(direction.Title)
	if direction.Title == "" {
		return ValidationError("direction.title is required")
	}
	if utf8.RuneCountInString(direction.Title) > MaxDirectionTitleLength {
		return ValidationError("direction.title is too long")
	}

	direction.Description = strings.TrimSpace(direction.Description)
	if utf8.RuneCountInString(direction.Description) > MaxDirectionDescLength {
		return ValidationError("direction.description is too long")
	}

	keywords, err := NormalizeKeywords(direction.Keywords)
	if err != nil {
		return err
	}
	direction.Keywords = keywords

	if direction.Relevance < 0 || direction.Relevance > 1 {
		return ValidationError("direction.relevance must be between 0 and 1")
	}

	return nil
}

func ValidateThoughtUpdate(update *models.ThoughtUpdate) error {
	if update == nil {
		return ValidationError("update payload is required")
	}

	if update.Content == nil && update.Direction == nil {
		return ValidationError("at least one field must be provided")
	}

	if update.Content != nil {
		trimmed := strings.TrimSpace(*update.Content)
		if trimmed == "" {
			return ValidationError("content must not be empty")
		}
		if utf8.RuneCountInString(trimmed) > MaxThoughtContentLength {
			return ValidationError("content is too long")
		}
		*update.Content = trimmed
	}

	if update.Direction != nil {
		if err := ValidateDirection(update.Direction); err != nil {
			return err
		}
	}

	return nil
}
