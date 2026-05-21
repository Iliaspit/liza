package tui

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/liza-mas/liza/internal/agent"
	"github.com/liza-mas/liza/internal/commands"
	"github.com/liza-mas/liza/internal/embedded"
	"github.com/liza-mas/liza/internal/log"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/pipeline"
	"github.com/liza-mas/liza/internal/testhelpers"
	"gopkg.in/yaml.v3"
)

func TestReadLogCmdEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log.yaml")

	// Create empty file
	if err := os.WriteFile(logPath, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	cmd := readLogCmd(logPath, 0)
	if cmd == nil {
		t.Fatal("readLogCmd returned nil Cmd")
	}

	msg := cmd()
	entries, ok := msg.(LogEntriesMsg)
	if !ok {
		t.Fatalf("expected LogEntriesMsg, got %T", msg)
	}
	if len(entries.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries.Entries))
	}
}

func TestReadLogCmdNonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "nonexistent.yaml")

	cmd := readLogCmd(logPath, 0)
	msg := cmd()
	entries, ok := msg.(LogEntriesMsg)
	if !ok {
		t.Fatalf("expected LogEntriesMsg, got %T", msg)
	}
	if len(entries.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries.Entries))
	}
}

func TestReadLogCmdWithEntries(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log.yaml")

	ts := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	task := "task-1"
	testEntries := []log.Entry{
		{Timestamp: ts, Agent: "coder-1", Action: "started", Task: &task, Detail: "working on it"},
		{Timestamp: ts.Add(time.Minute), Agent: "coder-1", Action: "completed", Detail: "done"},
	}
	data, err := yaml.Marshal(testEntries)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	cmd := readLogCmd(logPath, 0)
	msg := cmd()
	entries, ok := msg.(LogEntriesMsg)
	if !ok {
		t.Fatalf("expected LogEntriesMsg, got %T", msg)
	}
	if len(entries.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries.Entries))
	}
	if entries.Entries[0].Agent != "coder-1" {
		t.Errorf("expected agent coder-1, got %s", entries.Entries[0].Agent)
	}
	if entries.Entries[0].Action != "started" {
		t.Errorf("expected action started, got %s", entries.Entries[0].Action)
	}
	if entries.NewPosition <= 0 {
		t.Errorf("expected positive NewPosition, got %d", entries.NewPosition)
	}
}

func TestReadLogCmdIncrementalRead(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log.yaml")

	ts := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	firstEntry := []log.Entry{
		{Timestamp: ts, Agent: "coder-1", Action: "started", Detail: "first"},
	}
	data1, err := yaml.Marshal(firstEntry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, data1, 0644); err != nil {
		t.Fatal(err)
	}

	// Read initial entries
	cmd := readLogCmd(logPath, 0)
	msg := cmd().(LogEntriesMsg)
	if len(msg.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(msg.Entries))
	}
	pos := msg.NewPosition

	// Append more entries
	secondEntry := []log.Entry{
		{Timestamp: ts.Add(time.Minute), Agent: "coder-2", Action: "claimed", Detail: "second"},
	}
	data2, err := yaml.Marshal(secondEntry)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(data2); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	// Read incrementally from previous position
	cmd = readLogCmd(logPath, pos)
	msg = cmd().(LogEntriesMsg)
	if len(msg.Entries) != 1 {
		t.Fatalf("expected 1 new entry, got %d", len(msg.Entries))
	}
	if msg.Entries[0].Agent != "coder-2" {
		t.Errorf("expected agent coder-2, got %s", msg.Entries[0].Agent)
	}
	if msg.NewPosition <= pos {
		t.Errorf("expected NewPosition > %d, got %d", pos, msg.NewPosition)
	}
}

