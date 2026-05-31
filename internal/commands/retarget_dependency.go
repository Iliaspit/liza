package commands

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/liza-mas/liza/internal/ops"
)

// RetargetDependencyCommand retargets one task dependency edge and prints the
// result to stdout.
func RetargetDependencyCommand(projectRoot, taskID, oldDependency string, newDependencies []string, reason, agentID string) error {
	result, err := ops.RetargetDependency(projectRoot, taskID, oldDependency, newDependencies, reason, agentID)
	if err != nil {
		return fmt.Errorf("retarget dependency: %w", err)
	}

	fmt.Printf("Retargeted dependency for %s: %s -> %s\n",
		result.TaskID, result.OldDependency, strings.Join(result.NewDependencies, ", "))
	if !slices.Equal(result.NewDependencies, result.CanonicalDependencies) {
		fmt.Printf("Canonical dependencies: %s\n", strings.Join(result.CanonicalDependencies, ", "))
	}
	if result.RepairRequestCleared {
		fmt.Println("Cleared matching repair request")
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}
	return nil
}
