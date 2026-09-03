package statevalidate

import (
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestValidateDependencyGraphRejectsMissingProviderAndUntypedSchedulerEdge(t *testing.T) {
	t.Parallel()

	state := testhelpers.CreateValidState()
	state.DependencyContractVersion = models.DependencyContractVersion
	now := time.Now().UTC()
	consumer := testhelpers.BuildTaskByStatus("consumer", models.TaskStatusReady, now)
	consumer.DependsOn = []string{"missing-provider"}
	state.Tasks = []models.Task{consumer}

	err := ValidateDependencyGraph(state, nil)
	for _, want := range []string{
		"before_start providers [] must exactly match scheduler dependencies [missing-provider]",
	} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("ValidateDependencyGraph() error = %v, want %q", err, want)
		}
	}

	consumer.DependencyContracts = []models.DependencyContract{{
		ProviderTask: "missing-provider",
		Purpose:      "Consume the provider's runtime contract",
		Gate:         models.DependencyGateBeforeStart,
		Severity:     models.DependencySeverityCritical,
		Supplies:     "Runtime contract v1",
	}}
	state.Tasks[0] = consumer
	err = ValidateDependencyGraph(state, nil)
	if err == nil || !strings.Contains(err.Error(), "dependency provider missing-provider does not exist") {
		t.Fatalf("ValidateDependencyGraph() error = %v, want missing-provider diagnostic", err)
	}
}

func TestValidateDependencyGraphDetectsReverseValidationCycleBeforeExecution(t *testing.T) {
	t.Parallel()

	state := testhelpers.CreateValidState()
	state.DependencyContractVersion = models.DependencyContractVersion
	now := time.Now().UTC()

	// A can start independently, but its final runtime acceptance is gated by B.
	taskA := testhelpers.BuildTaskByStatus("task-a", models.TaskStatusReady, now)
	taskA.DependencyContracts = []models.DependencyContract{{
		ProviderTask: "task-b",
		Purpose:      "Run final runtime acceptance against B's landed provider",
		Gate:         models.DependencyGateBeforeApproval,
		Severity:     models.DependencySeverityHigh,
		Supplies:     "Final runtime acceptance provider",
	}}

	// B cannot start until A lands, forming A --approval--> B --start--> A.
	taskB := testhelpers.BuildTaskByStatus("task-b", models.TaskStatusReady, now)
	taskB.DependsOn = []string{"task-a"}
	taskB.DependencyContracts = []models.DependencyContract{{
		ProviderTask: "task-a",
		Purpose:      "Build against A's implementation contract",
		Gate:         models.DependencyGateBeforeStart,
		Severity:     models.DependencySeverityCritical,
		Supplies:     "Implementation contract A",
	}}
	state.Tasks = []models.Task{taskA, taskB}

	err := ValidateDependencyGraph(state, nil)
	if err == nil || !strings.Contains(err.Error(), "circular dependency detected") {
		t.Fatalf("ValidateDependencyGraph() error = %v, want cycle diagnostic", err)
	}
}

func TestValidateDependencyGraphRejectsGateStrongerThanSchedulerMeaning(t *testing.T) {
	t.Parallel()

	state := testhelpers.CreateValidState()
	state.DependencyContractVersion = models.DependencyContractVersion
	now := time.Now().UTC()
	provider := testhelpers.BuildTaskByStatus("provider", models.TaskStatusReady, now)
	consumer := testhelpers.BuildTaskByStatus("consumer", models.TaskStatusReady, now)
	consumer.DependsOn = []string{"provider"}
	consumer.DependencyContracts = []models.DependencyContract{{
		ProviderTask: "provider",
		Purpose:      "Only gate final validation",
		Gate:         models.DependencyGateBeforeApproval,
		Severity:     models.DependencySeverityHigh,
		Supplies:     "Final validation fixture",
	}}
	state.Tasks = []models.Task{provider, consumer}

	err := ValidateDependencyGraph(state, nil)
	if err == nil || !strings.Contains(err.Error(), "before_start providers [] must exactly match scheduler dependencies [provider]") {
		t.Fatalf("ValidateDependencyGraph() error = %v, want gate/scheduler mismatch", err)
	}
}

