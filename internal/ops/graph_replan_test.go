package ops

import (
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/statevalidate"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestGraphReplanRequestIsControllerReadOnlyAndOrchestratorRepairIsAudited(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	testhelpers.SetupTestGitRepo(t, projectRoot)
	statePath, _ := testhelpers.SetupLizaDir(t, projectRoot)
	state := graphReplanCycleState()
	testhelpers.WriteInitialState(t, statePath, state)

	beforeA := slices.Clone(state.FindTask("task-a").DependsOn)
	beforeB := slices.Clone(state.FindTask("task-b").DependsOn)
	requested, err := RequestGraphReplan(projectRoot, RequestGraphReplanInput{
		RunID:       state.Goal.ID,
		RequestedBy: "codex-controller-thread-1",
		Reason:      "A final runtime gate depends on B while B cannot start before A",
	})
	if err != nil {
		t.Fatalf("RequestGraphReplan() error: %v", err)
	}
	if requested.Request.Status != models.GraphReplanPending || requested.Request.Diagnostic == "" {
		t.Fatalf("request = %+v, want pending request with native diagnostic", requested.Request)
	}

	afterRequest, err := db.New(statePath).Read()
	if err != nil {
		t.Fatalf("read request state: %v", err)
	}
	if !slices.Equal(afterRequest.FindTask("task-a").DependsOn, beforeA) || !slices.Equal(afterRequest.FindTask("task-b").DependsOn, beforeB) {
		t.Fatal("controller request mutated the task graph")
	}

	replayed, err := RequestGraphReplan(projectRoot, RequestGraphReplanInput{
		RunID:       state.Goal.ID,
		RequestedBy: "codex-controller-thread-1",
		Reason:      "A final runtime gate depends on B while B cannot start before A",
	})
	if err != nil {
		t.Fatalf("replayed RequestGraphReplan() error: %v", err)
	}
	afterReplay, err := db.New(statePath).Read()
	if err != nil {
		t.Fatalf("read replayed request state: %v", err)
	}
	if !replayed.Idempotent || replayed.Request.ID != requested.Request.ID || len(afterReplay.GraphReplanRequests) != 1 {
		t.Fatalf("replayed request = %+v, want one idempotent request", replayed)
	}

	authority := models.AgentAuthority{ID: "orchestrator-1", Generation: testhelpers.TestAgentGeneration}
	claimed, err := ClaimGraphReplan(projectRoot, requested.Request.ID, authority)
	if err != nil {
		t.Fatalf("ClaimGraphReplan() error: %v", err)
	}
	if claimed.Orchestrator == nil || *claimed.Orchestrator != authority {
		t.Fatalf("claimed orchestrator = %+v, want %+v", claimed.Orchestrator, authority)
	}

	taskB := afterRequest.FindTask("task-b")
	completed, err := CompleteGraphReplan(projectRoot, CompleteGraphReplanInput{
		RequestID: requested.Request.ID,
		Diagnosis: "Remove B's unnecessary start gate on A; A's final gate on B remains authoritative.",
		Updates: []GraphDependencyUpdate{{
			TaskID:            "task-b",
			ExpectedDependsOn: slices.Clone(taskB.DependsOn),
			DesiredDependsOn:  []string{},
			ExpectedContracts: slices.Clone(taskB.DependencyContracts),
			DesiredContracts:  []models.DependencyContract{},
		}},
	}, authority)
	if err != nil {
		t.Fatalf("CompleteGraphReplan() error: %v", err)
	}
	if completed.Request.Status != models.GraphReplanCompleted || completed.Request.ValidationResult != "valid" || len(completed.Request.GraphChanges) == 0 {
		t.Fatalf("completed request = %+v, want audited valid graph changes", completed.Request)
	}

	finalState, err := db.New(statePath).Read()
	if err != nil {
		t.Fatalf("read final state: %v", err)
	}
	if finalState.OpenGraphReplanRequest() != nil {
		t.Fatal("completed request still blocks task allocation")
	}
	if got := finalState.FindTask("task-b"); len(got.DependsOn) != 0 || len(got.DependencyContracts) != 0 {
		t.Fatalf("task-b repair = depends_on %v contracts %v, want both empty", got.DependsOn, got.DependencyContracts)
	}
	if finalState.FindTask("task-a").DependencyContracts[0].ProviderTask != "task-b" {
		t.Fatal("repair changed the surviving final-runtime dependency")
	}
}

