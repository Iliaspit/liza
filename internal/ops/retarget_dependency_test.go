package ops

import (
	"slices"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestRetargetDependency_ReplacesEdgeAndClearsMatchingRepairRequest(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Goal.SpecRef = "README.md"
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
	task.DependsOn = []string{"old-dep", "replacement-a"}
	task.RepairRequest = &models.RepairRequest{
		Operation:  "retarget-dependency",
		Target:     "task-1",
		Command:    "liza retarget-dependency task-1 old-dep replacement-a,replacement-b --reason repair --agent-id orchestrator-1 --json",
		Evidence:   []string{"command=liza claim-task task-1 coder-1 exit_code=1 stderr=task has invalid dependency old-dep"},
		Validation: []string{"liza validate --json"},
	}
	state.Tasks = []models.Task{
		task,
		testhelpers.BuildTaskByStatus("replacement-a", models.TaskStatusMerged, now),
		testhelpers.BuildTaskByStatus("replacement-b", models.TaskStatusMerged, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := RetargetDependency(tmpDir, "task-1", "old-dep", []string{"replacement-a", "replacement-b"}, "Correct stale dependency", "orchestrator-1")
	if err != nil {
		t.Fatalf("RetargetDependency() error: %v", err)
	}
	if result.TaskID != "task-1" {
		t.Fatalf("TaskID = %q, want task-1", result.TaskID)
	}
	if !slices.Equal(result.NewDependencies, []string{"replacement-a", "replacement-b"}) {
		t.Fatalf("NewDependencies = %v", result.NewDependencies)
	}
	if !slices.Equal(result.CanonicalDependencies, []string{"replacement-a", "replacement-b"}) {
		t.Fatalf("CanonicalDependencies = %v", result.CanonicalDependencies)
	}
	if !result.RepairRequestCleared {
		t.Fatal("RepairRequestCleared = false, want true")
	}

	readState, err := db.New(stateFile).Read()
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	updated := readState.FindTask("task-1")
	if updated == nil {
		t.Fatal("task-1 not found")
	}
	if !slices.Equal(updated.DependsOn, []string{"replacement-a", "replacement-b"}) {
		t.Fatalf("DependsOn = %v, want [replacement-a replacement-b]", updated.DependsOn)
	}
	if updated.RepairRequest != nil {
		t.Fatal("RepairRequest should be cleared")
	}
	last := updated.History[len(updated.History)-1]
	if last.Event != models.TaskEventDependenciesRewritten {
		t.Fatalf("History event = %q, want %q", last.Event, models.TaskEventDependenciesRewritten)
	}
	if last.Extra["operation"] != "retarget-dependency" {
		t.Fatalf("history operation = %v, want retarget-dependency", last.Extra["operation"])
	}
	if last.Extra["repair_request_cleared"] != true {
		t.Fatalf("repair_request_cleared = %v, want true", last.Extra["repair_request_cleared"])
	}
}

func TestRetargetDependency_CanonicalizesSupersededReplacement(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Goal.SpecRef = "README.md"
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
	task.DependsOn = []string{"old-dep"}
	superseded := testhelpers.BuildTaskByStatus("replacement-old", models.TaskStatusSuperseded, now)
	superseded.RolePair = "coding-pair"
	superseded.SupersededBy = []string{"replacement-new"}
	superseded.RescopeReason = testhelpers.StringPtr("replaced")
	state.Tasks = []models.Task{
		task,
		superseded,
		testhelpers.BuildTaskByStatus("replacement-new", models.TaskStatusMerged, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := RetargetDependency(tmpDir, "task-1", "old-dep", []string{"replacement-old"}, "Follow replacement", "orchestrator-1")
	if err != nil {
		t.Fatalf("RetargetDependency() error: %v", err)
	}
	if !slices.Equal(result.CanonicalDependencies, []string{"replacement-new"}) {
		t.Fatalf("CanonicalDependencies = %v, want [replacement-new]", result.CanonicalDependencies)
	}

	readState, err := db.New(stateFile).Read()
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	updated := readState.FindTask("task-1")
	if !slices.Equal(updated.DependsOn, []string{"replacement-new"}) {
		t.Fatalf("DependsOn = %v, want [replacement-new]", updated.DependsOn)
	}
}

func TestRetargetDependency_RejectsCanonicalizationToEmptyDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Goal.SpecRef = "README.md"
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
	task.DependsOn = []string{"old-dep"}
	abandoned := testhelpers.BuildTaskByStatus("abandoned-dep", models.TaskStatusAbandoned, now)
	abandoned.RolePair = "coding-pair"
	state.Tasks = []models.Task{
		task,
		testhelpers.BuildTaskByStatus("old-dep", models.TaskStatusMerged, now),
		abandoned,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := RetargetDependency(tmpDir, "task-1", "old-dep", []string{"abandoned-dep"}, "Should fail", "orchestrator-1")
	testhelpers.RequireErrorContains(t, err, "retarget-dependency cannot remove all dependencies")

	readState, err := db.New(stateFile).Read()
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	updated := readState.FindTask("task-1")
	if !slices.Equal(updated.DependsOn, []string{"old-dep"}) {
		t.Fatalf("DependsOn = %v, want [old-dep]", updated.DependsOn)
	}
}

func TestRetargetDependency_DoesNotClearRepairRequestForDifferentTuple(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Goal.SpecRef = "README.md"
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
	task.DependsOn = []string{"old-dep"}
	task.RepairRequest = &models.RepairRequest{
		Operation:  "retarget-dependency",
		Target:     "task-1",
		Command:    "liza retarget-dependency task-1 other-old new-dep --reason repair --agent-id orchestrator-1 --json",
		Evidence:   []string{"command=liza claim-task task-1 coder-1 exit_code=1 stderr=task has invalid dependency other-old"},
		Validation: []string{"liza validate --json"},
	}
	state.Tasks = []models.Task{
		task,
		testhelpers.BuildTaskByStatus("old-dep", models.TaskStatusMerged, now),
		testhelpers.BuildTaskByStatus("new-dep", models.TaskStatusMerged, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := RetargetDependency(tmpDir, "task-1", "old-dep", []string{"new-dep"}, "Correct dependency", "orchestrator-1")
	if err != nil {
		t.Fatalf("RetargetDependency() error: %v", err)
	}
	if result.RepairRequestCleared {
		t.Fatal("RepairRequestCleared = true, want false")
	}

	readState, err := db.New(stateFile).Read()
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	updated := readState.FindTask("task-1")
	if updated.RepairRequest == nil {
		t.Fatal("RepairRequest should remain for different tuple")
	}
}

func TestRetargetDependency_RejectsTerminalDependentTask(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Goal.SpecRef = "README.md"
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now)
	task.DependsOn = []string{"old-dep"}
	state.Tasks = []models.Task{
		task,
		testhelpers.BuildTaskByStatus("old-dep", models.TaskStatusMerged, now),
		testhelpers.BuildTaskByStatus("new-dep", models.TaskStatusMerged, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := RetargetDependency(tmpDir, "task-1", "old-dep", []string{"new-dep"}, "Should fail", "orchestrator-1")
	testhelpers.RequireErrorContains(t, err, "cannot retarget dependencies on terminal task task-1")
}

func TestRetargetDependency_RejectsMissingOldEdge(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Goal.SpecRef = "README.md"
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
	task.DependsOn = []string{"actual-dep"}
	state.Tasks = []models.Task{
		task,
		testhelpers.BuildTaskByStatus("actual-dep", models.TaskStatusMerged, now),
		testhelpers.BuildTaskByStatus("new-dep", models.TaskStatusMerged, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := RetargetDependency(tmpDir, "task-1", "old-dep", []string{"new-dep"}, "Should fail", "orchestrator-1")
	testhelpers.RequireErrorContains(t, err, "task task-1 does not depend on old-dep")
}

func TestRetargetDependency_RequiresAtLeastOneNewDependency(t *testing.T) {
	_, err := RetargetDependency("/nonexistent", "task-1", "old-dep", nil, "Should fail", "orchestrator-1")
	testhelpers.RequireErrorContains(t, err, "at least one new dependency is required")
}

func TestRetargetDependency_RejectsCandidateStateCycle(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Goal.SpecRef = "README.md"
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
	task.DependsOn = []string{"old-dep"}
	newDep := testhelpers.BuildTaskByStatus("new-dep", models.TaskStatusReady, now)
	newDep.DependsOn = []string{"task-1"}
	state.Tasks = []models.Task{
		task,
		testhelpers.BuildTaskByStatus("old-dep", models.TaskStatusMerged, now),
		newDep,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := RetargetDependency(tmpDir, "task-1", "old-dep", []string{"new-dep"}, "Should fail", "orchestrator-1")
	testhelpers.RequireErrorContains(t, err, "circular dependency detected")

	readState, readErr := db.New(stateFile).Read()
	if readErr != nil {
		t.Fatalf("read state: %v", readErr)
	}
	unchanged := readState.FindTask("task-1")
	if !slices.Equal(unchanged.DependsOn, []string{"old-dep"}) {
		t.Fatalf("DependsOn persisted after failed validation: %v", unchanged.DependsOn)
	}
}
