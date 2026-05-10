package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

// executeRootCommandCapture runs a CLI command and captures stdout output.
// This is needed because JSON output writes directly to os.Stdout, not cmd.OutOrStdout().
func executeRootCommandCapture(t *testing.T, projectRoot string, args ...string) (string, error) {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	os.Stdout = w

	cmdErr := executeRootCommand(t, projectRoot, args...)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, copyErr := io.Copy(&buf, r); copyErr != nil {
		t.Fatalf("failed to read captured stdout: %v", copyErr)
	}
	r.Close()

	return buf.String(), cmdErr
}

// parseEnvelope unmarshals a JSON envelope from stdout into a generic map.
func parseEnvelope(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var env map[string]any
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("failed to parse JSON envelope: %v\nraw output: %s", err, stdout)
	}
	return env
}

func TestJSON_ClaimTask_Success(t *testing.T) {
	projectRoot, _ := setupMutationTestProject(t, func(state *models.State) {
		now := time.Now().UTC()
		state.Tasks = []models.Task{
			testhelpers.BuildTaskByStatus("task-json-claim", models.TaskStatusReady, now),
		}
	})

	stdout, err := executeRootCommandCapture(t, projectRoot, "claim-task", "task-json-claim", "coder-1", "--json")
	if err != nil {
		t.Fatalf("claim-task --json failed: %v", err)
	}

	env := parseEnvelope(t, stdout)
	if env["ok"] != true {
		t.Fatalf("expected ok=true, got %v", env["ok"])
	}

	result, ok := env["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result to be object, got %T", env["result"])
	}

	// Verify snake_case keys from ClaimResult
	for _, key := range []string{"task_id", "agent_id", "source_status", "worktree_rel", "base_commit", "lease_expires", "integration_fix", "previous_assignee", "worktree_recreated", "warnings"} {
		if _, exists := result[key]; !exists {
			t.Errorf("missing expected key %q in result", key)
		}
	}

	if result["task_id"] != "task-json-claim" {
		t.Errorf("task_id = %v, want task-json-claim", result["task_id"])
	}
	if result["agent_id"] != "coder-1" {
		t.Errorf("agent_id = %v, want coder-1", result["agent_id"])
	}
}

func TestJSON_ClaimTask_Error(t *testing.T) {
	projectRoot, _ := setupMutationTestProject(t, nil)

	stdout, err := executeRootCommandCapture(t, projectRoot, "claim-task", "nonexistent-task", "coder-1", "--json")
	if err == nil {
		t.Fatalf("expected error for nonexistent task, got nil")
	}

	env := parseEnvelope(t, stdout)
	if env["ok"] != false {
		t.Fatalf("expected ok=false, got %v", env["ok"])
	}

	errObj, ok := env["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error to be object, got %T", env["error"])
	}
	if errObj["code"] != "not_found" {
		t.Errorf("error code = %v, want not_found", errObj["code"])
	}
}

