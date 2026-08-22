package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestRunSupervisor_FinalPlanningQuorumApprovalAutoMerges(t *testing.T) {
	runSupervisorFinalPlanningQuorumApprovalAutoMerges(t, "claude")
}

func TestRunSupervisor_FinalPlanningQuorumApprovalAutoMergesSingleProvider(t *testing.T) {
	task := runSupervisorFinalPlanningQuorumApprovalAutoMerges(t, "codex")

	for _, approval := range task.Approvals {
		if approval.Provider != "codex" {
			t.Fatalf("approval provider = %q, want codex: %+v", approval.Provider, task.Approvals)
		}
	}
	for _, entry := range task.History {
		if entry.Event != models.TaskEventMerged {
			continue
		}
		if got, ok := entry.Extra["diversity_not_achievable"]; !ok || got != true {
			t.Fatalf("merged history diversity_not_achievable = %v, want true: %+v", got, entry.Extra)
		}
		reason, ok := entry.Extra["diversity_reason"].(string)
		if !ok || reason == "" {
			t.Fatalf("merged history diversity_reason = %v, want non-empty string: %+v", entry.Extra["diversity_reason"], entry.Extra)
		}
		if _, ok := entry.Extra["reason"]; ok {
			t.Fatalf("merged history contains inline reason key: %+v", entry.Extra)
		}
		return
	}

	t.Fatalf("merged history entry not found: %+v", task.History)
}

func runSupervisorFinalPlanningQuorumApprovalAutoMerges(t *testing.T, firstReviewerProvider string) *models.Task {
	t.Helper()
	projectRoot, statePath, taskID := setupAgentMergeRepo(t)
	supervisorLogPath := filepath.Join(t.TempDir(), "supervisor.log")
	supervisorLog, err := os.Create(supervisorLogPath)
	if err != nil {
		t.Fatalf("create supervisor log: %v", err)
	}
	restoreLogger := UseLoggerOutput(supervisorLog)
	t.Cleanup(func() {
		restoreLogger()
		_ = supervisorLog.Close()
	})

	bb := db.New(statePath)
	reviewer1 := "code-plan-reviewer-1"
	reviewer2 := "code-plan-reviewer-2"
	now := time.Now().UTC()
	initial, err := bb.Read()
	if err != nil {
		t.Fatalf("read initial state: %v", err)
	}
	reviewCommit := *initial.FindTask(taskID).ReviewCommit
	baseCommit := mustGitInDir(t, projectRoot, "merge-base", reviewCommit, "integration")

	if err := bb.Modify(func(state *models.State) error {
		off := false
		state.Config.AutoCheckpointSummary = &off
		state.Config.ReviewerPollInterval = 1
		state.Config.ReviewerMaxWait = 1
		state.Sprint.Scope.Planned = []string{taskID}

		task := state.FindTask(taskID)
		task.RolePair = "code-planning-main-pair"
		task.Status = models.TaskStatus("CODING_PLAN_MAIN_PARTIALLY_APPROVED")
		task.BaseCommit = &baseCommit
		task.ApprovedBy = &reviewer1
		task.Approvals = []models.Approval{{
			Agent:     reviewer1,
			Provider:  firstReviewerProvider,
			Timestamp: now,
		}}
		task.History = []models.TaskHistoryEntry{{
			Time:   now,
			Event:  models.TaskEventApproved,
			Agent:  &reviewer1,
			Commit: task.ReviewCommit,
		}}
		task.ReviewingBy = nil
		task.ReviewLeaseExpires = nil
		task.MergeCommit = nil
		task.IntegrationFailure = nil

		firstReviewer := testhelpers.RegisteredTestAgent("code-plan-reviewer")
		firstReviewer.Provider = firstReviewerProvider
		state.Agents = map[string]models.Agent{reviewer1: firstReviewer}
		return nil
	}); err != nil {
		t.Fatalf("prepare final-quorum state: %v", err)
	}

	mock := &MockLLMAgent{ExitCode: 0}
	mock.OnExecute = func(_ context.Context, _, agentID, _ string, root string, _ []string) error {
		_, err := ops.SubmitVerdict(root, taskID, "APPROVED", "", agentID, "")
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := RunSupervisor(ctx, SupervisorConfig{
		AgentID:          reviewer2,
		Role:             "code-plan-reviewer",
		ProjectRoot:      projectRoot,
		StatePath:        statePath,
		LogPath:          filepath.Join(projectRoot, ".liza", "log.yaml"),
		SpecsDir:         filepath.Join(projectRoot, "specs"),
		CLIName:          "codex",
		InitialTask:      taskID,
		LLMAgent:         mock,
		ExecutionTimeout: 10 * time.Second,
	}); err != nil {
		t.Fatalf("RunSupervisor() error = %v", err)
	}

	if calls := mock.GetCalls(); len(calls) != 1 {
		t.Fatalf("provider calls = %d, want 1", len(calls))
	}

	final, err := bb.Read()
	if err != nil {
		t.Fatalf("read final state: %v", err)
	}
	task := final.FindTask(taskID)
	if task.Status != models.TaskStatusMerged {
		t.Fatalf("task status = %s, want MERGED", task.Status)
	}
	if task.ApprovalCount() != 2 || task.LastApprover() != reviewer2 {
		t.Fatalf("approvals = %+v, want final approval owned by %s", task.Approvals, reviewer2)
	}
	integrationHead := mustGitInDir(t, projectRoot, "rev-parse", "integration")
	if task.MergeCommit == nil || *task.MergeCommit != integrationHead {
		t.Fatalf("merge_commit = %v, want integration HEAD %s", task.MergeCommit, integrationHead)
	}
	if integrationHead != reviewCommit {
		t.Fatalf("integration HEAD = %s, want %s", integrationHead, reviewCommit)
	}

	finalApprovalIndex := -1
	mergeIndex := -1
	for i, entry := range task.History {
		if entry.Agent == nil || *entry.Agent != reviewer2 {
			continue
		}
		switch entry.Event {
		case models.TaskEventApproved:
			finalApprovalIndex = i
		case models.TaskEventMerged:
			mergeIndex = i
		}
	}
	if finalApprovalIndex < 0 || mergeIndex <= finalApprovalIndex {
		t.Fatalf("history does not record final approval then supervisor merge by %s: %+v", reviewer2, task.History)
	}

	if err := supervisorLog.Sync(); err != nil {
		t.Fatalf("sync supervisor log: %v", err)
	}
	logOutput, err := os.ReadFile(supervisorLogPath)
	if err != nil {
		t.Fatalf("read supervisor log: %v", err)
	}
	for _, message := range []string{"Merging approved task", "Successfully merged task"} {
		if !strings.Contains(string(logOutput), message) {
			t.Fatalf("supervisor log missing %q:\n%s", message, logOutput)
		}
	}

	return task
}
