package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liza-mas/liza/internal/commands"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestUnblockTaskPendingDependencyLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	const (
		dependencyTaskID  = "dependency-task"
		dependentTaskID   = "dependent-task"
		dependentCoderID  = "coder-1"
		dependencyCoderID = "coder-2"
		reviewerID        = "code-reviewer-1"
		orchestratorID    = "orchestrator-1"
		taskCommitMessage = "Preserve dependent task change"
		taskContent       = "preserved dependent task content\n"
		dependencyContent = "merged dependency artifact\n"
	)

	projectDir, cleanup := setupTestProject(t)
	defer cleanup()

	bb, _, _ := setupIntegrationTest(t, projectDir, []string{dependencyTaskID, dependentTaskID})
	testhelpers.RegisterTestAgent(t, bb, dependentCoderID, "coder")
	testhelpers.RegisterTestAgent(t, bb, dependencyCoderID, "coder")
	testhelpers.RegisterTestAgent(t, bb, reviewerID, "code-reviewer")

	if _, err := ops.ClaimTask(projectDir, dependentTaskID, dependentCoderID); err != nil {
		t.Fatalf("ClaimTask(%s) error: %v", dependentTaskID, err)
	}
	state, err := bb.Read()
	testhelpers.AssertNoError(t, err)
	dependentTask := findTask(state.Tasks, dependentTaskID)
	if dependentTask == nil || dependentTask.Worktree == nil {
		t.Fatalf("claimed dependent task has no worktree: %+v", dependentTask)
	}
	dependentWorktree := filepath.Join(projectDir, *dependentTask.Worktree)
	dependentFile := filepath.Join(dependentWorktree, "task.txt")
	if err := os.WriteFile(dependentFile, []byte(taskContent), 0o644); err != nil {
		t.Fatalf("write preserved task content: %v", err)
	}
	testhelpers.MustGit(t, dependentWorktree, "add", "task.txt")
	testhelpers.MustGit(t, dependentWorktree, "commit", "-m", taskCommitMessage)
	preservedHead := testhelpers.MustGit(t, dependentWorktree, "rev-parse", "HEAD")

	if err := commands.MarkBlockedWithOptionsCommand(
		projectDir,
		dependentTaskID,
		"dependency must land first",
		[]string{"Can the dependency be completed?"},
		dependentCoderID,
		ops.MarkBlockedOptions{DependsOn: []string{dependencyTaskID}},
	); err != nil {
		t.Fatalf("MarkBlockedWithOptionsCommand() error: %v", err)
	}
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	dependentTask = findTask(state.Tasks, dependentTaskID)
	if dependentTask == nil || dependentTask.Status != models.TaskStatusBlocked {
		t.Fatalf("dependent task status = %v, want BLOCKED", dependentTask)
	}

	_, err = ops.UnblockTaskWithOptions(
		projectDir,
		dependentTaskID,
		"repair complete",
		orchestratorID,
		ops.UnblockTaskOptions{AssignTo: dependentCoderID},
	)
	if err == nil || !strings.Contains(err.Error(), "unmet dependencies") {
		t.Fatalf("direct assignment error = %v, want unmet dependencies", err)
	}

	unblockResult, err := ops.UnblockTaskWithOptions(
		projectDir,
		dependentTaskID,
		"repair complete; wait for dependency",
		orchestratorID,
		ops.UnblockTaskOptions{},
	)
	if err != nil {
		t.Fatalf("UnblockTaskWithOptions() error: %v", err)
	}
	if unblockResult.ToStatus != models.TaskStatusReady || unblockResult.Claimable {
		t.Fatalf("unblock result = %+v, want READY and claimable=false", unblockResult)
	}

	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	dependentTask = findTask(state.Tasks, dependentTaskID)
	if dependentTask == nil || dependentTask.Status != models.TaskStatusReady || dependentTask.AssignedTo != nil {
		t.Fatalf("dependency-held task = %+v, want unassigned READY", dependentTask)
	}
	if dependentTask.Worktree == nil || filepath.Join(projectDir, *dependentTask.Worktree) != dependentWorktree {
		t.Fatalf("preserved worktree = %v, want %s", dependentTask.Worktree, dependentWorktree)
	}

	_, err = ops.ClaimTask(projectDir, dependentTaskID, dependentCoderID)
	if err == nil || !strings.Contains(err.Error(), "unmet dependencies") {
		t.Fatalf("normal claim error = %v, want unmet dependencies", err)
	}
	if got := testhelpers.MustGit(t, dependentWorktree, "rev-parse", "HEAD"); got != preservedHead {
		t.Fatalf("dependency-gated claim moved preserved HEAD: got %s, want %s", got, preservedHead)
	}

	if _, err := ops.ClaimTask(projectDir, dependencyTaskID, dependencyCoderID); err != nil {
		t.Fatalf("ClaimTask(%s) error: %v", dependencyTaskID, err)
	}
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	dependencyTask := findTask(state.Tasks, dependencyTaskID)
	if dependencyTask == nil || dependencyTask.Worktree == nil {
		t.Fatalf("claimed dependency task has no worktree: %+v", dependencyTask)
	}
	dependencyWorktree := filepath.Join(projectDir, *dependencyTask.Worktree)
	if err := os.WriteFile(filepath.Join(dependencyWorktree, "dependency.txt"), []byte(dependencyContent), 0o644); err != nil {
		t.Fatalf("write dependency artifact: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dependencyWorktree, "dependency_test.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write dependency test: %v", err)
	}
	testhelpers.MustGit(t, dependencyWorktree, "add", "dependency.txt", "dependency_test.go")
	testhelpers.MustGit(t, dependencyWorktree, "commit", "-m", "Add dependency artifact")
	if err := ops.WriteCheckpoint(projectDir, &ops.WriteCheckpointInput{
		TaskID: dependencyTaskID, AgentID: dependencyCoderID,
		Intent: "Add the dependency artifact", ValidationPlan: "verify dependency artifact content",
		FilesToModify: []string{"dependency.txt", "dependency_test.go"},
	}); err != nil {
		t.Fatalf("WriteCheckpoint(%s) error: %v", dependencyTaskID, err)
	}
	dependencyCommit := testhelpers.MustGit(t, dependencyWorktree, "rev-parse", "HEAD")
	if err := commands.SubmitForReviewCommand(projectDir, dependencyTaskID, dependencyCommit, dependencyCoderID); err != nil {
		t.Fatalf("SubmitForReviewCommand(%s) error: %v", dependencyTaskID, err)
	}
	testhelpers.TransitionToReviewing(t, bb, dependencyTaskID, reviewerID)
	if err := commands.SubmitVerdictCommand(projectDir, dependencyTaskID, "APPROVED", "", reviewerID, ""); err != nil {
		t.Fatalf("SubmitVerdictCommand(%s) error: %v", dependencyTaskID, err)
	}
	t.Setenv("LIZA_AGENT_ID", reviewerID)
	if err := commands.WtMergeCommand(projectDir, dependencyTaskID, reviewerID); err != nil {
		t.Fatalf("WtMergeCommand(%s) error: %v", dependencyTaskID, err)
	}

	assignmentBase := testhelpers.MustGit(t, projectDir, "rev-parse", "integration")
	if got := testhelpers.MustGit(t, projectDir, "show", assignmentBase+":dependency.txt"); got != strings.TrimSpace(dependencyContent) {
		t.Fatalf("dependency artifact at integration SHA = %q, want %q", got, strings.TrimSpace(dependencyContent))
	}
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	dependencyTask = findTask(state.Tasks, dependencyTaskID)
	if dependencyTask == nil || dependencyTask.Status != models.TaskStatusMerged {
		t.Fatalf("dependency task status = %v, want MERGED", dependencyTask)
	}

	claimResult, err := ops.ClaimTask(projectDir, dependentTaskID, dependentCoderID)
	if err != nil {
		t.Fatalf("ClaimTask(%s) after dependency merge error: %v", dependentTaskID, err)
	}
	if claimResult.BaseCommit != assignmentBase {
		t.Fatalf("claim base_commit = %s, want assignment-linearized SHA %s", claimResult.BaseCommit, assignmentBase)
	}

	rebasedHead := testhelpers.MustGit(t, dependentWorktree, "rev-parse", "HEAD")
	testhelpers.MustGit(t, dependentWorktree, "merge-base", "--is-ancestor", assignmentBase, rebasedHead)
	if got := testhelpers.MustGit(t, dependentWorktree, "log", "-1", "--format=%s"); got != taskCommitMessage {
		t.Fatalf("rebased commit message = %q, want %q", got, taskCommitMessage)
	}
	gotTaskContent, err := os.ReadFile(dependentFile)
	if err != nil || string(gotTaskContent) != taskContent {
		t.Fatalf("rebased task content = %q, %v; want %q", gotTaskContent, err, taskContent)
	}
	gotDependencyContent, err := os.ReadFile(filepath.Join(dependentWorktree, "dependency.txt"))
	if err != nil || string(gotDependencyContent) != dependencyContent {
		t.Fatalf("rebased dependency content = %q, %v; want %q", gotDependencyContent, err, dependencyContent)
	}

	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	dependentTask = findTask(state.Tasks, dependentTaskID)
	if dependentTask == nil || dependentTask.Status != models.TaskStatusImplementing {
		t.Fatalf("dependent task after claim = %+v, want IMPLEMENTING", dependentTask)
	}
	if dependentTask.BaseCommit == nil || *dependentTask.BaseCommit != assignmentBase {
		t.Fatalf("assigned task base_commit = %v, want %s", dependentTask.BaseCommit, assignmentBase)
	}
	if dependentTask.AssignedTo == nil || *dependentTask.AssignedTo != dependentCoderID {
		t.Fatalf("assigned task = %+v, want assigned_to %s", dependentTask, dependentCoderID)
	}
}