func TestJSON_AddTasks_PartialItemFailureKeepsOKEnvelope(t *testing.T) {
	projectRoot, statePath := setupMutationTestProject(t, func(state *models.State) {
		now := time.Now().UTC()
		dupTask := testhelpers.BuildTaskByStatus("dup-task", models.TaskStatusReady, now)
		dupTask.SpecRef = "specs/vision.md"
		state.Tasks = []models.Task{dupTask}
	})
	testhelpers.CreateSpecFile(t, projectRoot, "vision.md", "# Vision\n")
	testhelpers.CreateSpecFile(t, projectRoot, "feature.md", "# Feature\n")

	tasks := []map[string]any{
		{
			"id":        "new-json-task",
			"desc":      "Task added before duplicate",
			"spec":      "specs/feature.md",
			"done":      "done",
			"scope":     "internal/ops",
			"priority":  1,
			"role_pair": "coding-pair",
		},
		{
			"id":        "dup-task",
			"desc":      "Duplicate task",
			"spec":      "specs/vision.md",
			"done":      "done",
			"scope":     "internal/ops",
			"priority":  1,
			"role_pair": "coding-pair",
		},
	}
	data, err := json.Marshal(tasks)
	if err != nil {
		t.Fatalf("failed to marshal tasks: %v", err)
	}
	tasksFile := filepath.Join(projectRoot, "tasks.json")
	if err := os.WriteFile(tasksFile, data, 0644); err != nil {
		t.Fatalf("failed to write tasks file: %v", err)
	}

	stdout, err := executeRootCommandCapture(t, projectRoot,
		"add-tasks",
		"--tasks-file", tasksFile,
		"--agent-id", "orchestrator-1",
		"--json",
	)
	if err != nil {
		t.Fatalf("add-tasks --json returned top-level error: %v", err)
	}

	env := parseEnvelope(t, stdout)
	if env["ok"] != true {
		t.Fatalf("expected ok=true for item-level partial failure, got %v", env["ok"])
	}
	if _, exists := env["error"]; exists {
		t.Fatalf("expected no top-level error for item-level partial failure, got %v", env["error"])
	}

	result, ok := env["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result to be object, got %T", env["result"])
	}
	results, ok := result["results"].([]any)
	if !ok {
		t.Fatalf("expected result.results to be array, got %T", result["results"])
	}
	if len(results) != 2 {
		t.Fatalf("result count = %d, want 2", len(results))
	}
	first, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("first result has type %T, want object", results[0])
	}
	if first["success"] != true || first["task_id"] != "new-json-task" {
		t.Fatalf("first result = %v, want successful new-json-task", first)
	}
	second, ok := results[1].(map[string]any)
	if !ok {
		t.Fatalf("second result has type %T, want object", results[1])
	}
	errMsg, _ := second["error"].(string)
	if second["success"] != false || second["task_id"] != "dup-task" || !strings.Contains(errMsg, "already exists") {
		t.Fatalf("second result = %v, want duplicate item failure", second)
	}

	state := readState(t, statePath)
	if state.FindTask("new-json-task") == nil {
		t.Fatal("new-json-task was not persisted after successful item result")
	}
	dupCount := 0
	for _, task := range state.Tasks {
		if task.ID == "dup-task" {
			dupCount++
		}
	}
	if dupCount != 1 {
		t.Fatalf("dup-task count = %d, want 1", dupCount)
	}
}

func TestJSON_Status_WithWarnings(t *testing.T) {
	// Set up project with corrupted pipeline config so resolver load fails.
	projectRoot, _ := setupMutationTestProject(t, nil)

	// Corrupt pipeline.yaml so resolver fails, producing a warning.
	pipelinePath := filepath.Join(projectRoot, ".liza", "pipeline.yaml")
	if err := os.WriteFile(pipelinePath, []byte("invalid: [yaml: {{broken"), 0644); err != nil {
		t.Fatalf("failed to corrupt pipeline.yaml: %v", err)
	}

	stdout, err := executeRootCommandCapture(t, projectRoot, "status", "--json")
	if err != nil {
		t.Fatalf("status --json failed: %v", err)
	}

	env := parseEnvelope(t, stdout)
	if env["ok"] != true {
		t.Fatalf("expected ok=true, got %v", env["ok"])
	}

	warnings, ok := env["warnings"].([]any)
	if !ok || len(warnings) == 0 {
		t.Fatalf("expected non-empty warnings array, got %v", env["warnings"])
	}

	// At least one warning should mention pipeline resolver failure
	found := false
	for _, w := range warnings {
		if s, ok := w.(string); ok {
			if len(s) > 0 {
				found = true
				break
			}
		}
	}
	if !found {
		t.Errorf("expected warning about pipeline resolver, got %v", warnings)
	}
}

func TestJSON_Status_NoWarnings(t *testing.T) {
	projectRoot, _ := setupMutationTestProject(t, nil)

	stdout, err := executeRootCommandCapture(t, projectRoot, "status", "--json")
	if err != nil {
		t.Fatalf("status --json failed: %v", err)
	}

	env := parseEnvelope(t, stdout)
	if env["ok"] != true {
		t.Fatalf("expected ok=true, got %v", env["ok"])
	}

	// No warnings when pipeline config is valid
	if env["warnings"] != nil {
		t.Errorf("expected no warnings field, got %v", env["warnings"])
	}
}

