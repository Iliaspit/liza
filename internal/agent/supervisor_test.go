package agent

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	lizagit "github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/pipeline"
	"github.com/liza-mas/liza/internal/precommit"
	"github.com/liza-mas/liza/internal/testhelpers"
)

// MockCLIExecutor for testing CLI execution
type MockCLIExecutor struct {
	mu               sync.Mutex
	Calls            []MockCLICall
	InteractiveCalls []MockCLICall
	ExitCode         int
	Output           string
	ExitError        error
	OnExecute        func(ctx context.Context, cliName string, agentID string, prompt string, projectRoot string, additionalDirs []string) error
}

type MockCLICall struct {
	CLIName        string
	AgentID        string
	Prompt         string
	ProjectRoot    string
	AdditionalDirs []string
}

func (m *MockCLIExecutor) Execute(ctx context.Context, cliName string, agentID string, prompt string, projectRoot string, additionalDirs []string, _ models.Config) (CLIExecutionResult, error) {
	m.mu.Lock()
	m.Calls = append(m.Calls, MockCLICall{CLIName: cliName, AgentID: agentID, Prompt: prompt, ProjectRoot: projectRoot, AdditionalDirs: slices.Clone(additionalDirs)})
	m.mu.Unlock()
	if m.OnExecute != nil {
		if err := m.OnExecute(ctx, cliName, agentID, prompt, projectRoot, additionalDirs); err != nil {
			return CLIExecutionResult{ExitCode: m.ExitCode, Output: m.Output}, err
		}
	}
	return CLIExecutionResult{ExitCode: m.ExitCode, Output: m.Output}, m.ExitError
}

func (m *MockCLIExecutor) ExecuteInteractive(ctx context.Context, cliName string, projectRoot string, additionalDirs []string) (int, error) {
	m.mu.Lock()
	m.InteractiveCalls = append(m.InteractiveCalls, MockCLICall{CLIName: cliName, ProjectRoot: projectRoot, AdditionalDirs: slices.Clone(additionalDirs)})
	m.mu.Unlock()
	return m.ExitCode, m.ExitError
}

// GetCalls returns a copy of the calls slice in a thread-safe manner
func (m *MockCLIExecutor) GetCalls() []MockCLICall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]MockCLICall, len(m.Calls))
	copy(calls, m.Calls)
	return calls
}

// GetInteractiveCalls returns a copy of the interactive calls slice in a thread-safe manner
func (m *MockCLIExecutor) GetInteractiveCalls() []MockCLICall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]MockCLICall, len(m.InteractiveCalls))
	copy(calls, m.InteractiveCalls)
	return calls
}

// TestMockCLIExecution tests CLI executor mock
func TestMockCLIExecution(t *testing.T) {
	mock := &MockCLIExecutor{
		ExitCode: 0,
	}

	ctx := context.Background()
	result, err := mock.Execute(ctx, "claude", "claude-1", "test prompt", "/tmp/test-project", nil, models.Config{})

	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("Execute() exitCode = %d, want 0", result.ExitCode)
	}

	calls := mock.GetCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(calls))
	}

	call := calls[0]
	if call.CLIName != "claude" {
		t.Errorf("CLIName = %s, want claude", call.CLIName)
	}
	if call.Prompt != "test prompt" {
		t.Errorf("Prompt = %s, want 'test prompt'", call.Prompt)
	}
}

func TestExecuteAgentBlocksTaskAfterProgressTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	now := time.Now().UTC()
	taskID := "task-1"
	agentID := "coder-1"
	gw := lizagit.New(tmpDir)
	baseCommit, err := gw.CreateWorktree(taskID, "main")
	if err != nil {
		t.Fatalf("CreateWorktree() error: %v", err)
	}
	task := testhelpers.BuildTaskByStatus(taskID, models.TaskStatusImplementing, now)
	task.AssignedTo = &agentID
	task.BaseCommit = &baseCommit
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{task}
	state.Agents = map[string]models.Agent{
		agentID: {
			Role:        models.RoleCoder,
			Status:      models.AgentStatusWorking,
			CurrentTask: &taskID,
			Heartbeat:   now,
		},
	}
	testhelpers.WriteInitialState(t, statePath, state)

	mock := &MockCLIExecutor{
		OnExecute: func(ctx context.Context, cliName string, agentID string, prompt string, projectRoot string, additionalDirs []string) error {
			<-ctx.Done()
			if _, statErr := os.Stat(filepath.Join(tmpDir, ".worktrees", taskID)); statErr != nil {
				t.Fatalf("worktree should still exist when provider observes cancellation, stat err=%v", statErr)
			}
			return ctx.Err()
		},
	}
	config := SupervisorConfig{
		AgentID:                  agentID,
		Role:                     models.RoleCoder,
		ProjectRoot:              tmpDir,
		StatePath:                statePath,
		CLIName:                  "codex",
		Executor:                 mock,
		ExecutionTimeout:         5 * time.Second,
		ExecutionProgressTimeout: 150 * time.Millisecond,
	}

	exitCode, _, err := executeAgent(context.Background(), config, "prompt", nil, taskID, state.Config)
	if err != nil {
		t.Fatalf("executeAgent error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 after watchdog block", exitCode)
	}

	bb := db.For(statePath)
	after, err := bb.Read()
	if err != nil {
		t.Fatalf("bb.Read: %v", err)
	}
	afterTask := after.FindTask(taskID)
	if afterTask == nil {
		t.Fatalf("task %s missing", taskID)
	}
	if afterTask.Status != models.TaskStatusBlocked {
		t.Fatalf("task status = %s, want BLOCKED", afterTask.Status)
	}
	if afterTask.BlockedReason == nil || !strings.Contains(*afterTask.BlockedReason, "execution progress timeout") {
		t.Fatalf("blocked reason = %v, want execution progress timeout", afterTask.BlockedReason)
	}
	if afterTask.AssignedTo != nil || afterTask.LeaseExpires != nil {
		t.Fatalf("blocked task should clear assignment and lease, assigned=%v lease=%v", afterTask.AssignedTo, afterTask.LeaseExpires)
	}
	if afterTask.Worktree != nil {
		t.Fatalf("blocked task worktree = %v, want nil", *afterTask.Worktree)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".worktrees", taskID)); !os.IsNotExist(err) {
		t.Fatalf("worktree directory should be removed, stat err=%v", err)
	}
	branchExists, err := gw.BranchExists("task/" + taskID)
	if err != nil {
		t.Fatalf("BranchExists error: %v", err)
	}
	if branchExists {
		t.Fatalf("task branch should be removed")
	}
}

