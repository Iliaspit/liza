package integration

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/commands"
	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/testhelpers"
)

const (
	reconciliationStaleDependency   = "stale-dependency"
	reconciliationReplacementAlias  = "replacement-alias"
	reconciliationCurrentDependency = "current-dependency"
)

func TestBlockedMetadataReconciliation(t *testing.T) {
	t.Run("partial repair retains only current blocker metadata", func(t *testing.T) {
		const taskID = "partial-repair"
		projectRoot, blackboard, retiredRequest := setupBlockedMetadataReconciliation(t, taskID)

		retargetResult, err := ops.RetargetDependency(
			projectRoot,
			taskID,
			reconciliationStaleDependency,
			[]string{reconciliationReplacementAlias},
			"Use the canonical replacement",
			"orchestrator-1",
		)
		if err != nil {
			t.Fatalf("RetargetDependency() error: %v", err)
		}
		if !retargetResult.RepairRequestCleared {
			t.Fatal("RetargetDependency() did not retire the fulfilled repair request")
		}
		if !slices.Equal(retargetResult.CanonicalDependencies, []string{reconciliationCurrentDependency}) {
			t.Fatalf("canonical dependencies = %v, want [%s]", retargetResult.CanonicalDependencies, reconciliationCurrentDependency)
		}

		currentRequest := &models.RepairRequest{
			Operation: "retarget-dependency",
			Target:    taskID,
			Command: "retarget-dependency " + taskID + " " + reconciliationCurrentDependency +
				" follow-up-dependency",
			Evidence:   []string{"error=current dependency still needs replacement"},
			Validation: []string{"inspect the task after the follow-up repair"},
		}
		currentReason := "The canonical dependency still needs a follow-up repair"
		currentQuestions := []string{"Can the orchestrator retarget the current dependency?"}
		if err := commands.AssessBlockedWithOptionsCommand(
			projectRoot,
			taskID,
			"Retargeting removed the stale edge but left one current blocker",
			"orchestrator-1",
			ops.AssessBlockedOptions{
				Reason:        currentReason,
				Questions:     currentQuestions,
				RepairRequest: currentRequest,
			},
		); err != nil {
			t.Fatalf("AssessBlockedWithOptionsCommand() error: %v", err)
		}

		state := readBlockedMetadataState(t, blackboard)
		task := mustFindBlockedMetadataTask(t, state, taskID)
		if task.Status != models.TaskStatusBlocked {
			t.Fatalf("task status = %s, want %s", task.Status, models.TaskStatusBlocked)
		}
		assertBlockedMetadataHistory(
			t,
			task,
			retiredRequest,
			models.TaskEventOrchestratorAssessment,
		)

		inspection, raw := inspectBlockedMetadataTask(t, projectRoot, taskID)
		if inspection.Status != models.TaskStatusBlocked {
			t.Fatalf("InspectCommand status = %s, want %s", inspection.Status, models.TaskStatusBlocked)
		}
		if !slices.Equal(inspection.DependsOn, []string{reconciliationCurrentDependency}) {
			t.Fatalf("InspectCommand depends_on = %v, want [%s]", inspection.DependsOn, reconciliationCurrentDependency)
		}
		if inspection.BlockedReason == nil || *inspection.BlockedReason != currentReason {
			t.Fatalf("InspectCommand blocked_reason = %v, want %q", inspection.BlockedReason, currentReason)
		}
		if !slices.Equal(inspection.BlockedQuestions, currentQuestions) {
			t.Fatalf("InspectCommand blocked_questions = %v, want %v", inspection.BlockedQuestions, currentQuestions)
		}
		if !reflect.DeepEqual(inspection.RepairRequest, currentRequest) {
			t.Fatalf("InspectCommand repair_request = %#v, want %#v", inspection.RepairRequest, currentRequest)
		}
		for _, field := range []string{"depends_on", "blocked_reason", "blocked_questions", "repair_request"} {
			if _, ok := raw[field]; !ok {
				t.Errorf("InspectCommand JSON omitted current field %q", field)
			}
		}
	})

	t.Run("complete repair unblocks to a claimable status without blocker metadata", func(t *testing.T) {
		const taskID = "complete-repair"
		projectRoot, blackboard, retiredRequest := setupBlockedMetadataReconciliation(t, taskID)

		retargetResult, err := ops.RetargetDependency(
			projectRoot,
			taskID,
			reconciliationStaleDependency,
			[]string{reconciliationReplacementAlias},
			"Use the canonical replacement",
			"orchestrator-1",
		)
		if err != nil {
			t.Fatalf("RetargetDependency() error: %v", err)
		}
		if !retargetResult.RepairRequestCleared {
			t.Fatal("RetargetDependency() did not retire the fulfilled repair request")
		}

		unblockResult, err := ops.UnblockTaskWithOptions(
			projectRoot,
			taskID,
			"The dependency repair is complete",
			"orchestrator-1",
			ops.UnblockTaskOptions{},
		)
		if err != nil {
			t.Fatalf("UnblockTaskWithOptions() error: %v", err)
		}
		if !unblockResult.Claimable {
			t.Fatalf("UnblockTaskWithOptions() result = %+v, want claimable", unblockResult)
		}

		state := readBlockedMetadataState(t, blackboard)
		task := mustFindBlockedMetadataTask(t, state, taskID)
		if task.Status != unblockResult.ToStatus {
			t.Fatalf("persisted status = %s, want unblock result status %s", task.Status, unblockResult.ToStatus)
		}
		assertBlockedMetadataHistory(t, task, retiredRequest, models.TaskEventUnblocked)

		inspection, raw := inspectBlockedMetadataTask(t, projectRoot, taskID)
		if inspection.Status != unblockResult.ToStatus {
			t.Fatalf("InspectCommand status = %s, want %s", inspection.Status, unblockResult.ToStatus)
		}
		if !slices.Equal(inspection.DependsOn, []string{reconciliationCurrentDependency}) {
			t.Fatalf("InspectCommand depends_on = %v, want [%s]", inspection.DependsOn, reconciliationCurrentDependency)
		}
		for _, field := range []string{"blocked_reason", "blocked_questions", "repair_request"} {
			if _, ok := raw[field]; ok {
				t.Errorf("InspectCommand JSON retained blocker field %q after unblock", field)
			}
		}
	})
}

