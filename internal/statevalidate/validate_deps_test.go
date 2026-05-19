package statevalidate

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestValidateDependencies_RejectsMalformedDependsOn(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name    string
		task    models.Task
		wantErr string
	}{
		{
			name: "untrimmed",
			task: func() models.Task {
				task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
				task.DependsOn = []string{" dep-1 "}
				return task
			}(),
			wantErr: "must be non-empty and trimmed",
		},
		{
			name: "duplicate",
			task: func() models.Task {
				task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
				task.DependsOn = []string{"dep-1", "dep-1"}
				return task
			}(),
			wantErr: "duplicate depends_on entry",
		},
		{
			name: "self",
			task: func() models.Task {
				task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
				task.DependsOn = []string{"task-1"}
				return task
			}(),
			wantErr: "referencing itself",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := testhelpers.CreateValidState()
			dep := testhelpers.BuildTaskByStatus("dep-1", models.TaskStatusMerged, now)
			state.Tasks = []models.Task{tt.task, dep}

			err := validateDependencies(state, "", true, nil, nil, nil)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateDependencies_WarnsForNonExecutingUnsatisfiedSupersession(t *testing.T) {
	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
	task.DependsOn = []string{"dep-1"}
	dep := models.Task{ID: "dep-1", Status: models.TaskStatusSuperseded}
	state.Tasks = []models.Task{task, dep}

	var warnings bytes.Buffer
	if err := validateDependencies(state, "", true, nil, nil, &warnings); err != nil {
		t.Fatalf("validateDependencies() error = %v", err)
	}
	if !strings.Contains(warnings.String(), "dependency dep-1 is not satisfied via supersession path") {
		t.Fatalf("warnings = %q, want unsatisfied supersession warning", warnings.String())
	}
}

func TestValidateDependencies_DoesNotWarnForNonExecutingDirectPendingDependency(t *testing.T) {
	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
	task.DependsOn = []string{"dep-1"}
	dep := testhelpers.BuildTaskByStatus("dep-1", models.TaskStatusReady, now)
	state.Tasks = []models.Task{task, dep}

	var warnings bytes.Buffer
	if err := validateDependencies(state, "", true, nil, nil, &warnings); err != nil {
		t.Fatalf("validateDependencies() error = %v", err)
	}
	if warnings.Len() != 0 {
		t.Fatalf("warnings = %q, want none for ordinary pending dependency", warnings.String())
	}
}

func TestValidateState_WarnsBlockedReasonReferencesTaskWithoutDependsOn(t *testing.T) {
	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	reason := "Waiting on dep-1 to merge first"
	blocked := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
	blocked.BlockedReason = &reason
	blocked.DependsOn = nil
	dep := testhelpers.BuildTaskByStatus("dep-1", models.TaskStatusReady, now)
	state.Tasks = []models.Task{blocked, dep}

	var warnings bytes.Buffer
	tmpDir := t.TempDir()
	testhelpers.SetupPipelineConfig(t, tmpDir)
	if err := ValidateState(state, tmpDir, true, &warnings); err != nil {
		t.Fatalf("ValidateState() error: %v", err)
	}
	if !strings.Contains(warnings.String(), "blocked_reason references task dep-1 but depends_on is empty") {
		t.Fatalf("warnings = %q, want blocked dependency warning", warnings.String())
	}
}

func TestValidateState_WarnsBlockedReasonMatchesWholeTaskID(t *testing.T) {
	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	reason := "Waiting on task-10 to merge first"
	blocked := testhelpers.BuildTaskByStatus("blocked-task", models.TaskStatusBlocked, now)
	blocked.BlockedReason = &reason
	blocked.DependsOn = nil
	task1 := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
	task10 := testhelpers.BuildTaskByStatus("task-10", models.TaskStatusReady, now)
	state.Tasks = []models.Task{blocked, task1, task10}

	var warnings bytes.Buffer
	tmpDir := t.TempDir()
	testhelpers.SetupPipelineConfig(t, tmpDir)
	if err := ValidateState(state, tmpDir, true, &warnings); err != nil {
		t.Fatalf("ValidateState() error: %v", err)
	}
	if !strings.Contains(warnings.String(), "blocked_reason references task task-10 but depends_on is empty") {
		t.Fatalf("warnings = %q, want task-10 dependency warning", warnings.String())
	}
	if strings.Contains(warnings.String(), "blocked_reason references task task-1 but depends_on is empty") {
		t.Fatalf("warnings = %q, should not match task-1 as a substring of task-10", warnings.String())
	}
}
