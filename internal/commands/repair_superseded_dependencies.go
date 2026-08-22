package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/liza-mas/liza/internal/ops"
)

// RepairSupersededDependenciesCommand removes illegal downstream dependencies
// from one superseded task and prints the audited result.
func RepairSupersededDependenciesCommand(projectRoot, taskID, reason, agentID string) error {
	result, err := ops.RepairSupersededDependencies(projectRoot, taskID, reason, agentID)
	if err != nil {
		return fmt.Errorf("repair superseded dependencies: %w", err)
	}

	fmt.Printf("Repaired superseded dependencies for %s\n", result.TaskID)
	fmt.Printf("Removed dependencies: %s\n", strings.Join(result.RemovedDependencies, ", "))
	fmt.Printf("Retained dependencies: %s\n", strings.Join(result.RetainedDependencies, ", "))
	for _, warning := range result.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}
	return nil
}
