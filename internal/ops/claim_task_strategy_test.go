package ops

import (
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
)

func TestFreshClaimStrategy_MutateTask_SetsAttemptOnFirstClaim(t *testing.T) {
	t.Parallel()

	task := &models.Task{Attempt: 0}
	ctx := &claimContext{
		worktreeRel: ".worktrees/test-task",
		baseCommit:  "abc123",
	}
	strategy := freshClaimStrategy{}
	strategy.mutateTask(task, ctx)

	if task.Attempt != 1 {
		t.Errorf("mutateTask() set Attempt = %d, want 1 for first claim", task.Attempt)
	}
}

func TestFreshClaimStrategy_MutateTask_PreservesNonZeroAttempt(t *testing.T) {
	t.Parallel()

	task := &models.Task{Attempt: 2}
	ctx := &claimContext{
		worktreeRel: ".worktrees/test-task",
		baseCommit:  "abc123",
	}
	strategy := freshClaimStrategy{}
	strategy.mutateTask(task, ctx)

	if task.Attempt != 2 {
		t.Errorf("mutateTask() changed Attempt to %d, want 2 (preserved)", task.Attempt)
	}
}

func TestIntegrationFixClaimStrategy_MutateTask_ClearsStaleAttemptMetadata(t *testing.T) {
	t.Parallel()

	reviewCommit := "review-sha"
	approvedBy := "reviewer-1"
	mergeCommit := "merge-sha"
	task := &models.Task{
		Output: []models.OutputEntry{{
			Desc:    "old output",
			PlanRef: "specs/plans/stale.md",
		}},
		ReviewCommit: &reviewCommit,
		ApprovedBy:   &approvedBy,
		Approvals: []models.Approval{{
			Agent:     "reviewer-1",
			Provider:  "codex",
			Timestamp: time.Now().UTC(),
		}},
		MergeCommit:        &mergeCommit,
		FailedBy:           []string{"reviewer-1"},
		IntegrationFailure: map[string]any{"reason": "post-merge state validation failed"},
	}

	strategy := integrationFixClaimStrategy{}
	strategy.mutateTask(task, nil)

	if len(task.Output) != 0 {
		t.Fatalf("Output = %v, want cleared", task.Output)
	}
	if task.ReviewCommit != nil {
		t.Fatalf("ReviewCommit = %v, want nil", *task.ReviewCommit)
	}
	if task.ApprovedBy != nil {
		t.Fatalf("ApprovedBy = %v, want nil", *task.ApprovedBy)
	}
	if len(task.Approvals) != 0 {
		t.Fatalf("Approvals = %v, want cleared", task.Approvals)
	}
	if task.MergeCommit != nil {
		t.Fatalf("MergeCommit = %v, want nil", *task.MergeCommit)
	}
	if task.IntegrationFailure != nil {
		t.Fatalf("IntegrationFailure = %v, want nil", task.IntegrationFailure)
	}
	if !task.IntegrationFix {
		t.Fatal("IntegrationFix = false, want true")
	}
	if len(task.FailedBy) != 1 || task.FailedBy[0] != "reviewer-1" {
		t.Fatalf("FailedBy = %v, want preserved", task.FailedBy)
	}
}
