package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/testhelpers"
)

// TestAutoResumeAction tests the pure decision function for auto-resume.
func TestAutoResumeAction(t *testing.T) {
	tests := []struct {
		name       string
		autoResume bool
		status     models.SprintStatus
		want       models.SprintStatus
	}{
		{"off_checkpoint", false, models.SprintStatusCheckpoint, ""},
		{"off_completed", false, models.SprintStatusCompleted, ""},
		{"on_checkpoint", true, models.SprintStatusCheckpoint, models.SprintStatusCheckpoint},
		{"on_completed", true, models.SprintStatusCompleted, models.SprintStatusCompleted},
		{"on_in_progress", true, models.SprintStatusInProgress, ""},
		{"on_empty", true, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.State{
				Config: models.Config{AutoResume: tt.autoResume},
				Sprint: models.Sprint{Status: tt.status},
			}
			got := autoResumeAction(state)
			if got != tt.want {
				t.Errorf("autoResumeAction() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckpointBlocksRole(t *testing.T) {
	tests := []struct {
		name     string
		trigger  string
		roleType string
		wantWait bool
	}{
		{"planning checkpoint allows doer", models.CheckpointTriggerPlanningComplete, "doer", false},
		{"planning checkpoint allows reviewer", models.CheckpointTriggerPlanningComplete, "reviewer", false},
		{"many-to-one checkpoint allows doer", models.CheckpointTriggerManyToOneReady, "doer", false},
		{"many-to-one checkpoint allows reviewer", models.CheckpointTriggerManyToOneReady, "reviewer", false},
		{"planning checkpoint blocks orchestrator", models.CheckpointTriggerPlanningComplete, "orchestrator", true},
		{"many-to-one checkpoint blocks orchestrator", models.CheckpointTriggerManyToOneReady, "orchestrator", true},
		{"planning checkpoint blocks unknown role", models.CheckpointTriggerPlanningComplete, "observer", true},
		{"manual checkpoint blocks doer", "", "doer", true},
		{"manual checkpoint blocks reviewer", "", "reviewer", true},
		{"manual checkpoint blocks orchestrator", "", "orchestrator", true},
		{"sprint-complete checkpoint blocks doer", models.CheckpointTriggerSprintComplete, "doer", true},
		{"sprint-complete checkpoint blocks reviewer", models.CheckpointTriggerSprintComplete, "reviewer", true},
		{"sprint-complete checkpoint blocks orchestrator", models.CheckpointTriggerSprintComplete, "orchestrator", true},
		{"non-checkpoint does not block unknown role", models.CheckpointTriggerPlanningComplete, "observer", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := models.SprintStatusCheckpoint
			if tt.name == "non-checkpoint does not block unknown role" {
				status = models.SprintStatusInProgress
			}
			state := &models.State{
				Sprint: models.Sprint{
					Status:            status,
					CheckpointTrigger: tt.trigger,
				},
			}
			gotWait, _ := checkpointBlocksRole(state, tt.roleType)
			if gotWait != tt.wantWait {
				t.Errorf("checkpointBlocksRole() wait = %v, want %v", gotWait, tt.wantWait)
			}
		})
	}
}

func TestWaitWhilePausedAutoResumePrecedesTransitionCheckpointRoleException(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Config.AutoResume = true
	state.Sprint.Status = models.SprintStatusCheckpoint
	state.Sprint.CheckpointTrigger = models.CheckpointTriggerPlanningComplete
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, time.Now().UTC()),
	}
	state.Sprint.Scope.Planned = []string{"task-1"}
	testhelpers.WriteInitialState(t, statePath, state)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := waitWhilePaused(ctx, tmpDir, "doer"); err != nil {
		t.Fatalf("waitWhilePaused() error = %v", err)
	}

	updated, err := db.For(statePath).Read()
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if updated.Sprint.Status != models.SprintStatusInProgress {
		t.Fatalf("sprint status = %s, want %s", updated.Sprint.Status, models.SprintStatusInProgress)
	}
}

func TestWaitWhilePausedHardModesBlockTransitionCheckpointRoleException(t *testing.T) {
	tests := []struct {
		name string
		mode models.SystemMode
	}{
		{"paused mode", models.SystemModePaused},
		{"circuit breaker mode", models.SystemModeCircuitBreakerTripped},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

			state := testhelpers.CreateValidState()
			state.Config.Mode = tt.mode
			state.Sprint.Status = models.SprintStatusCheckpoint
			state.Sprint.CheckpointTrigger = models.CheckpointTriggerPlanningComplete
			testhelpers.WriteInitialState(t, statePath, state)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()

			if err := waitWhilePaused(ctx, tmpDir, "doer"); err == nil {
				t.Fatal("waitWhilePaused() error = nil, want context timeout while hard mode blocks")
			}
		})
	}
}