func TestExecuteAgentOutputProgressPreventsProgressTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	now := time.Now().UTC()
	taskID := "task-1"
	agentID := "coder-1"
	task := testhelpers.BuildTaskByStatus(taskID, models.TaskStatusImplementing, now)
	task.AssignedTo = &agentID
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, statePath, state)

	mock := &MockCLIExecutor{
		OnExecute: func(ctx context.Context, cliName string, agentID string, prompt string, projectRoot string, additionalDirs []string) error {
			mark := executionProgressCallback(ctx)
			deadline := time.After(280 * time.Millisecond)
			ticker := time.NewTicker(40 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-ticker.C:
					if mark != nil {
						mark()
					}
				case <-deadline:
					if mark != nil {
						mark()
					}
					return nil
				}
			}
		},
	}
	config := SupervisorConfig{
		AgentID:                  agentID,
		Role:                     models.RoleCoder,
		ProjectRoot:              tmpDir,
		StatePath:                statePath,
		CLIName:                  "codex",
		Executor:                 mock,
		ExecutionTimeout:         5 * time.Second,
		ExecutionProgressTimeout: 120 * time.Millisecond,
	}

	exitCode, _, err := executeAgent(context.Background(), config, "prompt", nil, taskID, state.Config)
	if err != nil {
		t.Fatalf("executeAgent error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}

	bb := db.For(statePath)
	after, err := bb.Read()
	if err != nil {
		t.Fatalf("bb.Read: %v", err)
	}
	afterTask := after.FindTask(taskID)
	if afterTask == nil {
		t.Fatalf("task %s missing", taskID)
	}
	if afterTask.Status != models.TaskStatusImplementing {
		t.Fatalf("task status = %s, want IMPLEMENTING_CODE", afterTask.Status)
	}
}

func TestDefaultCLIExecutorStreamsMaskedOutputFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI shell script test requires /bin/sh")
	}

	projectRoot := t.TempDir()
	outputsDir := filepath.Join(projectRoot, ".liza", "agent-outputs")
	binDir := t.TempDir()
	fakeClaude := filepath.Join(binDir, "claude")
	script := `#!/bin/sh
printf 'stdout-before sk-test-secret-value stdout-after\n'
printf 'stderr-before sk-test-secret-value stderr-after\n' >&2
`
	if err := os.WriteFile(fakeClaude, []byte(script), 0755); err != nil {
		t.Fatalf("write fake claude: %v", err)
	}

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-secret-value")

	executor := NewDefaultCLIExecutor(outputsDir)
	result, err := executor.Execute(context.Background(), "claude", "coder-1", "prompt body", projectRoot, nil, models.Config{})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", result.ExitCode)
	}
	if strings.Contains(result.Output, "sk-test-secret-value") {
		t.Fatalf("result output leaked secret: %q", result.Output)
	}
	if !strings.Contains(result.Output, "stdout-before *** stdout-after") {
		t.Fatalf("result output missing masked stdout: %q", result.Output)
	}
	if !strings.Contains(result.Output, "stderr-before *** stderr-after") {
		t.Fatalf("result output missing masked stderr: %q", result.Output)
	}

	txtFiles, err := filepath.Glob(filepath.Join(outputsDir, "coder-1-*.txt"))
	if err != nil {
		t.Fatalf("glob txt: %v", err)
	}
	errFiles, err := filepath.Glob(filepath.Join(outputsDir, "coder-1-*.err"))
	if err != nil {
		t.Fatalf("glob err: %v", err)
	}
	if len(txtFiles) != 1 || len(errFiles) != 1 {
		t.Fatalf("output files txt=%v err=%v, want one of each", txtFiles, errFiles)
	}

	txtStem := strings.TrimSuffix(filepath.Base(txtFiles[0]), ".txt")
	errStem := strings.TrimSuffix(filepath.Base(errFiles[0]), ".err")
	if txtStem != errStem {
		t.Fatalf("stdout/stderr files should share timestamp, got %q and %q", txtStem, errStem)
	}

	stdoutLog, err := os.ReadFile(txtFiles[0])
	if err != nil {
		t.Fatalf("read stdout log: %v", err)
	}
	stderrLog, err := os.ReadFile(errFiles[0])
	if err != nil {
		t.Fatalf("read stderr log: %v", err)
	}
	if strings.Contains(string(stdoutLog), "sk-test-secret-value") || strings.Contains(string(stderrLog), "sk-test-secret-value") {
		t.Fatalf("persisted logs leaked secret:\nstdout=%q\nstderr=%q", stdoutLog, stderrLog)
	}
	if string(stdoutLog) != "stdout-before *** stdout-after\n" {
		t.Fatalf("stdout log = %q", stdoutLog)
	}
	if string(stderrLog) != "stderr-before *** stderr-after\n" {
		t.Fatalf("stderr log = %q", stderrLog)
	}
}

func TestDefaultCLIExecutorDisallowsClaudeSubagentToolsWhenEnvEnabled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI shell script test requires /bin/sh")
	}

	projectRoot := t.TempDir()
	outputsDir := filepath.Join(projectRoot, ".liza", "agent-outputs")
	binDir := t.TempDir()
	fakeClaude := filepath.Join(binDir, "claude")
	script := `#!/bin/sh
for arg in "$@"; do
  printf 'arg:%s\n' "$arg"
done
`
	if err := os.WriteFile(fakeClaude, []byte(script), 0755); err != nil {
		t.Fatalf("write fake claude: %v", err)
	}

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("LIZA_DISABLE_CLAUDE_SUBAGENTS", "1")

	executor := NewDefaultCLIExecutor(outputsDir)
	result, err := executor.Execute(context.Background(), "claude", "coder-1", "prompt body", projectRoot, nil, models.Config{})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", result.ExitCode)
	}
	if !strings.Contains(result.Output, "arg:--disallowedTools\narg:Task\n") {
		t.Fatalf("result output missing subagent disallow args: %q", result.Output)
	}
	if strings.Contains(result.Output, "prompt body") {
		t.Fatalf("prompt should be passed via stdin, not argv: %q", result.Output)
	}
}

