package ops

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
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
	// worktreeRecreated is set after the worktree phase when the strategy
	// rebuilt the worktree from the current integration HEAD (rather than
	// preserving an existing one).
	worktreeRecreated bool
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

type preservedInitialClaimStrategy struct{}

func (preservedInitialClaimStrategy) validate(task *models.Task, state *models.State, runtimeRole, doerRole string, ctx *claimContext) error {
	if runtimeRole != doerRole {
		return fmt.Errorf("task %s is %s (not claimable by %s)", task.ID, task.Status, runtimeRole)
	}
	if task.Worktree == nil || *task.Worktree == "" {
		return &PreconditionError{Reason: fmt.Sprintf("task %s preserved claim requires worktree metadata", task.ID)}
	}
	if *task.Worktree != ctx.worktreeRel {
		return &PreconditionError{Reason: fmt.Sprintf("task %s worktree = %q, want %q", task.ID, *task.Worktree, ctx.worktreeRel)}
	}
	if task.BaseCommit == nil || *task.BaseCommit == "" {
		return &PreconditionError{Reason: fmt.Sprintf("task %s preserved claim requires base_commit", task.ID)}
	}
	if unmet := unmetDependencies(task, state); len(unmet) > 0 {
		return fmt.Errorf("task has unmet dependencies: %s", formatDependencyResults(unmet))
	}
	return nil
}

func (preservedInitialClaimStrategy) enforceIterationLimit() bool {
	return false
}

func (preservedInitialClaimStrategy) requiresDependencyRecheck() bool {
	return true
}

func (preservedInitialClaimStrategy) handleWorktree(
	_ *db.Blackboard,
	gitWrapper *git.Git,
	ctx *claimContext,
) (claimWorktreePhaseResult, error) {
	result := claimWorktreePhaseResult{}
	if err := gitWrapper.ValidateWorktreeHealth(ctx.taskID); err != nil {
		return result, &PreconditionError{Reason: fmt.Sprintf("preserved worktree not healthy: %v", err)}
	}
	branch, err := gitWrapper.GetWorktreeBranch(ctx.worktreeDir)
	if err != nil {
		return result, err
	}
	expectedBranch := paths.TaskBranchPrefix + ctx.taskID
	if branch != expectedBranch {
		return result, &PreconditionError{Reason: fmt.Sprintf("preserved worktree branch = %q, want %q", branch, expectedBranch)}
	}
	head, err := gitWrapper.GetWorktreeHEAD(ctx.taskID)
	if err != nil {
		return result, err
	}
	if _, err := gitWrapper.GetCommitSHA(ctx.baseCommit); err != nil {
		return result, &PreconditionError{Reason: fmt.Sprintf("preserved base_commit %s does not resolve: %v", ctx.baseCommit, err)}
	}
	ancestor, err := gitWrapper.IsAncestor(ctx.baseCommit, head)
	if err != nil {
		return result, err
	}
	if !ancestor {
		return result, &PreconditionError{Reason: fmt.Sprintf("preserved base_commit %s is not an ancestor of worktree HEAD %s", ctx.baseCommit, head)}
	}
	return result, nil
}

func (preservedInitialClaimStrategy) shouldRunPostWorktreeCmd(claimWorktreePhaseResult) bool {
	return true
}

func (preservedInitialClaimStrategy) mutateTask(task *models.Task, ctx *claimContext) {
	task.Worktree = &ctx.worktreeRel
	task.BaseCommit = &ctx.baseCommit
	if task.Attempt == 0 {
		task.Attempt = 1
	}
}

func (preservedInitialClaimStrategy) historyEntry(now time.Time, ctx *claimContext) models.TaskHistoryEntry {
	agentPtr := &ctx.agentID
	return models.TaskHistoryEntry{
		Time:  now,
		Event: models.TaskEventClaimed,
		Agent: agentPtr,
		Extra: map[string]any{
			"preserved_worktree": true,
		},
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

func (rejectedClaimStrategy) mutateTask(task *models.Task, ctx *claimContext) {
	// Preserved worktree (same-coder reclaim): the original worktree and
	// base_commit still describe the work — leave them untouched. Recreated
	// worktree (reassignment or missing/orphaned state): the new worktree
	// branches from the current integration HEAD, so the task metadata must
	// follow or reviewers would diff against a stale base.
	if !ctx.worktreeRecreated {
		return
	}
	task.Worktree = &ctx.worktreeRel
	task.BaseCommit = &ctx.baseCommit
}

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