// TestIsGoalComplete tests the pure decision function for goal completion detection.
func TestIsGoalComplete(t *testing.T) {
	tests := []struct {
		name   string
		result *ops.ResumeResult
		want   bool
	}{
		{
			name: "goal_complete",
			result: &ops.ResumeResult{
				SprintAdvanced:      &ops.AdvanceSprintResult{CarriedTasks: nil},
				TransitionsExecuted: 0,
				TransitionError:     "",
			},
			want: true,
		},
		{
			name: "carried_tasks_remain",
			result: &ops.ResumeResult{
				SprintAdvanced:      &ops.AdvanceSprintResult{CarriedTasks: []string{"task-1"}},
				TransitionsExecuted: 0,
				TransitionError:     "",
			},
			want: false,
		},
		{
			name: "transitions_executed",
			result: &ops.ResumeResult{
				SprintAdvanced:      &ops.AdvanceSprintResult{CarriedTasks: nil},
				TransitionsExecuted: 2,
				TransitionError:     "",
			},
			want: false,
		},
		{
			name: "transition_error_not_goal_complete",
			result: &ops.ResumeResult{
				SprintAdvanced:      &ops.AdvanceSprintResult{CarriedTasks: nil},
				TransitionsExecuted: 0,
				TransitionError:     "failed to load pipeline config",
			},
			want: false,
		},
		{
			name: "no_sprint_advance",
			result: &ops.ResumeResult{
				SprintAdvanced:      nil,
				TransitionsExecuted: 0,
				TransitionError:     "",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isGoalComplete(tt.result)
			if got != tt.want {
				t.Errorf("isGoalComplete() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsSystemStopped tests the isSystemStopped helper function
func TestIsSystemStopped(t *testing.T) {
	tests := []struct {
		name         string
		stateMode    models.SystemMode
		wantStopped  bool
		wantReasonRe string
	}{
		{
			name:         "state-based STOPPED mode",
			stateMode:    models.SystemModeStopped,
			wantStopped:  true,
			wantReasonRe: "STOPPED",
		},
		{
			name:        "not stopped",
			stateMode:   models.SystemModeRunning,
			wantStopped: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := testhelpers.CreateValidState()
			state.Config.Mode = tt.stateMode

			stopped, reason := isSystemStopped(state)

			if stopped != tt.wantStopped {
				t.Errorf("isSystemStopped() stopped = %v, want %v", stopped, tt.wantStopped)
			}

			if tt.wantStopped && tt.wantReasonRe != "" && !strings.Contains(reason, tt.wantReasonRe) {
				t.Errorf("isSystemStopped() reason = %q, should contain %q", reason, tt.wantReasonRe)
			}

			if !tt.wantStopped && reason != "" {
				t.Errorf("isSystemStopped() reason should be empty when not stopped, got %q", reason)
			}
		})
	}
}

// TestVerifyOrchestratorStateChanges_BlockedNotResolved verifies that
// the orchestrator validation accepts when blocked tasks remain unchanged (no-op exit)
func TestVerifyOrchestratorStateChanges_BlockedNotResolved(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	now := time.Now().UTC()

	// State before: task is BLOCKED
	stateBefore := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Agents: map[string]models.Agent{
			"orchestrator-1": {Role: "orchestrator", Status: models.AgentStatusPlanning, Heartbeat: now},
		},
		Tasks: []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
		},
		Config: models.Config{IntegrationBranch: "main"},
	}

	// State after: task STILL BLOCKED (orchestrator couldn't resolve)
	stateAfter := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Agents: map[string]models.Agent{
			"orchestrator-1": {Role: "orchestrator", Status: models.AgentStatusIdle, Heartbeat: now},
		},
		Tasks: []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
		},
		Config: models.Config{IntegrationBranch: "main"},
	}

	testhelpers.WriteInitialState(t, statePath, stateAfter)

	bb := db.New(statePath)

	err := verifyOrchestratorStateChanges(bb, stateBefore, nil, nil, nil)
	if err != nil {
		t.Errorf("Expected no error for no-op BLOCKED exit (may require human intervention), got: %v", err)
	}
}

// TestVerifyOrchestratorStateChanges_HypothesisExhaustedNotResolved verifies that
// the orchestrator validation rejects when exhausted tasks remain claimable.
func TestVerifyOrchestratorStateChanges_HypothesisExhaustedNotResolved(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	now := time.Now().UTC()

	// State before: task has 2+ failed_by (hypothesis exhausted)
	exhaustedTask := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
	exhaustedTask.FailedBy = []string{"coder-1", "coder-2"}
	stateBefore := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Agents: map[string]models.Agent{
			"orchestrator-1": {Role: "orchestrator", Status: models.AgentStatusPlanning, Heartbeat: now},
		},
		Tasks:  []models.Task{exhaustedTask},
		Config: models.Config{IntegrationBranch: "main"},
	}

	// State after: task STILL exhausted (orchestrator couldn't resolve)
	exhaustedTaskAfter := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
	exhaustedTaskAfter.FailedBy = []string{"coder-1", "coder-2"}
	stateAfter := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Agents: map[string]models.Agent{
			"orchestrator-1": {Role: "orchestrator", Status: models.AgentStatusIdle, Heartbeat: now},
		},
		Tasks:  []models.Task{exhaustedTaskAfter},
		Config: models.Config{IntegrationBranch: "main"},
	}

	testhelpers.WriteInitialState(t, statePath, stateAfter)

	bb := db.New(statePath)

	err := verifyOrchestratorStateChanges(bb, stateBefore, nil, nil, nil)
	if err == nil {
		t.Fatal("Expected error for no-op HYPOTHESIS_EXHAUSTED exit, got nil")
	}
	if !strings.Contains(err.Error(), "unresolved exhausted count didn't decrease") {
		t.Fatalf("error = %v, want unresolved exhausted count message", err)
	}
}