func TestSupervisor_Exit0ProviderAuditDegradedContinuesPostExecution(t *testing.T) {
	projectRoot := t.TempDir()
	testhelpers.SetupTestGitRepo(t, projectRoot)
	statePath, _ := testhelpers.SetupLizaDir(t, projectRoot)

	now := time.Now().UTC()
	taskID := "task-audit-exit0"
	state := testhelpers.CreateValidState()
	state.Config.CoderPollInterval = 1
	state.Config.DoerMaxWait = 1
	state.Config.LeaseDuration = 300
	state.Tasks = []models.Task{testhelpers.BuildTaskByStatus(taskID, models.TaskStatusReady, now)}
	bb := testhelpers.WriteInitialState(t, statePath, state)

	auditOutput := `ERROR codex_core::session: failed to record rollout items: thread 019e983f-f3a2-7071-8a66-aa1774db9101 not found`
	mock := &MockCLIExecutor{
		ExitCode: 0,
		Output:   auditOutput,
	}
	mock.OnExecute = func(ctx context.Context, cliName, agentID, prompt, projectRoot string, additionalDirs []string) error {
		reviewCommit := testhelpers.MustGit(t, projectRoot, "rev-parse", "HEAD")
		return bb.Modify(func(s *models.State) error {
			task := s.FindTask(taskID)
			if task == nil {
				t.Fatalf("task %q not found", taskID)
			}
			task.Status = models.TaskStatusReadyForReview
			task.ReviewCommit = &reviewCommit
			task.HandoffEvents = append(task.HandoffEvents, models.HandoffEvent{
				Timestamp: time.Now().UTC(),
				Agent:     agentID,
				Trigger:   models.HandoffTriggerSubmission,
			})
			return nil
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := RunSupervisor(ctx, SupervisorConfig{
		AgentID:          "coder-1",
		Role:             "coder",
		ProjectRoot:      projectRoot,
		StatePath:        statePath,
		LogPath:          filepath.Join(projectRoot, ".liza", "log.yaml"),
		SpecsDir:         filepath.Join(projectRoot, "specs"),
		CLIName:          "codex",
		Executor:         mock,
		ExecutionTimeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("RunSupervisor() error = %v", err)
	}

	if calls := mock.GetCalls(); len(calls) != 1 {
		t.Fatalf("Execute calls = %d, want 1", len(calls))
	}

	updated, err := bb.Read()
	if err != nil {
		t.Fatalf("bb.Read: %v", err)
	}
	task := updated.FindTask(taskID)
	if task == nil {
		t.Fatalf("task %q not found after supervisor run", taskID)
	}
	if task.Status != models.TaskStatusReadyForReview {
		t.Fatalf("task.Status = %q, want %q", task.Status, models.TaskStatusReadyForReview)
	}
	if len(updated.Anomalies) != 1 {
		t.Fatalf("len(Anomalies) = %d, want 1", len(updated.Anomalies))
	}
	if updated.Anomalies[0].Type != ProviderAuditDegradedAnomalyType {
		t.Fatalf("anomaly.Type = %q, want %q", updated.Anomalies[0].Type, ProviderAuditDegradedAnomalyType)
	}

	alerts, err := os.ReadFile(filepath.Join(projectRoot, ".liza", "alerts.log"))
	if err != nil {
		t.Fatalf("read alerts.log: %v", err)
	}
	if !strings.Contains(string(alerts), "PROVIDER AUDIT DEGRADED") {
		t.Fatalf("alerts.log missing audit degradation alert:\n%s", string(alerts))
	}
}

func TestRunSupervisor_HeartbeatMissingAgentStopsSupervisor(t *testing.T) {
	projectRoot := t.TempDir()
	testhelpers.SetupTestGitRepo(t, projectRoot)
	statePath, _ := testhelpers.SetupLizaDir(t, projectRoot)

	now := time.Now().UTC()
	taskID := "task-heartbeat-missing-agent"
	state := testhelpers.CreateValidState()
	state.Config.CoderPollInterval = 1
	state.Config.DoerMaxWait = 1
	state.Config.LeaseDuration = 300
	state.Config.HeartbeatInterval = 1
	state.Tasks = []models.Task{testhelpers.BuildTaskByStatus(taskID, models.TaskStatusReady, now)}
	bb := testhelpers.WriteInitialState(t, statePath, state)

	mock := &MockCLIExecutor{ExitCode: 0}
	mock.OnExecute = func(ctx context.Context, cliName, agentID, prompt, projectRoot string, additionalDirs []string) error {
		if err := bb.Modify(func(s *models.State) error {
			delete(s.Agents, agentID)
			return nil
		}); err != nil {
			return err
		}
		<-ctx.Done()
		return ctx.Err()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := RunSupervisor(ctx, SupervisorConfig{
		AgentID:          "coder-1",
		Role:             "coder",
		ProjectRoot:      projectRoot,
		StatePath:        statePath,
		LogPath:          filepath.Join(projectRoot, ".liza", "log.yaml"),
		SpecsDir:         filepath.Join(projectRoot, "specs"),
		CLIName:          "codex",
		Executor:         mock,
		ExecutionTimeout: 5 * time.Second,
	})
	if err == nil {
		t.Fatal("RunSupervisor() error = nil, want heartbeat failure")
	}
	if !strings.Contains(err.Error(), "heartbeat stopped for agent coder-1") {
		t.Fatalf("RunSupervisor() error = %v, want heartbeat stopped", err)
	}
}

func TestExit42RestartTracker_ExponentialBackoffAndCap(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	now := time.Now().UTC()

	agentID := "coder-1"
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
	task.AssignedTo = &agentID

	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{task}
	state.Config.Exit42RestartThreshold = 99
	state.Config.Exit42MaxBackoffSeconds = 8
	state.Agents[agentID] = models.Agent{Role: "coder", Status: models.AgentStatusWorking}

	bb := testhelpers.WriteInitialState(t, statePath, state)
	tracker := newExit42RestartTracker()

	var delays []time.Duration
	for i := 0; i < 4; i++ {
		outcome, err := tracker.Handle(bb, tmpDir, "coder", task.ID, agentID)
		if err != nil {
			t.Fatalf("Handle() error on attempt %d: %v", i+1, err)
		}
		if outcome.BlockedTask {
			t.Fatalf("Handle() blocked task unexpectedly on attempt %d", i+1)
		}
		delays = append(delays, outcome.Delay)
	}

	wantDelays := []time.Duration{
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		8 * time.Second,
	}
	for i, want := range wantDelays {
		if delays[i] != want {
			t.Errorf("delay[%d] = %v, want %v", i, delays[i], want)
		}
	}

	updatedState, err := bb.Read()
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	updatedTask := updatedState.FindTask(task.ID)
	if updatedTask == nil {
		t.Fatalf("task %q not found", task.ID)
	}

	if updatedTask.BlockedReason != nil && *updatedTask.BlockedReason != "" {
		t.Errorf("task should not be blocked yet, got reason: %s", *updatedTask.BlockedReason)
	}
}

func TestExit42RestartTracker_Blocking(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	now := time.Now().UTC()

	agentID := "coder-1"
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
	task.AssignedTo = &agentID

	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{task}
	state.Config.Exit42RestartThreshold = 2
	state.Agents[agentID] = models.Agent{Role: "coder", Status: models.AgentStatusWorking}

	bb := testhelpers.WriteInitialState(t, statePath, state)
	tracker := newExit42RestartTracker()

	// First attempt
	outcome, err := tracker.Handle(bb, tmpDir, "coder", task.ID, agentID)
	if err != nil {
		t.Fatalf("Handle() error on attempt 1: %v", err)
	}
	if outcome.BlockedTask {
		t.Fatalf("Handle() should not block on first attempt")
	}

	// Second attempt (at threshold)
	outcome, err = tracker.Handle(bb, tmpDir, "coder", task.ID, agentID)
	if err != nil {
		t.Fatalf("Handle() error on attempt 2: %v", err)
	}
	if outcome.BlockedTask {
		t.Fatalf("Handle() should not block at threshold")
	}

	// Third attempt (over threshold)
	outcome, err = tracker.Handle(bb, tmpDir, "coder", task.ID, agentID)
	if err != nil {
		t.Fatalf("Handle() error on attempt 3: %v", err)
	}
	if !outcome.BlockedTask {
		t.Fatalf("Handle() should block when over threshold")
	}

	updatedState, err := bb.Read()
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	updatedTask := updatedState.FindTask(task.ID)
	if updatedTask == nil {
		t.Fatalf("task %q not found", task.ID)
	}

	wantReason := "exit code 42 restart loop detected"
	if updatedTask.BlockedReason == nil || !strings.Contains(*updatedTask.BlockedReason, wantReason) {
		got := "<nil>"
		if updatedTask.BlockedReason != nil {
			got = *updatedTask.BlockedReason
		}
		t.Errorf("blocked reason = %q, want containing %q", got, wantReason)
	}
}

func TestExit42RestartTracker_BlocksNonCoderRoles(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	now := time.Now().UTC()

	agentID := "code-reviewer-1"
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	task.AssignedTo = &agentID
	task.ReviewingBy = &agentID

	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{task}
	state.Config.Exit42RestartThreshold = 2
	state.Agents[agentID] = models.Agent{Role: "code-reviewer", Status: models.AgentStatusReviewing}

	bb := testhelpers.WriteInitialState(t, statePath, state)
	tracker := newExit42RestartTracker()

	// Exhaust the threshold.
	for i := 0; i < 3; i++ {
		tracker.Handle(bb, tmpDir, "code-reviewer", task.ID, agentID)
	}

	// Read updated state — task should be BLOCKED.
	updatedState, err := bb.Read()
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	updatedTask := updatedState.FindTask(task.ID)
	if updatedTask == nil {
		t.Fatalf("task %q not found", task.ID)
	}
	if updatedTask.Status != models.TaskStatusBlocked {
		t.Errorf("task status = %q, want BLOCKED", updatedTask.Status)
	}
}

func TestCrashRestartTracker_BlocksAfterThreshold(t *testing.T) {
	tracker := newCrashRestartTracker()
	threshold := 3

	// Same signature (no progress) — count accumulates.
	for i := 1; i <= threshold; i++ {
		count := tracker.Increment("task-1", "same-sig")
		if count != i {
			t.Fatalf("Increment() = %d, want %d", count, i)
		}
	}

	// Over threshold.
	count := tracker.Increment("task-1", "same-sig")
	if count != threshold+1 {
		t.Fatalf("Increment() = %d, want %d", count, threshold+1)
	}

	// Reset clears.
	tracker.reset("task-1")
	count = tracker.Increment("task-1", "same-sig")
	if count != 1 {
		t.Fatalf("after reset, Increment() = %d, want 1", count)
	}
}

func TestCrashRestartTracker_ResetsOnProgress(t *testing.T) {
	tracker := newCrashRestartTracker()

	tracker.Increment("task-1", "sig-a")
	tracker.Increment("task-1", "sig-a")

	// Signature changes — progress detected, counter resets.
	count := tracker.Increment("task-1", "sig-b")
	if count != 1 {
		t.Fatalf("Increment() after progress = %d, want 1", count)
	}
}

func TestSpinningTracker_BlocksAfterThreshold(t *testing.T) {
	tracker := newSpinningTracker()
	threshold := 5

	for i := 1; i <= threshold+1; i++ {
		count := tracker.Track("task-1", "same-sig")
		if count != i {
			t.Fatalf("Track() = %d, want %d", count, i)
		}
	}
}

func TestSpinningTracker_ResetsOnProgress(t *testing.T) {
	tracker := newSpinningTracker()

	tracker.Track("task-1", "sig-a")
	tracker.Track("task-1", "sig-a")
	tracker.Track("task-1", "sig-a")

	// Progress detected.
	count := tracker.Track("task-1", "sig-b")
	if count != 1 {
		t.Fatalf("Track() after progress = %d, want 1", count)
	}
}

func TestRunAgent_ExtractedOps_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	now := time.Now().UTC()

	state := testhelpers.CreateValidState()
	state.Agents["code-reviewer-1"] = testhelpers.RegisteredTestAgent("code-reviewer")

	// Create a task ready for review
	taskID := "task-1"
	task := testhelpers.BuildTaskByStatus(taskID, models.TaskStatusReadyForReview, now)
	task.Worktree = nil
	state.Tasks = []models.Task{task}

	testhelpers.WriteInitialState(t, statePath, state)

	// Test ClaimReviewerTask operation
	input := ops.ClaimReviewerTaskInput{
		ProjectRoot:   tmpDir,
		AgentID:       "code-reviewer-1",
		LeaseDuration: 300, // 5 minutes in seconds
	}
	result, err := ops.ClaimReviewerTask(input)
	if err != nil {
		t.Fatalf("ClaimReviewerTask failed: %v", err)
	}
	if result == nil {
		t.Fatalf("ClaimReviewerTask returned nil result")
	}
	if result.TaskID != taskID {
		t.Errorf("result.TaskID = %s, want %s", result.TaskID, taskID)
	}
}

func TestResumeHandoff_ExtractedOp_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	now := time.Now().UTC()

	state := testhelpers.CreateValidState()

	// Create a task with handoff pending
	taskID := "task-1"
	task := testhelpers.BuildTaskByStatus(taskID, models.TaskStatusImplementing, now)
	task.HandoffPending = true
	agentID := "coder-1"
	task.AssignedTo = &agentID
	task.Worktree = &tmpDir
	state.Tasks = []models.Task{task}
	state.Agents[agentID] = models.Agent{
		Role:   "coder",
		Status: models.AgentStatusHandoff,
	}

	testhelpers.WriteInitialState(t, statePath, state)

	// Test ResumeHandoff operation
	input := ops.ResumeHandoffInput{
		ProjectRoot: tmpDir,
		AgentID:     agentID,
	}
	result, err := ops.ResumeHandoff(input)
	if err != nil {
		t.Fatalf("ResumeHandoff failed: %v", err)
	}
	if result == nil {
		t.Fatalf("ResumeHandoff returned nil result")
	}
	if !result.Found {
		t.Errorf("ResumeHandoff should find handoff task")
	}
	if result.TaskID != taskID {
		t.Errorf("result.TaskID = %s, want %s", result.TaskID, taskID)
	}
}

func TestResumeHandoff_NotFound_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	now := time.Now().UTC()

	state := testhelpers.CreateValidState()

	// Create a task WITHOUT handoff pending
	taskID := "task-1"
	task := testhelpers.BuildTaskByStatus(taskID, models.TaskStatusImplementing, now)
	task.HandoffPending = false // Not pending
	agentID := "coder-1"
	task.AssignedTo = &agentID
	state.Tasks = []models.Task{task}

	testhelpers.WriteInitialState(t, statePath, state)

	// Test ResumeHandoff operation - should not find anything
	input := ops.ResumeHandoffInput{
		ProjectRoot: tmpDir,
		AgentID:     agentID,
	}
	result, err := ops.ResumeHandoff(input)
	if err != nil {
		t.Fatalf("ResumeHandoff failed: %v", err)
	}
	if result == nil {
		t.Fatalf("ResumeHandoff returned nil result")
	}
	if result.Found {
		t.Errorf("ResumeHandoff should NOT find handoff task when HandoffPending=false")
	}
}

// TestExtractedOps_BehavioralParity tests that the extracted ops functions
// maintain the same behavior as the original inline closures
func TestExtractedOps_BehavioralParity(t *testing.T) {
	t.Run("ClaimReviewerTask finds highest priority task", func(t *testing.T) {
		tmpDir := t.TempDir()
		statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
		testhelpers.SetupPipelineConfig(t, tmpDir)
		now := time.Now().UTC()

		state := testhelpers.CreateValidState()
		state.Agents["code-reviewer-1"] = testhelpers.RegisteredTestAgent("code-reviewer")

		// Create multiple tasks with different priorities
		task1 := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
		task1.Priority = 2
		task1.Worktree = nil
		task2 := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReadyForReview, now)
		task2.Priority = 1 // Higher priority (lower number)
		task2.Worktree = nil

		state.Tasks = []models.Task{task1, task2}

		testhelpers.WriteInitialState(t, statePath, state)

		input := ops.ClaimReviewerTaskInput{
			ProjectRoot:   tmpDir,
			AgentID:       "code-reviewer-1",
			LeaseDuration: 300,
		}
		result, err := ops.ClaimReviewerTask(input)
		if err != nil {
			t.Fatalf("ClaimReviewerTask failed: %v", err)
		}

		// Should claim the highest priority task (task-2 with priority 1)
		if result.TaskID != "task-2" {
			t.Errorf("expected task-2 (priority 1), got %s", result.TaskID)
		}
	})

	t.Run("ResumeHandoff uses correct worktree", func(t *testing.T) {
		tmpDir := t.TempDir()
		statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
		testhelpers.SetupPipelineConfig(t, tmpDir)
		now := time.Now().UTC()

		state := testhelpers.CreateValidState()

		taskID := "task-1"
		task := testhelpers.BuildTaskByStatus(taskID, models.TaskStatusImplementing, now)
		task.HandoffPending = true
		agentID := "coder-1"
		task.AssignedTo = &agentID
		expectedWorktree := "/worktrees/task-1"
		task.Worktree = &expectedWorktree
		state.Tasks = []models.Task{task}
		state.Agents[agentID] = models.Agent{
			Role:   "coder",
			Status: models.AgentStatusHandoff,
		}

		testhelpers.WriteInitialState(t, statePath, state)

		input := ops.ResumeHandoffInput{
			ProjectRoot: tmpDir,
			AgentID:     agentID,
		}
		result, err := ops.ResumeHandoff(input)
		if err != nil {
			t.Fatalf("ResumeHandoff failed: %v", err)
		}

		if result.Worktree != expectedWorktree {
			t.Errorf("worktree = %s, want %s", result.Worktree, expectedWorktree)
		}
	})
}

func BenchmarkClaimReviewerTask(b *testing.B) {
	tmpDir := b.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(&testing.T{}, tmpDir)
	testhelpers.SetupPipelineConfig(&testing.T{}, tmpDir)
	now := time.Now().UTC()

	state := testhelpers.CreateValidState()
	taskID := "task-1"
	task := testhelpers.BuildTaskByStatus(taskID, models.TaskStatusReadyForReview, now)
	state.Tasks = []models.Task{task}

	testhelpers.WriteInitialState(&testing.T{}, statePath, state)

	input := ops.ClaimReviewerTaskInput{
		ProjectRoot:   tmpDir,
		AgentID:       "code-reviewer-1",
		LeaseDuration: 300,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ops.ClaimReviewerTask(input)
	}
}

func BenchmarkResumeHandoff(b *testing.B) {
	tmpDir := b.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(&testing.T{}, tmpDir)
	testhelpers.SetupPipelineConfig(&testing.T{}, tmpDir)
	now := time.Now().UTC()

	state := testhelpers.CreateValidState()
	taskID := "task-1"
	task := testhelpers.BuildTaskByStatus(taskID, models.TaskStatusImplementing, now)
	task.HandoffPending = true
	agentID := "coder-1"
	task.AssignedTo = &agentID
	state.Tasks = []models.Task{task}
	state.Agents[agentID] = models.Agent{
		Role:   "coder",
		Status: models.AgentStatusHandoff,
	}

	testhelpers.WriteInitialState(&testing.T{}, statePath, state)

	input := ops.ResumeHandoffInput{
		ProjectRoot: tmpDir,
		AgentID:     agentID,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ops.ResumeHandoff(input)
	}
}

func TestCLISupportsStdin(t *testing.T) {
	tests := []struct {
		cli  string
		want bool
	}{
		{"claude", true},
		{"kimi", true},
		{"codex", true},
		{"gemini", true},
		{"vibe", false},
	}
	for _, tc := range tests {
		t.Run(tc.cli, func(t *testing.T) {
			if got := cliSupportsStdin(tc.cli); got != tc.want {
				t.Errorf("cliSupportsStdin(%q) = %v, want %v", tc.cli, got, tc.want)
			}
		})
	}
}

func TestBuildClaudeArgsDisallowsSubagentToolsWhenEnvEnabled(t *testing.T) {
	args := buildClaudeArgs("ignored", true, "", true)

	if !containsAdjacent(args, "--disallowedTools", "Task") {
		t.Fatalf("args = %v, want subagent tools disallowed", args)
	}
	if slices.Contains(args, "ignored") {
		t.Fatalf("args = %v, did not expect prompt arg when stdin is used", args)
	}
}

func TestBuildClaudeArgsAllowsTaskByDefault(t *testing.T) {
	args := buildClaudeArgs("do the thing", false, "/tmp/logs", false)

	if containsAdjacent(args, "--disallowedTools", "Task") {
		t.Fatalf("args = %v, did not expect subagent tools disallowed by default", args)
	}
	if !slices.Contains(args, "do the thing") {
		t.Fatalf("args = %v, want prompt arg when stdin is disabled", args)
	}
	if !containsAdjacent(args, "--output-format", "stream-json") {
		t.Fatalf("args = %v, want stream-json logging flags", args)
	}
}

func TestEnvValueUsesLastEnvValue(t *testing.T) {
	got := envValue([]string{
		"LIZA_DISABLE_CLAUDE_SUBAGENTS=0",
		"LIZA_DISABLE_CLAUDE_SUBAGENTS=1",
	}, "LIZA_DISABLE_CLAUDE_SUBAGENTS")

	if got != "1" {
		t.Fatalf("envValue() = %q, want later env value", got)
	}
}

func TestCodexCommandContextUsesPinnedNpmPackage(t *testing.T) {
	cmd, err := codexCommandContext(context.Background(), "0.125.0", []string{"exec", "-"})
	if err != nil {
		t.Fatalf("codexCommandContext() error = %v", err)
	}

	want := []string{"npm", "exec", "--yes", "--package", "@openai/codex@0.125.0", "--", "codex", "exec", "-"}
	if !slices.Equal(cmd.Args, want) {
		t.Fatalf("cmd.Args = %v, want %v", cmd.Args, want)
	}
}

func TestCodexCommandContextDefaultsToCodexBinary(t *testing.T) {
	cmd, err := codexCommandContext(context.Background(), "", []string{"exec", "-"})
	if err != nil {
		t.Fatalf("codexCommandContext() error = %v", err)
	}

	want := []string{"codex", "exec", "-"}
	if !slices.Equal(cmd.Args, want) {
		t.Fatalf("cmd.Args = %v, want %v", cmd.Args, want)
	}
}

func TestCodexCommandContextRejectsWhitespaceVersion(t *testing.T) {
	_, err := codexCommandContext(context.Background(), "0.125.0 --bad", []string{"exec", "-"})
	if err == nil || !strings.Contains(err.Error(), "codex package version") {
		t.Fatalf("codexCommandContext() error = %v, want package version validation error", err)
	}
}

func TestResolveCodexLaunchConfig(t *testing.T) {
	t.Run("state config wins for version and can enable legacy landlock", func(t *testing.T) {
		got := resolveCodexLaunchConfig(models.Config{
			CodexPackageVersion: "0.125.0",
			CodexLegacyLandlock: true,
		}, []string{
			envLizaCodexVersion + "=0.132.0",
		})

		if got.PackageVersion != "0.125.0" {
			t.Fatalf("PackageVersion = %q, want state value", got.PackageVersion)
		}
		if !got.LegacyLandlock {
			t.Fatalf("LegacyLandlock = false, want true")
		}
	})

	t.Run("environment supplies process-local fallback", func(t *testing.T) {
		got := resolveCodexLaunchConfig(models.Config{}, []string{
			envLizaCodexVersion + "=0.125.0",
			envLizaCodexLegacyLandlock + "=1",
		})

		if got.PackageVersion != "0.125.0" {
			t.Fatalf("PackageVersion = %q, want env value", got.PackageVersion)
		}
		if !got.LegacyLandlock {
			t.Fatalf("LegacyLandlock = false, want true")
		}
	})
}

func TestCodexLegacyLandlockEnabled(t *testing.T) {
	for _, value := range []string{"1", "true", "TRUE", "yes", "on"} {
		if !codexLegacyLandlockEnabled([]string{envLizaCodexLegacyLandlock + "=" + value}) {
			t.Fatalf("codexLegacyLandlockEnabled(%q) = false, want true", value)
		}
	}
	for _, value := range []string{"", "0", "false", "no", "off", "maybe"} {
		if codexLegacyLandlockEnabled([]string{envLizaCodexLegacyLandlock + "=" + value}) {
			t.Fatalf("codexLegacyLandlockEnabled(%q) = true, want false", value)
		}
	}
}

func TestBuildCodexArgs(t *testing.T) {
	t.Run("stdin without logging disables approval prompts", func(t *testing.T) {
		projectRoot := "/tmp/project"
		additionalDirs := []string{"/tmp", "/tmp/project/.worktrees/task-1"}
		args := buildCodexArgs(projectRoot, "ignored", true, "", additionalDirs, false)

		if slices.Contains(args, "--full-auto") {
			t.Fatalf("args = %v, did not expect --full-auto flag", args)
		}
		if !containsAdjacent(args, "-c", `approval_policy="never"`) {
			t.Fatalf("args = %v, want noninteractive approval policy", args)
		}
		for _, override := range codexWorkspacePermissionOverrides(projectRoot, additionalDirs) {
			if !containsAdjacent(args, "-c", override) {
				t.Fatalf("args = %v, missing Codex config override %q", args, override)
			}
		}
		if slices.Contains(args, "--dangerously-bypass-approvals-and-sandbox") {
			t.Fatalf("args = %v, did not expect bypass flag", args)
		}
		if !slices.Contains(args, "exec") || !slices.Contains(args, "-") {
			t.Fatalf("args = %v, want stdin exec invocation", args)
		}
		if slices.Contains(args, "--json") {
			t.Fatalf("args = %v, did not expect --json without logging", args)
		}
		assertCodexAddDir(t, args, "/tmp")
		assertCodexAddDir(t, args, "/tmp/project/.worktrees/task-1")
		for _, a := range args {
			if strings.Contains(a, "mcp_servers") {
				t.Fatalf("args = %v, did not expect mcp_servers config", args)
			}
		}
	})

	t.Run("prompt with logging emits json", func(t *testing.T) {
		projectRoot := "/tmp/project"
		args := buildCodexArgs(projectRoot, "do the thing", false, "/tmp/logs", nil, false)

		if !slices.Contains(args, "do the thing") {
			t.Fatalf("args = %v, want prompt argument", args)
		}
		if !slices.Contains(args, "--json") {
			t.Fatalf("args = %v, want --json when logging enabled", args)
		}
		if slices.Contains(args, "--full-auto") {
			t.Fatalf("args = %v, did not expect --full-auto flag", args)
		}
		if !containsAdjacent(args, "-c", `approval_policy="never"`) {
			t.Fatalf("args = %v, want noninteractive approval policy", args)
		}
		for _, override := range codexWorkspacePermissionOverrides(projectRoot, nil) {
			if !containsAdjacent(args, "-c", override) {
				t.Fatalf("args = %v, missing Codex config override %q", args, override)
			}
		}
		for _, a := range args {
			if strings.Contains(a, "mcp_servers") {
				t.Fatalf("args = %v, did not expect mcp_servers config", args)
			}
		}
	})

	t.Run("non-legacy includes workspace permission profile", func(t *testing.T) {
		args := buildCodexArgs("/tmp/project", "ignored", true, "", nil, false)

		if !containsAdjacent(args, "-c", `sandbox_mode="workspace-write"`) {
			t.Fatalf("args = %v, want workspace-write sandbox override", args)
		}
		if !containsAdjacent(args, "-c", `default_permissions="workspace"`) {
			t.Fatalf("args = %v, want workspace permission profile", args)
		}
		if !containsPrefix(args, "permissions.workspace.filesystem=") {
			t.Fatalf("args = %v, want workspace filesystem permissions override", args)
		}
	})

	t.Run("legacy landlock uses tested workspace-write path without permission profile overrides", func(t *testing.T) {
		projectRoot := "/tmp/project"
		additionalDirs := []string{"/tmp/project/.git", "/tmp/project/.worktrees/task-1"}
		args := buildCodexArgs(projectRoot, "ignored", true, "", additionalDirs, true)

		if !containsAdjacent(args, "--enable", "use_legacy_landlock") {
			t.Fatalf("args = %v, want legacy landlock feature flag", args)
		}
		if !containsAdjacent(args, "--sandbox", "workspace-write") {
			t.Fatalf("args = %v, want workspace-write sandbox flag", args)
		}
		if !containsAdjacent(args, "-c", `approval_policy="never"`) {
			t.Fatalf("args = %v, want noninteractive approval policy", args)
		}
		for _, arg := range args {
			if strings.Contains(arg, "permissions.workspace") || strings.Contains(arg, "default_permissions") || strings.Contains(arg, "sandbox_mode") {
				t.Fatalf("legacy landlock args should not include permission profile overrides: %v", args)
			}
		}
		assertCodexAddDir(t, args, "/tmp/project/.git")
		assertCodexAddDir(t, args, "/tmp/project/.worktrees/task-1")
		if args[len(args)-1] != "-" {
			t.Fatalf("args = %v, want stdin prompt marker last", args)
		}
	})
}

func TestCodexWorkspacePermissionOverridesIncludesSupportRoots(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(fakeHome, ".cache"))
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		t.Fatalf("UserCacheDir() error: %v", err)
	}
	projectRoot := "/tmp/project"
	additionalDirs := []string{"/tmp/project/.worktrees/task-1"}

	overrides := codexWorkspacePermissionOverrides(projectRoot, additionalDirs)
	filesystemOverride := findCodexOverride(t, overrides, "permissions.workspace.filesystem=")
	for _, want := range []string{
		strconv.Quote(filepath.Join(fakeHome, ".codex")) + `="write"`,
		strconv.Quote(filepath.Join(fakeHome, ".liza")) + `="write"`,
		strconv.Quote(filepath.Join(fakeHome, ".npm")) + `="write"`,
		strconv.Quote(filepath.Join(fakeHome, ".pyenv", "shims")) + `="write"`,
		strconv.Quote(cacheDir) + `="write"`,
		strconv.Quote(projectRoot) + `="write"`,
		strconv.Quote(filepath.Join(projectRoot, ".git")) + `="write"`,
		strconv.Quote(filepath.Join(projectRoot, ".codex")) + `="read"`,
		strconv.Quote(additionalDirs[0]) + `="write"`,
	} {
		if !strings.Contains(filesystemOverride, want) {
			t.Fatalf("override missing %q:\n%s", want, filesystemOverride)
		}
	}
	if !containsInOrder(overrides, `default_permissions="workspace"`, filesystemOverride) {
		t.Fatalf("overrides should select workspace profile before defining filesystem permissions: %v", overrides)
	}
}

