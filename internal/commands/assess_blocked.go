package commands

import (
	"fmt"
	"os"

	"github.com/liza-mas/liza/internal/ops"
)

// AssessBlockedCommand records an orchestrator assessment of a BLOCKED task.
// Delegates business logic to ops.AssessBlocked.
func AssessBlockedCommand(projectRoot, taskID, note, agentID string) error {
	result, err := ops.AssessBlocked(projectRoot, taskID, note, agentID)
	if err != nil {
		return fmt.Errorf("assess blocked: %w", err)
	}

	fmt.Printf("Task %s assessed by orchestrator\n", result.TaskID)
	for _, warning := range result.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}
	return nil
}
