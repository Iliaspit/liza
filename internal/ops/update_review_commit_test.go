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
	t.Parallel()

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
	task.BaseCommit = &staleCommit
	worktreeRel := g.GetWorktreeRelPath("task-1")
	task.Worktree = &worktreeRel
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := UpdateReviewCommit(tmpDir, "task-1", "human")
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}

	if result.OldReviewCommit == nil || *result.OldReviewCommit != staleCommit {
		t.Errorf("OldReviewCommit = %v, want %s", result.OldReviewCommit, staleCommit)
	}
	if result.NewReviewCommit != wtHEAD {
		t.Errorf("NewReviewCommit = %s, want %s", result.NewReviewCommit, wtHEAD)
	}
	expectedBase := testhelpers.MustGit(t, tmpDir, "merge-base", wtHEAD, "integration")
	if result.OldBaseCommit == nil || *result.OldBaseCommit != staleCommit {
		t.Errorf("OldBaseCommit = %v, want %s", result.OldBaseCommit, staleCommit)
	}
	if result.NewBaseCommit != expectedBase {
		t.Errorf("NewBaseCommit = %s, want %s", result.NewBaseCommit, expectedBase)
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
	if readTask.BaseCommit == nil || *readTask.BaseCommit != expectedBase {
		got := "<nil>"
		if readTask.BaseCommit != nil {
			got = *readTask.BaseCommit
		}
		t.Errorf("BaseCommit = %s, want %s", got, expectedBase)
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
			if got := entry.Extra["old_base_commit"]; got == nil {
				t.Errorf("old_base_commit extra = nil, want %s", staleCommit)
			}
			if entry.Extra["new_base_commit"] != expectedBase {
				t.Errorf("new_base_commit extra = %v, want %s", entry.Extra["new_base_commit"], expectedBase)
			}
			break
		}
	}
	if !found {
		t.Error("Expected review_commit_updated history entry")
	}
}

