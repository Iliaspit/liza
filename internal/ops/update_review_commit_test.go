package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestUpdateReviewCommit_HappyPath_Submitted(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	g := git.New(tmpDir)
	_, err := g.CreateWorktree("task-1", "integration")
	if err != nil {
		t.Fatalf("Failed to create worktree: %v", err)
	}
	wtPath := g.GetWorktreePath("task-1")

	// Make a commit so HEAD diverges from stale review_commit
	implFile := filepath.Join(wtPath, "feature.go")
	if err := os.WriteFile(implFile, []byte("package feature\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, wtPath, "add", "feature.go")
	testhelpers.MustGit(t, wtPath, "commit", "-m", "Add feature")

	staleCommit := testhelpers.MustGit(t, tmpDir, "rev-parse", "integration")
	wtHEAD := testhelpers.MustGit(t, wtPath, "rev-parse", "HEAD")

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
	task.ReviewCommit = &staleCommit
	worktreeRel := g.GetWorktreeRelPath("task-1")
	task.Worktree = &worktreeRel
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := UpdateReviewCommit(tmpDir, "task-1", "human")
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}

	if result.OldReviewCommit != staleCommit {
		t.Errorf("OldReviewCommit = %s, want %s", result.OldReviewCommit, staleCommit)
	}
	if result.NewReviewCommit != wtHEAD {
		t.Errorf("NewReviewCommit = %s, want %s", result.NewReviewCommit, wtHEAD)
	}
	if result.ReviewerReleased {
		t.Error("ReviewerReleased should be false (no reviewer claimed)")
	}

	// Verify state
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	readTask := readState.FindTask("task-1")
	if readTask.ReviewCommit == nil || *readTask.ReviewCommit != wtHEAD {
		got := "<nil>"
		if readTask.ReviewCommit != nil {
			got = *readTask.ReviewCommit
		}
		t.Errorf("ReviewCommit = %s, want %s", got, wtHEAD)
	}

	// Status should remain submitted (no reviewer to release)
	if readTask.Status != models.TaskStatusReadyForReview {
		t.Errorf("Status = %s, want %s", readTask.Status, models.TaskStatusReadyForReview)
	}

	// Verify history entry
	found := false
	for _, entry := range readTask.History {
		if entry.Event == models.TaskEventReviewCommitUpdated {
			found = true
			if entry.Reason == nil || !strings.Contains(*entry.Reason, staleCommit) {
				t.Errorf("history reason should reference old commit %s", staleCommit)
			}
			if entry.Commit == nil || *entry.Commit != wtHEAD {
				t.Errorf("history commit = %v, want %s", entry.Commit, wtHEAD)
			}
			if entry.Extra["old_review_commit"] != staleCommit {
				t.Errorf("old_review_commit extra = %v, want %s", entry.Extra["old_review_commit"], staleCommit)
			}
			if entry.Extra["new_review_commit"] != wtHEAD {
				t.Errorf("new_review_commit extra = %v, want %s", entry.Extra["new_review_commit"], wtHEAD)
			}
			break
		}
	}
	if !found {
		t.Error("Expected review_commit_updated history entry")
	}
}

func TestUpdateReviewCommit_ReleasesReviewer(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	g := git.New(tmpDir)
	_, err := g.CreateWorktree("task-1", "integration")
	if err != nil {
		t.Fatalf("Failed to create worktree: %v", err)
	}
	wtPath := g.GetWorktreePath("task-1")

	implFile := filepath.Join(wtPath, "feature.go")
	if err := os.WriteFile(implFile, []byte("package feature\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, wtPath, "add", "feature.go")
	testhelpers.MustGit(t, wtPath, "commit", "-m", "Add feature")

	staleCommit := testhelpers.MustGit(t, tmpDir, "rev-parse", "integration")

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	task.ReviewCommit = &staleCommit
	worktreeRel := g.GetWorktreeRelPath("task-1")
	task.Worktree = &worktreeRel
	reviewerID := "code-reviewer-1"
	task.ReviewingBy = &reviewerID
	leaseExpiry := now.Add(30 * time.Minute)
	task.ReviewLeaseExpires = &leaseExpiry
	state.Tasks = []models.Task{task}
	taskIDRef := "task-1"
	state.Agents["code-reviewer-1"] = models.Agent{
		Role:        "code-reviewer",
		Status:      models.AgentStatusReviewing,
		CurrentTask: &taskIDRef,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := UpdateReviewCommit(tmpDir, "task-1", "human")
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}

	if !result.ReviewerReleased {
		t.Error("ReviewerReleased should be true")
	}

	// Verify state: task back to submitted, reviewer released
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	readTask := readState.FindTask("task-1")
	if readTask.Status != models.TaskStatusReadyForReview {
		t.Errorf("Status = %s, want %s (reset to submitted)", readTask.Status, models.TaskStatusReadyForReview)
	}
	if readTask.ReviewingBy != nil {
		t.Errorf("ReviewingBy = %v, want nil", *readTask.ReviewingBy)
	}
	if readTask.ReviewLeaseExpires != nil {
		t.Error("ReviewLeaseExpires should be nil")
	}

	// Verify agent released
	agent := readState.Agents["code-reviewer-1"]
	if agent.CurrentTask != nil {
		t.Errorf("Agent CurrentTask = %v, want nil", *agent.CurrentTask)
	}
}

func TestUpdateReviewCommit_RejectsWrongStatus(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	commit := "abc123"
	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
	task.ReviewCommit = &commit
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := UpdateReviewCommit(tmpDir, "task-1", "human")
	if err == nil {
		t.Fatal("Expected error for wrong status")
	}
	if !strings.Contains(err.Error(), "submitted or reviewing") {
		t.Errorf("Error = %q, want to mention 'submitted or reviewing'", err.Error())
	}
}

func TestUpdateReviewCommit_RejectsNoMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	g := git.New(tmpDir)
	_, err := g.CreateWorktree("task-1", "integration")
	if err != nil {
		t.Fatalf("Failed to create worktree: %v", err)
	}

	// Set review_commit to the actual worktree HEAD (no mismatch)
	wtHEAD := testhelpers.MustGit(t, g.GetWorktreePath("task-1"), "rev-parse", "HEAD")

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
	task.ReviewCommit = &wtHEAD
	worktreeRel := g.GetWorktreeRelPath("task-1")
	task.Worktree = &worktreeRel
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err = UpdateReviewCommit(tmpDir, "task-1", "human")
	if err == nil {
		t.Fatal("Expected error when review_commit already matches")
	}
	if !strings.Contains(err.Error(), "no update needed") {
		t.Errorf("Error = %q, want to mention 'no update needed'", err.Error())
	}
}

func TestUpdateReviewCommit_RejectsNoReviewCommit(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
	task.ReviewCommit = nil
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := UpdateReviewCommit(tmpDir, "task-1", "human")
	if err == nil {
		t.Fatal("Expected error for missing review_commit")
	}
	if !strings.Contains(err.Error(), "no review_commit") {
		t.Errorf("Error = %q, want to mention 'no review_commit'", err.Error())
	}
}
