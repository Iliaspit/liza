package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/testhelpers"
	acpplugin "github.com/liza-mas/liza/plugin/acp"
)

type realACPXBenchmarkAgent struct {
	mu          sync.Mutex
	mode        string
	sessionName string
	acp         *acpplugin.RunManager
	runs        []acpplugin.RunMetric
	turn        int
}

func newRealACPXBenchmarkAgent(mode string) *realACPXBenchmarkAgent {
	return &realACPXBenchmarkAgent{
		mode:        mode,
		sessionName: "liza-real-acp-benchmark",
		acp:         acpplugin.NewRunManager(),
	}
}

func (a *realACPXBenchmarkAgent) Run(ctx context.Context, req LLMAgentRunRequest) (LLMAgentRunResult, error) {
	taskID := req.TaskID
	if taskID == "" {
		taskID = req.SessionID
	}
	if taskID == "" {
		taskID = "unknown-task"
	}

	a.mu.Lock()
	a.turn++
	turn := a.turn
	a.mu.Unlock()

	sessionKey := taskID
	prompt := realACPFullPrompt(req.Prompt)
	if a.mode == "warm-acp" {
		sessionKey = req.ProjectRoot + ":" + req.AgentID
		if turn == 1 {
			if err := ensureACPXSession(ctx, req.ProjectRoot, a.sessionName); err != nil {
				return LLMAgentRunResult{ExitCode: 1, Output: err.Error()}, err
			}
		} else {
			prompt = realACPTaskDeltaPrompt(req.ProjectRoot, taskID)
		}
	}

	run := a.acp.StartWithSessionKey(taskID, sessionKey)
	started := time.Now()
	result, err := runACPXTurn(ctx, req.ProjectRoot, a.mode, a.sessionName, prompt)
	if err != nil {
		metric, _ := a.acp.Finish(taskID, 1)
		a.record(metric)
		return LLMAgentRunResult{ExitCode: 1, Output: result.Output, Usage: toLLMUsage(result.Usage), WarmUsage: run.Warm, SessionID: run.SessionID}, err
	}

	_ = a.acp.HandleEvent(acpplugin.Event{
		TaskID:  taskID,
		Kind:    acpplugin.EventMessage,
		Message: result.Output,
	})
	_ = a.acp.HandleEvent(acpplugin.Event{
		TaskID: taskID,
		Kind:   acpplugin.EventUsage,
		Payload: map[string]any{
			"usage": map[string]any{
				"input_tokens":        result.Usage.InputTokens,
				"output_tokens":       result.Usage.OutputTokens,
				"cached_read_tokens":  result.Usage.CachedReadTokens,
				"cached_write_tokens": result.Usage.CachedWriteTokens,
			},
		},
	})

	if err := markBenchmarkTaskReadyForReview(req.ProjectRoot, taskID, req.AgentID); err != nil {
		metric, _ := a.acp.Finish(taskID, 1)
		a.record(metric)
		return LLMAgentRunResult{ExitCode: 1, Output: err.Error(), Usage: toLLMUsage(metric.Usage), WarmUsage: metric.Warm, SessionID: metric.SessionID}, err
	}

	metric, err := a.acp.Finish(taskID, 0)
	if err != nil {
		return LLMAgentRunResult{ExitCode: 1, Output: err.Error()}, err
	}
	metric.Duration = time.Since(started)
	a.record(metric)
	return LLMAgentRunResult{
		ExitCode:  0,
		Output:    metric.Output,
		Usage:     toLLMUsage(metric.Usage),
		WarmUsage: metric.Warm,
		SessionID: metric.SessionID,
	}, nil
}

func (a *realACPXBenchmarkAgent) RunInteractive(context.Context, LLMAgentInteractiveRequest) (int, error) {
	return 1, fmt.Errorf("interactive mode is not supported by real ACP benchmark agent")
}

func (a *realACPXBenchmarkAgent) record(metric acpplugin.RunMetric) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.runs = append(a.runs, metric)
}

func (a *realACPXBenchmarkAgent) Runs() []acpplugin.RunMetric {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]acpplugin.RunMetric, len(a.runs))
	copy(out, a.runs)
	return out
}

type acpxTurnResult struct {
	Output string
	Usage  acpplugin.Usage
}