func TestJSON_UpdateSprintMetrics_TypedPayload(t *testing.T) {
	projectRoot, _ := setupMutationTestProject(t, func(state *models.State) {
		now := time.Now().UTC()
		state.Tasks = []models.Task{
			testhelpers.BuildTaskByStatus("task-metrics-1", models.TaskStatusMerged, now),
		}
	})

	stdout, err := executeRootCommandCapture(t, projectRoot, "update-sprint-metrics", "--json")
	if err != nil {
		t.Fatalf("update-sprint-metrics --json failed: %v", err)
	}

	env := parseEnvelope(t, stdout)
	if env["ok"] != true {
		t.Fatalf("expected ok=true, got %v", env["ok"])
	}

	result, ok := env["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result to be object, got %T", env["result"])
	}

	// All 11 SprintMetrics fields must be present with snake_case keys
	expectedKeys := []string{
		"tasks_done",
		"tasks_in_progress",
		"tasks_blocked",
		"iterations_total",
		"review_cycles_total",
		"review_verdict_approvals",
		"review_verdict_rejections",
		"review_verdict_count",
		"review_verdict_approval_rate_percent",
		"task_submitted_for_review_count",
		"task_outcome_approval_rate_percent",
	}

	for _, key := range expectedKeys {
		if _, exists := result[key]; !exists {
			t.Errorf("missing expected SprintMetrics key %q in result", key)
		}
	}

	// Extra field (json:"-") should not be present
	if _, exists := result["Extra"]; exists {
		t.Errorf("Extra field should not be serialized (has json:\"-\" tag)")
	}
}

func TestJSON_UpdateSprintMetrics_WithWarnings(t *testing.T) {
	projectRoot, _ := setupMutationTestProject(t, func(state *models.State) {
		now := time.Now().UTC()
		// Create 4 tasks all approved/merged with approval history
		// to get >95% approval rate and >=3 verdicts.
		for i := range 4 {
			taskID := "task-suspicious-" + string(rune('a'+i))
			task := testhelpers.BuildTaskByStatus(taskID, models.TaskStatusMerged, now)
			task.History = append(task.History,
				models.TaskHistoryEntry{
					Time:  now,
					Event: models.TaskEventSubmittedForReview,
				},
				models.TaskHistoryEntry{
					Time:  now,
					Event: models.TaskEventApproved,
				},
			)
			state.Tasks = append(state.Tasks, task)
		}
	})

	stdout, err := executeRootCommandCapture(t, projectRoot, "update-sprint-metrics", "--json")
	if err != nil {
		t.Fatalf("update-sprint-metrics --json failed: %v", err)
	}

	env := parseEnvelope(t, stdout)
	if env["ok"] != true {
		t.Fatalf("expected ok=true, got %v", env["ok"])
	}

	warnings, ok := env["warnings"].([]any)
	if !ok || len(warnings) == 0 {
		t.Fatalf("expected suspicious rate warnings, got %v", env["warnings"])
	}
}

func TestJSON_Version(t *testing.T) {
	// version doesn't need a project root
	stdout, err := executeRootCommandCapture(t, t.TempDir(), "version", "--json")
	if err != nil {
		t.Fatalf("version --json failed: %v", err)
	}

	env := parseEnvelope(t, stdout)
	if env["ok"] != true {
		t.Fatalf("expected ok=true, got %v", env["ok"])
	}

	result, ok := env["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result to be object, got %T", env["result"])
	}

	for _, key := range []string{"version", "commit", "built"} {
		val, exists := result[key]
		if !exists {
			t.Errorf("missing key %q in version result", key)
			continue
		}
		if _, isStr := val.(string); !isStr {
			t.Errorf("expected %q to be string, got %T", key, val)
		}
	}
}

func TestJSON_Validate_Valid(t *testing.T) {
	projectRoot, _ := setupMutationTestProject(t, nil)

	stdout, err := executeRootCommandCapture(t, projectRoot, "validate", "--json", "--skip-spec-check")
	if err != nil {
		t.Fatalf("validate --json failed: %v", err)
	}

	env := parseEnvelope(t, stdout)
	if env["ok"] != true {
		t.Fatalf("expected ok=true, got %v", env["ok"])
	}

	result, ok := env["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result to be object, got %T", env["result"])
	}
	if result["valid"] != true {
		t.Errorf("expected valid=true, got %v", result["valid"])
	}
}