func TestUpdateReviewCommit_ReleasesReviewer(t *testing.T) {
	t.Parallel()

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
	if result.OldReviewCommit == nil || *result.OldReviewCommit != staleCommit {
		t.Errorf("OldReviewCommit = %v, want %s", result.OldReviewCommit, staleCommit)
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

func TestUpdateReviewCommit_SetsMissingReviewCommitAndReleasesReviewer(t *testing.T) {
	t.Parallel()

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
	wtHEAD := testhelpers.MustGit(t, wtPath, "rev-parse", "HEAD")

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	task.ReviewCommit = nil
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
	if result.OldReviewCommit != nil {
		t.Errorf("OldReviewCommit = %v, want nil", *result.OldReviewCommit)
	}

	readState, err := db.New(stateFile).Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	readTask := readState.FindTask("task-1")
	if readTask.Status != models.TaskStatusReadyForReview {
		t.Errorf("Status = %s, want %s (reset to submitted)", readTask.Status, models.TaskStatusReadyForReview)
	}
	if readTask.ReviewCommit == nil || *readTask.ReviewCommit != wtHEAD {
		t.Fatalf("ReviewCommit = %v, want %s", readTask.ReviewCommit, wtHEAD)
	}
	if readTask.ReviewingBy != nil {
		t.Errorf("ReviewingBy = %v, want nil", *readTask.ReviewingBy)
	}
	if readTask.ReviewLeaseExpires != nil {
		t.Error("ReviewLeaseExpires should be nil")
	}

	agent := readState.Agents["code-reviewer-1"]
	if agent.CurrentTask != nil {
		t.Errorf("Agent CurrentTask = %v, want nil", *agent.CurrentTask)
	}
}

func TestUpdateReviewCommit_RejectsWrongStatus(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
	task.ReviewCommit = nil
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
	t.Parallel()

	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	g := git.New(tmpDir)
	_, err := g.CreateWorktree("task-1", "integration")
	if err != nil {
		t.Fatalf("Failed to create worktree: %v", err)
	}

	// Set review_commit and base_commit to the actual current boundary (no mismatch)
	wtHEAD := testhelpers.MustGit(t, g.GetWorktreePath("task-1"), "rev-parse", "HEAD")
	baseCommit := testhelpers.MustGit(t, tmpDir, "merge-base", wtHEAD, "integration")

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
	task.ReviewCommit = &wtHEAD
	task.BaseCommit = &baseCommit
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

func TestUpdateReviewCommit_UpdatesStaleBaseWhenReviewCommitMatchesHead(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	g := git.New(tmpDir)
	_, err := g.CreateWorktree("task-1", "integration")
	if err != nil {
		t.Fatalf("Failed to create worktree: %v", err)
	}
	wtPath := g.GetWorktreePath("task-1")
	if err := os.WriteFile(filepath.Join(wtPath, "feature.go"), []byte("package feature\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, wtPath, "add", "feature.go")
	testhelpers.MustGit(t, wtPath, "commit", "-m", "Add feature")
	wtHEAD := testhelpers.MustGit(t, wtPath, "rev-parse", "HEAD")
	effectiveBase := testhelpers.MustGit(t, tmpDir, "merge-base", wtHEAD, "integration")
	staleBase := wtHEAD
	if staleBase == effectiveBase {
		t.Fatal("test setup failed: stale base equals effective base")
	}

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
	task.ReviewCommit = &wtHEAD
	task.BaseCommit = &staleBase
	worktreeRel := g.GetWorktreeRelPath("task-1")
	task.Worktree = &worktreeRel
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := UpdateReviewCommit(tmpDir, "task-1", "human")
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if result.NewReviewCommit != wtHEAD {
		t.Errorf("NewReviewCommit = %s, want unchanged %s", result.NewReviewCommit, wtHEAD)
	}
	if result.OldBaseCommit == nil || *result.OldBaseCommit != staleBase {
		t.Errorf("OldBaseCommit = %v, want %s", result.OldBaseCommit, staleBase)
	}
	if result.NewBaseCommit != effectiveBase {
		t.Errorf("NewBaseCommit = %s, want %s", result.NewBaseCommit, effectiveBase)
	}
}

func TestUpdateReviewCommit_SetsMissingBaseCommit(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	g := git.New(tmpDir)
	_, err := g.CreateWorktree("task-1", "integration")
	if err != nil {
		t.Fatalf("Failed to create worktree: %v", err)
	}
	wtHEAD := testhelpers.MustGit(t, g.GetWorktreePath("task-1"), "rev-parse", "HEAD")
	effectiveBase := testhelpers.MustGit(t, tmpDir, "merge-base", wtHEAD, "integration")

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
	task.ReviewCommit = &wtHEAD
	task.BaseCommit = nil
	worktreeRel := g.GetWorktreeRelPath("task-1")
	task.Worktree = &worktreeRel
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := UpdateReviewCommit(tmpDir, "task-1", "human")
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if result.OldBaseCommit != nil {
		t.Errorf("OldBaseCommit = %v, want nil", *result.OldBaseCommit)
	}
	if result.NewBaseCommit != effectiveBase {
		t.Errorf("NewBaseCommit = %s, want %s", result.NewBaseCommit, effectiveBase)
	}

	readState, err := db.New(stateFile).Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	readTask := readState.FindTask("task-1")
	if readTask.BaseCommit == nil || *readTask.BaseCommit != effectiveBase {
		t.Fatalf("BaseCommit = %v, want %s", readTask.BaseCommit, effectiveBase)
	}
}

func TestUpdateReviewCommit_SetsMissingReviewCommit(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	g := git.New(tmpDir)
	_, err := g.CreateWorktree("task-1", "integration")
	if err != nil {
		t.Fatalf("Failed to create worktree: %v", err)
	}
	wtPath := g.GetWorktreePath("task-1")
	if err := os.WriteFile(filepath.Join(wtPath, "feature.go"), []byte("package feature\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, wtPath, "add", "feature.go")
	testhelpers.MustGit(t, wtPath, "commit", "-m", "Add feature")
	wtHEAD := testhelpers.MustGit(t, wtPath, "rev-parse", "HEAD")
	effectiveBase := testhelpers.MustGit(t, tmpDir, "merge-base", wtHEAD, "integration")

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
	task.ReviewCommit = nil
	worktreeRel := g.GetWorktreeRelPath("task-1")
	task.Worktree = &worktreeRel
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := UpdateReviewCommit(tmpDir, "task-1", "human")
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if result.OldReviewCommit != nil {
		t.Errorf("OldReviewCommit = %v, want nil", *result.OldReviewCommit)
	}
	if result.NewReviewCommit != wtHEAD {
		t.Errorf("NewReviewCommit = %s, want %s", result.NewReviewCommit, wtHEAD)
	}
	if result.NewBaseCommit != effectiveBase {
		t.Errorf("NewBaseCommit = %s, want %s", result.NewBaseCommit, effectiveBase)
	}

	readState, err := db.New(stateFile).Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	readTask := readState.FindTask("task-1")
	if readTask.ReviewCommit == nil || *readTask.ReviewCommit != wtHEAD {
		t.Fatalf("ReviewCommit = %v, want %s", readTask.ReviewCommit, wtHEAD)
	}
	if readTask.BaseCommit == nil || *readTask.BaseCommit != effectiveBase {
		t.Fatalf("BaseCommit = %v, want %s", readTask.BaseCommit, effectiveBase)
	}

	found := false
	for _, entry := range readTask.History {
		if entry.Event == models.TaskEventReviewCommitUpdated {
			found = true
			if entry.Reason == nil || !strings.Contains(*entry.Reason, "<nil>") {
				t.Errorf("history reason should reference missing old review_commit, got %v", entry.Reason)
			}
			if got := entry.Extra["old_review_commit"]; got != nil {
				t.Errorf("old_review_commit extra = %v, want nil", got)
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