func TestClaimGraphReplanFailsClosedWhenGraphGenerationChanged(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	testhelpers.SetupTestGitRepo(t, projectRoot)
	statePath, _ := testhelpers.SetupLizaDir(t, projectRoot)
	state := graphReplanCycleState()
	bb := testhelpers.WriteInitialState(t, statePath, state)
	request, err := RequestGraphReplan(projectRoot, RequestGraphReplanInput{
		RunID: state.Goal.ID, RequestedBy: "controller-1", Reason: "proven cycle",
	})
	if err != nil {
		t.Fatalf("RequestGraphReplan() error: %v", err)
	}
	if err := bb.Modify(func(current *models.State) error {
		current.FindTask("task-b").DependencyContracts[0].Purpose = "Concurrent graph change"
		return nil
	}); err != nil {
		t.Fatalf("change graph generation: %v", err)
	}

	_, err = ClaimGraphReplan(projectRoot, request.Request.ID, models.AgentAuthority{ID: "orchestrator-1", Generation: testhelpers.TestAgentGeneration})
	if err == nil {
		t.Fatal("ClaimGraphReplan() succeeded after graph generation changed")
	}
}

func TestRefreshGraphReplanAtomicallySupersedesStaleCandidateRequest(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	testhelpers.SetupTestGitRepo(t, projectRoot)
	statePath, _ := testhelpers.SetupLizaDir(t, projectRoot)
	state := graphReplanDependencyStallState()
	bb := testhelpers.WriteInitialState(t, statePath, state)
	const (
		controller = "codex-controller-thread-1"
		reason     = "blocked implementation still requires a concrete provider"
	)
	requested, err := RequestGraphReplan(projectRoot, RequestGraphReplanInput{
		RunID: state.Goal.ID, RequestedBy: controller, Reason: reason,
	})
	if err != nil {
		t.Fatalf("RequestGraphReplan() error: %v", err)
	}
	authority := models.AgentAuthority{ID: "orchestrator-1", Generation: testhelpers.TestAgentGeneration}
	if _, err := ClaimGraphReplan(projectRoot, requested.Request.ID, authority); err != nil {
		t.Fatalf("ClaimGraphReplan() error: %v", err)
	}
	if err := bb.Modify(func(current *models.State) error {
		commit := "candidate-after-request"
		current.FindTask("blocked-implementation").BaseCommit = &commit
		return nil
	}); err != nil {
		t.Fatalf("advance candidate lineage: %v", err)
	}

	refreshed, err := RefreshGraphReplan(projectRoot, RefreshGraphReplanInput{
		RequestID: requested.Request.ID, RunID: state.Goal.ID, RequestedBy: controller, Reason: reason,
	})
	if err != nil {
		t.Fatalf("RefreshGraphReplan() error: %v", err)
	}
	if refreshed.Superseded.Status != models.GraphReplanSuperseded || refreshed.Request.Status != models.GraphReplanPending {
		t.Fatalf("refresh result = %+v, want superseded predecessor and pending successor", refreshed)
	}
	if refreshed.Request.ID == requested.Request.ID || refreshed.Request.PredecessorRequestID != requested.Request.ID || refreshed.Superseded.SuccessorRequestID != refreshed.Request.ID {
		t.Fatalf("refresh linkage = %+v, want distinct bidirectionally linked requests", refreshed)
	}

	replayed, err := RefreshGraphReplan(projectRoot, RefreshGraphReplanInput{
		RequestID: requested.Request.ID, RunID: state.Goal.ID, RequestedBy: controller, Reason: reason,
	})
	if err != nil {
		t.Fatalf("replayed RefreshGraphReplan() error: %v", err)
	}
	persisted, err := db.New(statePath).Read()
	if err != nil {
		t.Fatalf("read refreshed state: %v", err)
	}
	if !replayed.Idempotent || replayed.Request.ID != refreshed.Request.ID || len(persisted.GraphReplanRequests) != 2 {
		t.Fatalf("replayed refresh = %+v with %d requests, want one idempotent successor", replayed, len(persisted.GraphReplanRequests))
	}
	if open := persisted.OpenGraphReplanRequest(); open == nil || open.ID != refreshed.Request.ID {
		t.Fatalf("open request = %+v, want refreshed successor", open)
	}
	if got := persisted.FindTask("dependency-held-plan"); !slices.Equal(got.DependsOn, []string{"blocked-implementation"}) {
		t.Fatalf("refresh mutated task graph: %v", got.DependsOn)
	}
	if err := statevalidate.ValidateState(persisted, projectRoot, false, io.Discard); err != nil {
		t.Fatalf("refreshed state is invalid: %v", err)
	}
}