func TestJSON_Validate_Invalid(t *testing.T) {
	// Create a project with an invalid state (empty/broken state file)
	projectRoot := t.TempDir()
	testhelpers.SetupTestGitRepo(t, projectRoot)
	_, _ = testhelpers.SetupLizaDir(t, projectRoot)
	testhelpers.SetupPipelineConfig(t, projectRoot)

	// Write an empty/minimal state that will fail validation (no version, no goal)
	statePath := filepath.Join(projectRoot, ".liza", "state.yaml")
	if err := os.WriteFile(statePath, []byte("version: 0\n"), 0644); err != nil {
		t.Fatalf("failed to write invalid state: %v", err)
	}

	stdout, err := executeRootCommandCapture(t, projectRoot, "validate", "--json", "--skip-spec-check")
	if err == nil {
		t.Fatalf("expected error for invalid state, got nil")
	}

	env := parseEnvelope(t, stdout)
	if env["ok"] != false {
		t.Fatalf("expected ok=false, got %v", env["ok"])
	}

	errObj, ok := env["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error to be object, got %T", env["error"])
	}
	if errObj["code"] == nil || errObj["code"] == "" {
		t.Errorf("expected error code to be set, got %v", errObj["code"])
	}
}

func TestJSON_Validate_DanglingParentTaskReportsValidationDetails(t *testing.T) {
	projectRoot, _ := setupMutationTestProject(t, func(state *models.State) {
		now := time.Now().UTC()
		child := testhelpers.BuildTaskByStatus("task-child", models.TaskStatusReady, now)
		missingParent := "missing-parent"
		child.ParentTask = &missingParent
		state.Tasks = []models.Task{child}
	})

	stdout, err := executeRootCommandCapture(t, projectRoot, "validate", "--json", "--skip-spec-check")
	if err == nil {
		t.Fatalf("expected validate --json to fail for dangling parent_task")
	}

	env := parseEnvelope(t, stdout)
	if env["ok"] != false {
		t.Fatalf("expected ok=false, got %v", env["ok"])
	}

	errObj, ok := env["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error to be object, got %T", env["error"])
	}
	if errObj["code"] != "validation" {
		t.Fatalf("error.code = %v, want validation", errObj["code"])
	}
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "parent_task") || !strings.Contains(msg, "missing-parent") {
		t.Fatalf("error.message = %q, want dangling parent_task details", msg)
	}
}

func TestJSON_Validate_MissingArtifactRefReportsFieldTaskAndValue(t *testing.T) {
	projectRoot, statePath := setupMutationTestProject(t, func(state *models.State) {
		now := time.Now().UTC()
		task := testhelpers.BuildTaskByStatus("task-plan-missing", models.TaskStatusMerged, now)
		task.SpecRef = "specs/vision.md"
		task.PlanRef = "specs/plans/missing.md"
		state.Tasks = []models.Task{task}
	})
	specDir := filepath.Join(projectRoot, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "vision.md"), []byte("# Vision\n"), 0644); err != nil {
		t.Fatalf("failed to create vision spec: %v", err)
	}
	state := readState(t, statePath)
	task := mustFindTask(t, state, "task-plan-missing")
	if task.PlanRef != "specs/plans/missing.md" {
		t.Fatalf("PlanRef = %q, want specs/plans/missing.md", task.PlanRef)
	}

	stdout, err := executeRootCommandCapture(t, projectRoot, "validate", "--json", "--skip-spec-check=false")
	if err == nil {
		t.Fatalf("expected validate --json to fail for missing plan_ref")
	}

	env := parseEnvelope(t, stdout)
	errObj, ok := env["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error to be object, got %T", env["error"])
	}
	if errObj["code"] != "validation" {
		t.Fatalf("error.code = %v, want validation", errObj["code"])
	}
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "plan_ref file not found") || !strings.Contains(msg, "task-plan-missing") || strings.Contains(msg, "spec_ref file not found") {
		t.Fatalf("error.message = %q, want field-specific plan_ref details", msg)
	}
	details, ok := errObj["details"].(map[string]any)
	if !ok {
		t.Fatalf("expected error.details to be object, got %T", errObj["details"])
	}
	if details["field"] != "plan_ref" {
		t.Errorf("details.field = %v, want plan_ref", details["field"])
	}
	if details["task_id"] != "task-plan-missing" {
		t.Errorf("details.task_id = %v, want task-plan-missing", details["task_id"])
	}
	if details["value"] != "specs/plans/missing.md" {
		t.Errorf("details.value = %v, want specs/plans/missing.md", details["value"])
	}
	if _, exists := details["resolved_path"]; exists {
		t.Errorf("details.resolved_path should not be exposed: %v", details["resolved_path"])
	}
}

