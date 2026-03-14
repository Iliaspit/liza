package ops

import (
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestSubmitForReview_Validation(t *testing.T) {
	tests := []struct {
		name        string
		taskID      string
		commitSHA   string
		agentID     string
		errContains string
	}{
		{
			name: "empty task ID", commitSHA: "abc123", agentID: "coder-1",
			errContains: "task ID is required",
		},
		{
			name: "empty commit SHA", taskID: "t1", agentID: "coder-1",
			errContains: "commit SHA is required",
		},
		{
			name: "empty agent ID", taskID: "t1", commitSHA: "abc123",
			errContains: "LIZA_AGENT_ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SubmitForReview("/nonexistent", tt.taskID, tt.commitSHA, tt.agentID)
			testhelpers.RequireErrorContains(t, err, tt.errContains)
		})
	}
}

func TestSubmitForReview_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := SubmitForReview(tmpDir, "nonexistent", "abc123", "coder-1")
	if err == nil {
		t.Fatal("Expected error for nonexistent task")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestSubmitForReview_WrongStatus(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := SubmitForReview(tmpDir, "task-1", "abc123", "coder-1")
	testhelpers.RequireErrorContains(t, err, "not IMPLEMENTING")
}

func TestSubmitForReview_WrongAgent(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := SubmitForReview(tmpDir, "task-1", "abc123", "coder-2")
	testhelpers.RequireErrorContains(t, err, "not assigned to agent")
}

func TestSubmitForReview_NoWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
	task.Worktree = nil // No worktree
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := SubmitForReview(tmpDir, "task-1", "abc123", "coder-1")
	testhelpers.RequireErrorContains(t, err, "no worktree")
}

func TestSubmitForReview_TDDWaiverBypassesTestRequirement(t *testing.T) {
	// Unit test: verify that GetTDDWaiver check in SubmitForReview
	// allows submission without test files when waiver is declared.
	// This tests the waiver logic at the data level since the full
	// SubmitForReview path requires a real git worktree.
	agent := "coder-1"
	history := []models.TaskHistoryEntry{
		{
			Event: models.TaskEventPreExecutionCheckpoint,
			Agent: &agent,
			Extra: map[string]any{
				"intent":           "Fix comment typo",
				"tdd_not_required": "cosmetic-only: comment fix, no behavior change",
			},
		},
	}

	// With waiver, GetTDDWaiver should return non-empty
	waiver := GetTDDWaiver(history, "coder-1")
	if waiver == "" {
		t.Fatal("Expected non-empty waiver from checkpoint with tdd_not_required")
	}
	if waiver != "cosmetic-only: comment fix, no behavior change" {
		t.Errorf("Unexpected waiver value: %q", waiver)
	}

	// Without waiver, GetTDDWaiver should return empty
	historyNoWaiver := []models.TaskHistoryEntry{
		{
			Event: models.TaskEventPreExecutionCheckpoint,
			Agent: &agent,
			Extra: map[string]any{
				"intent": "Add feature",
			},
		},
	}
	if GetTDDWaiver(historyNoWaiver, "coder-1") != "" {
		t.Fatal("Expected empty waiver from checkpoint without tdd_not_required")
	}
}

