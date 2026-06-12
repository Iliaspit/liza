package ops

import (
	"fmt"
	"log"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// TransitionAttemptResult contains the outcome of transitioning to a new attempt.
type TransitionAttemptResult struct {
	TaskID          string
	NewAttempt      int
	WorktreeDeleted bool
	InitialStatus   models.TaskStatus
}

// transitioning is the sentinel value for AssignedTo during attempt transition.
const transitioning = "$transitioning"

// transitionTestHooks provides injection points for testing intermediate states
// in TransitionToNewAttempt. Nil in production — zero overhead.
type transitionTestHooks struct {
	// afterPhase1 is called after Phase 1 commits to the blackboard,
	// before Phase 2 git operations. Use to inspect intermediate state.
	afterPhase1 func()
}

// testTransitionHooks is nil in production. Tests set it to observe/inject
// behavior between phases.
var testTransitionHooks *transitionTestHooks

// TransitionToNewAttempt implements the 3-phase attempt boundary operation.
//
// Phase 1 (bb.Modify): Set Attempt=2, reset counters, set sentinel, append
// history, release agent. Preserves Status, Worktree, BaseCommit, RejectionReason.
//
// Phase 2 (git ops, outside lock): Delete worktree and branch best-effort.
//
// Phase 3 (bb.Modify): Re-check sentinel, clear AssignedTo/RejectionReason/
// Worktree/BaseCommit, transition to initial pipeline status.
func TransitionToNewAttempt(projectRoot, taskID, reason string) (*TransitionAttemptResult, error) {
	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())

	pb, err := loadPipelineBundle(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to load pipeline config: %w", err)
	}

	var (
		worktreePath   string
		previousAgent  string
		originalStatus models.TaskStatus
		initialStatus  models.TaskStatus
	)

	// Phase 1: mark attempt boundary, block claims via sentinel.
	err = bb.ModifyOp("transition_attempt", func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return &errors.NotFoundError{Entity: "task", ID: taskID}
		}

		if task.EffectiveAttempt() != 1 {
			return &PreconditionError{
				Reason: fmt.Sprintf("task %s is on attempt %d, only attempt 1 can transition", taskID, task.EffectiveAttempt()),
			}
		}

		// Resolve initial status for the task's role-pair.
		var resolveErr error
		initialStatus, resolveErr = pb.pr.InitialStatus(task.RolePair)
		if resolveErr != nil {
			return fmt.Errorf("failed to resolve initial status for role-pair %s: %w", task.RolePair, resolveErr)
		}

		// Capture fields for Phase 2 and Phase 3.
		originalStatus = task.Status
		if task.Worktree != nil {
			worktreePath = *task.Worktree
		}
		if task.AssignedTo != nil {
			previousAgent = *task.AssignedTo
		}

		// Capture rejection feedback before mutations for history preservation.
		var rejectionNote *string
		if task.RejectionReason != nil && *task.RejectionReason != "" {
			rr := *task.RejectionReason
			rejectionNote = &rr
		}

		// Mutations.
		task.Attempt = 2
		task.Iteration = 0
		task.ReviewCyclesCurrent = 0
		task.LeaseExpires = nil

		sentinel := transitioning
		task.AssignedTo = &sentinel
		// The sentinel is not an agent — the previous doer's claim ends here.
		releaseDoerClaimRecord(state, task.ID)

		now := time.Now().UTC()
		task.History = append(task.History, models.TaskHistoryEntry{
			Time:   now,
			Event:  models.TaskEventNewAttempt,
			Reason: &reason,
			Note:   rejectionNote,
		})

		if previousAgent != "" {
			state.ReleaseAgent(previousAgent)
		}

		// Preserved: Status, Worktree, BaseCommit, RejectionReason.
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Test hook: inspect intermediate state after Phase 1.
	if testTransitionHooks != nil && testTransitionHooks.afterPhase1 != nil {
		testTransitionHooks.afterPhase1()
	}

	// Phase 2: delete worktree best-effort (outside lock).
	worktreeDeleted := false
	if worktreePath != "" {
		gw := git.New(projectRoot)
		if rmErr := gw.RemoveWorktree(taskID); rmErr != nil {
			log.Printf("WARNING: failed to remove worktree for task %s: %v", taskID, rmErr)
		} else {
			worktreeDeleted = true
		}
	}

	// Add attempt-boundary transition: original status → initial status.
	// This transition is specific to the attempt boundary and not part of
	// the standard pipeline transitions (e.g. REJECTED → initial).
	pb.transitions[originalStatus] = append(pb.transitions[originalStatus], initialStatus)

	// Phase 3: release sentinel, make task claimable.
	err = bb.ModifyOp("transition_attempt", func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return &errors.NotFoundError{Entity: "task", ID: taskID}
		}

		// Re-check sentinel — concurrent modification detection.
		if task.AssignedTo == nil || *task.AssignedTo != transitioning {
			actual := "<nil>"
			if task.AssignedTo != nil {
				actual = *task.AssignedTo
			}
			return fmt.Errorf("sentinel replaced: expected %s, got %s (concurrent modification)", transitioning, actual)
		}

		// Release reviewer agent before clearing task fields (same transaction).
		if task.ReviewingBy != nil {
			state.ReleaseAgent(*task.ReviewingBy)
		}

		task.AssignedTo = nil
		task.RejectionReason = nil
		task.Worktree = nil
		task.BaseCommit = nil
		task.ReviewingBy = nil
		task.ReviewLeaseExpires = nil
		releaseDoerClaimRecord(state, task.ID)
		releaseReviewerClaimRecord(state, task.ID)
		clearAttemptState(task, attemptStateInitialReset)

		return task.TransitionWith(initialStatus, pb.transitions)
	})
	if err != nil {
		return nil, fmt.Errorf("phase 3 failed: %w", err)
	}

	return &TransitionAttemptResult{
		TaskID:          taskID,
		NewAttempt:      2,
		WorktreeDeleted: worktreeDeleted,
		InitialStatus:   initialStatus,
	}, nil
}
