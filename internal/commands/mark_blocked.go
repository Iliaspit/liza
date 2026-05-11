package commands

import (
	"fmt"

	"github.com/liza-mas/liza/internal/ops"
)

// MarkBlockedCommand marks a task as BLOCKED and prints the result to stdout.
// Delegates business logic to ops.MarkBlocked.
func MarkBlockedCommand(projectRoot, taskID, reason string, questions []string, agentID string) error {
	return MarkBlockedWithOptionsCommand(projectRoot, taskID, reason, questions, agentID, ops.MarkBlockedOptions{})
}

// MarkBlockedWithOptionsCommand marks a task as BLOCKED with optional structured
// metadata and prints the result to stdout.
func MarkBlockedWithOptionsCommand(projectRoot, taskID, reason string, questions []string, agentID string, opts ops.MarkBlockedOptions) error {
	result, err := ops.MarkBlockedWithOptions(projectRoot, taskID, reason, questions, agentID, opts)
	if err != nil {
		return fmt.Errorf("mark blocked: %w", err)
	}

	fmt.Printf("Task %s marked as BLOCKED\nReason: %s\n", result.TaskID, result.Reason)
	if result.RepairRequest != nil {
		fmt.Printf("Repair request: %s\n", result.RepairRequest.Operation)
	}
	return nil
}
