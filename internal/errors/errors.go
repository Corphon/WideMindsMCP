package errors

import "errors"

var (
	// ErrSessionNotFound indicates the requested session was not found in storage.
	ErrSessionNotFound = errors.New("session not found")

	// ErrToolNotFound indicates an MCP tool lookup failed.
	ErrToolNotFound = errors.New("mcp tool not found")

	// ErrInvalidRequest indicates the request payload failed validation.
	ErrInvalidRequest = errors.New("invalid request")
)