func TestVerifyOrchestratorStateChanges_HypothesisExhaustedBlocked(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	now := time.Now().UTC()

	exhaustedTask := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
	exhaustedTask.FailedBy = []string{"coder-1", "coder-2"}
	stateBefore := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Agents: map[string]models.Agent{
			"orchestrator-1": {Role: "orchestrator", Status: models.AgentStatusPlanning, Heartbeat: now},
		},
		Tasks:  []models.Task{exhaustedTask},
		Config: models.Config{IntegrationBranch: "main"},
	}

	blockedTask := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
	blockedTask.FailedBy = []string{"coder-1", "coder-2"}
	stateAfter := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Agents: map[string]models.Agent{
			"orchestrator-1": {Role: "orchestrator", Status: models.AgentStatusIdle, Heartbeat: now},
		},
		Tasks:  []models.Task{blockedTask},
		Config: models.Config{IntegrationBranch: "main"},
	}

	testhelpers.WriteInitialState(t, statePath, stateAfter)

	bb := db.New(statePath)

	err := verifyOrchestratorStateChanges(bb, stateBefore, nil, nil, nil)
	if err != nil {
		t.Errorf("Expected no error for blocked HYPOTHESIS_EXHAUSTED exit, got: %v", err)
	}
}

// TestVerifyOrchestratorStateChanges_CodingCompleteNoIntegration verifies that
// CODING_COMPLETE wake rejects when no integration-pair task was created.
func TestVerifyOrchestratorStateChanges_CodingCompleteNoIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	now := time.Now().UTC()
	baseCommit := "abc123"

	// State before: all tasks terminal, base_commit set, no integration task
	stateBefore := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
			BaseCommit:  &baseCommit,
		},
		Agents: map[string]models.Agent{
			"orchestrator-1": {Role: "orchestrator", Status: models.AgentStatusPlanning, Heartbeat: now},
		},
		Sprint: models.Sprint{
			Number: 1,
			Status: models.SprintStatusInProgress,
			Scope:  models.SprintScope{Planned: []string{"task-1"}},
		},
		Tasks: []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
		},
		Config: models.Config{IntegrationBranch: "main"},
	}

	// State after: still no integration-pair task (orchestrator failed to create one)
	stateAfter := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
			BaseCommit:  &baseCommit,
		},
		Agents: map[string]models.Agent{
			"orchestrator-1": {Role: "orchestrator", Status: models.AgentStatusIdle, Heartbeat: now},
		},
		Sprint: models.Sprint{
			Number: 1,
			Status: models.SprintStatusInProgress,
			Scope:  models.SprintScope{Planned: []string{"task-1"}},
		},
		Tasks: []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
		},
		Config: models.Config{IntegrationBranch: "main"},
	}

	testhelpers.WriteInitialState(t, statePath, stateAfter)

	bb := db.New(statePath)

	err := verifyOrchestratorStateChanges(bb, stateBefore, nil, nil, nil)
	if err == nil {
		t.Error("Expected error when CODING_COMPLETE trigger but no integration-pair task created")
	}
	if err != nil && !strings.Contains(err.Error(), "integration-pair") {
		t.Errorf("Expected error mentioning integration-pair, got: %v", err)
	}
}

