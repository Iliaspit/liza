package agent

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/scipsearch"
	"github.com/liza-mas/liza/internal/semble"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestEnsureReviewerWorktree_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	state := testhelpers.CreateValidState()
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	state.Tasks = []models.Task{task}
	bb := testhelpers.WriteInitialState(t, statePath, state)

	// Create the worktree directory so it "exists"
	testhelpers.CreateTestWorktree(t, tmpDir, "task-1")

	recovered, err := ensureReviewerWorktree(tmpDir, bb, "task-1", "code-reviewer-1")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if recovered {
		t.Error("Expected recovered=false when worktree exists")
	}
}

func TestEnsureReviewerWorktree_MissingRecoverable(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	state := testhelpers.CreateValidState()
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	state.Tasks = []models.Task{task}
	bb := testhelpers.WriteInitialState(t, statePath, state)

	// Create the branch (task/task-1) so recovery can find it.
	branchName := paths.TaskBranchPrefix + "task-1"
	cmd := exec.Command("git", "branch", branchName)
	cmd.Dir = tmpDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to create branch: %v\n%s", err, out)
	}

	// No worktree directory — recovery should recreate it.
	recovered, err := ensureReviewerWorktree(tmpDir, bb, "task-1", "code-reviewer-1")
	if err != nil {
		t.Fatalf("Expected successful recovery, got error: %v", err)
	}
	if !recovered {
		t.Error("Expected recovered=true")
	}

	// Verify worktree was created.
	wtPath := filepath.Join(tmpDir, paths.WorktreesDirName, "task-1")
	if _, statErr := os.Stat(wtPath); os.IsNotExist(statErr) {
		t.Error("Expected worktree directory to exist after recovery")
	}

	// Verify history entry was added.
	readState, _ := bb.Read()
	readTask := readState.FindTask("task-1")
	if readTask == nil {
		t.Fatal("Task not found in state")
	}
	found := false
	for _, h := range readTask.History {
		if h.Event == models.TaskEventWorktreeRecovered {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'worktree_recovered' history entry")
	}
}

func TestEnsureReviewerWorktree_MissingRecoverable_RunsPostWorktreeCmd(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	state := testhelpers.CreateValidState()
	now := time.Now().UTC()

	// Configure a post-worktree command that creates a marker file.
	postCmd := "touch .post-worktree-ran"
	state.Config.PostWorktreeCmd = &postCmd
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	state.Tasks = []models.Task{task}
	bb := testhelpers.WriteInitialState(t, statePath, state)

	// Create the branch so recovery can find it.
	branchName := paths.TaskBranchPrefix + "task-1"
	cmd := exec.Command("git", "branch", branchName)
	cmd.Dir = tmpDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to create branch: %v\n%s", err, out)
	}

	// No worktree directory — recovery should recreate it.
	recovered, err := ensureReviewerWorktree(tmpDir, bb, "task-1", "code-reviewer-1")
	if err != nil {
		t.Fatalf("Expected successful recovery, got error: %v", err)
	}
	if !recovered {
		t.Error("Expected recovered=true")
	}

	// Verify the post-worktree command ran in the recovered worktree.
	markerPath := filepath.Join(tmpDir, paths.WorktreesDirName, "task-1", ".post-worktree-ran")
	if _, statErr := os.Stat(markerPath); os.IsNotExist(statErr) {
		t.Error("Post-worktree command did not run after worktree recovery: marker file missing")
	}
}

func TestEnsureReviewerWorktree_MissingRecoverable_CopyWorktreeEnvFilesBeforePostWorktreeCmd(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	commitReviewerEnvIgnoreForWorktreeTest(t, tmpDir)
	writeReviewerRootFile(t, tmpDir, ".env", "ROOT_ENV=1\n")
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Config.CopyWorktreeEnvFiles = true
	postCmd := "test -f .env && touch ../reviewer-saw-env"
	state.Config.PostWorktreeCmd = &postCmd
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	state.Tasks = []models.Task{task}
	bb := testhelpers.WriteInitialState(t, statePath, state)

	branchName := paths.TaskBranchPrefix + "task-1"
	testhelpers.MustGit(t, tmpDir, "branch", branchName)

	recovered, err := ensureReviewerWorktree(tmpDir, bb, "task-1", "code-reviewer-1")
	if err != nil {
		t.Fatalf("Expected successful recovery, got error: %v", err)
	}
	if !recovered {
		t.Error("Expected recovered=true")
	}

	wtPath := filepath.Join(tmpDir, paths.WorktreesDirName, "task-1")
	assertReviewerWorktreeFileExists(t, wtPath, ".env")
	assertReviewerWorktreeFileExists(t, filepath.Join(tmpDir, paths.WorktreesDirName), "reviewer-saw-env")
	assertReviewerWorktreePathIgnored(t, wtPath, ".env")
	assertReviewerGitStatusClean(t, wtPath)
}

