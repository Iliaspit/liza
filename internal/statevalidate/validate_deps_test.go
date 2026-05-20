package statevalidate

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/pipeline"
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

func TestValidateDependencies_RejectsDownstreamDependency(t *testing.T) {
	now := time.Now().UTC()
	resolver, cfg := dependencyDirectionResolver(t)

	task := testhelpers.BuildTaskByStatus("plan-1", models.TaskStatusDraftCodingPlan, now)
	task.RolePair = "code-planning-pair"
	task.DependsOn = []string{"coding-1"}
	dep := testhelpers.BuildTaskByStatus("coding-1", models.TaskStatusMerged, now)
	dep.RolePair = "coding-pair"
	state := &models.State{Tasks: []models.Task{task, dep}}

	err := validateDependencies(state, "", true, resolver, cfg, nil)
	if err == nil {
		t.Fatal("validateDependencies() error = nil, want downstream dependency error")
	}
	if !strings.Contains(err.Error(), "role_pair coding-pair is downstream of code-planning-pair") {
		t.Fatalf("error = %q, want downstream role-pair error", err.Error())
	}
}

func TestValidateDependencies_RejectsDownstreamSupersessionPath(t *testing.T) {
	now := time.Now().UTC()
	resolver, cfg := dependencyDirectionResolver(t)

	task := testhelpers.BuildTaskByStatus("plan-1", models.TaskStatusDraftCodingPlan, now)
	task.RolePair = "code-planning-pair"
	task.DependsOn = []string{"old-plan"}
	oldPlan := testhelpers.BuildTaskByStatus("old-plan", models.TaskStatusSuperseded, now)
	oldPlan.RolePair = "code-planning-pair"
	oldPlan.SupersededBy = []string{"coding-1"}
	coding := testhelpers.BuildTaskByStatus("coding-1", models.TaskStatusMerged, now)
	coding.RolePair = "coding-pair"
	state := &models.State{Tasks: []models.Task{task, oldPlan, coding}}

	err := validateDependencies(state, "", true, resolver, cfg, nil)
	if err == nil {
		t.Fatal("validateDependencies() error = nil, want downstream supersession error")
	}
	if !strings.Contains(err.Error(), "resolves through downstream task coding-1") {
		t.Fatalf("error = %q, want supersession downstream error", err.Error())
	}
}

func dependencyDirectionResolver(t *testing.T) (*pipeline.Resolver, *pipeline.PipelineConfig) {
	t.Helper()
	tmpDir := t.TempDir()
	testhelpers.SetupPipelineConfig(t, tmpDir)
	cfg, err := pipeline.LoadFrozen(tmpDir)
	if err != nil {
		t.Fatalf("LoadFrozen: %v", err)
	}
	return pipeline.NewResolver(cfg), cfg
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