func TestRealACPXBenchmarkSimplifiedLizaProject(t *testing.T) {
	if os.Getenv("LIZA_RUN_REAL_ACP_BENCH") != "1" {
		t.Skip("set LIZA_RUN_REAL_ACP_BENCH=1 to run real acpx/codex model benchmark")
	}

	baseline := runRealACPXBenchmark(t, "fresh")
	warm := runRealACPXBenchmark(t, "warm-acp")

	baselineDuration := totalDuration(baseline)
	warmDuration := totalDuration(warm)
	baselineInput := totalInputTokens(baseline)
	warmInput := totalInputTokens(warm)
	baselineFreshInput := totalFreshInputTokens(baseline)
	warmFreshInput := totalFreshInputTokens(warm)
	warmRun := findWarmRun(t, warm)
	matchingBaselineInput := findRunTokens(t, baseline, warmRun.TaskID)

	t.Logf("real fresh baseline: runs=%d duration=%s input_tokens=%d fresh_input_tokens=%d cached_read_tokens=%d output_tokens=%d",
		len(baseline), baselineDuration, baselineInput, baselineFreshInput, totalCachedReadTokens(baseline), totalOutputTokens(baseline))
	t.Logf("real warm ACP: runs=%d duration=%s input_tokens=%d fresh_input_tokens=%d cached_read_tokens=%d output_tokens=%d warm_runs=%d",
		len(warm), warmDuration, warmInput, warmFreshInput, totalCachedReadTokens(warm), totalOutputTokens(warm), warmRunCount(warm))
	t.Logf("real difference: duration_delta=%.2f%% input_delta=%.2f%% fresh_input_delta=%.2f%%",
		percentImprovement(float64(baselineDuration), float64(warmDuration)),
		percentImprovement(float64(baselineInput), float64(warmInput)),
		percentImprovement(float64(baselineFreshInput), float64(warmFreshInput)))
	t.Logf("real warm task delta: task=%s baseline_input=%d warm_acp_input=%d savings=%.2f%%",
		warmRun.TaskID,
		matchingBaselineInput,
		warmRun.Usage.InputTokens,
		percentImprovement(float64(matchingBaselineInput), float64(warmRun.Usage.InputTokens)))

	if len(baseline) != 2 || len(warm) != 2 {
		t.Fatalf("runs: baseline=%d warm=%d, want 2 each", len(baseline), len(warm))
	}
	if !warmRun.Warm {
		t.Fatalf("expected one warm ACP run")
	}
	if warmRun.Usage.InputTokens >= matchingBaselineInput {
		t.Fatalf("warm ACP task input=%d, baseline matching task input=%d; expected lower warm input", warmRun.Usage.InputTokens, matchingBaselineInput)
	}
}

func runRealACPXBenchmark(t *testing.T, mode string) []acpplugin.RunMetric {
	t.Helper()

	projectRoot := t.TempDir()
	testhelpers.SetupTestGitRepo(t, projectRoot)
	statePath, _ := testhelpers.SetupLizaDir(t, projectRoot)
	testhelpers.CreateCommittedSpecFileOnIntegration(t, projectRoot, "vision.md", "# Real ACP Benchmark Vision\n\nRun two no-op benchmark tasks.\n")

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Config.CoderPollInterval = 1
	state.Config.DoerMaxWait = 1
	state.Config.LeaseDuration = 300
	state.Config.AgentProgressTimeout = 240
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-alpha", models.TaskStatusReady, now),
		testhelpers.BuildTaskByStatus("task-beta", models.TaskStatusReady, now),
	}
	testhelpers.WriteInitialState(t, statePath, state)

	benchAgent := newRealACPXBenchmarkAgent(mode)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	err := RunSupervisor(ctx, SupervisorConfig{
		AgentID:                  "coder-1",
		Role:                     "coder",
		ProjectRoot:              projectRoot,
		StatePath:                statePath,
		LogPath:                  filepath.Join(projectRoot, paths.ProjectDirName(), "log.yaml"),
		SpecsDir:                 filepath.Join(projectRoot, "specs"),
		CLIName:                  mode,
		LLMAgent:                 benchAgent,
		ExecutionTimeout:         4 * time.Minute,
		ExecutionProgressTimeout: 4 * time.Minute,
	})
	if err != nil {
		t.Fatalf("RunSupervisor(%s): %v", mode, err)
	}

	return benchAgent.Runs()
}