// TestVerifyOrchestratorStateChanges_ManyToOneReady verifies that
// MANY_TO_ONE_READY trigger passes verification when sprint is checkpointed.
func TestVerifyOrchestratorStateChanges_ManyToOneReady(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	checkpointAt := now
	parentID := "epic-1"

	m2oTransitions := []ops.ManyToOneTransitionInfo{
		{Name: "us-to-coding", SourceRolePair: "us-writing-pair"},
	}

	// Build MERGED us-writing-pair tasks sharing a parent
	us1 := testhelpers.BuildTaskByStatus("us-1", models.TaskStatusMerged, now)
	us1.RolePair = "us-writing-pair"
	us1.ParentTask = &parentID
	us2 := testhelpers.BuildTaskByStatus("us-2", models.TaskStatusMerged, now)
	us2.RolePair = "us-writing-pair"
	us2.ParentTask = &parentID

	// State before: complete m2o cohort, sprint in progress
	stateBefore := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Agents: map[string]models.Agent{
			"orchestrator-1": {Role: "orchestrator", Status: models.AgentStatusPlanning, Heartbeat: now},
		},
		Sprint: models.Sprint{
			Number: 1,
			Status: models.SprintStatusInProgress,
			Scope:  models.SprintScope{Planned: []string{"us-1", "us-2"}},
		},
		Tasks:  []models.Task{us1, us2},
		Config: models.Config{IntegrationBranch: "main"},
	}

	// State after: sprint checkpointed (orchestrator did its job)
	stateAfter := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Agents: map[string]models.Agent{
			"orchestrator-1": {Role: "orchestrator", Status: models.AgentStatusIdle, Heartbeat: now},
		},
		Sprint: models.Sprint{
			Number:   1,
			Status:   models.SprintStatusCheckpoint,
			Scope:    models.SprintScope{Planned: []string{"us-1", "us-2"}},
			Timeline: models.SprintTimeline{CheckpointAt: &checkpointAt},
		},
		Tasks:  []models.Task{us1, us2},
		Config: models.Config{IntegrationBranch: "main"},
	}

	testhelpers.WriteInitialState(t, statePath, stateAfter)
	bb := db.New(statePath)

	// Pipeline terminals: MERGED is terminal for this test
	pipelineTerminals := []models.TaskStatus{models.TaskStatusMerged}

	err := verifyOrchestratorStateChanges(bb, stateBefore, pipelineTerminals, nil, m2oTransitions)
	if err != nil {
		t.Errorf("Expected no error when sprint checkpointed for MANY_TO_ONE_READY trigger, got: %v", err)
	}
}

