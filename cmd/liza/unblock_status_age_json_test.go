package main

import (
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestJSON_UnblockTaskResetsTimeInStatus(t *testing.T) {
	const taskID = "task-unblock-status-age"
	now := time.Now().UTC()
	projectRoot, _ := setupMutationTestProject(t, func(state *models.State) {
		task := testhelpers.BuildTaskByStatus(taskID, models.TaskStatusBlocked, now.Add(-2*time.Hour))
		task.RolePair = "code-planning-pair"
		task.Worktree = nil
		task.BaseCommit = nil
		task.History = []models.TaskHistoryEntry{
			{Time: now.Add(-2 * time.Hour), Event: models.TaskEventBlocked},
		}
		state.Tasks = []models.Task{task}
	})

	if err := executeRootCommand(
		t,
		projectRoot,
		"unblock-task",
		taskID,
		"--reason",
		"repair verified",
		"--agent-id",
		"orchestrator-1",
	); err != nil {
		t.Fatalf("unblock-task execute failed: %v", err)
	}

	stdout, err := executeRootCommandCapture(t, projectRoot, "get", taskID, "--json")
	if err != nil {
		t.Fatalf("get %s --json failed: %v", taskID, err)
	}

	env := parseEnvelope(t, stdout)
	if env["ok"] != true {
		t.Fatalf("expected ok=true, got %v", env["ok"])
	}
	result, ok := env["result"].(map[string]any)
	if !ok {
		t.Fatalf("result = %T, want task object", env["result"])
	}
	if result["status"] != string(models.TaskStatusDraftCodingPlan) {
		t.Fatalf("status = %v, want %s", result["status"], models.TaskStatusDraftCodingPlan)
	}
	if assignedTo, exists := result["assigned_to"]; exists {
		t.Fatalf("assigned_to = %v, want no assignee", assignedTo)
	}

	timeInStatus, ok := result["time_in_status"].(string)
	if !ok {
		t.Fatalf("time_in_status = %T, want duration string", result["time_in_status"])
	}
	statusAge, err := time.ParseDuration(timeInStatus)
	if err != nil {
		t.Fatalf("time_in_status = %q, want sub-minute duration: %v", timeInStatus, err)
	}
	if statusAge < 0 || statusAge >= time.Minute {
		t.Fatalf("time_in_status = %s, want fresh age under one minute", statusAge)
	}
}