type blockedMetadataInspection struct {
	Status           models.TaskStatus     `json:"status"`
	DependsOn        []string              `json:"depends_on"`
	BlockedReason    *string               `json:"blocked_reason"`
	BlockedQuestions []string              `json:"blocked_questions"`
	RepairRequest    *models.RepairRequest `json:"repair_request"`
}

func setupBlockedMetadataReconciliation(t *testing.T, taskID string) (string, *db.Blackboard, models.RepairRequest) {
	t.Helper()
	projectRoot := t.TempDir()
	testhelpers.SetupTestGitRepo(t, projectRoot)
	statePath, _ := testhelpers.SetupLizaDir(t, projectRoot)
	testhelpers.SetupPipelineConfig(t, projectRoot)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Goal.SpecRef = "README.md"
	task := testhelpers.BuildTaskByStatus(taskID, models.TaskStatusBlocked, now)
	task.AssignedTo = nil
	task.Worktree = nil
	task.DependsOn = []string{reconciliationStaleDependency}
	staleReason := "The task still depends on an obsolete task"
	task.BlockedReason = &staleReason
	task.BlockedQuestions = []string{"Can the obsolete edge be repaired?"}
	retiredRequest := models.RepairRequest{
		Operation: "retarget-dependency",
		Target:    taskID,
		Command: "retarget-dependency " + taskID + " " + reconciliationStaleDependency +
			" " + reconciliationCurrentDependency,
		Evidence:   []string{"error=the stale dependency prevents progress"},
		Validation: []string{"inspect the canonical dependency after retargeting"},
	}
	task.RepairRequest = &retiredRequest

	replacementAlias := testhelpers.BuildTaskByStatus(reconciliationReplacementAlias, models.TaskStatusSuperseded, now)
	replacementAlias.RolePair = "coding-pair"
	replacementAlias.SupersededBy = []string{reconciliationCurrentDependency}
	replacementAlias.RescopeReason = testhelpers.StringPtr("Use the canonical replacement")
	state.Tasks = []models.Task{
		task,
		testhelpers.BuildTaskByStatus(reconciliationStaleDependency, models.TaskStatusMerged, now),
		replacementAlias,
		testhelpers.BuildTaskByStatus(reconciliationCurrentDependency, models.TaskStatusMerged, now),
		testhelpers.BuildTaskByStatus("follow-up-dependency", models.TaskStatusMerged, now),
	}

	return projectRoot, testhelpers.WriteInitialState(t, statePath, state), retiredRequest
}

func inspectBlockedMetadataTask(t *testing.T, projectRoot, taskID string) (blockedMetadataInspection, map[string]json.RawMessage) {
	t.Helper()
	output, err := commands.InspectCommand([]string{taskID}, commands.InspectOptions{
		Format:      "json",
		ProjectRoot: projectRoot,
	})
	if err != nil {
		t.Fatalf("InspectCommand() error: %v", err)
	}

	var inspection blockedMetadataInspection
	if err := json.Unmarshal([]byte(output), &inspection); err != nil {
		t.Fatalf("decode InspectCommand task JSON: %v\n%s", err, output)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &raw); err != nil {
		t.Fatalf("decode InspectCommand field map: %v\n%s", err, output)
	}
	return inspection, raw
}

func readBlockedMetadataState(t *testing.T, blackboard *db.Blackboard) *models.State {
	t.Helper()
	state, err := blackboard.Read()
	if err != nil {
		t.Fatalf("read blocked metadata state: %v", err)
	}
	return state
}

func mustFindBlockedMetadataTask(t *testing.T, state *models.State, taskID string) *models.Task {
	t.Helper()
	task := state.FindTask(taskID)
	if task == nil {
		t.Fatalf("task %s not found", taskID)
	}
	return task
}

func assertBlockedMetadataHistory(
	t *testing.T,
	task *models.Task,
	retiredRequest models.RepairRequest,
	finalEvent models.TaskEventName,
) {
	t.Helper()
	wantEvents := []models.TaskEventName{models.TaskEventDependenciesRewritten, finalEvent}
	if len(task.History) != len(wantEvents) {
		t.Fatalf("history length = %d, want %d: %#v", len(task.History), len(wantEvents), task.History)
	}
	for i, want := range wantEvents {
		if task.History[i].Event != want {
			t.Fatalf("history[%d].event = %q, want %q", i, task.History[i].Event, want)
		}
	}
	retarget := task.History[0]
	if retarget.Extra["repair_request_cleared"] != true {
		t.Fatalf("dependency rewrite repair_request_cleared = %#v, want true", retarget.Extra["repair_request_cleared"])
	}
	if retarget.Extra["repair_command"] != retiredRequest.Command {
		t.Fatalf("dependency rewrite repair_command = %#v, want %q", retarget.Extra["repair_command"], retiredRequest.Command)
	}
}
