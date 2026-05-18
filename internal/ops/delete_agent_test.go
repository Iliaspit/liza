package ops

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/procscan"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestDeleteAgent_Validation(t *testing.T) {
	_, err := DeleteAgent("/nonexistent", "", false, false, "reason")
	if err == nil {
		t.Fatal("Expected error for empty agent ID")
	}
	if !strings.Contains(err.Error(), "agent ID required") {
		t.Errorf("Error = %q, want to contain 'agent ID required'", err.Error())
	}
}

func TestDeleteAgent_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := DeleteAgent(tmpDir, "nonexistent", false, false, "reason")
	if err == nil {
		t.Fatal("Expected error for nonexistent agent")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestDeleteAgent_IdleAgent(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Agents["coder-1"] = models.Agent{
		Role:   "coder",
		Status: models.AgentStatusIdle,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := DeleteAgent(tmpDir, "coder-1", false, false, "no longer needed")
	if err != nil {
		t.Fatalf("DeleteAgent() error: %v", err)
	}
	if result.AgentID != "coder-1" {
		t.Errorf("AgentID = %q, want %q", result.AgentID, "coder-1")
	}

	// Verify agent removed
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if _, exists := readState.Agents["coder-1"]; exists {
		t.Error("Agent should be removed from state")
	}

	// Verify human note added
	if len(readState.HumanNotes) == 0 {
		t.Fatal("Expected human note to be added")
	}
	lastNote := readState.HumanNotes[len(readState.HumanNotes)-1]
	if !strings.Contains(lastNote.Message, "coder-1") {
		t.Errorf("Note message = %q, want to contain agent ID", lastNote.Message)
	}
}

func TestDeleteAgent_ActiveLease_NoForce(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	leaseExpires := now.Add(30 * time.Minute)
	state := testhelpers.CreateValidState()
	state.Agents["coder-1"] = models.Agent{
		Role:         "coder",
		Status:       models.AgentStatusWorking,
		LeaseExpires: &leaseExpires,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := DeleteAgent(tmpDir, "coder-1", false, false, "reason")
	if err == nil {
		t.Fatal("Expected error for active lease without force")
	}
	if !strings.Contains(err.Error(), "active lease") {
		t.Errorf("Error = %q, want to contain 'active lease'", err.Error())
	}
}

func TestDeleteAgent_ActiveLease_Force(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	leaseExpires := now.Add(30 * time.Minute)
	state := testhelpers.CreateValidState()
	state.Agents["coder-1"] = models.Agent{
		Role:         "coder",
		Status:       models.AgentStatusWorking,
		LeaseExpires: &leaseExpires,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := DeleteAgent(tmpDir, "coder-1", true, false, "force remove")
	if err != nil {
		t.Fatalf("DeleteAgent() with force error: %v", err)
	}
	if result.AgentID != "coder-1" {
		t.Errorf("AgentID = %q, want %q", result.AgentID, "coder-1")
	}
}

func TestDeleteAgent_BusyWithTask_NoForce(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	taskRef := "task-1"
	state := testhelpers.CreateValidState()
	state.Agents["coder-1"] = models.Agent{
		Role:        "coder",
		Status:      models.AgentStatusWorking,
		CurrentTask: &taskRef,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := DeleteAgent(tmpDir, "coder-1", false, false, "reason")
	if err == nil {
		t.Fatal("Expected error for busy agent without force")
	}
	if !strings.Contains(err.Error(), "working on task") {
		t.Errorf("Error = %q, want to contain 'working on task'", err.Error())
	}
}

func TestDeleteAgent_AllowRunningPID_BypassesPIDOnly(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	leaseExpires := now.Add(30 * time.Minute)
	taskRef := "task-1"
	state := testhelpers.CreateValidState()
	state.Agents["coder-1"] = models.Agent{
		Role:         "coder",
		Status:       models.AgentStatusWorking,
		PID:          os.Getpid(), // alive PID
		LeaseExpires: &leaseExpires,
		CurrentTask:  &taskRef,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	// allowRunningPID=true should still refuse due to active lease
	_, err := DeleteAgent(tmpDir, "coder-1", false, true, "pid confirmed")
	if err == nil {
		t.Fatal("Expected error: allowRunningPID should not bypass lease check")
	}
	if !strings.Contains(err.Error(), "active lease") {
		t.Errorf("Error = %q, want to contain 'active lease'", err.Error())
	}
}

func TestDeleteAgent_AllowRunningPID_BypassesPIDWithNoLease(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Agents["coder-1"] = models.Agent{
		Role:   "coder",
		Status: models.AgentStatusIdle,
		PID:    os.Getpid(), // alive PID
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	// allowRunningPID=true with no lease/task should succeed
	result, err := DeleteAgent(tmpDir, "coder-1", false, true, "pid confirmed")
	if err != nil {
		t.Fatalf("DeleteAgent() error: %v", err)
	}
	if result.AgentID != "coder-1" {
		t.Errorf("AgentID = %q, want %q", result.AgentID, "coder-1")
	}

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if _, exists := readState.Agents["coder-1"]; exists {
		t.Error("Agent should be removed from state")
	}
}

func TestIsAgentProcessRunning_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	_, _, err := IsAgentProcessRunning(tmpDir, "nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent agent")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestDeleteAgent_BusyWithTask_Force(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	taskRef := "task-1"
	state := testhelpers.CreateValidState()
	state.Agents["coder-1"] = models.Agent{
		Role:        "coder",
		Status:      models.AgentStatusWorking,
		CurrentTask: &taskRef,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := DeleteAgent(tmpDir, "coder-1", true, false, "force remove")
	if err != nil {
		t.Fatalf("DeleteAgent() with force error: %v", err)
	}
	if result.AgentID != "coder-1" {
		t.Errorf("AgentID = %q, want %q", result.AgentID, "coder-1")
	}
}

func TestMatchLizaAgentCmdline(t *testing.T) {
	tests := []struct {
		name     string
		cmdline  string
		expected bool
	}{
		{"exact match", "liza\x00agent\x00coder\x00--cli\x00claude\x00", true},
		{"full path", "/home/user/.local/bin/liza\x00agent\x00code-reviewer\x00", true},
		{"wrong binary", "codex\x00agent\x00coder\x00", false},
		{"wrong subcommand", "liza\x00status\x00", false},
		{"empty cmdline", "", false},
		{"single arg", "liza\x00", false},
		{"go test runner", "go\x00test\x00./internal/ops/...\x00", false},
		{"liza without agent", "liza\x00validate\x00", false},
		{"agent without liza", "other\x00agent\x00", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := procscan.IsLizaAgentArgv(procscan.ParseCmdlineBytes([]byte(tt.cmdline)))
			if got != tt.expected {
				t.Errorf("IsLizaAgentArgv(ParseCmdlineBytes(%q)) = %v, want %v", tt.cmdline, got, tt.expected)
			}
		})
	}
}

func TestSignalProcess_ZeroPID(t *testing.T) {
	r := &DeleteAgentResult{AgentID: "test", PID: 0}
	if r.SignalProcess() {
		t.Error("SignalProcess should return false for PID 0")
	}
}

func TestSignalProcess_CurrentProcess(t *testing.T) {
	// os.Getpid() is a go test runner, not "liza agent" — identity check rejects it.
	r := &DeleteAgentResult{AgentID: "test", PID: os.Getpid()}
	if r.SignalProcess() {
		t.Error("SignalProcess should return false for non-liza process")
	}
}

func TestDeleteAgent_ReturnsPID(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Agents["coder-1"] = models.Agent{
		Role:   "coder",
		Status: models.AgentStatusIdle,
		PID:    12345,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := DeleteAgent(tmpDir, "coder-1", false, false, "test")
	if err != nil {
		t.Fatalf("DeleteAgent() error: %v", err)
	}
	if result.PID != 12345 {
		t.Errorf("PID = %d, want 12345", result.PID)
	}
}
