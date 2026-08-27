package commands

import (
	"fmt"
	"os"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
)

// AddTasksCommand adds multiple tasks in batch and prints results.
// Delegates business logic to ops.AddTasks.
func AddTasksCommand(statePath, logPath string, input *ops.AddTasksInput) error {
	result, err := ops.AddTasks(statePath, logPath, input)
	return printAddTasksResult(result, err)
}

// AddTasksWithAuthorityCommand adds tasks using generation-fenced authority.
func AddTasksWithAuthorityCommand(statePath, logPath string, input *ops.AddTasksInput, authority models.AgentAuthority) error {
	result, err := ops.AddTasksWithAuthority(statePath, logPath, input, authority)
	return printAddTasksResult(result, err)
}

func printAddTasksResult(result *ops.AddTasksResult, err error) error {
	if err != nil {
		return fmt.Errorf("add tasks: %w", err)
	}

	for _, r := range result.Results {
		if r.Success {
			fmt.Printf("Added task %s\n", r.TaskID)
			for _, warning := range r.Warnings {
				fmt.Fprintf(os.Stderr, "warning: task %s: %s\n", r.TaskID, warning)
			}
		} else {
			fmt.Printf("Failed task %s: %s\n", r.TaskID, r.Error)
		}
	}
	return nil
}
