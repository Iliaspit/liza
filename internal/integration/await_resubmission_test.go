package integration

// await_resubmission_test.go contains an end-to-end integration test for the
// reject -> await_resubmission -> resubmit -> re-review -> approve -> merge flow.

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/commands"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/testhelpers"
)

type awaitResubmissionCall struct {
	result *commands.AwaitResubmissionResult
	err    error
}

// TestAwaitResubmission_RejectResubmitFlow exercises the full reject -> await
// -> resubmit -> re-review -> approve -> merge lifecycle using a real
// AwaitResubmission blocking call.
func TestAwaitResubmission_RejectResubmitFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// --- Setup: project, task, agents ---
	projectDir, cleanup := setupTestProject(t)
	defer cleanup()

	bb, _, _ := setupIntegrationTest(t, projectDir, []string{"task-1"})

	coderID := "coder-1"
	reviewerID := "code-reviewer-1"
	testhelpers.RegisterTestAgent(t, bb, coderID, "coder")
	testhelpers.RegisterTestAgent(t, bb, reviewerID, "code-reviewer")

	// --- Phase 1: Coder claims, implements, submits ---
	if err := commands.ClaimTaskCommand(projectDir, "task-1", coderID); err != nil {
		t.Fatalf("ClaimTask failed: %v", err)
	}

	state, err := bb.Read()
	testhelpers.AssertNoError(t, err)
	task := findTask(state.Tasks, "task-1")
	if task == nil {
		t.Fatal("Task not found after claim")
	}
	worktreePath := filepath.Join(projectDir, *task.Worktree)

	// Create code + test file in worktree
	if err := os.WriteFile(filepath.Join(worktreePath, "feature.go"),
		[]byte("package main\n\nfunc Feature() {}\n"), 0644); err != nil {
		t.Fatalf("Failed to create feature.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, "feature_test.go"),
		[]byte("package main\n"), 0644); err != nil {
		t.Fatalf("Failed to create feature_test.go: %v", err)
	}

	if err := exec.Command("git", "-C", worktreePath, "add", "feature.go", "feature_test.go").Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if err := exec.Command("git", "-C", worktreePath, "commit", "-m", "feat: initial implementation").Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Write checkpoint (required for submission)
	if err := ops.WriteCheckpoint(projectDir, &ops.WriteCheckpointInput{
		TaskID: "task-1", AgentID: coderID,
		Intent: "Implement feature", ValidationPlan: "go test ./...",
		FilesToModify: []string{"feature.go"},
	}); err != nil {
		t.Fatalf("WriteCheckpoint failed: %v", err)
	}

	commitSHA := getHeadSHA(t, worktreePath)
	if err := commands.SubmitForReviewCommand(projectDir, "task-1", commitSHA, coderID); err != nil {
		t.Fatalf("SubmitForReview failed: %v", err)
	}

	// Verify task is ready for review
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	task = findTask(state.Tasks, "task-1")
	if task.Status != models.TaskStatusReadyForReview {
		t.Fatalf("Expected CODE_READY_FOR_REVIEW, got %s", task.Status)
	}

	// --- Phase 2: Reviewer claims, reviews, rejects ---
	testhelpers.TransitionToReviewing(t, bb, "task-1", reviewerID)
	if err := commands.SubmitVerdictCommand(projectDir, "task-1", "REJECTED", "Missing error handling", reviewerID, ""); err != nil {
		t.Fatalf("SubmitVerdict (reject) failed: %v", err)
	}

	// Verify task is rejected
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	task = findTask(state.Tasks, "task-1")
	if task.Status != models.TaskStatusRejected {
		t.Fatalf("Expected CODE_REJECTED after rejection, got %s", task.Status)
	}
	if task.BaseCommit == nil {
		t.Fatal("Expected non-empty BaseCommit after rejection")
	}
	expectedBaseCommit := *task.BaseCommit

	// --- Phase 3: Reviewer calls the public command adapter ---
	awaitCall := startAwaitResubmission(projectDir, "task-1", reviewerID, 30*time.Second)

	// Let AwaitResubmission start watching before coder acts
	testhelpers.WaitForAsyncSetup()

	// --- Phase 4: Coder reclaims, fixes, resubmits ---
	if err := commands.ClaimTaskCommand(projectDir, "task-1", coderID); err != nil {
		t.Fatalf("ClaimTask (reclaim) failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(worktreePath, "feature.go"),
		[]byte("package main\n\nimport \"errors\"\n\nvar ErrInvalid = errors.New(\"invalid\")\n\nfunc Feature() error { return nil }\n"), 0644); err != nil {
		t.Fatalf("Failed to write fix: %v", err)
	}
	if err := exec.Command("git", "-C", worktreePath, "add", "feature.go").Run(); err != nil {
		t.Fatalf("git add (fix) failed: %v", err)
	}
	if err := exec.Command("git", "-C", worktreePath, "commit", "-m", "fix: add error handling").Run(); err != nil {
		t.Fatalf("git commit (fix) failed: %v", err)
	}

	newSHA := getHeadSHA(t, worktreePath)
	if err := commands.SubmitForReviewCommand(projectDir, "task-1", newSHA, coderID); err != nil {
		t.Fatalf("SubmitForReview (resubmit) failed: %v", err)
	}

	// --- Phase 5: Verify AwaitResubmission returns RESUBMITTED ---
	awaitResult := receiveAwaitResubmission(t, awaitCall, 10*time.Second)
	if awaitResult.Verdict != ops.ResubmissionResubmitted {
		t.Fatalf("Verdict = %q, want %q", awaitResult.Verdict, ops.ResubmissionResubmitted)
	}
	if awaitResult.ReviewCommit == "" {
		t.Error("Expected non-empty ReviewCommit on resubmission")
	}
	if awaitResult.ReviewCommit != newSHA {
		t.Errorf("ReviewCommit = %q, want %q", awaitResult.ReviewCommit, newSHA)
	}
	if awaitResult.BaseCommit != expectedBaseCommit {
		t.Errorf("BaseCommit = %q, want %q", awaitResult.BaseCommit, expectedBaseCommit)
	}

	// Verify task is now in REVIEWING state
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	task = findTask(state.Tasks, "task-1")
	if task.Status != models.TaskStatusReviewing {
		t.Errorf("After resubmission: status = %s, want %s", task.Status, models.TaskStatusReviewing)
	}

	// --- Phase 6: Reviewer approves ---
	if err := commands.SubmitVerdictCommand(projectDir, "task-1", "APPROVED", "", reviewerID, ""); err != nil {
		t.Fatalf("SubmitVerdict (approve) failed: %v", err)
	}

	// --- Phase 7: Merge ---
	if err := commands.WtMergeCommand(projectDir, "task-1", reviewerID); err != nil {
		t.Fatalf("WtMerge failed: %v", err)
	}

	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	task = findTask(state.Tasks, "task-1")
	if task.Status != models.TaskStatusMerged {
		t.Errorf("After merge: status = %s, want %s", task.Status, models.TaskStatusMerged)
	}
	if task.MergeCommit == nil {
		t.Error("Expected merge commit to be set")
	}
}

func TestAwaitResubmission_TerminalFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	testhelpers.SetupGlobalLiza(t)
	fixture := setupIsolatedAwaitProject(t)
	rejectAwaitFixture(t, fixture)

	awaitCall := startAwaitResubmission(
		fixture.projectRoot, awaitTaskID, awaitReviewerID, 30*time.Second)
	testhelpers.WaitForAsyncSetup()
	if err := commands.ClaimTaskCommand(fixture.projectRoot, awaitTaskID, awaitCoderID); err != nil {
		t.Fatalf("ClaimTask (reclaim) failed: %v", err)
	}
	if err := commands.MarkBlockedCommand(
		fixture.projectRoot,
		awaitTaskID,
		"Spec ambiguity",
		[]string{"Which behavior is required?"},
		awaitCoderID,
	); err != nil {
		t.Fatalf("MarkBlocked failed: %v", err)
	}

	result := receiveAwaitResubmission(t, awaitCall, 10*time.Second)
	if result.Verdict != ops.ResubmissionTerminal {
		t.Fatalf("Verdict = %q, want %q", result.Verdict, ops.ResubmissionTerminal)
	}
	if result.TaskStatus != models.TaskStatusBlocked {
		t.Fatalf("Task status = %q, want %q", result.TaskStatus, models.TaskStatusBlocked)
	}
	assertBoundedAwaitState(t, fixture, true)
}

func startAwaitResubmission(
	projectRoot, taskID, agentID string,
	remaining time.Duration,
) <-chan awaitResubmissionCall {
	result := make(chan awaitResubmissionCall, 1)
	go func() {
		awaitResult, err := commands.AwaitResubmission(projectRoot, taskID, agentID, remaining)
		result <- awaitResubmissionCall{result: awaitResult, err: err}
	}()
	return result
}

func receiveAwaitResubmission(
	t *testing.T,
	call <-chan awaitResubmissionCall,
	guard time.Duration,
) *commands.AwaitResubmissionResult {
	t.Helper()
	select {
	case outcome := <-call:
		if outcome.err != nil {
			t.Fatalf("AwaitResubmission returned error: %v", outcome.err)
		}
		if outcome.result == nil {
			t.Fatal("AwaitResubmission returned a nil result")
		}
		return outcome.result
	case <-time.After(guard):
		t.Fatalf("AwaitResubmission caller did not return within %s", guard)
		return nil
	}
}