func TestValidateApprovalDependenciesEnforcesLaterLifecycleGate(t *testing.T) {
	t.Parallel()

	state := testhelpers.CreateValidState()
	now := time.Now().UTC()
	provider := testhelpers.BuildTaskByStatus("provider", models.TaskStatusReady, now)
	consumer := testhelpers.BuildTaskByStatus("consumer", models.TaskStatusReady, now)
	consumer.DependencyContracts = []models.DependencyContract{{
		ProviderTask: "provider",
		Purpose:      "Use provider during final acceptance",
		Gate:         models.DependencyGateBeforeApproval,
		Severity:     models.DependencySeverityHigh,
		Supplies:     "Final acceptance fixture",
	}}
	state.Tasks = []models.Task{provider, consumer}

	if err := ValidateApprovalDependencies(state, &state.Tasks[1]); err == nil || !strings.Contains(err.Error(), "cannot be approved or merged") {
		t.Fatalf("ValidateApprovalDependencies() error = %v, want unmet approval gate", err)
	}
	state.Tasks[0] = testhelpers.BuildTaskByStatus("provider", models.TaskStatusMerged, now)
	if err := ValidateApprovalDependencies(state, &state.Tasks[1]); err != nil {
		t.Fatalf("ValidateApprovalDependencies() after provider merge: %v", err)
	}
}

func TestValidateDependencyGraphAcceptsRecordedPerSubtaskDeduplication(t *testing.T) {
	t.Parallel()

	state := testhelpers.CreateValidState()
	state.DependencyContractVersion = models.DependencyContractVersion
	now := time.Now().UTC()
	parent := testhelpers.BuildTaskByStatus("plan", models.TaskStatus("CODING_PLAN_APPROVED"), now)
	parent.Output = []models.OutputEntry{{Kind: "shared-provider"}}
	parent.TransitionsExecuted = map[string]bool{"code-plan-to-coding": true}
	parent.History = append(parent.History, models.TaskHistoryEntry{
		Time: now, Event: models.TaskEventTransitionExecuted,
		Extra: map[string]any{
			"transition": "code-plan-to-coding",
			"skipped_entries": []map[string]any{{
				"output_index": 0,
				"kind":         "shared-provider",
				"reason":       "kind already has a non-terminal provider",
				"remapped_to":  "existing-provider",
			}},
		},
	})
	provider := testhelpers.BuildTaskByStatus("existing-provider", models.TaskStatusReady, now)
	provider.Kind = "shared-provider"
	state.Tasks = []models.Task{parent, provider}

	if err := ValidateDependencyGraph(state, loadTestResolver(t)); err != nil {
		t.Fatalf("ValidateDependencyGraph() rejected recorded deduplication: %v", err)
	}
	state.Tasks = state.Tasks[:1]
	if err := ValidateDependencyGraph(state, loadTestResolver(t)); err == nil || !strings.Contains(err.Error(), "remapped provider existing-provider is missing") {
		t.Fatalf("ValidateDependencyGraph() error = %v, want missing remapped provider", err)
	}
}

func TestDependencyGraphSnapshotIncludesLegacyPlannerOutputEdges(t *testing.T) {
	t.Parallel()

	state := testhelpers.CreateValidState()
	now := time.Now().UTC()
	parent := testhelpers.BuildTaskByStatus("plan", models.TaskStatusReady, now)
	parent.Output = []models.OutputEntry{
		{DependsOn: []string{"1"}},
		{TaskDependsOn: []string{"external-provider"}},
	}
	provider := testhelpers.BuildTaskByStatus("external-provider", models.TaskStatusReady, now)
	state.Tasks = []models.Task{parent, provider}

	_, edges, _ := DependencyGraphSnapshot(state)
	want := map[string]bool{
		"plan#output:0->plan#output:1":     false,
		"plan#output:1->external-provider": false,
	}
	for _, edge := range edges {
		key := edge.Consumer + "->" + edge.Provider
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for edge, found := range want {
		if !found {
			t.Errorf("DependencyGraphSnapshot() missing %s: %+v", edge, edges)
		}
	}
}

func TestCandidateLineageFingerprintIgnoresUncommittedMaterializedTasks(t *testing.T) {
	t.Parallel()

	state := testhelpers.CreateValidState()
	now := time.Now().UTC()
	commit := "candidate"
	parent := testhelpers.BuildTaskByStatus("plan", models.TaskStatusReady, now)
	parent.ReviewCommit = &commit
	state.Tasks = []models.Task{parent}
	before := CandidateLineageFingerprint(state)

	state.Tasks = append(state.Tasks, testhelpers.BuildTaskByStatus("materialized-child", models.TaskStatusReady, now))
	if after := CandidateLineageFingerprint(state); after != before {
		t.Fatalf("uncommitted materialized task changed candidate lineage: before %s after %s", before, after)
	}
}
