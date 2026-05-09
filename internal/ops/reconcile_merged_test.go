package ops

import (
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestReconcileMerged_FromIntegrationFailedMissingWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	mergeCommit := testhelpers.MustGit(t, tmpDir, "rev-parse", "HEAD")
	now := time.Now().UTC()
	taskID := "task-external-merged"
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus(taskID, models.TaskStatusIntegrationFailed, now)
	worktree := ".worktrees/" + taskID
	task.Worktree = &worktree
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := ReconcileMerged(tmpDir, taskID, mergeCommit, "https://github.com/example/repo/pull/414", "PR merged externally", "orchestrator-1")
	if err != nil {
		t.Fatalf("ReconcileMerged() error: %v", err)
	}
	if result.OriginalStatus != models.TaskStatusIntegrationFailed {
		t.Errorf("OriginalStatus = %s, want %s", result.OriginalStatus, models.TaskStatusIntegrationFailed)
	}
	if result.MergeCommit != mergeCommit {
		t.Errorf("MergeCommit = %s, want %s", result.MergeCommit, mergeCommit)
	}

	readState, err := db.New(stateFile).Read()
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	updated := readState.FindTask(taskID)
	if updated == nil {
		t.Fatal("Task not found")
	}
	if updated.Status != models.TaskStatusMerged {
		t.Errorf("Status = %s, want %s", updated.Status, models.TaskStatusMerged)
	}
	if updated.Worktree != nil {
		t.Errorf("Worktree = %v, want nil", *updated.Worktree)
	}
	if updated.MergeCommit == nil || *updated.MergeCommit != mergeCommit {
		t.Errorf("MergeCommit = %v, want %s", updated.MergeCommit, mergeCommit)
	}
	if !hasHandoffTriggerForTest(updated.HandoffEvents, models.HandoffTriggerCompletion) {
		t.Fatalf("completion handoff missing; handoff_events = %+v", updated.HandoffEvents)
	}

	last := updated.History[len(updated.History)-1]
	if last.Event != models.TaskEventMerged {
		t.Fatalf("last history event = %s, want %s", last.Event, models.TaskEventMerged)
	}
	if last.Extra["external_reconciliation"] != true {
		t.Errorf("external_reconciliation = %v, want true", last.Extra["external_reconciliation"])
	}
	if last.Extra["pr_url"] != "https://github.com/example/repo/pull/414" {
		t.Errorf("pr_url = %v", last.Extra["pr_url"])
	}
}