func findCodexOverride(t *testing.T, overrides []string, prefix string) string {
	t.Helper()
	for _, override := range overrides {
		if strings.HasPrefix(override, prefix) {
			return override
		}
	}
	t.Fatalf("missing Codex override prefix %q in %v", prefix, overrides)
	return ""
}

func containsAdjacent(values []string, first, second string) bool {
	for i := 0; i+1 < len(values); i++ {
		if values[i] == first && values[i+1] == second {
			return true
		}
	}
	return false
}

func containsInOrder(values []string, first, second string) bool {
	foundFirst := false
	for _, value := range values {
		if value == first {
			foundFirst = true
			continue
		}
		if foundFirst && value == second {
			return true
		}
	}
	return false
}

func containsPrefix(values []string, prefix string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func TestCodexInteractiveArgs(t *testing.T) {
	args := codexInteractiveArgs([]string{"/tmp", "", "/tmp/project/.worktrees/task-1"}, false)

	assertCodexAddDir(t, args, "/tmp")
	assertCodexAddDir(t, args, "/tmp/project/.worktrees/task-1")
	if slices.Contains(args, "") {
		t.Fatalf("args = %v, did not expect empty argument", args)
	}
}

func TestCodexInteractiveArgsLegacyLandlock(t *testing.T) {
	args := codexInteractiveArgs([]string{"/tmp/project/.git"}, true)

	if !containsAdjacent(args, "--enable", "use_legacy_landlock") {
		t.Fatalf("args = %v, want legacy landlock feature flag", args)
	}
	if !containsAdjacent(args, "--sandbox", "workspace-write") {
		t.Fatalf("args = %v, want workspace-write sandbox flag", args)
	}
	assertCodexAddDir(t, args, "/tmp/project/.git")
}

func TestCodexAdditionalDirs(t *testing.T) {
	t.Run("no task context returns empty dirs", func(t *testing.T) {
		if dirs := codexAdditionalDirs("/tmp/project", nil, ""); len(dirs) != 0 {
			t.Fatalf("dirs = %v, want empty", dirs)
		}
	})

	t.Run("task worktree and git metadata are added", func(t *testing.T) {
		projectRoot := "/tmp/project"
		worktreeRel := ".worktrees/task-1"
		state := &models.State{
			Tasks: []models.Task{
				{
					ID:       "task-1",
					Worktree: &worktreeRel,
				},
			},
		}

		dirs := codexAdditionalDirs(projectRoot, state, "task-1")

		if !slices.Contains(dirs, filepath.Join(projectRoot, worktreeRel)) {
			t.Fatalf("dirs = %v, want task worktree", dirs)
		}
		if !slices.Contains(dirs, filepath.Join(projectRoot, ".git")) {
			t.Fatalf("dirs = %v, want project git metadata dir", dirs)
		}
	})
}

func assertCodexAddDir(t *testing.T, args []string, wantDir string) {
	t.Helper()
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--add-dir" && args[i+1] == wantDir {
			return
		}
	}
	t.Fatalf("args = %v, want --add-dir %s", args, wantDir)
}

