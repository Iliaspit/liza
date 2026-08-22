package ops

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	activitylog "github.com/liza-mas/liza/internal/log"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestApplyDependencyRepair_MultiTaskAtomicSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	source := testhelpers.BuildTaskByStatus("repair-source", models.TaskStatusBlocked, now)
	source.DependsOn = []string{"old-source"}
	source.RepairRequest = dependencyRepairRequest([]models.DependencyUpdate{
		{
			TaskID:            "repair-source",
			ExpectedDependsOn: []string{"old-source"},
			DesiredDependsOn:  []string{"replacement-old"},
		},
		{
			TaskID:            "consumer",
			ExpectedDependsOn: []string{"old-consumer"},
			DesiredDependsOn:  []string{},
		},
	})
	sourceHistoryLen := len(source.History)

	consumer := testhelpers.BuildTaskByStatus("consumer", models.TaskStatusReady, now)
	consumer.DependsOn = []string{"old-consumer"}
	consumerHistoryLen := len(consumer.History)

	replacementOld := testhelpers.BuildTaskByStatus("replacement-old", models.TaskStatusSuperseded, now)
	replacementOld.RolePair = "coding-pair"
	replacementOld.SupersededBy = []string{"replacement-new"}
	replacementOld.RescopeReason = testhelpers.StringPtr("Canonical replacement")

	state := testhelpers.CreateValidState()
	state.Goal.SpecRef = "README.md"
	state.Tasks = []models.Task{
		source,
		consumer,
		testhelpers.BuildTaskByStatus("old-source", models.TaskStatusMerged, now),
		testhelpers.BuildTaskByStatus("old-consumer", models.TaskStatusMerged, now),
		replacementOld,
		testhelpers.BuildTaskByStatus("replacement-new", models.TaskStatusMerged, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := ApplyDependencyRepair(tmpDir, "repair-source", "Apply stored graph repair", "orchestrator-1")
	if err != nil {
		t.Fatalf("ApplyDependencyRepair() error: %v", err)
	}
	if result.SourceTaskID != "repair-source" {
		t.Fatalf("SourceTaskID = %q, want repair-source", result.SourceTaskID)
	}
	wantUpdates := []AppliedDependencyUpdate{
		{TaskID: "repair-source", CanonicalDependencies: []string{"replacement-new"}},
		{TaskID: "consumer", CanonicalDependencies: []string{}},
	}
	if !reflect.DeepEqual(result.Updates, wantUpdates) {
		t.Fatalf("Updates = %#v, want %#v", result.Updates, wantUpdates)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("Warnings = %v, want none", result.Warnings)
	}

	readState, err := db.New(stateFile).Read()
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	updatedSource := readState.FindTask("repair-source")
	if updatedSource == nil {
		t.Fatal("repair-source not found")
	}
	if updatedSource.Status != models.TaskStatusBlocked {
		t.Fatalf("source status = %s, want BLOCKED", updatedSource.Status)
	}
	if updatedSource.RepairRequest != nil {
		t.Fatal("source repair request was not cleared")
	}
	if !slices.Equal(updatedSource.DependsOn, []string{"replacement-new"}) {
		t.Fatalf("source depends_on = %v, want [replacement-new]", updatedSource.DependsOn)
	}
	assertAppliedDependencyHistory(t, updatedSource, sourceHistoryLen, "repair-source", []any{"old-source"}, []any{"replacement-old"}, []any{"replacement-new"})

	updatedConsumer := readState.FindTask("consumer")
	if updatedConsumer == nil {
		t.Fatal("consumer not found")
	}
	if len(updatedConsumer.DependsOn) != 0 {
		t.Fatalf("consumer depends_on = %v, want empty", updatedConsumer.DependsOn)
	}
	assertAppliedDependencyHistory(t, updatedConsumer, consumerHistoryLen, "repair-source", []any{"old-consumer"}, []any{}, []any{})

	entries, err := activitylog.New(paths.New(tmpDir).LogPath()).Read()
	if err != nil {
		t.Fatalf("read activity log: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("activity log entries = %d, want 1", len(entries))
	}
	if entries[0].Action != models.RepairOperationApplyDependencyRepair || entries[0].Agent != "orchestrator-1" {
		t.Fatalf("activity log action/agent = %q/%q", entries[0].Action, entries[0].Agent)
	}
}

func TestApplyDependencyRepair_PreservesSourceValidationReceipt(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	source := testhelpers.BuildTaskByStatus("repair-source", models.TaskStatusBlocked, now)
	source.DependsOn = []string{"old-source"}
	source.RepairRequest = dependencyRepairRequest([]models.DependencyUpdate{
		{
			TaskID:            "consumer",
			ExpectedDependsOn: []string{"old-consumer"},
			DesiredDependsOn:  []string{"new-consumer"},
		},
	})
	sourceHistoryLen := len(source.History)

	consumer := testhelpers.BuildTaskByStatus("consumer", models.TaskStatusReady, now)
	consumer.DependsOn = []string{"old-consumer"}

	state := testhelpers.CreateValidState()
	state.Goal.SpecRef = "README.md"
	state.Tasks = []models.Task{
		source,
		consumer,
		testhelpers.BuildTaskByStatus("old-source", models.TaskStatusMerged, now),
		testhelpers.BuildTaskByStatus("old-consumer", models.TaskStatusMerged, now),
		testhelpers.BuildTaskByStatus("new-consumer", models.TaskStatusMerged, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := ApplyDependencyRepair(tmpDir, "repair-source", "Apply stored graph repair", "orchestrator-1")
	if err != nil {
		t.Fatalf("ApplyDependencyRepair() error: %v", err)
	}

	readState, err := db.New(stateFile).Read()
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	updatedSource := readState.FindTask("repair-source")
	if updatedSource == nil {
		t.Fatal("repair-source not found")
	}
	if updatedSource.RepairRequest != nil {
		t.Fatal("source repair request was not cleared")
	}
	if !slices.Equal(updatedSource.DependsOn, []string{"old-source"}) {
		t.Fatalf("source depends_on = %v, want unchanged [old-source]", updatedSource.DependsOn)
	}
	if len(updatedSource.History) != sourceHistoryLen+1 {
		t.Fatalf("source history length = %d, want %d", len(updatedSource.History), sourceHistoryLen+1)
	}
	receipt := updatedSource.History[len(updatedSource.History)-1]
	if receipt.Event != "dependency_repair_applied" {
		t.Fatalf("source receipt event = %q, want dependency_repair_applied", receipt.Event)
	}
	if !reflect.DeepEqual(receipt.Extra["affected_task_ids"], []any{"consumer"}) {
		t.Fatalf("affected_task_ids = %#v, want [consumer]", receipt.Extra["affected_task_ids"])
	}
	if !reflect.DeepEqual(receipt.Extra["repair_validation"], []any{"validate repaired dependency graph"}) {
		t.Fatalf("repair_validation = %#v, want declared validation", receipt.Extra["repair_validation"])
	}
	if receipt.Extra["repair_request_cleared"] != true {
		t.Fatalf("repair_request_cleared = %#v, want true", receipt.Extra["repair_request_cleared"])
	}
}

func TestApplyDependencyRepair_StaleOrInvalidBatchRollsBack(t *testing.T) {
	assertDependencyRepairBatchRollsBack(t, "stale expectation", models.TaskStatusReady, []models.DependencyUpdate{
		{TaskID: "repair-source", ExpectedDependsOn: []string{"old-source"}, DesiredDependsOn: []string{"new-source"}},
		{TaskID: "consumer", ExpectedDependsOn: []string{"stale-consumer"}, DesiredDependsOn: []string{"new-consumer"}},
	}, "consumer dependencies changed since the repair request was created")

	assertDependencyRepairBatchRollsBack(t, "invalid dependency", models.TaskStatusReady, []models.DependencyUpdate{
		{TaskID: "repair-source", ExpectedDependsOn: []string{"old-source"}, DesiredDependsOn: []string{"new-source"}},
		{TaskID: "consumer", ExpectedDependsOn: []string{"old-consumer"}, DesiredDependsOn: []string{"missing-dependency"}},
	}, `desired dependency "missing-dependency" for task consumer does not exist`)

	assertDependencyRepairBatchRollsBack(t, "cyclic candidate", models.TaskStatusReady, []models.DependencyUpdate{
		{TaskID: "repair-source", ExpectedDependsOn: []string{"old-source"}, DesiredDependsOn: []string{"consumer"}},
		{TaskID: "consumer", ExpectedDependsOn: []string{"old-consumer"}, DesiredDependsOn: []string{"repair-source"}},
	}, "circular dependency detected")

	assertDependencyRepairBatchRollsBack(t, "terminal affected task", models.TaskStatusMerged, []models.DependencyUpdate{
		{TaskID: "repair-source", ExpectedDependsOn: []string{"old-source"}, DesiredDependsOn: []string{"new-source"}},
		{TaskID: "consumer", ExpectedDependsOn: []string{"old-consumer"}, DesiredDependsOn: []string{"new-consumer"}},
	}, "cannot apply dependency repair to terminal task consumer")
}

func assertDependencyRepairBatchRollsBack(t *testing.T, caseName string, consumerStatus models.TaskStatus, updates []models.DependencyUpdate, wantError string) {
	t.Helper()
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	source := testhelpers.BuildTaskByStatus("repair-source", models.TaskStatusBlocked, now)
	source.DependsOn = []string{"old-source"}
	source.RepairRequest = dependencyRepairRequest(updates)
	consumer := testhelpers.BuildTaskByStatus("consumer", consumerStatus, now)
	consumer.DependsOn = []string{"old-consumer"}

	state := testhelpers.CreateValidState()
	state.Goal.SpecRef = "README.md"
	state.Tasks = []models.Task{
		source,
		consumer,
		testhelpers.BuildTaskByStatus("old-source", models.TaskStatusMerged, now),
		testhelpers.BuildTaskByStatus("old-consumer", models.TaskStatusMerged, now),
		testhelpers.BuildTaskByStatus("new-source", models.TaskStatusMerged, now),
		testhelpers.BuildTaskByStatus("new-consumer", models.TaskStatusMerged, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	bb := db.New(stateFile)
	before, err := bb.Read()
	if err != nil {
		t.Fatalf("%s: read initial state: %v", caseName, err)
	}

	_, err = ApplyDependencyRepair(tmpDir, "repair-source", "Apply stored graph repair", "orchestrator-1")
	testhelpers.RequireErrorContains(t, err, wantError)

	after, err := bb.Read()
	if err != nil {
		t.Fatalf("%s: read state after rejected repair: %v", caseName, err)
	}
	if !reflect.DeepEqual(after.Tasks, before.Tasks) {
		t.Fatalf("%s: tasks changed after rejected repair\nbefore: %#v\nafter:  %#v", caseName, before.Tasks, after.Tasks)
	}
	unchangedSource := after.FindTask("repair-source")
	if unchangedSource == nil || unchangedSource.Status != models.TaskStatusBlocked || unchangedSource.RepairRequest == nil {
		t.Fatalf("%s: source task changed after rejected repair: %#v", caseName, unchangedSource)
	}
}

func dependencyRepairRequest(updates []models.DependencyUpdate) *models.RepairRequest {
	return &models.RepairRequest{
		Operation:         models.RepairOperationApplyDependencyRepair,
		Target:            "repair-source",
		DependencyUpdates: updates,
		Evidence:          []string{"command=blocked-operation exit_code=1 stderr=orchestrator repair required"},
		Validation:        []string{"validate repaired dependency graph"},
	}
}

func assertAppliedDependencyHistory(t *testing.T, task *models.Task, initialLen int, sourceTaskID string, expected, desired, canonical []any) {
	t.Helper()
	if len(task.History) != initialLen+1 {
		t.Fatalf("%s history length = %d, want %d", task.ID, len(task.History), initialLen+1)
	}
	last := task.History[len(task.History)-1]
	if last.Event != models.TaskEventDependenciesRewritten {
		t.Fatalf("%s history event = %q, want %q", task.ID, last.Event, models.TaskEventDependenciesRewritten)
	}
	if last.Agent == nil || *last.Agent != "orchestrator-1" {
		t.Fatalf("%s history agent = %v, want orchestrator-1", task.ID, last.Agent)
	}
	if last.Reason == nil || *last.Reason != "Apply stored graph repair" {
		t.Fatalf("%s history reason = %v, want repair reason", task.ID, last.Reason)
	}
	if last.Extra["operation"] != models.RepairOperationApplyDependencyRepair {
		t.Fatalf("%s history operation = %v", task.ID, last.Extra["operation"])
	}
	if last.Extra["repair_source_task"] != sourceTaskID {
		t.Fatalf("%s repair_source_task = %v, want %s", task.ID, last.Extra["repair_source_task"], sourceTaskID)
	}
	if !reflect.DeepEqual(last.Extra["expected_dependencies"], expected) {
		t.Fatalf("%s expected_dependencies = %#v, want %#v", task.ID, last.Extra["expected_dependencies"], expected)
	}
	if !reflect.DeepEqual(last.Extra["desired_dependencies"], desired) {
		t.Fatalf("%s desired_dependencies = %#v, want %#v", task.ID, last.Extra["desired_dependencies"], desired)
	}
	if !reflect.DeepEqual(last.Extra["canonical_dependencies"], canonical) {
		t.Fatalf("%s canonical_dependencies = %#v, want %#v", task.ID, last.Extra["canonical_dependencies"], canonical)
	}
}