func TestRefreshGraphReplanRejectsCurrentRequest(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	testhelpers.SetupTestGitRepo(t, projectRoot)
	statePath, _ := testhelpers.SetupLizaDir(t, projectRoot)
	state := graphReplanDependencyStallState()
	testhelpers.WriteInitialState(t, statePath, state)
	request, err := RequestGraphReplan(projectRoot, RequestGraphReplanInput{
		RunID: state.Goal.ID, RequestedBy: "controller-1", Reason: "proven dependency stall",
	})
	if err != nil {
		t.Fatalf("RequestGraphReplan() error: %v", err)
	}

	_, err = RefreshGraphReplan(projectRoot, RefreshGraphReplanInput{
		RequestID: request.Request.ID, RunID: state.Goal.ID, RequestedBy: "controller-1", Reason: "proven dependency stall",
	})
	if err == nil || !strings.Contains(err.Error(), "still current") {
		t.Fatalf("RefreshGraphReplan() error = %v, want current-request rejection", err)
	}
}

func TestRefreshGraphReplanRejectsScopeOrAcceptanceChange(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	testhelpers.SetupTestGitRepo(t, projectRoot)
	statePath, _ := testhelpers.SetupLizaDir(t, projectRoot)
	state := graphReplanDependencyStallState()
	bb := testhelpers.WriteInitialState(t, statePath, state)
	request, err := RequestGraphReplan(projectRoot, RequestGraphReplanInput{
		RunID: state.Goal.ID, RequestedBy: "controller-1", Reason: "proven dependency stall",
	})
	if err != nil {
		t.Fatalf("RequestGraphReplan() error: %v", err)
	}
	if err := bb.Modify(func(current *models.State) error {
		current.FindTask("blocked-implementation").Scope = "broadened product scope"
		return nil
	}); err != nil {
		t.Fatalf("change scope: %v", err)
	}

	_, err = RefreshGraphReplan(projectRoot, RefreshGraphReplanInput{
		RequestID: request.Request.ID, RunID: state.Goal.ID, RequestedBy: "controller-1", Reason: "proven dependency stall",
	})
	if err == nil || !strings.Contains(err.Error(), "refresh is forbidden") {
		t.Fatalf("RefreshGraphReplan() error = %v, want scope-boundary rejection", err)
	}
}

func TestClaimTaskRejectsUnsafeCompleteDependencyGraph(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	testhelpers.SetupTestGitRepo(t, projectRoot)
	statePath, _ := testhelpers.SetupLizaDir(t, projectRoot)
	state := graphReplanCycleState()
	testhelpers.WriteInitialState(t, statePath, state)

	_, err := ClaimTaskWithAuthority(projectRoot, "task-a", models.AgentAuthority{ID: "coder-1", Generation: testhelpers.TestAgentGeneration})
	if err == nil || !strings.Contains(err.Error(), "circular dependency detected") {
		t.Fatalf("ClaimTaskWithAuthority() error = %v, want dependency-cycle rejection", err)
	}
}

func TestGraphReplanRequestAcceptsFullyDependencyStalledRun(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	testhelpers.SetupTestGitRepo(t, projectRoot)
	statePath, _ := testhelpers.SetupLizaDir(t, projectRoot)
	state := graphReplanDependencyStallState()
	testhelpers.WriteInitialState(t, statePath, state)

	requested, err := RequestGraphReplan(projectRoot, RequestGraphReplanInput{
		RunID:       state.Goal.ID,
		RequestedBy: "codex-controller-thread-1",
		Reason:      "blocked implementation waits for a provider that dependency-held planning has not materialized",
	})
	if err != nil {
		t.Fatalf("RequestGraphReplan() error: %v", err)
	}
	for _, want := range []string{
		"no claimable or reviewable work",
		"blocked-implementation",
		"dependency-held-plan",
	} {
		if !strings.Contains(requested.Request.Diagnostic, want) {
			t.Fatalf("diagnostic = %q, want %q", requested.Request.Diagnostic, want)
		}
	}

	persisted, err := db.New(statePath).Read()
	if err != nil {
		t.Fatalf("read request state: %v", err)
	}
	if persisted.FindTask("blocked-implementation").Status != models.TaskStatusBlocked ||
		!slices.Equal(persisted.FindTask("dependency-held-plan").DependsOn, []string{"blocked-implementation"}) {
		t.Fatal("controller request mutated task lifecycle or dependency state")
	}
}