func ensureACPXSession(ctx context.Context, cwd, sessionName string) error {
	cmd := exec.CommandContext(ctx, "acpx", "--cwd", cwd, "codex", "sessions", "ensure", "--name", sessionName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("acpx sessions ensure: %w\n%s", err, string(output))
	}
	return nil
}

func runACPXTurn(ctx context.Context, cwd, mode, sessionName, prompt string) (acpxTurnResult, error) {
	args := []string{
		"--cwd", cwd,
		"--format", "json",
		"--timeout", "240",
		"--approve-reads",
		"--non-interactive-permissions", "deny",
		"codex",
	}
	if mode == "warm-acp" {
		args = append(args, "prompt", "-s", sessionName, "--file", "-")
	} else {
		args = append(args, "exec", "--file", "-")
	}

	cmd := exec.CommandContext(ctx, "acpx", args...)
	cmd.Stdin = strings.NewReader(prompt)
	output, err := cmd.CombinedOutput()
	result := parseACPXJSONLines(string(output))
	if err != nil {
		return result, fmt.Errorf("acpx %s turn failed: %w\n%s", mode, err, string(output))
	}
	return result, nil
}

func parseACPXJSONLines(output string) acpxTurnResult {
	var result acpxTurnResult
	scanner := bufio.NewScanner(strings.NewReader(output))
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg map[string]any
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if params, ok := msg["params"].(map[string]any); ok {
			if update, ok := params["update"].(map[string]any); ok {
				appendACPXUpdate(&result, update)
			}
		}
		if rawResult, ok := msg["result"].(map[string]any); ok {
			if usage, ok := usageFromACPX(rawResult["usage"]); ok {
				result.Usage = usage
			}
		}
	}
	return result
}

func appendACPXUpdate(result *acpxTurnResult, update map[string]any) {
	if update["sessionUpdate"] != "agent_message_chunk" {
		return
	}
	content, ok := update["content"].(map[string]any)
	if !ok {
		return
	}
	text, ok := content["text"].(string)
	if !ok {
		return
	}
	result.Output += text
}

func usageFromACPX(raw any) (acpplugin.Usage, bool) {
	m, ok := raw.(map[string]any)
	if !ok {
		return acpplugin.Usage{}, false
	}
	return acpplugin.Usage{
		InputTokens:       intFromJSONNumber(m["inputTokens"]),
		OutputTokens:      intFromJSONNumber(m["outputTokens"]),
		CachedReadTokens:  intFromJSONNumber(m["cachedReadTokens"]),
		CachedWriteTokens: 0,
	}, true
}

func intFromJSONNumber(raw any) int {
	switch v := raw.(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

func realACPFullPrompt(prompt string) string {
	return prompt + "\n\nBENCHMARK OVERRIDE:\nThis is a metrics-only Liza ACP benchmark. Do not edit files. Do not run write commands. Reply with exactly: BENCHMARK_DONE.\n"
}

func realACPTaskDeltaPrompt(projectRoot, taskID string) string {
	return "Use the Liza contract and project context already bootstrapped in this ACP session. " +
		"This is the next task delta only; do not edit files or run write commands. " +
		"Task delta:\n" + realACPTaskDelta(projectRoot, taskID) + "\nReply with exactly: BENCHMARK_DONE.\n"
}

func realACPTaskDelta(projectRoot, taskID string) string {
	tokens := estimateACPTaskDeltaTokens(projectRoot, taskID)
	return fmt.Sprintf("task_id=%s\nestimated_delta_tokens=%d\n", taskID, tokens)
}

func totalFreshInputTokens(runs []acpplugin.RunMetric) int {
	var total int
	for _, run := range runs {
		total += run.Usage.InputTokens
	}
	return total
}

func totalCachedReadTokens(runs []acpplugin.RunMetric) int {
	var total int
	for _, run := range runs {
		total += run.Usage.CachedReadTokens
	}
	return total
}

func totalOutputTokens(runs []acpplugin.RunMetric) int {
	var total int
	for _, run := range runs {
		total += run.Usage.OutputTokens
	}
	return total
}
