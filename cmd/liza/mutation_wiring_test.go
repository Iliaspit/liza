package main

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

func TestMutationCommandWiring(t *testing.T) {
	t.Run("claim-task wires positional args to handler", func(t *testing.T) {
		projectRoot, statePath := setupMutationTestProject(t, func(state *models.State) {
			now := time.Now().UTC()
			state.Tasks = []models.Task{
				testhelpers.BuildTaskByStatus("task-claim-alpha", models.TaskStatusReady, now),
			}
			state.Agents["coder-42"] = testhelpers.RegisteredTestAgent("coder")
		})

		err := executeRootCommand(t, projectRoot, "claim-task", "task-claim-alpha", "coder-42")
		if err != nil {
			t.Fatalf("claim-task execute failed: %v", err)
		}

		state := readState(t, statePath)
		task := mustFindTask(t, state, "task-claim-alpha")
		if task.Status != models.TaskStatusImplementing {
			t.Fatalf("task status = %s, want %s", task.Status, models.TaskStatusImplementing)
		}
		if task.AssignedTo == nil || *task.AssignedTo != "coder-42" {
			t.Fatalf("task assigned_to = %v, want coder-42", task.AssignedTo)
		}

		agent, ok := state.Agents["coder-42"]
		if !ok {
			t.Fatalf("agent coder-42 missing")
		}
		if agent.CurrentTask == nil || *agent.CurrentTask != "task-claim-alpha" {
			t.Fatalf("agent current_task = %v, want task-claim-alpha", agent.CurrentTask)
		}
	})

	t.Run("submit-verdict uses --agent-id flag", func(t *testing.T) {
		projectRoot, statePath := setupMutationTestProject(t, func(state *models.State) {
			now := time.Now().UTC()
			state.Tasks = []models.Task{
				testhelpers.BuildTaskByStatus("task-review-flag", models.TaskStatusReviewing, now),
			}
		})

		err := executeRootCommand(t, projectRoot, "submit-verdict", "task-review-flag", "APPROVED", "--agent-id", "code-reviewer-9")
		if err != nil {
			t.Fatalf("submit-verdict execute failed: %v", err)
		}

		state := readState(t, statePath)
		task := mustFindTask(t, state, "task-review-flag")
		if task.Status != models.TaskStatusApproved {
			t.Fatalf("task status = %s, want %s", task.Status, models.TaskStatusApproved)
		}
		if task.ApprovedBy == nil || *task.ApprovedBy != "code-reviewer-9" {
			t.Fatalf("approved_by = %v, want code-reviewer-9", task.ApprovedBy)
		}
	})

	t.Run("submit-verdict falls back to LIZA_AGENT_ID and forwards reason", func(t *testing.T) {
		projectRoot, statePath := setupMutationTestProject(t, func(state *models.State) {
			now := time.Now().UTC()
			state.Tasks = []models.Task{
				testhelpers.BuildTaskByStatus("task-review-env", models.TaskStatusReviewing, now),
			}
		})

		t.Setenv("LIZA_AGENT_ID", "code-reviewer-8")
		err := executeRootCommand(t, projectRoot, "submit-verdict", "task-review-env", "REJECTED", "needs-work")
		if err != nil {
			t.Fatalf("submit-verdict execute failed: %v", err)
		}

		state := readState(t, statePath)
		task := mustFindTask(t, state, "task-review-env")
		if task.Status != models.TaskStatusRejected {
			t.Fatalf("task status = %s, want %s", task.Status, models.TaskStatusRejected)
		}
		if task.RejectionReason == nil || *task.RejectionReason != "needs-work" {
			t.Fatalf("rejection_reason = %v, want needs-work", task.RejectionReason)
		}
		if len(task.History) == 0 {
			t.Fatalf("expected history entry for verdict")
		}
		last := task.History[len(task.History)-1]
		if last.Event != "rejected" {
			t.Fatalf("history event = %s, want rejected", last.Event)
		}
		if last.Agent == nil || *last.Agent != "code-reviewer-8" {
			t.Fatalf("history agent = %v, want code-reviewer-8", last.Agent)
		}
		if last.Reason == nil || *last.Reason != "needs-work" {
			t.Fatalf("history reason = %v, want needs-work", last.Reason)
		}
	})

	t.Run("submit-verdict --reason flag overrides positional arg", func(t *testing.T) {
		projectRoot, statePath := setupMutationTestProject(t, func(state *models.State) {
			now := time.Now().UTC()
			state.Tasks = []models.Task{
				testhelpers.BuildTaskByStatus("task-reason-flag", models.TaskStatusReviewing, now),
			}
		})

		// Pass both positional reason and --reason flag; flag should win
		t.Setenv("LIZA_AGENT_ID", "code-reviewer-8")
		err := executeRootCommand(t, projectRoot, "submit-verdict", "task-reason-flag", "REJECTED", "positional-reason", "--reason", "---\n# Blockers\nArchitecture plan missing")
		if err != nil {
			t.Fatalf("submit-verdict execute failed: %v", err)
		}

		state := readState(t, statePath)
		task := mustFindTask(t, state, "task-reason-flag")
		if task.RejectionReason == nil || *task.RejectionReason != "---\n# Blockers\nArchitecture plan missing" {
			t.Fatalf("rejection_reason = %v, want markdown content from --reason flag", task.RejectionReason)
		}
	})

	t.Run("wt-merge routes parsed args to merge handler", func(t *testing.T) {
		projectRoot, _ := setupMutationTestProject(t, func(state *models.State) {
			now := time.Now().UTC()
			state.Tasks = []models.Task{
				testhelpers.BuildTaskByStatus("task-wt-merge", models.TaskStatusReady, now),
			}
		})

		err := executeRootCommand(t, projectRoot, "wt-merge", "task-wt-merge", "--agent-id", "code-reviewer-3")
		if err == nil {
			t.Fatalf("expected wt-merge error, got nil")
		}
		if !strings.Contains(err.Error(), "task must be in an approved state to merge (current status: DRAFT_CODE)") {
			t.Fatalf("unexpected wt-merge error: %v", err)
		}
	})

	t.Run("supersede-task with replacements and --reason flag", func(t *testing.T) {
		projectRoot, statePath := setupMutationTestProject(t, func(state *models.State) {
			now := time.Now().UTC()
			state.Tasks = []models.Task{
				testhelpers.BuildTaskByStatus("task-supersede-repl", models.TaskStatusBlocked, now),
			}
		})

		err := executeRootCommand(t, projectRoot, "supersede-task", "task-supersede-repl", "task-new-1,task-new-2", "--reason", "Split into smaller tasks", "--agent-id", "orchestrator-1")
		if err != nil {
			t.Fatalf("supersede-task execute failed: %v", err)
		}

		state := readState(t, statePath)
		task := mustFindTask(t, state, "task-supersede-repl")
		if task.Status != models.TaskStatusSuperseded {
			t.Fatalf("task status = %s, want %s", task.Status, models.TaskStatusSuperseded)
		}
		if len(task.SupersededBy) != 2 || task.SupersededBy[0] != "task-new-1" || task.SupersededBy[1] != "task-new-2" {
			t.Fatalf("superseded_by = %v, want [task-new-1 task-new-2]", task.SupersededBy)
		}
		if task.RescopeReason == nil || *task.RescopeReason != "Split into smaller tasks" {
			t.Fatalf("rescope_reason = %v, want 'Split into smaller tasks'", task.RescopeReason)
		}
	})

	t.Run("supersede-task without replacements", func(t *testing.T) {
		projectRoot, statePath := setupMutationTestProject(t, func(state *models.State) {
			now := time.Now().UTC()
			state.Tasks = []models.Task{
				testhelpers.BuildTaskByStatus("task-supersede-norep", models.TaskStatusBlocked, now),
			}
		})

		err := executeRootCommand(t, projectRoot, "supersede-task", "task-supersede-norep", "--reason", "Work already merged", "--agent-id", "orchestrator-1")
		if err != nil {
			t.Fatalf("supersede-task without replacements failed: %v", err)
		}

		state := readState(t, statePath)
		task := mustFindTask(t, state, "task-supersede-norep")
		if task.Status != models.TaskStatusSuperseded {
			t.Fatalf("task status = %s, want %s", task.Status, models.TaskStatusSuperseded)
		}
		if len(task.SupersededBy) != 0 {
			t.Fatalf("superseded_by = %v, want empty", task.SupersededBy)
		}
		if task.RescopeReason == nil || *task.RescopeReason != "Work already merged" {
			t.Fatalf("rescope_reason = %v, want 'Work already merged'", task.RescopeReason)
		}
	})

	t.Run("handoff rejects code-reviewer agent via RBAC", func(t *testing.T) {
		projectRoot, _ := setupMutationTestProject(t, func(state *models.State) {
			now := time.Now().UTC()
			state.Tasks = []models.Task{
				testhelpers.BuildTaskByStatus("task-handoff-rbac", models.TaskStatusImplementing, now),
			}
		})

		err := executeRootCommand(t, projectRoot, "handoff", "task-handoff-rbac", "summary", "next", "--agent-id", "code-reviewer-1")
		if err == nil {
			t.Fatalf("expected RBAC error for code-reviewer calling handoff, got nil")
		}
		if !strings.Contains(err.Error(), `operation "handoff" not allowed for role "code-reviewer"`) {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("claim-task rejects orchestrator agent via RBAC", func(t *testing.T) {
		projectRoot, _ := setupMutationTestProject(t, func(state *models.State) {
			now := time.Now().UTC()
			state.Tasks = []models.Task{
				testhelpers.BuildTaskByStatus("task-claim-rbac", models.TaskStatusReady, now),
			}
		})

		err := executeRootCommand(t, projectRoot, "claim-task", "task-claim-rbac", "orchestrator-1")
		if err == nil {
			t.Fatalf("expected RBAC error for orchestrator calling claim-task, got nil")
		}
		if !strings.Contains(err.Error(), "command requires role type [doer]") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("submit-verdict rejects coder agent via RBAC", func(t *testing.T) {
		projectRoot, _ := setupMutationTestProject(t, func(state *models.State) {
			now := time.Now().UTC()
			state.Tasks = []models.Task{
				testhelpers.BuildTaskByStatus("task-verdict-rbac", models.TaskStatusReviewing, now),
			}
		})

		err := executeRootCommand(t, projectRoot, "submit-verdict", "task-verdict-rbac", "APPROVED", "--agent-id", "coder-1")
		if err == nil {
			t.Fatalf("expected RBAC error for coder calling submit-verdict, got nil")
		}
		if !strings.Contains(err.Error(), `operation "submit-verdict" not allowed for role "coder"`) {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("add-task rejects non-orchestrator via env-var RBAC", func(t *testing.T) {
		projectRoot, _ := setupMutationTestProject(t, nil)

		t.Setenv("LIZA_AGENT_ID", "coder-1")
		err := executeRootCommand(t, projectRoot, "add-task", "--id", "new-task", "--desc", "test", "--spec", "s", "--done", "d", "--scope", "sc")
		if err == nil {
			t.Fatalf("expected RBAC error for coder calling add-task via env-var, got nil")
		}
		if !strings.Contains(err.Error(), "command requires role type [orchestrator]") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("add-task accepts role-pair from direct flags", func(t *testing.T) {
		projectRoot, statePath := setupMutationTestProject(t, nil)
		testhelpers.CreateSpecFile(t, projectRoot, "vision.md", "# Vision\n")
		testhelpers.CreateSpecFile(t, projectRoot, "feature.md", "# Feature\n")

		err := executeRootCommand(
			t,
			projectRoot,
			"add-task",
			"--id",
			"planning-task",
			"--desc",
			"Plan the change",
			"--spec",
			"specs/feature.md",
			"--done",
			"Plan reviewed",
			"--scope",
			"internal/ops",
			"--role-pair",
			"code-planning-pair",
			"--agent-id",
			"orchestrator-1",
		)
		if err != nil {
			t.Fatalf("add-task execute failed: %v", err)
		}

		state := readState(t, statePath)
		task := mustFindTask(t, state, "planning-task")
		if task.RolePair != "code-planning-pair" {
			t.Fatalf("task role_pair = %s, want code-planning-pair", task.RolePair)
		}
		if task.Type != models.TaskTypePlanning {
			t.Fatalf("task type = %s, want %s", task.Type, models.TaskTypePlanning)
		}
		if task.Status != models.TaskStatusDraftCodingPlan {
			t.Fatalf("task status = %s, want %s", task.Status, models.TaskStatusDraftCodingPlan)
		}
	})

	t.Run("unblock-task rejects non-orchestrator via allowed-operation RBAC", func(t *testing.T) {
		projectRoot, _ := setupMutationTestProject(t, func(state *models.State) {
			now := time.Now().UTC()
			task := testhelpers.BuildTaskByStatus("task-unblock-rbac", models.TaskStatusBlocked, now)
			task.RolePair = "code-planning-pair"
			state.Tasks = []models.Task{task}
		})

		err := executeRootCommand(
			t,
			projectRoot,
			"unblock-task",
			"task-unblock-rbac",
			"--assign-to",
			"code-planner-1",
			"--reason",
			"repair verified",
			"--agent-id",
			"coder-1",
		)
		if err == nil {
			t.Fatalf("expected RBAC error for coder calling unblock-task, got nil")
		}
		if !strings.Contains(err.Error(), `operation "unblock-task" not allowed for role "coder"`) {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("mark-blocked persists orchestrator repair request", func(t *testing.T) {
		projectRoot, statePath := setupMutationTestProject(t, func(state *models.State) {
			now := time.Now().UTC()
			state.Tasks = []models.Task{
				testhelpers.BuildTaskByStatus("task-repair-request", models.TaskStatusImplementing, now),
			}
		})

		err := executeRootCommand(
			t,
			projectRoot,
			"mark-blocked",
			"task-repair-request",
			"--agent-id",
			"coder-1",
			"--reason",
			"Required state repair is orchestrator-only",
			"--questions",
			"Can the orchestrator restore the missing parent task?",
			"--repair-operation",
			"add-task",
			"--repair-target",
			"architecture-2",
			"--repair-command",
			"liza add-task --id architecture-2 --agent-id orchestrator-1 --json",
			"--repair-evidence",
			`command=liza add-task --id architecture-2 --agent-id coder-1 --json exit_code=1 stderr=command requires role type [orchestrator] but agent "coder-1" has type "doer"`,
			"--repair-validation",
			"python -m pytest -q tests/backend/test_workflow_contract.py -q",
		)
		if err != nil {
			t.Fatalf("mark-blocked execute failed: %v", err)
		}

		state := readState(t, statePath)
		task := mustFindTask(t, state, "task-repair-request")
		if task.Status != models.TaskStatusBlocked {
			t.Fatalf("task status = %s, want %s", task.Status, models.TaskStatusBlocked)
		}
		if task.RepairRequest == nil {
			t.Fatal("repair_request not persisted")
		}
		if task.RepairRequest.Operation != "add-task" {
			t.Fatalf("repair operation = %q, want add-task", task.RepairRequest.Operation)
		}
		if task.RepairRequest.Target != "architecture-2" {
			t.Fatalf("repair target = %q, want architecture-2", task.RepairRequest.Target)
		}
		if len(task.RepairRequest.Evidence) != 1 {
			t.Fatalf("repair evidence len = %d, want 1", len(task.RepairRequest.Evidence))
		}
	})

	t.Run("mark-blocked persists depends-on", func(t *testing.T) {
		projectRoot, statePath := setupMutationTestProject(t, func(state *models.State) {
			now := time.Now().UTC()
			state.Tasks = []models.Task{
				testhelpers.BuildTaskByStatus("task-blocked-on-dep", models.TaskStatusImplementing, now),
				testhelpers.BuildTaskByStatus("dep-task", models.TaskStatusImplementing, now),
			}
		})

		err := executeRootCommand(
			t,
			projectRoot,
			"mark-blocked",
			"task-blocked-on-dep",
			"--agent-id",
			"coder-1",
			"--reason",
			"Waiting on dep-task",
			"--questions",
			"Should dep-task merge first?",
			"--depends-on",
			"dep-task",
		)
		if err != nil {
			t.Fatalf("mark-blocked execute failed: %v", err)
		}

		state := readState(t, statePath)
		task := mustFindTask(t, state, "task-blocked-on-dep")
		if len(task.DependsOn) != 1 || task.DependsOn[0] != "dep-task" {
			t.Fatalf("task DependsOn = %v, want [dep-task]", task.DependsOn)
		}
	})

	t.Run("mark-blocked validates incomplete repair request flags", func(t *testing.T) {
		projectRoot, _ := setupMutationTestProject(t, func(state *models.State) {
			now := time.Now().UTC()
			state.Tasks = []models.Task{
				testhelpers.BuildTaskByStatus("task-incomplete-repair-request", models.TaskStatusImplementing, now),
			}
		})

		err := executeRootCommand(
			t,
			projectRoot,
			"mark-blocked",
			"task-incomplete-repair-request",
			"--agent-id",
			"coder-1",
			"--reason",
			"Required state repair is orchestrator-only",
			"--questions",
			"Can the orchestrator restore the missing parent task?",
			"--repair-operation",
			"add-task",
		)
		if err == nil {
			t.Fatal("expected incomplete repair request error, got nil")
		}
		if !strings.Contains(err.Error(), "--repair-target is required when repair request fields are provided") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("update-review-commit uses --changed-by and updates state", func(t *testing.T) {
		// This command is RBAC-exempt (--changed-by, same as release-claim):
		// it is an operator recovery action for rebased worktrees, not an
		// agent workflow command. See specs/goals/20260412-cli-native-access-control.md.
		projectRoot, statePath := setupMutationTestProject(t, func(state *models.State) {
			now := time.Now().UTC()
			state.Tasks = []models.Task{
				testhelpers.BuildTaskByStatus("task-urc", models.TaskStatusReadyForReview, now),
			}
		})

		// Create a worktree and make a commit so HEAD diverges from review_commit
		g := git.New(projectRoot)
		_, err := g.CreateWorktree("task-urc", "integration")
		if err != nil {
			t.Fatalf("Failed to create worktree: %v", err)
		}
		wtPath := g.GetWorktreePath("task-urc")
		implFile := filepath.Join(wtPath, "feature.go")
		if err := os.WriteFile(implFile, []byte("package feature\n"), 0644); err != nil {
			t.Fatal(err)
		}
		testhelpers.MustGit(t, wtPath, "add", "feature.go")
		testhelpers.MustGit(t, wtPath, "commit", "-m", "diverge")

		// Set stale review_commit and worktree path in state
		staleCommit := testhelpers.MustGit(t, projectRoot, "rev-parse", "integration")
		wtHEAD := testhelpers.MustGit(t, wtPath, "rev-parse", "HEAD")
		expectedBase := testhelpers.MustGit(t, projectRoot, "merge-base", wtHEAD, "integration")
		bb := db.For(statePath)
		if err := bb.Modify(func(s *models.State) error {
			task := s.FindTask("task-urc")
			task.ReviewCommit = &staleCommit
			task.BaseCommit = &staleCommit
			worktreeRel := g.GetWorktreeRelPath("task-urc")
			task.Worktree = &worktreeRel
			return nil
		}); err != nil {
			t.Fatalf("Failed to update state: %v", err)
		}

		err = executeRootCommand(t, projectRoot, "update-review-commit", "task-urc", "--changed-by", "operator-1")
		if err != nil {
			t.Fatalf("update-review-commit execute failed: %v", err)
		}

		state := readState(t, statePath)
		task := mustFindTask(t, state, "task-urc")
		if task.ReviewCommit == nil || *task.ReviewCommit != wtHEAD {
			got := "<nil>"
			if task.ReviewCommit != nil {
				got = *task.ReviewCommit
			}
			t.Fatalf("review_commit = %s, want %s", got, wtHEAD)
		}
		if task.BaseCommit == nil || *task.BaseCommit != expectedBase {
			got := "<nil>"
			if task.BaseCommit != nil {
				got = *task.BaseCommit
			}
			t.Fatalf("base_commit = %s, want %s", got, expectedBase)
		}

		// Verify history entry records the operator
		found := false
		for _, entry := range task.History {
			if entry.Event == models.TaskEventReviewCommitUpdated {
				found = true
				if entry.Agent == nil || *entry.Agent != "operator-1" {
					t.Fatalf("history agent = %v, want operator-1", entry.Agent)
				}
				break
			}
		}
		if !found {
			t.Fatal("expected review_commit_updated history entry")
		}
	})

	t.Run("release-claim uses --changed-by over env fallback", func(t *testing.T) {
		projectRoot, statePath := setupMutationTestProject(t, func(state *models.State) {
			now := time.Now().UTC()
			state.Tasks = []models.Task{
				testhelpers.BuildTaskByStatus("task-release-claim", models.TaskStatusImplementing, now),
			}
		})

		t.Setenv("LIZA_AGENT_ID", "coder-99")
		err := executeRootCommand(t, projectRoot, "release-claim", "task-release-claim", "--role", "doer", "--force", "--changed-by", "auditor-7")
		if err != nil {
			t.Fatalf("release-claim execute failed: %v", err)
		}

		state := readState(t, statePath)
		task := mustFindTask(t, state, "task-release-claim")
		if task.Status != models.TaskStatusReady {
			t.Fatalf("task status = %s, want %s", task.Status, models.TaskStatusReady)
		}
		if len(task.History) == 0 {
			t.Fatalf("expected history entry for released claim")
		}
		last := task.History[len(task.History)-1]
		if last.Event != "doer_claim_released" {
			t.Fatalf("history event = %s, want doer_claim_released", last.Event)
		}
		if last.Agent == nil || *last.Agent != "auditor-7" {
			t.Fatalf("history agent = %v, want auditor-7", last.Agent)
		}
	})
}

func setupMutationTestProject(t *testing.T, mutateState func(*models.State)) (string, string) {
	t.Helper()

	projectRoot := t.TempDir()
	projectRoot, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		t.Fatalf("failed to resolve temp project root: %v", err)
	}
	testhelpers.SetupTestGitRepo(t, projectRoot)
	statePath, _ := testhelpers.SetupLizaDir(t, projectRoot)
	testhelpers.SetupPipelineConfig(t, projectRoot)

	state := testhelpers.CreateValidState()
	if mutateState != nil {
		mutateState(state)
	}
	testhelpers.WriteInitialState(t, statePath, state)

	return projectRoot, statePath
}

// executeRootCommand runs a CLI command against the given project root.
// NOTE: os.Chdir is process-global state, which prevents t.Parallel() in this
// package. rootCmd resolves the project root from CWD; until it accepts an
// explicit --project-root flag, Chdir is the only option.
func executeRootCommand(t *testing.T, projectRoot string, args ...string) error {
	t.Helper()

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	defer func() {
		if chdirErr := os.Chdir(oldDir); chdirErr != nil {
			t.Fatalf("failed to restore working directory: %v", chdirErr)
		}
	}()

	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("failed to chdir to project root: %v", err)
	}

	// rootCmd and db singletons are process globals; reset before each command
	// execution so tests don't leak state across runs.
	resetRootCmdForTest(t)
	rootCmd.SetArgs(args)
	return rootCmd.Execute()
}

func readState(t *testing.T, statePath string) *models.State {
	t.Helper()
	state, err := db.For(statePath).Read()
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	return state
}

func mustFindTask(t *testing.T, state *models.State, taskID string) *models.Task {
	t.Helper()
	task := state.FindTask(taskID)
	if task == nil {
		t.Fatalf("task %s not found", taskID)
	}
	return task
}
