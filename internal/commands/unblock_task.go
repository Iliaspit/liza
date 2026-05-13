package commands

import (
	"fmt"

	"github.com/liza-mas/liza/internal/ops"
)

// UnblockTaskCommand restores a repaired BLOCKED task to its executing status.
func UnblockTaskCommand(projectRoot, taskID, assignTo, reason, agentID string) error {
	result, err := ops.UnblockTask(projectRoot, taskID, assignTo, reason, agentID)
	if err != nil {
		return fmt.Errorf("unblock task: %w", err)
	}

	fmt.Printf("Task %s unblocked: %s -> %s, assigned to %s\n",
		result.TaskID, result.FromStatus, result.ToStatus, result.AssignedTo)
	return nil
}
