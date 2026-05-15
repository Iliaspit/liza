package commands

import (
	"os"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
	"gopkg.in/yaml.v3"
)

func TestMigrateCommand_NormalizesUnderscoreRoles(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Agents = map[string]models.Agent{
		"coder-1": {
			Role:      "coder",
			Status:    models.AgentStatusIdle,
			Heartbeat: now,
			Terminal:  "term-1",
		},
		"code-reviewer-1": {
			Role:      "code_reviewer", // underscore form — should be normalized
			Status:    models.AgentStatusIdle,
			Heartbeat: now,
			Terminal:  "term-2",
		},
		"orchestrator-1": {
			Role:      "orchestrator",
			Status:    models.AgentStatusIdle,
			Heartbeat: now,
			Terminal:  "term-3",
		},
	}
	testhelpers.WriteInitialState(t, statePath, state)

	changed, err := MigrateCommand(statePath)
	if err != nil {
		t.Fatalf("MigrateCommand() error = %v", err)
	}
	if !changed {
		t.Error("MigrateCommand() changed = false, want true")
	}

	// Verify the file was updated
	bb := db.New(statePath)
	updated, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read updated state: %v", err)
	}

	agent := updated.Agents["code-reviewer-1"]
	if agent.Role != "code-reviewer" {
		t.Errorf("Agent role = %q, want %q", agent.Role, "code-reviewer")
	}
}

func TestMigrateCommand_AlreadyMigratedNoChanges(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Agents = map[string]models.Agent{
		"coder-1": {
			Role:      "coder",
			Status:    models.AgentStatusIdle,
			Heartbeat: now,
			Terminal:  "term-1",
		},
		"code-reviewer-1": {
			Role:      "code-reviewer", // already hyphenated
			Status:    models.AgentStatusIdle,
			Heartbeat: now,
			Terminal:  "term-2",
		},
	}
	testhelpers.WriteInitialState(t, statePath, state)

	// Capture file mod time before migration
	infoBefore, _ := os.Stat(statePath)

	changed, err := MigrateCommand(statePath)
	if err != nil {
		t.Fatalf("MigrateCommand() error = %v", err)
	}
	if changed {
		t.Error("MigrateCommand() changed = true, want false (already migrated)")
	}

	// File should not have been modified
	infoAfter, _ := os.Stat(statePath)
	if infoBefore.ModTime() != infoAfter.ModTime() {
		t.Error("State file was modified even though no changes were needed")
	}
}

func TestMigrateCommand_MultipleUnderscoreRoles(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Agents = map[string]models.Agent{
		"code-planner-1": {
			Role:      "code_planner",
			Status:    models.AgentStatusIdle,
			Heartbeat: now,
			Terminal:  "term-1",
		},
		"epic-plan-reviewer-1": {
			Role:      "epic_plan_reviewer",
			Status:    models.AgentStatusIdle,
			Heartbeat: now,
			Terminal:  "term-2",
		},
		"us-writer-1": {
			Role:      "us_writer",
			Status:    models.AgentStatusIdle,
			Heartbeat: now,
			Terminal:  "term-3",
		},
	}
	testhelpers.WriteInitialState(t, statePath, state)

	changed, err := MigrateCommand(statePath)
	if err != nil {
		t.Fatalf("MigrateCommand() error = %v", err)
	}
	if !changed {
		t.Error("MigrateCommand() changed = false, want true")
	}

	bb := db.New(statePath)
	updated, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read updated state: %v", err)
	}

	expectations := map[string]string{
		"code-planner-1":       "code-planner",
		"epic-plan-reviewer-1": "epic-plan-reviewer",
		"us-writer-1":          "us-writer",
	}
	for agentID, wantRole := range expectations {
		got := updated.Agents[agentID].Role
		if got != wantRole {
			t.Errorf("Agent %q role = %q, want %q", agentID, got, wantRole)
		}
	}
}

func TestMigrateCommand_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Agents = map[string]models.Agent{
		"code-reviewer-1": {
			Role:      "code_reviewer",
			Status:    models.AgentStatusIdle,
			Heartbeat: now,
			Terminal:  "term-1",
		},
	}
	testhelpers.WriteInitialState(t, statePath, state)

	// First migration — should change
	changed1, err := MigrateCommand(statePath)
	if err != nil {
		t.Fatalf("First MigrateCommand() error = %v", err)
	}
	if !changed1 {
		t.Error("First MigrateCommand() changed = false, want true")
	}

	// Second migration — should report no changes
	changed2, err := MigrateCommand(statePath)
	if err != nil {
		t.Fatalf("Second MigrateCommand() error = %v", err)
	}
	if changed2 {
		t.Error("Second MigrateCommand() changed = true, want false")
	}
}

