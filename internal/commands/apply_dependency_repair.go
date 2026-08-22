package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/liza-mas/liza/internal/ops"
)

// ApplyDependencyRepairCommand applies a blocked task's stored dependency
// repair and prints every committed canonical dependency list.
func ApplyDependencyRepairCommand(projectRoot, sourceTaskID, reason, agentID string) error {
	result, err := ops.ApplyDependencyRepair(projectRoot, sourceTaskID, reason, agentID)
	if err != nil {
		return fmt.Errorf("apply dependency repair: %w", err)
	}

	fmt.Printf("Applied dependency repair requested by %s\n", result.SourceTaskID)
	for _, update := range result.Updates {
		fmt.Printf("%s dependencies: %s\n", update.TaskID, strings.Join(update.CanonicalDependencies, ", "))
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}
	return nil
}
