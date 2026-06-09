package commands

import (
	"fmt"
	"strings"

	"github.com/liza-mas/liza/internal/ops"
)

// SupersedeTaskCommand marks a task as SUPERSEDED and prints the result to stdout.
// Delegates business logic to ops.SupersedeTask.
func SupersedeTaskCommand(projectRoot, taskID string, replacementIDs []string, reason, agentID, recoverabilityCommand string) error {
	result, err := ops.SupersedeTaskWithOptions(projectRoot, taskID, replacementIDs, reason, agentID, ops.SupersedeTaskOptions{
		RecoverabilityCommand: recoverabilityCommand,
	})
	if err != nil {
		return fmt.Errorf("supersede task: %w", err)
	}

	if len(result.ReplacementIDs) > 0 {
		fmt.Printf("Superseded task %s (was %s) with replacements: %s\n",
			result.TaskID, result.OriginalStatus, strings.Join(result.ReplacementIDs, ", "))
	} else {
		fmt.Printf("Superseded task %s (was %s) without replacements\n",
			result.TaskID, result.OriginalStatus)
	}
	return nil
}
