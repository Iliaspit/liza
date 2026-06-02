package commands

import (
	"fmt"

	"github.com/liza-mas/liza/internal/ops"
)

// UnblockTaskCommand restores a repaired BLOCKED task to its executing status.
func UnblockTaskCommand(projectRoot, taskID, assignTo, reason, agentID string) error {
	return UnblockTaskWithOptionsCommand(projectRoot, taskID, reason, agentID, ops.UnblockTaskOptions{AssignTo: assignTo})
}

// UnblockTaskWithOptionsCommand restores a repaired BLOCKED task.
func UnblockTaskWithOptionsCommand(projectRoot, taskID, reason, agentID string, opts ops.UnblockTaskOptions) error {
	result, err := ops.UnblockTaskWithOptions(projectRoot, taskID, reason, agentID, opts)
	if err != nil {
		return fmt.Errorf("unblock task: %w", err)
	}

	if result.AssignedTo != "" {
		fmt.Printf("Task %s unblocked: %s -> %s, assigned to %s\n",
			result.TaskID, result.FromStatus, result.ToStatus, result.AssignedTo)
	} else {
		fmt.Printf("Task %s unblocked: %s -> %s, claimable\n",
			result.TaskID, result.FromStatus, result.ToStatus)
	}
	if result.Rebase != nil {
		fmt.Printf("Rebased %s: %s -> %s onto %s (%s)\n",
			result.TaskID,
			result.Rebase.OldHead,
			result.Rebase.NewHead,
			result.Rebase.TargetRef,
			result.Rebase.TargetSHA,
		)
	}
	return nil
}