func TestMigrateCommand_UnknownRolePassesThrough(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Agents = map[string]models.Agent{
		"custom-1": {
			Role:      "custom_agent",
			Status:    models.AgentStatusIdle,
			Heartbeat: now,
			Terminal:  "term-1",
		},
	}
	testhelpers.WriteInitialState(t, statePath, state)

	changed, err := MigrateCommand(statePath)
	if err != nil {
		t.Fatalf("MigrateCommand() error = %v", err)
	}
	if changed {
		t.Error("MigrateCommand() changed = true, want false (unknown role should not be modified)")
	}

	// Verify the unknown role was not mutated
	data, _ := os.ReadFile(statePath)
	var updated models.State
	_ = yaml.Unmarshal(data, &updated)
	if updated.Agents["custom-1"].Role != "custom_agent" {
		t.Errorf("Unknown role was mutated: got %q, want %q", updated.Agents["custom-1"].Role, "custom_agent")
	}
}

func TestMigrateCommand_NoAgents(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Agents = map[string]models.Agent{}
	testhelpers.WriteInitialState(t, statePath, state)

	changed, err := MigrateCommand(statePath)
	if err != nil {
		t.Fatalf("MigrateCommand() error = %v", err)
	}
	if changed {
		t.Error("MigrateCommand() changed = true, want false (no agents to migrate)")
	}
}

func TestMigrateCommand_NormalizesLegacyAttempted(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		{
			ID:      "task-1",
			Status:  models.TaskStatusDraft,
			Created: now,
			Extra:   map[string]any{"attempted": []any{"agent-1"}},
		},
	}
	testhelpers.WriteInitialState(t, statePath, state)

	changed, err := MigrateCommand(statePath)
	if err != nil {
		t.Fatalf("MigrateCommand() error = %v", err)
	}
	if !changed {
		t.Error("MigrateCommand() changed = false, want true")
	}

	// Read raw YAML to verify persisted state
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var updated models.State
	if err := yaml.Unmarshal(data, &updated); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	task := updated.Tasks[0]
	if task.Attempt != 2 {
		t.Errorf("task.Attempt = %d, want 2", task.Attempt)
	}
	if _, exists := task.Extra["attempted"]; exists {
		t.Error("task.Extra[\"attempted\"] still present, want deleted")
	}
}

func TestMigrateCommand_LegacyAttempted_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		{
			ID:      "task-1",
			Status:  models.TaskStatusDraft,
			Created: now,
			Extra:   map[string]any{"attempted": []any{"agent-1"}},
		},
	}
	testhelpers.WriteInitialState(t, statePath, state)

	// First run — should change
	changed1, err := MigrateCommand(statePath)
	if err != nil {
		t.Fatalf("First MigrateCommand() error = %v", err)
	}
	if !changed1 {
		t.Error("First MigrateCommand() changed = false, want true")
	}

	// Second run — should report no changes
	changed2, err := MigrateCommand(statePath)
	if err != nil {
		t.Fatalf("Second MigrateCommand() error = %v", err)
	}
	if changed2 {
		t.Error("Second MigrateCommand() changed = true, want false")
	}
}

func TestMigrateCommand_ScrubsRawProviderAuditMessage(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Anomalies = []models.Anomaly{
		{
			Timestamp: time.Now().UTC(),
			Task:      "task-1",
			Reporter:  "orchestrator-1",
			Type:      "provider_audit_degraded",
			Details: map[string]any{
				"provider": "codex",
				"agent_id": "orchestrator-1",
				"impact":   "provider transcript or rollout persistence may be incomplete",
				"message":  `{"type":"item.completed","item":{"type":"command_execution","aggregated_output":"raw output"}}`,
			},
		},
		{
			Timestamp: time.Now().UTC(),
			Task:      "task-2",
			Reporter:  "orchestrator-1",
			Type:      "provider_audit_degraded",
			Details: map[string]any{
				"provider": "codex",
				"agent_id": "orchestrator-1",
				"message":  "Agent coder-2 deleted: terminated via TUI",
			},
		},
	}
	data, err := yaml.Marshal(state)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	changed, err := MigrateCommand(statePath)
	if err != nil {
		t.Fatalf("MigrateCommand() error = %v", err)
	}
	if !changed {
		t.Fatal("MigrateCommand() changed = false, want true")
	}

	updated, err := db.New(statePath).Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	got := updated.Anomalies[0].Details["message"]
	want := "provider audit degraded; raw provider event omitted from state; inspect .liza/agent-outputs and alerts for transcript evidence"
	if got != want {
		t.Fatalf("scrubbed message = %q, want %q", got, want)
	}
	if updated.Anomalies[0].Details["message_scrubbed"] != true {
		t.Fatalf("message_scrubbed = %v, want true", updated.Anomalies[0].Details["message_scrubbed"])
	}
	if got := updated.Anomalies[1].Details["message"]; got != "Agent coder-2 deleted: terminated via TUI" {
		t.Fatalf("ordinary message changed to %q", got)
	}
}

func TestMigrateCommand_InvalidStatePath(t *testing.T) {
	_, err := MigrateCommand("/nonexistent/path/state.yaml")
	if err == nil {
		t.Error("MigrateCommand() error = nil, want error for nonexistent file")
	}
}
