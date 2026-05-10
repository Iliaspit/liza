package errors

import (
	"errors"
	"fmt"
)

// Exit codes used by liza commands
const (
	ExitSuccess  = 0  // Success
	ExitError    = 1  // General error
	ExitLock     = 2  // Lock timeout
	ExitNotFound = 3  // Entity or field not found
	ExitRestart  = 42 // Agent restart request
)

// NotFoundError represents an error where an entity or field was not found
type NotFoundError struct {
	Entity string // "task", "agent", "config"
	ID     string // optional: "task-42", "orchestrator-1"
	Field  string // optional: field name
}

func (e *NotFoundError) Error() string {
	base := e.Entity
	if e.ID != "" {
		base += " " + e.ID
	}
	if e.Field != "" {
		return fmt.Sprintf("%s field '%s' not found", base, e.Field)
	}
	if e.ID != "" {
		return fmt.Sprintf("%s not found: %s", e.Entity, e.ID)
	}
	return fmt.Sprintf("%s not found", e.Entity)
}

// IsNotFound checks if an error is a NotFoundError.
// Supports wrapped errors (e.g. from bb.Modify).
func IsNotFound(err error) bool {
	var nfe *NotFoundError
	return errors.As(err, &nfe)
}

// AgentCollisionError is returned when an agent ID is already registered
// with a valid lease.
type AgentCollisionError struct {
	AgentID string
}

func (e *AgentCollisionError) Error() string {
	return fmt.Sprintf("agent ID collision: %s already registered with valid lease", e.AgentID)
}

// IsAgentCollision checks if an error is an AgentCollisionError.
// Supports wrapped errors (e.g. from bb.Modify).
func IsAgentCollision(err error) bool {
	var ace *AgentCollisionError
	return errors.As(err, &ace)
}

// ValidationError represents a client input validation failure (e.g. missing
// or invalid field values). Mapped to HTTP 400 by the API layer.
type ValidationError struct {
	Message string
	Err     error
}

func (e *ValidationError) Error() string { return e.Message }

func (e *ValidationError) Unwrap() error { return e.Err }