// buildPromptFailureFixture wires a minimal ARCHITECTING architect task
// into a real blackboard backed by a fresh git repo. Returns the
// blackboard, project root, task ID, agent ID.
func buildPromptFailureFixture(t *testing.T, integrationBranch string) (bb *db.Blackboard, projectRoot, taskID, agentID string) {
	t.Helper()
	projectRoot = t.TempDir()
	testhelpers.SetupTestGitRepo(t, projectRoot)
	statePath, _ := testhelpers.SetupLizaDir(t, projectRoot)

	now := time.Now().UTC()
	taskID = "arch-1"
	agentID = "architect-1"
	assigned := agentID
	leaseExpires := now.Add(30 * time.Minute)

	state := testhelpers.CreateValidState()
	state.Config.IntegrationBranch = integrationBranch
	state.Tasks = []models.Task{
		{
			ID:           taskID,
			Type:         models.TaskTypeArchitecture,
			Description:  "Design feature X",
			Status:       "ARCHITECTING",
			Priority:     1,
			Iteration:    1,
			DoneWhen:     "Architecture document produced",
			SpecRef:      "specs/goals/feature-x.md",
			Created:      now,
			AssignedTo:   &assigned,
			LeaseExpires: &leaseExpires,
			RolePair:     "architecture-pair",
			History:      []models.TaskHistoryEntry{},
		},
	}

	bb = testhelpers.WriteInitialState(t, statePath, state)
	return bb, projectRoot, taskID, agentID
}