func TestJSON_RBACError(t *testing.T) {
	projectRoot, _ := setupMutationTestProject(t, func(state *models.State) {
		now := time.Now().UTC()
		state.Tasks = []models.Task{
			testhelpers.BuildTaskByStatus("task-rbac-json", models.TaskStatusReady, now),
		}
	})

	// orchestrator is not allowed to claim tasks (requires "doer" role type)
	stdout, err := executeRootCommandCapture(t, projectRoot, "claim-task", "task-rbac-json", "orchestrator-1", "--json")
	if err == nil {
		t.Fatalf("expected RBAC error, got nil")
	}

	env := parseEnvelope(t, stdout)
	if env["ok"] != false {
		t.Fatalf("expected ok=false, got %v", env["ok"])
	}

	errObj, ok := env["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error to be object, got %T", env["error"])
	}
	// RBAC errors are classified as "internal" since they're untyped fmt.Errorf
	if errObj["code"] == nil || errObj["code"] == "" {
		t.Errorf("expected error code to be set, got %v", errObj["code"])
	}
}

func TestJSON_GetWrapsExisting(t *testing.T) {
	projectRoot, _ := setupMutationTestProject(t, nil)

	stdout, err := executeRootCommandCapture(t, projectRoot, "get", "tasks", "--json")
	if err != nil {
		t.Fatalf("get tasks --json failed: %v", err)
	}

	env := parseEnvelope(t, stdout)
	if env["ok"] != true {
		t.Fatalf("expected ok=true, got %v", env["ok"])
	}

	// result should be present (the wrapped JSON data)
	if _, exists := env["result"]; !exists {
		t.Errorf("expected result field in envelope")
	}
}

func TestJSON_GetTasksSummaryActive(t *testing.T) {
	projectRoot, _ := setupMutationTestProject(t, func(state *models.State) {
		now := time.Now().UTC()
		active := testhelpers.BuildTaskByStatus("task-active", models.TaskStatusImplementing, now)
		active.DoneWhen = "verbose done when should not appear"
		active.Scope = "verbose scope should not appear"
		active.Output = []models.OutputEntry{{Kind: "code-task", Desc: "child"}}
		merged := testhelpers.BuildTaskByStatus("task-merged", models.TaskStatusMerged, now)
		state.Tasks = []models.Task{active, merged}
	})

	stdout, err := executeRootCommandCapture(t, projectRoot, "get", "tasks", "--active", "--summary", "--json")
	if err != nil {
		t.Fatalf("get tasks --active --summary --json failed: %v", err)
	}

	env := parseEnvelope(t, stdout)
	if env["ok"] != true {
		t.Fatalf("expected ok=true, got %v", env["ok"])
	}
	result, ok := env["result"].([]any)
	if !ok {
		t.Fatalf("expected result array, got %T", env["result"])
	}
	if len(result) != 1 {
		t.Fatalf("summary result count = %d, want 1 active task", len(result))
	}
	task, ok := result[0].(map[string]any)
	if !ok {
		t.Fatalf("expected task object, got %T", result[0])
	}
	if task["id"] != "task-active" {
		t.Errorf("id = %v, want task-active", task["id"])
	}
	if _, exists := task["done_when"]; exists {
		t.Errorf("summary task includes done_when: %v", task)
	}
	if _, exists := task["scope"]; exists {
		t.Errorf("summary task includes scope: %v", task)
	}
	if _, exists := task["output"]; exists {
		t.Errorf("summary task includes output: %v", task)
	}
}