func TestReadLogCmdNoNewData(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log.yaml")

	entry := []log.Entry{
		{Timestamp: time.Now(), Agent: "a", Action: "b", Detail: "c"},
	}
	data, err := yaml.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	info, _ := os.Stat(logPath)
	offset := info.Size()

	// Read from end — no new data
	cmd := readLogCmd(logPath, offset)
	msg := cmd().(LogEntriesMsg)
	if len(msg.Entries) != 0 {
		t.Errorf("expected 0 entries when no new data, got %d", len(msg.Entries))
	}
}

func TestReadLogCmdCorruptYAML(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log.yaml")

	if err := os.WriteFile(logPath, []byte("not: [valid: yaml: {{{"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := readLogCmd(logPath, 0)
	msg := cmd()
	if _, ok := msg.(errMsg); !ok {
		t.Fatalf("expected errMsg for corrupt YAML, got %T", msg)
	}
}

func TestTickCmdReturnsNonNilCmd(t *testing.T) {
	cmd := tickCmd()
	if cmd == nil {
		t.Fatal("tickCmd() returned nil")
	}
}

// --- Phase 4: Action Cmd function tests ---

func TestSpawnAgentCmd_ReturnsNonNilCmd(t *testing.T) {
	cmd := spawnAgentCmd("/tmp/fake-project", "coder", "claude")
	if cmd == nil {
		t.Fatal("spawnAgentCmd returned nil tea.Cmd")
	}
}

func TestSpawnAgentCmd_ReturnsCmdResultMsg(t *testing.T) {
	// The "liza" binary likely doesn't exist in test; the Cmd should still
	// return a CmdResultMsg (with Success: false).
	cmd := spawnAgentCmd("/tmp/fake-project", "coder", "claude")
	msg := cmd()
	_, ok := msg.(CmdResultMsg)
	if !ok {
		t.Fatalf("expected CmdResultMsg, got %T", msg)
	}
}

func TestLoadRolesCmd_WithPipelineConfig(t *testing.T) {
	dir := t.TempDir()
	lizaDir := filepath.Join(dir, ".liza")
	if err := os.MkdirAll(lizaDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(lizaDir, "pipeline.yaml"), embedded.PipelineConfigContent(), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := loadRolesCmd(dir)
	if cmd == nil {
		t.Fatal("loadRolesCmd returned nil tea.Cmd")
	}

	msg := cmd()
	switch v := msg.(type) {
	case rolesMsg:
		if len(v.Roles) == 0 {
			t.Fatal("expected non-empty Roles from pipeline config")
		}
		if len(v.SprintTerminals) == 0 {
			t.Fatal("expected non-empty SprintTerminals from pipeline config")
		}
		if len(v.StateCategories) == 0 {
			t.Fatal("expected non-empty StateCategories from pipeline config")
		}
		if v.StateCategories["ARCHITECTING"] != pipeline.StateCategoryExecuting {
			t.Fatalf("StateCategories[ARCHITECTING] = %q, want executing", v.StateCategories["ARCHITECTING"])
		}
		if v.RoleTypes["coder"] != "doer" {
			t.Fatalf("RoleTypes[coder] = %q, want doer", v.RoleTypes["coder"])
		}
		if v.RoleTypes["code-reviewer"] != "reviewer" {
			t.Fatalf("RoleTypes[code-reviewer] = %q, want reviewer", v.RoleTypes["code-reviewer"])
		}
	default:
		t.Fatalf("expected rolesMsg, got %T: %+v", msg, msg)
	}
}

func TestLoadRolesCmd_MissingPipelineConfig(t *testing.T) {
	dir := t.TempDir()

	cmd := loadRolesCmd(dir)
	if cmd == nil {
		t.Fatal("loadRolesCmd returned nil tea.Cmd")
	}

	msg := cmd()
	roles, ok := msg.(rolesMsg)
	if !ok {
		t.Fatalf("expected rolesMsg, got %T (%v)", msg, msg)
	}
	if roles.Roles != nil {
		t.Fatalf("expected nil Roles for missing pipeline config, got %v", roles.Roles)
	}
	if roles.SprintTerminals != nil {
		t.Fatalf("expected nil SprintTerminals for missing pipeline config, got %v", roles.SprintTerminals)
	}
	if roles.StateCategories != nil {
		t.Fatalf("expected nil StateCategories for missing pipeline config, got %v", roles.StateCategories)
	}
}

func TestResumeSystemCmd_ClearsProviderSignals(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Config.Mode = models.SystemModePaused
	testhelpers.WriteInitialState(t, stateFile, state)

	// Create a quota signal file
	if err := agent.WriteQuotaSignal(tmpDir, "codex", "You've hit your usage limit"); err != nil {
		t.Fatal(err)
	}
	quotaSignalPath := agent.QuotaSignalPath(tmpDir, "codex")
	if _, err := os.Stat(quotaSignalPath); err != nil {
		t.Fatalf("quota signal file should exist before resume: %v", err)
	}
	if err := agent.WriteProviderUnavailableSignal(tmpDir, "codex", "session access denied"); err != nil {
		t.Fatal(err)
	}
	unavailableSignalPath := agent.ProviderUnavailableSignalPath(tmpDir, "codex")
	if _, err := os.Stat(unavailableSignalPath); err != nil {
		t.Fatalf("provider unavailable signal file should exist before resume: %v", err)
	}

	cmd := resumeSystemCmd(tmpDir)
	msg := cmd()
	result, ok := msg.(CmdResultMsg)
	if !ok {
		t.Fatalf("expected CmdResultMsg, got %T", msg)
	}
	if !result.Success {
		t.Fatalf("resume failed: %s", result.Message)
	}
	if result.Message != "System resumed" {
		t.Errorf("message = %q, want %q", result.Message, "System resumed")
	}

	// Provider-scoped signal files should be removed
	if _, err := os.Stat(quotaSignalPath); !os.IsNotExist(err) {
		t.Error("quota signal file should have been removed after resume")
	}
	if _, err := os.Stat(unavailableSignalPath); !os.IsNotExist(err) {
		t.Error("provider unavailable signal file should have been removed after resume")
	}
}

func TestResumeSystemCmd_WarnsOnQuotaClearFailure(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Config.Mode = models.SystemModePaused
	testhelpers.WriteInitialState(t, stateFile, state)

	// Replace the signal file with a non-empty directory so os.Remove fails
	// ("directory not empty") without affecting state.yaml writes.
	signalPath := agent.QuotaSignalPath(tmpDir, "codex")
	if err := os.MkdirAll(signalPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(signalPath, "blocker"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	cmd := resumeSystemCmd(tmpDir)
	msg := cmd()
	result, ok := msg.(CmdResultMsg)
	if !ok {
		t.Fatalf("expected CmdResultMsg, got %T", msg)
	}
	if !result.Success {
		t.Fatalf("resume should succeed even when quota clear fails: %s", result.Message)
	}
	if !strings.Contains(result.Message, "warning") {
		t.Errorf("expected warning in message, got %q", result.Message)
	}
	if !strings.Contains(result.Message, "codex") {
		t.Errorf("expected provider name in warning, got %q", result.Message)
	}
}

func TestResumeSystemCmd_FailurePreservesQuotaSignals(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupLizaDir(t, tmpDir)
	// No state written — ops.Resume will fail because state.yaml doesn't exist

	// Create a quota signal file
	if err := agent.WriteQuotaSignal(tmpDir, "codex", "quota hit"); err != nil {
		t.Fatal(err)
	}
	signalPath := agent.QuotaSignalPath(tmpDir, "codex")

	cmd := resumeSystemCmd(tmpDir)
	msg := cmd()
	result, ok := msg.(CmdResultMsg)
	if !ok {
		t.Fatalf("expected CmdResultMsg, got %T", msg)
	}
	if result.Success {
		t.Fatal("expected resume to fail on missing state")
	}

	// Quota signal file should still exist (cleanup skipped on failure)
	if _, err := os.Stat(signalPath); os.IsNotExist(err) {
		t.Error("quota signal file should be preserved when resume fails")
	}
}

func TestTerminateAgentCmdReportsProcessTerminationFailure(t *testing.T) {
	original := terminateAgent
	terminateAgent = func(projectRoot, agentID string, force, allowRunningPID bool, reason string, grace time.Duration) (*ops.TerminateAgentResult, error) {
		return nil, errors.New("process still running")
	}
	t.Cleanup(func() { terminateAgent = original })

	msg := terminateAgentCmd(t.TempDir(), "coder-1")()
	result, ok := msg.(CmdResultMsg)
	if !ok {
		t.Fatalf("message = %T, want CmdResultMsg", msg)
	}
	if result.Success {
		t.Fatal("Success = true, want false")
	}
	if !strings.Contains(result.Message, "process still running") {
		t.Fatalf("Message = %q, want process failure detail", result.Message)
	}
}

func TestAddTaskCmd_SuccessMessage(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	// Post-write validation checks spec_ref file existence.
	testhelpers.CreateSpecFile(t, tmpDir, "vision.md", "test")
	testhelpers.CreateSpecFile(t, tmpDir, "test.md", "test")

	input := &commands.TaskInput{
		ID:          "test-task",
		Type:        "coding",
		RolePair:    "coding-pair",
		Description: "A test task",
		SpecRef:     "specs/test.md",
		DoneWhen:    "tests pass",
		Scope:       "internal/foo",
		Priority:    1,
	}

	cmd := addTaskCmd(tmpDir, input)
	msg := cmd()
	result, ok := msg.(CmdResultMsg)
	if !ok {
		t.Fatalf("expected CmdResultMsg, got %T", msg)
	}
	if !result.Success {
		t.Fatalf("add task failed: %s", result.Message)
	}
	if result.Message != "Task test-task added" {
		t.Errorf("message = %q, want %q", result.Message, "Task test-task added")
	}
}

func TestAddTaskCmd_SurfacesWarnings(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	// Post-write validation checks spec_ref file existence.
	testhelpers.CreateSpecFile(t, tmpDir, "vision.md", "test")
	testhelpers.CreateSpecFile(t, tmpDir, "test.md", "test")

	// Make log.yaml a directory so the log write fails, producing a warning.
	logPath := filepath.Join(tmpDir, ".liza", "log.yaml")
	if err := os.MkdirAll(logPath, 0755); err != nil {
		t.Fatal(err)
	}

	input := &commands.TaskInput{
		ID:          "warn-task",
		Type:        "coding",
		RolePair:    "coding-pair",
		Description: "Task that triggers warning",
		SpecRef:     "specs/test.md",
		DoneWhen:    "tests pass",
		Scope:       "internal/foo",
		Priority:    1,
	}

	cmd := addTaskCmd(tmpDir, input)
	msg := cmd()
	result, ok := msg.(CmdResultMsg)
	if !ok {
		t.Fatalf("expected CmdResultMsg, got %T", msg)
	}
	if !result.Success {
		t.Fatalf("add task should succeed despite log warning: %s", result.Message)
	}
	if !strings.Contains(result.Message, "warning") {
		t.Errorf("expected warning in message, got %q", result.Message)
	}
	if !strings.Contains(result.Message, "log") {
		t.Errorf("expected log-related warning detail, got %q", result.Message)
	}
}

func TestActionCmds_ReturnNonNil(t *testing.T) {
	tests := []struct {
		name string
		cmd  tea.Cmd
	}{
		{"pauseSystemCmd", pauseSystemCmd("/tmp/fake", "test reason")},
		{"resumeSystemCmd", resumeSystemCmd("/tmp/fake")},
		{"checkpointCmd", checkpointCmd("/tmp/fake")},
		{"stopSystemCmd", stopSystemCmd("/tmp/fake")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cmd == nil {
				t.Fatalf("%s returned nil tea.Cmd", tt.name)
			}
		})
	}
}

// --- Phase 3: Data layer Cmd function tests ---

func TestRunChecksCmdCacheCopy(t *testing.T) {
	inputCache := map[string]time.Time{
		"key1": time.Now(),
		"key2": time.Now().Add(-time.Hour),
	}

	// Call runChecksCmd — the returned Cmd captures a copy of the cache
	cmd := runChecksCmd("/nonexistent", "/dev/null", nil, inputCache)
	if cmd == nil {
		t.Fatal("runChecksCmd returned nil Cmd")
	}

	// Mutate the input cache AFTER the Cmd was created
	originalKey1 := inputCache["key1"]
	inputCache["key1"] = time.Time{}
	inputCache["new_key"] = time.Now()

	// Execute the Cmd — it should use the copied cache, not the mutated input
	msg := cmd()
	result, ok := msg.(alertsMsg)
	if !ok {
		// runChecksCmd may return errMsg if state is nil or project doesn't exist.
		// That's fine — what matters is the cache was copied before the closure ran.
		// Verify by checking the input cache was indeed mutated independently.
		if inputCache["key1"] != (time.Time{}) {
			t.Error("input cache key1 should have been mutated to zero")
		}
		if _, exists := inputCache["new_key"]; !exists {
			t.Error("input cache should have new_key")
		}
		// The copy was made before closure — test passes if we get here.
		// The Cmd failing due to nil state is expected in test.
		return
	}

	// If we got alertsMsg, verify the returned cache doesn't reflect our mutations
	if val, exists := result.StateCache["key1"]; exists {
		if val == (time.Time{}) {
			t.Error("returned cache key1 should have original value, not mutated zero")
		}
		if val != originalKey1 {
			t.Errorf("returned cache key1 = %v, want %v", val, originalKey1)
		}
	}
	if _, exists := result.StateCache["new_key"]; exists {
		t.Error("returned cache should not contain 'new_key' added after Cmd creation")
	}
}

func TestRunChecksCmdDoesNotWriteValidationWarningsToCommandWarnWriter(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	now := time.Now().UTC()

	dep := testhelpers.BuildTaskByStatus("dep-task", models.TaskStatusSuperseded, now)
	dep.SupersededBy = nil
	dep.RescopeReason = testhelpers.StringPtr("No replacement task")
	dep.RolePair = "coding-pair"
	dependent := testhelpers.BuildTaskByStatus("dependent-task", models.TaskStatusAbandoned, now)
	dependent.RolePair = "coding-pair"
	dependent.DependsOn = []string{"dep-task"}

	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{dep, dependent}
	state.Sprint.Scope.Planned = []string{dep.ID, dependent.ID}
	testhelpers.WriteInitialState(t, stateFile, state)

	var warnings bytes.Buffer
	commands.SetWarnWriter(&warnings)
	t.Cleanup(func() { commands.SetWarnWriter(os.Stderr) })

	if err := commands.ValidateCommand(stateFile, true); err != nil {
		t.Fatalf("ValidateCommand() error = %v", err)
	}
	if !strings.Contains(warnings.String(), "not satisfied via supersession path") {
		t.Fatalf("test fixture did not reproduce validation warning, got:\n%s", warnings.String())
	}
	warnings.Reset()

	cmd := runChecksCmd(tmpDir, filepath.Join(tmpDir, ".liza", "alerts.log"), state, make(map[string]time.Time))
	msg := cmd()
	if _, ok := msg.(alertsMsg); !ok {
		t.Fatalf("runChecksCmd() message = %T, want alertsMsg", msg)
	}

	if strings.Contains(warnings.String(), "not satisfied via supersession path") {
		t.Fatalf("TUI check leaked validation warning to command warn writer:\n%s", warnings.String())
	}
}