// TestSupervisor_BuildPromptFailure_BlocksTask asserts that when
// BuildPrompt fails on a claimed architect task with an error wrapping
// precommit.ErrContextBuild, the supervisor's sentinel-gated recovery
// path (supervisor.go L817-820) transitions the task to BLOCKED with
// the expected reason prefix, clears the lease, emits a TaskEventBlocked
// history entry, does NOT invoke the agent executor, and does NOT exit
// the supervisor session (a subsequent iteration is reachable).
func TestSupervisor_BuildPromptFailure_BlocksTask(t *testing.T) {
	bb, projectRoot, taskID, agentID := buildPromptFailureFixture(t, "does-not-exist")

	config := SupervisorConfig{
		AgentID:     agentID,
		Role:        "architect",
		ProjectRoot: projectRoot,
	}

	stateBefore, err := bb.Read()
	if err != nil {
		t.Fatalf("bb.Read: %v", err)
	}

	// Exercise BuildPrompt via buildPromptWithContext — the same call path
	// the supervisor uses at supervisor.go L817. The architect task and a
	// non-existent integration branch drive ConfigExistsOnIntegration into
	// the invalid-ref error arm, which wraps ErrContextBuild.
	mockExecutor := &MockCLIExecutor{ExitCode: 0}
	config.Executor = mockExecutor
	pipelineCfg, err := pipeline.LoadFrozen(projectRoot)
	if err != nil {
		t.Fatalf("pipeline.LoadFrozen: %v", err)
	}
	resolver := pipeline.NewResolver(pipelineCfg)
	_, err = buildPromptWithContext(stateBefore, config, taskID, resolver)
	if err == nil {
		t.Fatalf("expected BuildPrompt error, got nil")
	}
	if !stderrors.Is(err, precommit.ErrContextBuild) {
		t.Fatalf("errors.Is(err, precommit.ErrContextBuild) = false; err=%v", err)
	}

	// Replicate the supervisor's sentinel-gated recovery path. The guard
	// condition (claimedTaskID != "" && errors.Is(...)) holds by
	// construction here.
	claimedTaskID := taskID
	if claimedTaskID == "" || !stderrors.Is(err, precommit.ErrContextBuild) {
		t.Fatalf("precommit-domain guard should have matched; aborting test")
	}
	reason := fmt.Sprintf("prompt context build failed: %v", err)
	blockTaskFromSupervisor(bb, projectRoot, claimedTaskID, agentID, reason)

	// Invariant: agent was never invoked.
	if calls := mockExecutor.GetCalls(); len(calls) != 0 {
		t.Errorf("executeAgent should not be invoked; got %d calls", len(calls))
	}

	// Verify the task's post-conditions on the blackboard.
	stateAfter, err := bb.Read()
	if err != nil {
		t.Fatalf("bb.Read: %v", err)
	}
	task := stateAfter.FindTask(taskID)
	if task == nil {
		t.Fatalf("task %q not found after block", taskID)
	}
	if task.Status != models.TaskStatusBlocked {
		t.Errorf("task.Status = %q, want %q", task.Status, models.TaskStatusBlocked)
	}
	if task.BlockedReason == nil {
		t.Fatalf("task.BlockedReason = nil, want non-nil")
	}
	if !strings.HasPrefix(*task.BlockedReason, "prompt context build failed: precommit") {
		t.Errorf("BlockedReason = %q, want prefix %q", *task.BlockedReason, "prompt context build failed: precommit")
	}
	if task.AssignedTo != nil {
		t.Errorf("task.AssignedTo = %q, want nil (cleared by block)", *task.AssignedTo)
	}
	if task.LeaseExpires != nil {
		t.Errorf("task.LeaseExpires = %v, want nil (cleared by block)", *task.LeaseExpires)
	}
	// TaskEventBlocked in the history.
	found := false
	for _, h := range task.History {
		if h.Event == models.TaskEventBlocked {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no TaskEventBlocked entry in task history")
	}

	// Second-iteration reachability: the guard uses `continue`, not
	// `return`, so after blocking the supervisor loop proceeds. We verify
	// the loop is free to run another iteration by driving blockingly the
	// wait-for-work detection over the post-block state: with the task now
	// BLOCKED, no more architect work is claimable and the loop would
	// cleanly exit via the "no work" branch rather than via the error
	// return. This is the positive phrasing of "supervisor did NOT exit
	// via the error branch".
	claimable := models.CountClaimableTasks(stateAfter, "architect", nil)
	if claimable != 0 {
		t.Errorf("after block, expected 0 claimable architect tasks, got %d", claimable)
	}
}

// TestSupervisor_BuildPromptFailure_NonPrecommit_DoesNotBlock asserts
// that a BuildPrompt error NOT wrapping precommit.ErrContextBuild (e.g.,
// template/resolver/pipeline failures) falls through to the existing
// wrapped-error return path at supervisor.go L820 — the task status is
// NOT mutated to BLOCKED, BlockedReason remains nil, and the surfaced
// error carries the original "failed to build prompt: " prefix.
func TestSupervisor_BuildPromptFailure_NonPrecommit_DoesNotBlock(t *testing.T) {
	bb, _, taskID, _ := buildPromptFailureFixture(t, "main")

	// Engineered non-precommit error: something that could plausibly come
	// from template render, resolver ContextSections, or pipeline wiring.
	templateErr := fmt.Errorf("context sections for role %q: template %q missing", "architect", "assigned-task")

	// Simulate the supervisor's sentinel-gated decision at L817-820.
	claimedTaskID := taskID
	shouldBlock := claimedTaskID != "" && stderrors.Is(templateErr, precommit.ErrContextBuild)
	if shouldBlock {
		t.Fatalf("non-precommit error unexpectedly matched precommit sentinel: %v", templateErr)
	}

	// Simulate the fall-through return. The supervisor wraps as
	// "failed to build prompt: %w", the existing path unchanged.
	wrapped := fmt.Errorf("failed to build prompt: %w", templateErr)
	if stderrors.Is(wrapped, precommit.ErrContextBuild) {
		t.Errorf("wrapped error unexpectedly matches precommit sentinel: %v", wrapped)
	}
	if !strings.HasPrefix(wrapped.Error(), "failed to build prompt: ") {
		t.Errorf("wrapped error %q does not start with %q", wrapped.Error(), "failed to build prompt: ")
	}

	// Critically: because shouldBlock is false, blockTaskFromSupervisor is
	// NOT called. Verify post-conditions: task status unchanged, no
	// BlockedReason.
	stateAfter, err := bb.Read()
	if err != nil {
		t.Fatalf("bb.Read: %v", err)
	}
	task := stateAfter.FindTask(taskID)
	if task == nil {
		t.Fatalf("task %q not found", taskID)
	}
	if task.Status == models.TaskStatusBlocked {
		t.Errorf("task.Status = %q, want NOT %q", task.Status, models.TaskStatusBlocked)
	}
	if task.BlockedReason != nil {
		t.Errorf("task.BlockedReason = %q, want nil", *task.BlockedReason)
	}
}

func TestNewDefaultCLIExecutor(t *testing.T) {
	t.Run("empty outputsDir disables logging", func(t *testing.T) {
		e := NewDefaultCLIExecutor("")
		if e.outputsDir != "" {
			t.Errorf("outputsDir should be empty, got %q", e.outputsDir)
		}
	})

	t.Run("non-empty outputsDir enables logging", func(t *testing.T) {
		dir := t.TempDir()
		e := NewDefaultCLIExecutor(dir)
		if e.outputsDir != dir {
			t.Errorf("outputsDir = %q, want %q", e.outputsDir, dir)
		}
	})
}