// TestSelfHealCheckpoint_SprintComplete verifies that selfHealCheckpoint
// creates a checkpoint when the orchestrator agent failed to do so.
func TestSelfHealCheckpoint_SprintComplete(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	now := time.Now().UTC()
	state := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Sprint: models.Sprint{
			ID:     "sprint-1",
			Number: 1,
			Status: models.SprintStatusInProgress,
			Scope:  models.SprintScope{Planned: []string{"task-1"}},
			Timeline: models.SprintTimeline{
				Started:  now.Add(-1 * time.Hour),
				Deadline: now.Add(1 * time.Hour),
			},
		},
		Tasks: []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
		},
		Config:         models.Config{IntegrationBranch: "main"},
		CircuitBreaker: models.CircuitBreaker{Status: "OK"},
	}

	testhelpers.WriteInitialState(t, statePath, state)

	healed := selfHealCheckpoint(tmpDir, WakeTriggerSprintComplete)
	if !healed {
		t.Fatal("Expected selfHealCheckpoint to succeed for SPRINT_COMPLETE")
	}

	// Verify sprint is now at CHECKPOINT
	bb := db.New(statePath)
	after, err := bb.Read()
	if err != nil {
		t.Fatal(err)
	}
	if after.Sprint.Status != models.SprintStatusCheckpoint {
		t.Errorf("Sprint status = %s, want CHECKPOINT", after.Sprint.Status)
	}
}

// TestSelfHealCheckpoint_AlreadyCheckpointed verifies that selfHealCheckpoint
// returns true when the sprint is already at CHECKPOINT.
func TestSelfHealCheckpoint_AlreadyCheckpointed(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	now := time.Now().UTC()
	checkpointAt := now.Add(-10 * time.Second)
	state := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Sprint: models.Sprint{
			ID:     "sprint-1",
			Number: 1,
			Status: models.SprintStatusCheckpoint,
			Scope:  models.SprintScope{Planned: []string{"task-1"}},
			Timeline: models.SprintTimeline{
				Started:      now.Add(-1 * time.Hour),
				Deadline:     now.Add(1 * time.Hour),
				CheckpointAt: &checkpointAt,
			},
		},
		Tasks: []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
		},
		Config:         models.Config{IntegrationBranch: "main"},
		CircuitBreaker: models.CircuitBreaker{Status: "OK"},
	}

	testhelpers.WriteInitialState(t, statePath, state)

	healed := selfHealCheckpoint(tmpDir, WakeTriggerSprintComplete)
	if !healed {
		t.Fatal("Expected selfHealCheckpoint to return true for already-checkpointed sprint")
	}
}

// TestSelfHealCheckpoint_ManyToOneReady verifies that selfHealCheckpoint
// handles the MANY_TO_ONE_READY trigger (same checkpoint-only pattern).
func TestSelfHealCheckpoint_ManyToOneReady(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	now := time.Now().UTC()
	state := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Sprint: models.Sprint{
			ID:     "sprint-1",
			Number: 1,
			Status: models.SprintStatusInProgress,
			Scope:  models.SprintScope{Planned: []string{"task-1"}},
			Timeline: models.SprintTimeline{
				Started:  now.Add(-1 * time.Hour),
				Deadline: now.Add(1 * time.Hour),
			},
		},
		Tasks: []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
		},
		Config:         models.Config{IntegrationBranch: "main"},
		CircuitBreaker: models.CircuitBreaker{Status: "OK"},
	}

	testhelpers.WriteInitialState(t, statePath, state)

	healed := selfHealCheckpoint(tmpDir, WakeTriggerManyToOneReady)
	if !healed {
		t.Fatal("Expected selfHealCheckpoint to succeed for MANY_TO_ONE_READY")
	}

	bb := db.New(statePath)
	after, err := bb.Read()
	if err != nil {
		t.Fatal(err)
	}
	if after.Sprint.Status != models.SprintStatusCheckpoint {
		t.Errorf("Sprint status = %s, want CHECKPOINT", after.Sprint.Status)
	}
}

// TestSelfHealCheckpoint_NonMechanicalTrigger verifies that selfHealCheckpoint
// does nothing for triggers that require LLM creativity.
func TestSelfHealCheckpoint_NonMechanicalTrigger(t *testing.T) {
	nonMechanical := []OrchestratorWakeTrigger{
		WakeTriggerInitialPlanning,
		WakeTriggerBlocked,
		WakeTriggerHypothesisExhausted,
		WakeTriggerImmediateDiscovery,
		WakeTriggerCodingComplete,
		WakeTriggerNone,
	}

	for _, trigger := range nonMechanical {
		t.Run(string(trigger), func(t *testing.T) {
			// projectRoot doesn't matter — function should return false before touching disk
			if selfHealCheckpoint("/nonexistent", trigger) {
				t.Errorf("Expected selfHealCheckpoint to return false for trigger %s", trigger)
			}
		})
	}
}

