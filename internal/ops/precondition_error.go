package ops

import "fmt"

// PreconditionError indicates a precondition check failed before the main
// operation could execute. The Reason field is intentionally client-facing
// so that agents receive actionable feedback.
type PreconditionError struct {
	Reason  string
	Details map[string]any
}

func (e *PreconditionError) Error() string {
	return fmt.Sprintf("validation failed: %s", e.Reason)
}

func (e *PreconditionError) SafeDetails() map[string]any {
	return e.Details
}