func TestSubmitForReview_TDDDisabled_NoTestFiles_Succeeds(t *testing.T) {
	// Behavior test: when TDDEnabled is false, SubmitForReview must succeed
	// for a code task with no test files. This exercises the actual gate
	// at submit_review.go:124, not just the Config helper.
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	testhelpers.MustGit(t, tmpDir, "checkout", "integration")

	g := git.New(tmpDir)
	taskID := "task-tdd-disabled"
	baseCommit, err := g.CreateWorktree(taskID, "integration")
	if err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}
	wtPath := g.GetWorktreePath(taskID)

	// Add only implementation files — NO test files
	if err := os.WriteFile(filepath.Join(wtPath, "feature.go"), []byte("package main\n\nfunc feature() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, wtPath, "add", "feature.go")
	testhelpers.MustGit(t, wtPath, "commit", "-m", "Add feature without tests")
	wtCommit := testhelpers.MustGit(t, wtPath, "rev-parse", "HEAD")

	agentID := "coder-1"
	leaseExpires := time.Now().UTC().Add(30 * time.Minute)
	worktree := g.GetWorktreeRelPath(taskID)
	tddEnabled := false
	initialState := &models.State{
		Config: models.Config{
			IntegrationBranch: "integration",
			LeaseDuration:     1800,
			TDDEnabled:        &tddEnabled,
		},
		Tasks: []models.Task{
			{
				ID:           taskID,
				Description:  "Feature with TDD disabled",
				Status:       models.TaskStatusImplementing,
				RolePair:     "coding-pair",
				AssignedTo:   &agentID,
				LeaseExpires: &leaseExpires,
				Worktree:     &worktree,
				BaseCommit:   &baseCommit,
				Iteration:    1,
				Created:      time.Now().UTC(),
				History: []models.TaskHistoryEntry{
					{
						Time:  time.Now().UTC(),
						Event: models.TaskEventPreExecutionCheckpoint,
						Agent: &agentID,
						Extra: map[string]any{
							"intent":          "Add feature",
							"validation_plan": "go build",
							"files_to_modify": []string{"feature.go"},
						},
					},
				},
			},
		},
		Agents: map[string]models.Agent{
			agentID: {Status: models.AgentStatusWorking, CurrentTask: &taskID},
		},
	}

	testhelpers.WriteInitialState(t, statePath, initialState)

	result, err := SubmitForReview(tmpDir, taskID, wtCommit, agentID)
	if err != nil {
		t.Fatalf("SubmitForReview() with TDD disabled should succeed without test files, got error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.TaskID != taskID {
		t.Errorf("expected TaskID %q, got %q", taskID, result.TaskID)
	}
}

func TestSubmitForReview_TDDEnabled_NoTestFiles_Fails(t *testing.T) {
	// Companion behavior test: with TDD enabled (default), the same scenario
	// must fail with a precondition error about missing test files.
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	testhelpers.MustGit(t, tmpDir, "checkout", "integration")

	g := git.New(tmpDir)
	taskID := "task-tdd-enabled"
	baseCommit, err := g.CreateWorktree(taskID, "integration")
	if err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}
	wtPath := g.GetWorktreePath(taskID)

	// Add only implementation files — NO test files
	if err := os.WriteFile(filepath.Join(wtPath, "feature.go"), []byte("package main\n\nfunc feature() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, wtPath, "add", "feature.go")
	testhelpers.MustGit(t, wtPath, "commit", "-m", "Add feature without tests")
	wtCommit := testhelpers.MustGit(t, wtPath, "rev-parse", "HEAD")

	agentID := "coder-1"
	leaseExpires := time.Now().UTC().Add(30 * time.Minute)
	worktree := g.GetWorktreeRelPath(taskID)
	// TDDEnabled is nil (default = true)
	initialState := &models.State{
		Config: models.Config{
			IntegrationBranch: "integration",
			LeaseDuration:     1800,
		},
		Tasks: []models.Task{
			{
				ID:           taskID,
				Description:  "Feature with TDD enabled",
				Status:       models.TaskStatusImplementing,
				RolePair:     "coding-pair",
				AssignedTo:   &agentID,
				LeaseExpires: &leaseExpires,
				Worktree:     &worktree,
				BaseCommit:   &baseCommit,
				Iteration:    1,
				Created:      time.Now().UTC(),
				History: []models.TaskHistoryEntry{
					{
						Time:  time.Now().UTC(),
						Event: models.TaskEventPreExecutionCheckpoint,
						Agent: &agentID,
						Extra: map[string]any{
							"intent":          "Add feature",
							"validation_plan": "go build",
							"files_to_modify": []string{"feature.go"},
						},
					},
				},
			},
		},
		Agents: map[string]models.Agent{
			agentID: {Status: models.AgentStatusWorking, CurrentTask: &taskID},
		},
	}

	testhelpers.WriteInitialState(t, statePath, initialState)

	_, err = SubmitForReview(tmpDir, taskID, wtCommit, agentID)
	if err == nil {
		t.Fatal("SubmitForReview() with TDD enabled should fail without test files")
	}
	testhelpers.RequireErrorContains(t, err, "code tasks must include test files")
}

func TestSubmitForReview_NoCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	// Task has worktree but no checkpoint in history
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := SubmitForReview(tmpDir, "task-1", "abc123", "coder-1")
	testhelpers.RequireErrorContains(t, err, "pre-execution checkpoint required")
}

// setupRebaseConflictScenario creates a git repo with a worktree whose branch
// conflicts with integration. Returns (tmpDir, taskID, worktreeCommitSHA, agentID, blackboard).
func setupRebaseConflictScenario(t *testing.T) (string, string, string, string, *db.Blackboard) {
	t.Helper()

	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	testhelpers.MustGit(t, tmpDir, "checkout", "integration")

	g := git.New(tmpDir)
	taskID := "task-rebase-conflict"
	baseCommit, err := g.CreateWorktree(taskID, "integration")
	if err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}
	wtPath := g.GetWorktreePath(taskID)

	// Modify README in worktree (will conflict) and add test file for TDD
	if err := os.WriteFile(filepath.Join(wtPath, "README.md"), []byte("# Task version\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "task_test.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, wtPath, "add", "README.md", "task_test.go")
	testhelpers.MustGit(t, wtPath, "commit", "-m", "Task commit")
	wtCommit := testhelpers.MustGit(t, wtPath, "rev-parse", "HEAD")

	// Create conflicting change on integration branch
	testhelpers.MustGit(t, tmpDir, "checkout", "integration")
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Integration version\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, tmpDir, "add", "README.md")
	testhelpers.MustGit(t, tmpDir, "commit", "-m", "Integration commit")

	agentID := "coder-1"
	leaseExpires := time.Now().UTC().Add(30 * time.Minute)
	worktree := g.GetWorktreeRelPath(taskID)
	initialState := &models.State{
		Config: models.Config{
			IntegrationBranch: "integration",
			LeaseDuration:     1800,
		},
		Tasks: []models.Task{
			{
				ID:           taskID,
				Description:  "Task with rebase conflict",
				Status:       models.TaskStatusImplementing,
				RolePair:     "coding-pair",
				AssignedTo:   &agentID,
				LeaseExpires: &leaseExpires,
				Worktree:     &worktree,
				BaseCommit:   &baseCommit,
				Iteration:    1,
				Created:      time.Now().UTC(),
				History: []models.TaskHistoryEntry{
					{
						Time:  time.Now().UTC(),
						Event: models.TaskEventPreExecutionCheckpoint,
						Agent: &agentID,
						Extra: map[string]any{
							"intent":          "test",
							"validation_plan": "test",
							"files_to_modify": []string{"README.md"},
						},
					},
				},
			},
		},
		Agents: map[string]models.Agent{
			agentID: {Status: models.AgentStatusWorking, CurrentTask: &taskID},
		},
	}

	bb := testhelpers.WriteInitialState(t, statePath, initialState)
	return tmpDir, taskID, wtCommit, agentID, bb
}

func TestSubmitForReview_RebaseConflict_TransitionsToIntegrationFailed(t *testing.T) {
	tmpDir, taskID, wtCommit, agentID, bb := setupRebaseConflictScenario(t)

	_, err := SubmitForReview(tmpDir, taskID, wtCommit, agentID)
	if err == nil {
		t.Fatal("expected error due to rebase conflict, got nil")
	}

	// Should return IntegrationFailedError
	var ifErr *IntegrationFailedError
	if !stderrors.As(err, &ifErr) {
		t.Fatalf("expected *IntegrationFailedError, got %T: %v", err, err)
	}
	if ifErr.Reason != IntegrationReasonMergeConflict {
		t.Errorf("expected reason %q, got %q", IntegrationReasonMergeConflict, ifErr.Reason)
	}

	// Task should be INTEGRATION_FAILED
	state, err := bb.Read()
	testhelpers.AssertNoError(t, err)
	task := state.FindTask(taskID)
	if task.Status != models.TaskStatusIntegrationFailed {
		t.Errorf("expected INTEGRATION_FAILED, got %s", task.Status)
	}

	// Agent should be released
	if task.AssignedTo != nil {
		t.Errorf("expected agent released (AssignedTo nil), got %v", *task.AssignedTo)
	}
	agent := state.Agents[agentID]
	if agent.Status != models.AgentStatusWaiting {
		t.Errorf("expected agent status WAITING, got %s", agent.Status)
	}
	if agent.CurrentTask != nil {
		t.Errorf("expected agent CurrentTask nil, got %v", *agent.CurrentTask)
	}

	// FailedBy should include the agent
	if len(task.FailedBy) == 0 || task.FailedBy[0] != agentID {
		t.Errorf("expected FailedBy to include %s, got %v", agentID, task.FailedBy)
	}

	// History should have integration_failed entry
	found := false
	for _, h := range task.History {
		if h.Event == models.TaskEventIntegrationFailed {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected integration_failed history entry")
	}

	// Worktree should be clean (rebase aborted) — verify by checking branch
	g := git.New(tmpDir)
	wtPath := g.GetWorktreePath(taskID)
	branch, err := g.GetWorktreeBranch(wtPath)
	if err != nil {
		t.Fatalf("failed to get worktree branch: %v", err)
	}
	if branch == "" {
		t.Error("worktree in detached HEAD state — rebase was not aborted")
	}

	// ReviewCommit should NOT be set
	if task.ReviewCommit != nil {
		t.Errorf("expected ReviewCommit nil, got %v", *task.ReviewCommit)
	}
}

// setupSuccessfulSubmitScenario creates a git repo with a worktree that can be
// cleanly rebased onto integration. Returns (tmpDir, taskID, worktreeCommitSHA, agentID, blackboard).
func setupSuccessfulSubmitScenario(t *testing.T) (string, string, string, string, *db.Blackboard) {
	t.Helper()

	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	testhelpers.MustGit(t, tmpDir, "checkout", "integration")

	g := git.New(tmpDir)
	taskID := "task-submit-handoff"
	baseCommit, err := g.CreateWorktree(taskID, "integration")
	if err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}
	wtPath := g.GetWorktreePath(taskID)

	// Add a source file and a test file (TDD requirement)
	if err := os.WriteFile(filepath.Join(wtPath, "feature.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "feature_test.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, wtPath, "add", "feature.go", "feature_test.go")
	testhelpers.MustGit(t, wtPath, "commit", "-m", "Add feature with tests")
	wtCommit := testhelpers.MustGit(t, wtPath, "rev-parse", "HEAD")

	agentID := "coder-1"
	leaseExpires := time.Now().UTC().Add(30 * time.Minute)
	worktree := g.GetWorktreeRelPath(taskID)
	initialState := &models.State{
		Config: models.Config{
			IntegrationBranch: "integration",
			LeaseDuration:     1800,
		},
		Tasks: []models.Task{
			{
				ID:           taskID,
				Description:  "Task for handoff event test",
				Status:       models.TaskStatusImplementing,
				RolePair:     "coding-pair",
				AssignedTo:   &agentID,
				LeaseExpires: &leaseExpires,
				Worktree:     &worktree,
				BaseCommit:   &baseCommit,
				Iteration:    1,
				Created:      time.Now().UTC(),
				History: []models.TaskHistoryEntry{
					{
						Time:  time.Now().UTC(),
						Event: models.TaskEventPreExecutionCheckpoint,
						Agent: &agentID,
						Extra: map[string]any{
							"intent":          "test handoff event",
							"validation_plan": "verify handoff event appended",
							"files_to_modify": []string{"feature.go"},
						},
					},
				},
			},
		},
		Agents: map[string]models.Agent{
			agentID: {Status: models.AgentStatusWorking, CurrentTask: &taskID},
		},
	}

	bb := testhelpers.WriteInitialState(t, statePath, initialState)
	return tmpDir, taskID, wtCommit, agentID, bb
}

func TestSubmitForReview_WritesHandoffEvent(t *testing.T) {
	tmpDir, taskID, wtCommit, agentID, bb := setupSuccessfulSubmitScenario(t)

	result, err := SubmitForReview(tmpDir, taskID, wtCommit, agentID)
	if err != nil {
		t.Fatalf("SubmitForReview() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("SubmitForReview() returned nil result")
	}

	// Read state and verify HandoffEvent
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("bb.Read() error: %v", err)
	}
	task := state.FindTask(taskID)
	if task == nil {
		t.Fatal("task not found after submission")
	}

	if len(task.HandoffEvents) != 1 {
		t.Fatalf("expected 1 HandoffEvent, got %d", len(task.HandoffEvents))
	}

	event := task.HandoffEvents[0]
	if event.Trigger != models.HandoffTriggerSubmission {
		t.Errorf("HandoffEvent.Trigger = %q, want %q", event.Trigger, models.HandoffTriggerSubmission)
	}
	if event.Agent != agentID {
		t.Errorf("HandoffEvent.Agent = %q, want %q", event.Agent, agentID)
	}
	if event.Timestamp.IsZero() {
		t.Error("HandoffEvent.Timestamp is zero")
	}
	// Submission is auto-generated: succeeded/failed should be empty
	if len(event.Succeeded) != 0 {
		t.Errorf("HandoffEvent.Succeeded should be empty for submission, got %v", event.Succeeded)
	}
	if len(event.Failed) != 0 {
		t.Errorf("HandoffEvent.Failed should be empty for submission, got %v", event.Failed)
	}
}

// TestSubmitForReview_TDDEnforcement_CustomDoerRole verifies that TDD enforcement
// applies to any doer role (not just the literal "coder" role) by using a custom
// pipeline config with a custom doer role name.
func TestSubmitForReview_TDDEnforcement_CustomDoerRole(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	// Overwrite the default pipeline config with one that defines a custom doer role.
	customPipeline := `pipeline:
  roles:
    custom-doer:
      type: doer
      display-name: "Custom Doer"
      allowed-operations:
        - write-checkpoint
        - submit-for-review
        - mark-blocked
        - handoff
    custom-reviewer:
      type: reviewer
      display-name: "Custom Reviewer"
      allowed-operations:
        - submit-verdict
  role-pairs:
    custom-pair:
      doer: custom-doer
      reviewer: custom-reviewer
      states:
        initial: CUSTOM_READY
        executing: CUSTOM_EXECUTING
        submitted: CUSTOM_SUBMITTED
        reviewing: CUSTOM_REVIEWING
        approved: CUSTOM_APPROVED
        rejected: CUSTOM_REJECTED
  sub-pipelines: {}
  pipeline-transitions: []
  entry-points: {}
`
	if err := os.WriteFile(filepath.Join(tmpDir, ".liza", "pipeline.yaml"), []byte(customPipeline), 0644); err != nil {
		t.Fatalf("Failed to write custom pipeline config: %v", err)
	}

	testhelpers.MustGit(t, tmpDir, "checkout", "integration")

	g := git.New(tmpDir)
	taskID := "task-custom-doer-tdd"
	baseCommit, err := g.CreateWorktree(taskID, "integration")
	if err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}
	wtPath := g.GetWorktreePath(taskID)

	// Add a non-test file only (no test files — should trigger TDD enforcement).
	if err := os.WriteFile(filepath.Join(wtPath, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, wtPath, "add", "main.go")
	testhelpers.MustGit(t, wtPath, "commit", "-m", "Non-test commit")
	wtCommit := testhelpers.MustGit(t, wtPath, "rev-parse", "HEAD")

	agentID := "custom-doer-1"
	leaseExpires := time.Now().UTC().Add(30 * time.Minute)
	worktree := g.GetWorktreeRelPath(taskID)
	initialState := &models.State{
		Config: models.Config{
			IntegrationBranch: "integration",
			LeaseDuration:     1800,
		},
		Tasks: []models.Task{
			{
				ID:           taskID,
				Type:         models.TaskTypeCoding,
				Description:  "Custom doer TDD enforcement test",
				Status:       "CUSTOM_EXECUTING",
				RolePair:     "custom-pair",
				AssignedTo:   &agentID,
				LeaseExpires: &leaseExpires,
				Worktree:     &worktree,
				BaseCommit:   &baseCommit,
				Iteration:    1,
				Created:      time.Now().UTC(),
				History: []models.TaskHistoryEntry{
					{
						Time:  time.Now().UTC(),
						Event: models.TaskEventPreExecutionCheckpoint,
						Agent: &agentID,
						Extra: map[string]any{
							"intent":          "test custom doer TDD",
							"validation_plan": "verify TDD enforcement",
							"files_to_modify": []string{"main.go"},
						},
					},
				},
			},
		},
		Agents: map[string]models.Agent{
			agentID: {Role: "custom-doer", Status: models.AgentStatusWorking, CurrentTask: &taskID},
		},
	}

	testhelpers.WriteInitialState(t, statePath, initialState)

	_, err = SubmitForReview(tmpDir, taskID, wtCommit, agentID)
	if err == nil {
		t.Fatal("Expected TDD enforcement error for custom doer role, got nil")
	}
	testhelpers.RequireErrorContains(t, err, "code tasks must include test files")
}
