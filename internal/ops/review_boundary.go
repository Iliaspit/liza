package ops

import (
	"fmt"
	"os"
	"time"

	lizaerrors "github.com/liza-mas/liza/internal/errors"
	gitpkg "github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
)

const (
	reviewBoundaryOperationAssignment = "review-assignment"
)

// validateReviewBoundaryForAssignment verifies that assigning a reviewer would
// point them at the same commit currently checked out in the task worktree.
func validateReviewBoundaryForAssignment(projectRoot string, task *models.Task) error {
	if task.ReviewCommit == nil {
		return &PreconditionError{Reason: fmt.Sprintf("task %s has no review_commit — cannot assign for review", task.ID)}
	}
	return validateReviewBoundaryCommit(projectRoot, task, *task.ReviewCommit)
}

func validateReviewBoundaryCommit(projectRoot string, task *models.Task, reviewCommit string) error {
	if task.Worktree == nil {
		return nil
	}

	g := gitpkg.New(projectRoot)
	wtPath := g.GetWorktreePath(task.ID)
	if _, err := os.Stat(wtPath); err != nil {
		if os.IsNotExist(err) {
			return &lizaerrors.WorktreeContextError{
				Operation: reviewBoundaryOperationAssignment,
				TaskID:    task.ID,
				Reason:    "task has recorded worktree but worktree directory does not exist",
				Err:       err,
			}
		}
		return &OperationalError{
			Code:    "worktree_context",
			Phase:   "review-boundary",
			Message: "failed to stat task worktree",
			Details: map[string]any{
				"operation":     reviewBoundaryOperationAssignment,
				"task_id":       task.ID,
				"recovery_hint": "Inspect the task worktree path and filesystem permissions, then retry the review operation.",
			},
			Err: err,
		}
	}

	wtHEAD, err := g.GetWorktreeHEAD(task.ID)
	if err != nil {
		return &OperationalError{
			Code:    "git_operation",
			Phase:   "review-boundary",
			Message: "failed to get task worktree HEAD",
			Details: map[string]any{
				"operation":     reviewBoundaryOperationAssignment,
				"task_id":       task.ID,
				"recovery_hint": "Inspect the task worktree git metadata, ensure HEAD resolves, then retry the review operation.",
			},
			Err: err,
		}
	}
	if reviewCommit != wtHEAD {
		return &PreconditionError{Reason: fmt.Sprintf("review_commit %s does not match worktree HEAD %s — cannot assign task %s for review", reviewCommit, wtHEAD, task.ID)}
	}
	return nil
}

func markReviewBoundaryIntegrationFailed(state *models.State, task *models.Task, agentID string, transitions map[models.TaskStatus][]models.TaskStatus, cause error) error {
	reason := IntegrationReasonReviewBoundaryMismatch
	if err := task.TransitionWith(models.TaskStatusIntegrationFailed, transitions); err != nil {
		return err
	}

	if task.ReviewingBy != nil {
		state.ReleaseAgent(*task.ReviewingBy)
	}
	if task.AssignedTo != nil {
		assignedAgent := *task.AssignedTo
		if agent, ok := state.Agents[assignedAgent]; ok && agent.CurrentTask != nil && *agent.CurrentTask == task.ID {
			state.ReleaseAgent(assignedAgent)
		}
	}

	task.FailedBy = appendUniqueAgentID(task.FailedBy, agentID)
	task.IntegrationFix = false
	task.AssignedTo = nil
	task.LeaseExpires = nil
	task.ReviewingBy = nil
	task.ReviewLeaseExpires = nil

	diagnostic := map[string]any{
		"operation":     reviewBoundaryOperationAssignment,
		"reason":        reason,
		"detail":        cause.Error(),
		"recovery_hint": "claim the task for integration fix, verify the worktree HEAD, then resubmit for review",
	}
	task.IntegrationFailure = cloneMapForTaskDiagnostic(diagnostic)

	now := time.Now().UTC()
	task.History = append(task.History, models.TaskHistoryEntry{
		Time:   now,
		Event:  models.TaskEventIntegrationFailed,
		Agent:  &agentID,
		Reason: &reason,
		Extra: map[string]any{
			"diagnostic": diagnostic,
		},
	})
	return nil
}
