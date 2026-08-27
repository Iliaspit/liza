package commands

import (
	"fmt"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
)

// WriteCheckpointCommand writes a pre-execution checkpoint to a task's history.
// Delegates business logic to ops.WriteCheckpoint.
func WriteCheckpointCommand(projectRoot string, input *ops.WriteCheckpointInput) error {
	if err := ops.WriteCheckpoint(projectRoot, input); err != nil {
		return fmt.Errorf("write checkpoint: %w", err)
	}

	fmt.Printf("Pre-execution checkpoint written for task %s\n", input.TaskID)
	return nil
}

// WriteCheckpointWithAuthorityCommand writes a generation-fenced checkpoint.
func WriteCheckpointWithAuthorityCommand(projectRoot string, input *ops.WriteCheckpointInput, authority models.AgentAuthority) error {
	if err := ops.WriteCheckpointWithAuthority(projectRoot, input, authority); err != nil {
		return fmt.Errorf("write checkpoint: %w", err)
	}

	fmt.Printf("Pre-execution checkpoint written for task %s\n", input.TaskID)
	return nil
}