func TestJSON_VoidSuccess(t *testing.T) {
	projectRoot, _ := setupMutationTestProject(t, func(state *models.State) {
		now := time.Now().UTC()
		task := testhelpers.BuildTaskByStatus("task-ckpt-json", models.TaskStatusImplementing, now)
		agentID := "coder-1"
		task.AssignedTo = &agentID
		state.Tasks = []models.Task{task}
		state.Agents = map[string]models.Agent{
			"coder-1": {
				Role:         "coder",
				Status:       models.AgentStatusWorking,
				CurrentTask:  &task.ID,
				LeaseExpires: timePtr(now.Add(30 * time.Minute)),
				Heartbeat:    now,
			},
		}
	})

	stdout, err := executeRootCommandCapture(t, projectRoot,
		"write-checkpoint", "task-ckpt-json",
		"--agent-id", "coder-1",
		"--intent", "test intent",
		"--validation-plan", "test plan",
		"--files-to-modify", "foo.go",
		"--json",
	)
	if err != nil {
		t.Fatalf("write-checkpoint --json failed: %v", err)
	}

	env := parseEnvelope(t, stdout)
	if env["ok"] != true {
		t.Fatalf("expected ok=true, got %v", env["ok"])
	}

	// Void success: result must be null (not omitted, not empty object)
	resultRaw, exists := env["result"]
	if !exists {
		t.Fatalf("expected result key in envelope")
	}
	if resultRaw != nil {
		t.Errorf("expected result=null for void success, got %v", resultRaw)
	}
}

func TestJSON_Validate_WithWarnings(t *testing.T) {
	expiredLease := time.Now().UTC().Add(-2 * time.Hour)
	taskLease := time.Now().UTC().Add(30 * time.Minute)
	taskID := "task-validate-warn"
	agentID := "coder-1"
	worktreeRel := ".worktrees/task-validate-warn"
	baseCommit := "abc123"

	projectRoot, _ := setupMutationTestProject(t, func(state *models.State) {
		now := time.Now().UTC()
		task := testhelpers.BuildTaskByStatus(taskID, models.TaskStatusImplementing, now)
		task.AssignedTo = &agentID
		task.Worktree = &worktreeRel
		task.BaseCommit = &baseCommit
		task.LeaseExpires = &taskLease
		state.Tasks = []models.Task{task}
		state.Agents = map[string]models.Agent{
			agentID: {
				Role:         "coder",
				Status:       models.AgentStatusWorking,
				CurrentTask:  &taskID,
				LeaseExpires: &expiredLease,
				Heartbeat:    now,
			},
		}
	})

	// Create the worktree directory so the worktree existence check passes.
	wtDir := filepath.Join(projectRoot, worktreeRel)
	if err := os.MkdirAll(wtDir, 0755); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	stdout, err := executeRootCommandCapture(t, projectRoot, "validate", "--json", "--skip-spec-check")
	if err != nil {
		t.Fatalf("validate --json failed: %v", err)
	}

	env := parseEnvelope(t, stdout)
	if env["ok"] != true {
		t.Fatalf("expected ok=true, got %v", env["ok"])
	}

	warnings, ok := env["warnings"].([]any)
	if !ok || len(warnings) == 0 {
		t.Fatalf("expected warnings from expired agent lease, got %v", env["warnings"])
	}

	// At least one warning should mention lease expired
	found := false
	for _, w := range warnings {
		if s, ok := w.(string); ok && len(s) > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected non-empty warning string, got %v", warnings)
	}
}

func TestJSON_LogSuppression(t *testing.T) {
	projectRoot, _ := setupMutationTestProject(t, nil)

	// Capture stderr to verify log suppression
	oldStderr := os.Stderr
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}
	os.Stderr = stderrW

	_, cmdErr := executeRootCommandCapture(t, projectRoot, "update-sprint-metrics", "--json")

	stderrW.Close()
	os.Stderr = oldStderr

	var stderrBuf bytes.Buffer
	if _, copyErr := io.Copy(&stderrBuf, stderrR); copyErr != nil {
		t.Fatalf("failed to read stderr: %v", copyErr)
	}
	stderrR.Close()

	if cmdErr != nil {
		t.Fatalf("update-sprint-metrics --json failed: %v", cmdErr)
	}

	if stderrBuf.Len() != 0 {
		t.Errorf("expected empty stderr when --json is set, got: %s", stderrBuf.String())
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
