package ops

import "fmt"

// OperationalError indicates an infrastructure or environment failure during
// operation execution. Unlike PreconditionError (caller mistake) this represents
// "execution failed" scenarios (git failures, config loading issues, etc.).
//
// The Message field is safe to expose to agents via MCP — it contains actionable
// context without leaking internal paths or error chains. The Err field holds the
// underlying cause for logging/CLI but is NOT exposed over MCP.
type OperationalError struct {
	Message string         // safe-to-expose description (shown to agents via MCP)
	Details map[string]any // bounded, safe diagnostics for agent recovery
	Err     error          // underlying cause (NOT exposed to agents, only in logs/CLI)
}

func (e *OperationalError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *OperationalError) Unwrap() error {
	return e.Err
}

func (e *OperationalError) SafeDetails() map[string]any {
	return e.Details
}