// TestOrchestratorProgressSignature verifies signature changes when state changes.
func TestOrchestratorProgressSignature(t *testing.T) {
	now := time.Now().UTC()

	base := &models.State{
		Sprint: models.Sprint{
			Status: models.SprintStatusInProgress,
			Number: 1,
			Scope:  models.SprintScope{Planned: []string{"task-1"}},
		},
		Tasks: []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
		},
	}

	baseSig := orchestratorProgressSignature(base)

	// Same state → same signature
	sameSig := orchestratorProgressSignature(base)
	if sameSig != baseSig {
		t.Errorf("Same state should produce same signature: got %q vs %q", sameSig, baseSig)
	}

	// Sprint status change → different signature
	withCheckpoint := &models.State{
		Sprint: models.Sprint{
			Status: models.SprintStatusCheckpoint,
			Number: 1,
			Scope:  models.SprintScope{Planned: []string{"task-1"}},
		},
		Tasks: []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
		},
	}
	if orchestratorProgressSignature(withCheckpoint) == baseSig {
		t.Error("Sprint status change should produce different signature")
	}

	// Sprint number change → different signature
	withNewSprint := &models.State{
		Sprint: models.Sprint{
			Status: models.SprintStatusInProgress,
			Number: 2,
			Scope:  models.SprintScope{Planned: []string{"task-1"}},
		},
		Tasks: []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
		},
	}
	if orchestratorProgressSignature(withNewSprint) == baseSig {
		t.Error("Sprint number change should produce different signature")
	}

	// Task count change → different signature
	withMoreTasks := &models.State{
		Sprint: models.Sprint{
			Status: models.SprintStatusInProgress,
			Number: 1,
			Scope:  models.SprintScope{Planned: []string{"task-1"}},
		},
		Tasks: []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
			testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReady, now),
		},
	}
	if orchestratorProgressSignature(withMoreTasks) == baseSig {
		t.Error("Task count change should produce different signature")
	}

	// Planned count change → different signature
	withMorePlanned := &models.State{
		Sprint: models.Sprint{
			Status: models.SprintStatusInProgress,
			Number: 1,
			Scope:  models.SprintScope{Planned: []string{"task-1", "task-2"}},
		},
		Tasks: []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
		},
	}
	if orchestratorProgressSignature(withMorePlanned) == baseSig {
		t.Error("Planned count change should produce different signature")
	}

	// Task status change (same count) → different signature
	// This catches the blocker: resolving a blocked task changes status distribution
	withBlockedResolved := &models.State{
		Sprint: models.Sprint{
			Status: models.SprintStatusInProgress,
			Number: 1,
			Scope:  models.SprintScope{Planned: []string{"task-1"}},
		},
		Tasks: []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
		},
	}
	if orchestratorProgressSignature(withBlockedResolved) == baseSig {
		t.Error("Task status distribution change should produce different signature")
	}

	// Discovery count change → different signature
	withDiscovery := &models.State{
		Sprint: models.Sprint{
			Status: models.SprintStatusInProgress,
			Number: 1,
			Scope:  models.SprintScope{Planned: []string{"task-1"}},
		},
		Tasks: []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
		},
		Discovered: []models.Discovery{
			{ID: "disc-1", Urgency: "immediate"},
		},
	}
	if orchestratorProgressSignature(withDiscovery) == baseSig {
		t.Error("Discovery count change should produce different signature")
	}
}

// TestOrchestratorSpinningTracker verifies spinning detection for orchestrator.
func TestOrchestratorSpinningTracker(t *testing.T) {
	tracker := newSpinningTracker()
	sig := "sprint:IN_PROGRESS:1:tasks:3:planned:3"

	// Same signature N times → count increases
	for i := 1; i <= 5; i++ {
		count := tracker.Track("orchestrator", sig)
		if count != i {
			t.Errorf("Track() = %d, want %d", count, i)
		}
	}

	// Different signature → resets
	count := tracker.Track("orchestrator", "sprint:CHECKPOINT:1:tasks:3:planned:3")
	if count != 1 {
		t.Errorf("Track() after signature change = %d, want 1", count)
	}
}