func TestEnsureReviewerWorktree_MissingRecoverable_RefreshesScipAfterPostWorktreeCmd(t *testing.T) {
	t.Setenv(scipsearch.EnvEnableScipSearch, "true")

	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	commitGoModuleForReviewerScip(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Config.ScipSearch = []string{"go"}
	// Keep the ordering marker outside the recovered worktree so clean-status
	// assertions isolate scip-generated files from post-command side effects.
	postCmd := "touch ../post-worktree-ran"
	state.Config.PostWorktreeCmd = &postCmd
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	state.Tasks = []models.Task{task}
	bb := testhelpers.WriteInitialState(t, statePath, state)

	branchName := paths.TaskBranchPrefix + "task-1"
	testhelpers.MustGit(t, tmpDir, "branch", branchName)

	var calls []scipsearch.RuntimeCommandPlan
	markerSeenByIndexer := false
	restoreRefresh := replaceReviewerWorktreeScipRefreshForTest(func(opts scipsearch.RefreshOptions) (scipsearch.RefreshResult, error) {
		opts.Runner = func(plan scipsearch.RuntimeCommandPlan) (string, error) {
			calls = append(calls, plan)
			if _, err := os.Stat(filepath.Join(plan.Dir, "..", "post-worktree-ran")); err == nil {
				markerSeenByIndexer = true
			}
			return writeReviewerScipIndex(plan, []byte("indexed after post-worktree\n"))
		}
		return scipsearch.RefreshIndexes(opts)
	})
	defer restoreRefresh()

	recovered, err := ensureReviewerWorktree(tmpDir, bb, "task-1", "code-reviewer-1")
	if err != nil {
		t.Fatalf("Expected successful recovery, got error: %v", err)
	}
	if !recovered {
		t.Error("Expected recovered=true")
	}

	wtPath := filepath.Join(tmpDir, paths.WorktreesDirName, "task-1")
	if !markerSeenByIndexer {
		t.Fatal("scip indexer ran before PostWorktreeCmd marker existed")
	}
	if len(calls) != 1 {
		t.Fatalf("indexer calls = %#v, want one go refresh", calls)
	}
	wantIndexPath := filepath.Join(wtPath, ".liza", "scip", "go.scip")
	if calls[0].Language != "go" || calls[0].Dir != wtPath || calls[0].OutputPath != wantIndexPath {
		t.Fatalf("indexer call = %#v, want go plan for recovered worktree", calls[0])
	}

	available := availableReviewerScipIndexes(t, wtPath, []string{"go"})
	if len(available) != 1 || available[0].Language != "go" || available[0].Path != wantIndexPath {
		t.Fatalf("AvailableIndexes() = %#v, want go index at %s", available, wantIndexPath)
	}
	if status := testhelpers.MustGit(t, wtPath, "status", "--porcelain"); status != "" {
		t.Fatalf("git status --porcelain = %q, want clean", status)
	}
}

func TestEnsureReviewerWorktree_Exists_DoesNotRefreshScip(t *testing.T) {
	t.Setenv(scipsearch.EnvEnableScipSearch, "true")

	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Config.ScipSearch = []string{"go"}
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	state.Tasks = []models.Task{task}
	bb := testhelpers.WriteInitialState(t, statePath, state)

	testhelpers.CreateTestWorktree(t, tmpDir, "task-1")

	refreshCalled := false
	restoreRefresh := replaceReviewerWorktreeScipRefreshForTest(func(scipsearch.RefreshOptions) (scipsearch.RefreshResult, error) {
		refreshCalled = true
		return scipsearch.RefreshResult{}, nil
	})
	defer restoreRefresh()

	recovered, err := ensureReviewerWorktree(tmpDir, bb, "task-1", "code-reviewer-1")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if recovered {
		t.Error("Expected recovered=false when worktree exists")
	}
	if refreshCalled {
		t.Fatal("existing reviewer worktree triggered redundant scip refresh")
	}
}

func TestEnsureReviewerWorktree_MissingRecoverable_ScipFailureWarningOnlyAndOmitsIndex(t *testing.T) {
	t.Setenv(scipsearch.EnvEnableScipSearch, "true")

	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	commitGoModuleForReviewerScip(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Config.ScipSearch = []string{"go"}
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	state.Tasks = []models.Task{task}
	bb := testhelpers.WriteInitialState(t, statePath, state)

	branchName := paths.TaskBranchPrefix + "task-1"
	testhelpers.MustGit(t, tmpDir, "branch", branchName)

	logs := captureAgentLogs(t)
	restoreRefresh := replaceReviewerWorktreeScipRefreshForTest(func(opts scipsearch.RefreshOptions) (scipsearch.RefreshResult, error) {
		opts.Runner = func(scipsearch.RuntimeCommandPlan) (string, error) {
			return "compiler exploded", errors.New("scip-go failed")
		}
		return scipsearch.RefreshIndexes(opts)
	})
	defer restoreRefresh()

	recovered, err := ensureReviewerWorktree(tmpDir, bb, "task-1", "code-reviewer-1")
	if err != nil {
		t.Fatalf("Expected warning-only scip failure, got error: %v", err)
	}
	if !recovered {
		t.Error("Expected recovered=true")
	}

	wtPath := filepath.Join(tmpDir, paths.WorktreesDirName, "task-1")
	available := availableReviewerScipIndexes(t, wtPath, []string{"go"})
	if len(available) != 0 {
		t.Fatalf("AvailableIndexes() = %#v, want failed go language omitted", available)
	}
	logOutput := logs.String()
	if !strings.Contains(logOutput, "scip-search indexer failed after worktree recovery") ||
		!strings.Contains(logOutput, "language=go") ||
		!strings.Contains(logOutput, "scip-go failed") {
		t.Fatalf("logs = %q, want warning with failed go indexer diagnostic", logOutput)
	}
	if status := testhelpers.MustGit(t, wtPath, "status", "--porcelain"); status != "" {
		t.Fatalf("git status --porcelain = %q, want clean", status)
	}
}

func TestEnsureReviewerWorktreeRecoveryPreparesSembleIgnore(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	state := testhelpers.CreateValidState()
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	state.Tasks = []models.Task{task}
	bb := testhelpers.WriteInitialState(t, statePath, state)

	branchName := paths.TaskBranchPrefix + "task-1"
	testhelpers.MustGit(t, tmpDir, "branch", branchName)

	recovered, err := ensureReviewerWorktree(tmpDir, bb, "task-1", "code-reviewer-1")
	if err != nil {
		t.Fatalf("ensureReviewerWorktree() error = %v", err)
	}
	if !recovered {
		t.Fatal("ensureReviewerWorktree() recovered = false, want true")
	}

	wtPath := filepath.Join(tmpDir, paths.WorktreesDirName, "task-1")
	ignorePath := filepath.Join(wtPath, ".sembleignore")
	content, err := os.ReadFile(ignorePath)
	if err != nil {
		t.Fatalf("ReadFile(.sembleignore) error = %v", err)
	}
	lines := nonEmptyTestLines(string(content))
	if !reflect.DeepEqual(lines, semble.DefaultIgnorePatterns()) {
		t.Fatalf(".sembleignore lines = %#v, want DefaultIgnorePatterns()", lines)
	}
	if status := testhelpers.MustGit(t, wtPath, "status", "--porcelain"); status != "" {
		t.Fatalf("git status --porcelain = %q, want clean", status)
	}
}

func TestEnsureReviewerWorktreeRecoveryRunsSemblePreparationBeforeIndexRefresh(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	state := testhelpers.CreateValidState()
	postCmd := "touch ../post-worktree-ran"
	state.Config.PostWorktreeCmd = &postCmd
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	state.Tasks = []models.Task{task}
	bb := testhelpers.WriteInitialState(t, statePath, state)

	branchName := paths.TaskBranchPrefix + "task-1"
	testhelpers.MustGit(t, tmpDir, "branch", branchName)

	semblePreparedBeforeRefresh := false
	postCommandRanBeforeRefresh := false
	historyRecordedBeforeRefresh := false
	restoreRefresh := replaceReviewerWorktreeScipRefreshForTest(func(opts scipsearch.RefreshOptions) (scipsearch.RefreshResult, error) {
		if _, err := os.Stat(filepath.Join(opts.TargetRoot, ".sembleignore")); err == nil {
			semblePreparedBeforeRefresh = true
		}
		if _, err := os.Stat(filepath.Join(opts.TargetRoot, "..", "post-worktree-ran")); err == nil {
			postCommandRanBeforeRefresh = true
		}
		readState, err := bb.Read()
		if err != nil {
			t.Fatalf("bb.Read() error = %v", err)
		}
		if hasWorktreeRecoveredHistory(readState.FindTask("task-1")) {
			historyRecordedBeforeRefresh = true
		}
		return scipsearch.RefreshResult{}, nil
	})
	defer restoreRefresh()

	recovered, err := ensureReviewerWorktree(tmpDir, bb, "task-1", "code-reviewer-1")
	if err != nil {
		t.Fatalf("ensureReviewerWorktree() error = %v", err)
	}
	if !recovered {
		t.Fatal("ensureReviewerWorktree() recovered = false, want true")
	}
	if !postCommandRanBeforeRefresh {
		t.Fatal("SCIP refresh ran before post-worktree command marker existed")
	}
	if !semblePreparedBeforeRefresh {
		t.Fatal("SCIP refresh ran before Semble .sembleignore preparation")
	}
	if historyRecordedBeforeRefresh {
		t.Fatal("recovery history was recorded before Semble preparation and index refresh")
	}
}

func TestEnsureReviewerWorktreeRecoveryLogsBoundedSembleWarning(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	if err := os.WriteFile(filepath.Join(tmpDir, ".sembleignore"), []byte(".liza/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, tmpDir, "add", ".sembleignore")
	testhelpers.MustGit(t, tmpDir, "commit", "-m", "Add operator Semble ignore")

	state := testhelpers.CreateValidState()
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	state.Tasks = []models.Task{task}
	bb := testhelpers.WriteInitialState(t, statePath, state)

	branchName := paths.TaskBranchPrefix + "task-1"
	testhelpers.MustGit(t, tmpDir, "branch", branchName)

	logs := captureAgentLogs(t)
	recovered, err := ensureReviewerWorktree(tmpDir, bb, "task-1", "code-reviewer-1")
	if err != nil {
		t.Fatalf("ensureReviewerWorktree() error = %v", err)
	}
	if !recovered {
		t.Fatal("ensureReviewerWorktree() recovered = false, want true")
	}

	logOutput := logs.String()
	if !strings.Contains(logOutput, "semble .sembleignore preparation warning after worktree recovery") ||
		!strings.Contains(logOutput, "tracked .sembleignore missing required patterns") ||
		!strings.Contains(logOutput, "(+") {
		t.Fatalf("logs = %q, want bounded Semble missing-pattern warning", logOutput)
	}
	if len(logOutput) > 1200 {
		t.Fatalf("logs length = %d, want bounded warning output", len(logOutput))
	}
}

func TestEnsureReviewerWorktreeRecoveryLeavesTrackedSembleIgnoreUnmodified(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	operatorContent := []byte("# operator-owned\ncustom-pattern\n")
	if err := os.WriteFile(filepath.Join(tmpDir, ".sembleignore"), operatorContent, 0o644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, tmpDir, "add", ".sembleignore")
	testhelpers.MustGit(t, tmpDir, "commit", "-m", "Add operator Semble ignore")

	state := testhelpers.CreateValidState()
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	state.Tasks = []models.Task{task}
	bb := testhelpers.WriteInitialState(t, statePath, state)

	branchName := paths.TaskBranchPrefix + "task-1"
	testhelpers.MustGit(t, tmpDir, "branch", branchName)

	logs := captureAgentLogs(t)
	recovered, err := ensureReviewerWorktree(tmpDir, bb, "task-1", "code-reviewer-1")
	if err != nil {
		t.Fatalf("ensureReviewerWorktree() error = %v", err)
	}
	if !recovered {
		t.Fatal("ensureReviewerWorktree() recovered = false, want true")
	}

	wtPath := filepath.Join(tmpDir, paths.WorktreesDirName, "task-1")
	got, err := os.ReadFile(filepath.Join(wtPath, ".sembleignore"))
	if err != nil {
		t.Fatalf("ReadFile(.sembleignore) error = %v", err)
	}
	if !bytes.Equal(got, operatorContent) {
		t.Fatalf("tracked .sembleignore mutated:\ngot  %q\nwant %q", got, operatorContent)
	}
	if !strings.Contains(logs.String(), "tracked .sembleignore missing required patterns") {
		t.Fatalf("logs = %q, want explicit missing-pattern warning", logs.String())
	}
	if status := testhelpers.MustGit(t, wtPath, "status", "--porcelain"); status != "" {
		t.Fatalf("git status --porcelain = %q, want clean tracked operator file", status)
	}
}

func TestEnsureReviewerWorktreeRecoveryExistingWorktreeFastPathUnchanged(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Config.ScipSearch = []string{"go"}
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	state.Tasks = []models.Task{task}
	bb := testhelpers.WriteInitialState(t, statePath, state)

	testhelpers.CreateTestWorktree(t, tmpDir, "task-1")

	semblePrepared := false
	restorePrepare := replaceReviewerWorktreeSemblePreparationForTest(func(string) []string {
		semblePrepared = true
		return nil
	})
	defer restorePrepare()
	refreshCalled := false
	restoreRefresh := replaceReviewerWorktreeScipRefreshForTest(func(scipsearch.RefreshOptions) (scipsearch.RefreshResult, error) {
		refreshCalled = true
		return scipsearch.RefreshResult{}, nil
	})
	defer restoreRefresh()

	recovered, err := ensureReviewerWorktree(tmpDir, bb, "task-1", "code-reviewer-1")
	if err != nil {
		t.Fatalf("ensureReviewerWorktree() error = %v", err)
	}
	if recovered {
		t.Fatal("ensureReviewerWorktree() recovered = true, want false")
	}
	if semblePrepared {
		t.Fatal("existing reviewer worktree triggered Semble preparation")
	}
	if refreshCalled {
		t.Fatal("existing reviewer worktree triggered SCIP refresh")
	}
}

func TestEnsureReviewerWorktree_MissingAlreadyRecovered(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	state := testhelpers.CreateValidState()
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	// Simulate reviewer claim.
	reviewerID := "code-reviewer-1"
	task.ReviewingBy = &reviewerID
	// Simulate a prior recovery.
	task.History = append(task.History, models.TaskHistoryEntry{
		Time:  now.Add(-5 * time.Minute),
		Event: models.TaskEventWorktreeRecovered,
		Agent: &reviewerID,
	})
	state.Tasks = []models.Task{task}
	// Register the reviewer agent so ReleaseAgent can reset it.
	taskPtr := "task-1"
	state.Agents[reviewerID] = models.Agent{
		Role:        models.RoleCodeReviewer,
		Status:      models.AgentStatusReviewing,
		CurrentTask: &taskPtr,
	}
	bb := testhelpers.WriteInitialState(t, statePath, state)

	// No worktree and already recovered once — should block.
	recovered, err := ensureReviewerWorktree(tmpDir, bb, "task-1", reviewerID)
	if err == nil {
		t.Fatal("Expected error for already-recovered task")
	}
	if recovered {
		t.Error("Expected recovered=false")
	}
	if !errors.Is(err, errTaskBlocked) {
		t.Errorf("Expected errTaskBlocked, got: %v", err)
	}

	// Verify task was blocked in state.
	readState, _ := bb.Read()
	readTask := readState.FindTask("task-1")
	if readTask == nil {
		t.Fatal("Task not found")
	}
	if readTask.Status != models.TaskStatusBlocked {
		t.Errorf("Expected BLOCKED status, got %s", readTask.Status)
	}

	// Verify reviewer agent was released to IDLE.
	agent, exists := readState.Agents[reviewerID]
	if !exists {
		t.Fatal("Agent not found in state")
	}
	if agent.Status != models.AgentStatusIdle {
		t.Errorf("Expected agent status IDLE, got %s", agent.Status)
	}
	if agent.CurrentTask != nil {
		t.Errorf("Expected agent CurrentTask nil, got %v", agent.CurrentTask)
	}
}

func replaceReviewerWorktreeScipRefreshForTest(refresh func(scipsearch.RefreshOptions) (scipsearch.RefreshResult, error)) func() {
	previous := reviewerWorktreeRefreshIndexes
	reviewerWorktreeRefreshIndexes = refresh
	return func() {
		reviewerWorktreeRefreshIndexes = previous
	}
}

func replaceReviewerWorktreeSemblePreparationForTest(prepare func(string) []string) func() {
	previous := reviewerWorktreePrepareSembleIgnore
	reviewerWorktreePrepareSembleIgnore = prepare
	return func() {
		reviewerWorktreePrepareSembleIgnore = previous
	}
}

func commitGoModuleForReviewerScip(t *testing.T, repoDir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte("module example.com/reviewer\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, repoDir, "add", "go.mod", "main.go")
	testhelpers.MustGit(t, repoDir, "commit", "-m", "Add Go module")
}

func writeReviewerScipIndex(plan scipsearch.RuntimeCommandPlan, content []byte) (string, error) {
	if err := os.MkdirAll(filepath.Dir(plan.OutputPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(plan.OutputPath, content, 0o644); err != nil {
		return "", err
	}
	return "indexed", nil
}

func availableReviewerScipIndexes(t *testing.T, worktreeDir string, languages []string) []scipsearch.IndexRef {
	t.Helper()
	indexes, err := scipsearch.AvailableIndexes(scipsearch.RuntimePlanOptions{
		TargetRoot:          worktreeDir,
		ConfiguredLanguages: languages,
	})
	if err != nil {
		t.Fatalf("AvailableIndexes() error: %v", err)
	}
	return indexes
}

func captureAgentLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logs bytes.Buffer
	previous := logger
	logger = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}))
	t.Cleanup(func() {
		logger = previous
	})
	return &logs
}

func nonEmptyTestLines(content string) []string {
	lines := make([]string, 0)
	for _, line := range strings.Split(content, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func hasWorktreeRecoveredHistory(task *models.Task) bool {
	if task == nil {
		return false
	}
	for _, entry := range task.History {
		if entry.Event == models.TaskEventWorktreeRecovered {
			return true
		}
	}
	return false
}

func TestEnsureReviewerWorktree_MissingBranchGone(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	state := testhelpers.CreateValidState()
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	reviewerID := "code-reviewer-1"
	task.ReviewingBy = &reviewerID
	state.Tasks = []models.Task{task}
	taskPtr := "task-1"
	state.Agents[reviewerID] = models.Agent{
		Role:        models.RoleCodeReviewer,
		Status:      models.AgentStatusReviewing,
		CurrentTask: &taskPtr,
	}
	bb := testhelpers.WriteInitialState(t, statePath, state)

	// No worktree AND no branch — unrecoverable.
	recovered, err := ensureReviewerWorktree(tmpDir, bb, "task-1", reviewerID)
	if err == nil {
		t.Fatal("Expected error when branch is missing")
	}
	if recovered {
		t.Error("Expected recovered=false")
	}
	if !errors.Is(err, errTaskBlocked) {
		t.Errorf("Expected errTaskBlocked, got: %v", err)
	}

	// Verify task was blocked.
	readState, _ := bb.Read()
	readTask := readState.FindTask("task-1")
	if readTask == nil {
		t.Fatal("Task not found")
	}
	if readTask.Status != models.TaskStatusBlocked {
		t.Errorf("Expected BLOCKED status, got %s", readTask.Status)
	}

	// Verify reviewer agent was released to IDLE.
	agent, exists := readState.Agents[reviewerID]
	if !exists {
		t.Fatal("Agent not found in state")
	}
	if agent.Status != models.AgentStatusIdle {
		t.Errorf("Expected agent status IDLE, got %s", agent.Status)
	}
}

func commitReviewerEnvIgnoreForWorktreeTest(t *testing.T, projectRoot string) {
	t.Helper()
	ignore := ".env\n.env.*\n*.env\n.envrc\n"
	if err := os.WriteFile(filepath.Join(projectRoot, ".gitignore"), []byte(ignore), 0o644); err != nil {
		t.Fatalf("WriteFile(.gitignore) error: %v", err)
	}
	testhelpers.MustGit(t, projectRoot, "add", ".gitignore")
	testhelpers.MustGit(t, projectRoot, "commit", "-m", "Ignore env files")
	testhelpers.MustGit(t, projectRoot, "branch", "-f", "integration", "HEAD")
}

func writeReviewerRootFile(t *testing.T, projectRoot, rel, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(projectRoot, rel), []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error: %v", rel, err)
	}
}

func assertReviewerWorktreePathIgnored(t *testing.T, worktreeDir, rel string) {
	t.Helper()
	cmd := exec.Command("git", "-C", worktreeDir, "check-ignore", "--quiet", "--", rel)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git check-ignore %s in %s failed: %v: %s", rel, worktreeDir, err, output)
	}
}

func assertReviewerWorktreeFileExists(t *testing.T, worktreeDir, rel string) {
	t.Helper()
	path := filepath.Join(worktreeDir, rel)
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("Lstat(%s) error: %v", path, err)
	}
}

func assertReviewerGitStatusClean(t *testing.T, worktreeDir string) {
	t.Helper()
	cmd := exec.Command("git", "-C", worktreeDir, "status", "--porcelain")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status --porcelain in %s failed: %v: %s", worktreeDir, err, output)
	}
	if strings.TrimSpace(string(output)) != "" {
		t.Fatalf("git status --porcelain = %q, want clean", output)
	}
}
