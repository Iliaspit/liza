package commands

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
)

// ClaimTaskCommand claims a task and prints the result to stdout.
// Delegates business logic to ops.ClaimTask.
func ClaimTaskCommand(projectRoot, taskID, agentID string) error {
	result, err := ops.ClaimTask(projectRoot, taskID, agentID)
	if err != nil {
		return err
	}

	printClaimResult(result)
	return nil
}

// ClaimTaskWithAuthorityCommand claims a task using generation-fenced authority.
func ClaimTaskWithAuthorityCommand(projectRoot, taskID string, authority models.AgentAuthority) error {
	result, err := ops.ClaimTaskWithAuthority(projectRoot, taskID, authority)
	if err != nil {
		return err
	}

	printClaimResult(result)
	return nil
}

func printClaimResult(r *ops.ClaimResult) {
	switch r.SourceStatus {
	case models.TaskStatusRejected:
		if r.PreviousAssignee == r.AgentID {
			fmt.Println("Same coder re-claiming REJECTED task, preserving worktree")
		} else {
			fmt.Println("Different coder claiming REJECTED task, recreating worktree")
		}
	case models.TaskStatusIntegrationFailed:
		fmt.Println("Claiming INTEGRATION_FAILED task, preserving worktree for conflict resolution")
	}

	fmt.Printf("IMPLEMENTING: %s by %s (from %s)\n", r.TaskID, r.AgentID, r.SourceStatus)
	fmt.Printf("  worktree: %s\n", r.WorktreeRel)
	fmt.Printf("  base_commit: %s\n", r.BaseCommit)
	fmt.Printf("  lease_expires: %s\n", r.LeaseExpires.Format(time.RFC3339))
	if r.IntegrationFix {
		fmt.Println("  integration_fix: true")
	}
	if r.SourceStatus == models.TaskStatusRejected && r.PreviousAssignee != r.AgentID && r.PreviousAssignee != "" {
		if r.WorktreeRecreated {
			fmt.Printf("  previous_assignee: %s (worktree recreated fresh)\n", r.PreviousAssignee)
		} else {
			fmt.Printf("  previous_assignee: %s\n", r.PreviousAssignee)
		}
	}
	for _, w := range r.Warnings {
		fmt.Printf("  WARNING: %s\n", w)
	}
}
