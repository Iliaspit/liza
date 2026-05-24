package ops

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
)

type claimContext struct {
	taskID            string
	agentID           string
	taskStatus        models.TaskStatus
	targetStatus      models.TaskStatus
	worktreeDir       string
	worktreeRel       string
	integrationBranch string
	previousAssignee  string
	baseCommit        string
	leaseExpires      time.Time
}

type claimStrategy interface {
	validate(*models.Task, *models.State, string, string, *claimContext) error
	enforceIterationLimit() bool
	requiresDependencyRecheck() bool
	handleWorktree(*db.Blackboard, *git.Git, *claimContext) (claimWorktreePhaseResult, error)
	shouldRunPostWorktreeCmd(claimWorktreePhaseResult) bool
	mutateTask(*models.Task, *claimContext)
	historyEntry(time.Time, *claimContext) models.TaskHistoryEntry
}

type freshClaimStrategy struct{}

func (freshClaimStrategy) validate(task *models.Task, state *models.State, runtimeRole, doerRole string, ctx *claimContext) error {
	if runtimeRole != doerRole {
		return fmt.Errorf("task %s is %s (not claimable by %s)", task.ID, task.Status, runtimeRole)
	}
	if unmet := unmetDependencies(task, state); len(unmet) > 0 {
		return fmt.Errorf("task has unmet dependencies: %s", formatDependencyResults(unmet))
	}
	return nil
}

func (freshClaimStrategy) enforceIterationLimit() bool {
	return false
}

func (freshClaimStrategy) requiresDependencyRecheck() bool {
	return true
}

func (freshClaimStrategy) handleWorktree(
	bb *db.Blackboard,
	gitWrapper *git.Git,
	ctx *claimContext,
) (claimWorktreePhaseResult, error) {
	result := claimWorktreePhaseResult{}
	cleanupAllowed, err := readyClaimHasStaleResources(gitWrapper, ctx.taskID, ctx.worktreeDir)
	if err != nil {
		return result, err
	}
	if err := handleReadyClaimWorktree(
		bb,
		gitWrapper,
		ctx.taskID,
		ctx.taskStatus,
		ctx.integrationBranch,
		ctx.worktreeDir,
		ctx.worktreeRel,
		cleanupAllowed,
	); err != nil {
		return result, err
	}
	result.created = true
	return result, nil
}

func (freshClaimStrategy) shouldRunPostWorktreeCmd(phase claimWorktreePhaseResult) bool {
	return phase.created
}

func (freshClaimStrategy) mutateTask(task *models.Task, ctx *claimContext) {
	task.Worktree = &ctx.worktreeRel
	task.BaseCommit = &ctx.baseCommit
	if task.Attempt == 0 {
		task.Attempt = 1
	}
}

func (freshClaimStrategy) historyEntry(now time.Time, ctx *claimContext) models.TaskHistoryEntry {
	agentPtr := &ctx.agentID
	return models.TaskHistoryEntry{
		Time:  now,
		Event: models.TaskEventClaimed,
		Agent: agentPtr,
	}
}

type rejectedClaimStrategy struct{}

func (rejectedClaimStrategy) validate(task *models.Task, _ *models.State, runtimeRole, doerRole string, ctx *claimContext) error {
	if runtimeRole != doerRole {
		return fmt.Errorf("task %s is %s (not claimable by %s)", task.ID, task.Status, runtimeRole)
	}
	if task.AssignedTo != nil {
		ctx.previousAssignee = *task.AssignedTo
	}
	return nil
}

func (rejectedClaimStrategy) enforceIterationLimit() bool {
	return true
}

func (rejectedClaimStrategy) requiresDependencyRecheck() bool {
	return false
}

func (rejectedClaimStrategy) handleWorktree(
	_ *db.Blackboard,
	gitWrapper *git.Git,
	ctx *claimContext,
) (claimWorktreePhaseResult, error) {
	return ensureRejectedWorktreeExists(gitWrapper, ctx)
}

func (rejectedClaimStrategy) shouldRunPostWorktreeCmd(claimWorktreePhaseResult) bool {
	return true
}

func (rejectedClaimStrategy) mutateTask(_ *models.Task, _ *claimContext) {}

func (rejectedClaimStrategy) historyEntry(now time.Time, ctx *claimContext) models.TaskHistoryEntry {
	agentPtr := &ctx.agentID
	entry := models.TaskHistoryEntry{
		Time:  now,
		Agent: agentPtr,
	}
	if ctx.previousAssignee == ctx.agentID {
		entry.Event = models.TaskEventReclaimedAfterRejection
		return entry
	}

	entry.Event = models.TaskEventReassignedAfterRejection
	if ctx.previousAssignee != "" {
		entry.PreviousAssignee = &ctx.previousAssignee
	}
	return entry
}

type integrationFixClaimStrategy struct{}

func (integrationFixClaimStrategy) validate(task *models.Task, _ *models.State, runtimeRole, doerRole string, ctx *claimContext) error {
	if runtimeRole != doerRole {
		return fmt.Errorf("task %s is %s (not claimable by %s)", task.ID, task.Status, runtimeRole)
	}
	if task.AssignedTo != nil {
		ctx.previousAssignee = *task.AssignedTo
	}
	return nil
}

func (integrationFixClaimStrategy) enforceIterationLimit() bool {
	return false
}

func (integrationFixClaimStrategy) requiresDependencyRecheck() bool {
	return false
}

func (integrationFixClaimStrategy) handleWorktree(
	_ *db.Blackboard,
	_ *git.Git,
	ctx *claimContext,
) (claimWorktreePhaseResult, error) {
	result := claimWorktreePhaseResult{}
	if err := ensureIntegrationFailedWorktreeExists(ctx.worktreeDir, ctx.worktreeRel); err != nil {
		return result, err
	}
	return result, nil
}

func (integrationFixClaimStrategy) shouldRunPostWorktreeCmd(claimWorktreePhaseResult) bool {
	return true
}

func (integrationFixClaimStrategy) mutateTask(task *models.Task, _ *claimContext) {
	clearAttemptState(task, attemptStateIntegrationFixClaim)
	// FailedBy is audit/escalation history, not stale attempt state.
	task.IntegrationFix = true
}

func (integrationFixClaimStrategy) historyEntry(now time.Time, ctx *claimContext) models.TaskHistoryEntry {
	agentPtr := &ctx.agentID
	return models.TaskHistoryEntry{
		Time:  now,
		Event: models.TaskEventClaimedForIntegrationFix,
		Agent: agentPtr,
	}
}