func TestGraphReplanRequestRejectsUnprovenSemanticStall(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*models.State)
	}{
		{
			name: "claimable work remains",
			mutate: func(state *models.State) {
				state.Tasks = append(state.Tasks, testhelpers.BuildTaskByStatus("independent-work", models.TaskStatusReady, time.Now().UTC()))
				state.Sprint.Scope.Planned = append(state.Sprint.Scope.Planned, "independent-work")
			},
		},
		{
			name: "blocked evidence has no durable reason",
			mutate: func(state *models.State) {
				state.FindTask("blocked-implementation").BlockedReason = nil
			},
		},
		{
			name: "worker ownership remains active",
			mutate: func(state *models.State) {
				owner := "coder-1"
				state.FindTask("blocked-implementation").AssignedTo = &owner
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			projectRoot := t.TempDir()
			testhelpers.SetupTestGitRepo(t, projectRoot)
			statePath, _ := testhelpers.SetupLizaDir(t, projectRoot)
			state := graphReplanDependencyStallState()
			tt.mutate(state)
			testhelpers.WriteInitialState(t, statePath, state)

			_, err := RequestGraphReplan(projectRoot, RequestGraphReplanInput{
				RunID: state.Goal.ID, RequestedBy: "controller-1", Reason: "unproven stall",
			})
			if err == nil || !strings.Contains(err.Error(), "fully dependency-stalled run") {
				t.Fatalf("RequestGraphReplan() error = %v, want fail-closed semantic-stall rejection", err)
			}
		})
	}
}

func graphReplanDependencyStallState() *models.State {
	state := testhelpers.CreateValidState()
	state.DependencyContractVersion = models.DependencyContractVersion
	state.Goal.SpecRef = "README.md"
	now := time.Now().UTC()
	blocked := testhelpers.BuildTaskByStatus("blocked-implementation", models.TaskStatusBlocked, now)
	blocked.AssignedTo = nil
	plan := testhelpers.BuildTaskByStatus("dependency-held-plan", models.TaskStatusReady, now)
	plan.DependsOn = []string{blocked.ID}
	plan.DependencyContracts = []models.DependencyContract{{
		ProviderTask: blocked.ID, Purpose: "Consume the implementation contract", Gate: models.DependencyGateBeforeStart,
		Severity: models.DependencySeverityCritical, Supplies: "Implementation contract",
	}}
	state.Tasks = []models.Task{blocked, plan}
	state.Sprint.Scope.Planned = []string{blocked.ID, plan.ID}
	state.Agents["orchestrator-1"] = testhelpers.RegisteredTestAgent("orchestrator")
	return state
}

func graphReplanCycleState() *models.State {
	state := testhelpers.CreateValidState()
	state.DependencyContractVersion = models.DependencyContractVersion
	state.Goal.SpecRef = "README.md"
	now := time.Now().UTC()
	taskA := testhelpers.BuildTaskByStatus("task-a", models.TaskStatusReady, now)
	taskA.DependencyContracts = []models.DependencyContract{{
		ProviderTask: "task-b", Purpose: "Final runtime validation", Gate: models.DependencyGateBeforeApproval,
		Severity: models.DependencySeverityHigh, Supplies: "Runtime validation provider",
	}}
	taskB := testhelpers.BuildTaskByStatus("task-b", models.TaskStatusReady, now)
	taskB.DependsOn = []string{"task-a"}
	taskB.DependencyContracts = []models.DependencyContract{{
		ProviderTask: "task-a", Purpose: "Implementation start", Gate: models.DependencyGateBeforeStart,
		Severity: models.DependencySeverityCritical, Supplies: "Implementation contract",
	}}
	state.Tasks = []models.Task{taskA, taskB}
	state.Sprint.Scope.Planned = []string{"task-a", "task-b"}
	state.Agents["orchestrator-1"] = testhelpers.RegisteredTestAgent("orchestrator")
	state.Agents["coder-1"] = testhelpers.RegisteredTestAgent("coder")
	return state
}
