package utils

import "github.com/google/uuid"

// NewUUID returns a RFC 4122 compliant random UUID string.
func NewUUID() string {
	return uuid.NewString()
}
