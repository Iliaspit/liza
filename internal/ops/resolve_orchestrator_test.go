package ops

import (
	"testing"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/pipeline"
	"github.com/liza-mas/liza/internal/testhelpers"
)

// testOrchestratorPipelineYAML defines a pipeline with a custom orchestrator role key.
var testOrchestratorPipelineYAML = []byte(`
pipeline:
  roles:
    coder:
      type: doer
      display-name: "Coder"
      allowed-operations:
        - submit-for-review
    code-reviewer:
      type: reviewer
      display-name: "Code Reviewer"
      allowed-operations:
        - submit-verdict
    orchestrator:
      type: orchestrator
      display-name: "Orchestrator"
      allowed-operations:
        - add-tasks
    lead-orchestrator:
      type: orchestrator
      display-name: "Lead Orchestrator"
      allowed-operations:
        - add-tasks
  role-pairs:
    coding-pair:
      doer: coder
      reviewer: code-reviewer
      states:
        initial: READY
        executing: CODING
        submitted: CODE_SUBMITTED
        reviewing: CODE_REVIEWING
        approved: CODE_APPROVED
        rejected: CODE_REJECTED
  sub-pipelines:
    coding:
      steps:
        - coding-pair
      transitions: []
  entry-points:
    default: coding.coding-pair
`)

func testOrchestratorResolver(t *testing.T) *pipeline.Resolver {
	t.Helper()
	cfg, err := pipeline.LoadFromBytes(testOrchestratorPipelineYAML)
	if err != nil {
		t.Fatalf("testOrchestratorResolver: %v", err)
	}
	return pipeline.NewResolver(cfg)
}

func TestFindOrchestratorID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		agents    map[string]models.Agent
		wantID    string
		wantErr   bool
		errSubstr string
	}{
		{
			name: "single orchestrator found",
			agents: map[string]models.Agent{
				"orchestrator-1": {Role: "orchestrator"},
				"coder-1":        {Role: "coder"},
			},
			wantID: "orchestrator-1",
		},
		{
			name: "custom orchestrator ID",
			agents: map[string]models.Agent{
				"my-orch-1": {Role: "orchestrator"},
				"coder-1":   {Role: "coder"},
			},
			wantID: "my-orch-1",
		},
		{
			name: "no orchestrator registered",
			agents: map[string]models.Agent{
				"coder-1": {Role: "coder"},
			},
			wantErr:   true,
			errSubstr: "no orchestrator agent registered",
		},
		{
			name:      "empty agents map",
			agents:    map[string]models.Agent{},
			wantErr:   true,
			errSubstr: "no orchestrator agent registered",
		},
		{
			name: "multiple orchestrators",
			agents: map[string]models.Agent{
				"orchestrator-1": {Role: "orchestrator"},
				"orchestrator-2": {Role: "orchestrator"},
			},
			wantErr:   true,
			errSubstr: "multiple orchestrators registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.State{Agents: tt.agents}
			id, err := state.FindOrchestratorID()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				testhelpers.AssertErrorContains(t, err, tt.errSubstr)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id != tt.wantID {
				t.Errorf("got %q, want %q", id, tt.wantID)
			}
		})
	}
}

func TestResolveOrchestratorFromState(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Agents["orchestrator-1"] = models.Agent{Role: "orchestrator"}
	testhelpers.WriteInitialState(t, stateFile, state)

	id, err := ResolveOrchestratorFromState(stateFile, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "orchestrator-1" {
		t.Errorf("got %q, want %q", id, "orchestrator-1")
	}
}

func TestResolveOrchestratorFromState_NoOrchestrator(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := ResolveOrchestratorFromState(stateFile, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	testhelpers.AssertErrorContains(t, err, "no orchestrator agent registered")
}

func TestResolveOrchestratorFromState_CustomRoleKeyWithResolver(t *testing.T) {
	t.Parallel()

	resolver := testOrchestratorResolver(t)

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Agents["lead-1"] = models.Agent{Role: "lead-orchestrator"}
	state.Agents["coder-1"] = models.Agent{Role: "coder"}
	testhelpers.WriteInitialState(t, stateFile, state)

	id, err := ResolveOrchestratorFromState(stateFile, resolver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "lead-1" {
		t.Errorf("got %q, want %q", id, "lead-1")
	}
}

func TestResolveOrchestratorFromState_StandardRoleKeyWithResolver(t *testing.T) {
	t.Parallel()

	resolver := testOrchestratorResolver(t)

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Agents["orchestrator-1"] = models.Agent{Role: "orchestrator"}
	testhelpers.WriteInitialState(t, stateFile, state)

	id, err := ResolveOrchestratorFromState(stateFile, resolver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "orchestrator-1" {
		t.Errorf("got %q, want %q", id, "orchestrator-1")
	}
}

func TestResolveOrchestratorFromState_MultipleOrchestratorTypesWithResolver(t *testing.T) {
	t.Parallel()

	resolver := testOrchestratorResolver(t)

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Agents["orchestrator-1"] = models.Agent{Role: "orchestrator"}
	state.Agents["lead-1"] = models.Agent{Role: "lead-orchestrator"}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := ResolveOrchestratorFromState(stateFile, resolver)
	if err == nil {
		t.Fatal("expected error for multiple orchestrator-type agents, got nil")
	}
	testhelpers.AssertErrorContains(t, err, "multiple orchestrators registered")
}
