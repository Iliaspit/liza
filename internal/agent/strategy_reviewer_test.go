package agent

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/testhelpers"
)

// TestReviewerPreWork_ExecutesAutoTransitions verifies that reviewer PreWork
// executes auto transitions (e.g., integration-to-fix) for merged tasks,
// creating child fix tasks with DRAFT_CODE status and coding-pair role_pair.
func TestReviewerPreWork_ExecutesAutoTransitions(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2

	// Integration-analyst task: MERGED (goes through merge path like all tasks).
	reviewCommit := "abc123"
	mergeCommit := "def456"
	parentID := "integration-task-1"
	task := models.Task{
		ID:           parentID,
		Type:         models.TaskTypeIntegration,
		RolePair:     "integration-pair",
		Description:  "Integration analysis for goal",
		Status:       models.TaskStatusMerged,
		Priority:     1,
		Created:      now,
		SpecRef:      "specs/goals/test.md",
		DoneWhen:     "Analysis approved",
		Scope:        "full branch",
		ReviewCommit: &reviewCommit,
		MergeCommit:  &mergeCommit,
		Output: []models.OutputEntry{
			{Desc: "Fix type alignment in auth", DoneWhen: "Types match across modules", Scope: "internal/auth", SpecRef: "specs/goals/test.md"},
			{Desc: "Fix error mapping in handler", DoneWhen: "All errors propagated", Scope: "internal/handler", SpecRef: "specs/goals/test.md"},
		},
		History: []models.TaskHistoryEntry{},
	}

	state.Tasks = []models.Task{task}
	state.Sprint.Scope.Planned = []string{parentID}
	testhelpers.WriteInitialState(t, statePath, state)

	bb := db.New(statePath)
	resolver := testResolver(t)
	s, err := NewRoleStrategy("code-reviewer", resolver)
	if err != nil {
		t.Fatalf("NewRoleStrategy() error = %v", err)
	}

	shouldContinue, err := s.PreWork(context.Background(), bb, SupervisorConfig{ProjectRoot: tmpDir})
	if err != nil {
		t.Fatalf("PreWork() error = %v", err)
	}
	if shouldContinue {
		t.Error("PreWork() shouldContinue = true, want false")
	}

	// Read final state
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	// Verify: parent is in MERGED state (integration tasks now go through merge)
	parent := readState.FindTask(parentID)
	if parent == nil {
		t.Fatal("Parent task not found")
	}
	if parent.Status != models.TaskStatusMerged {
		t.Errorf("Parent status = %q, want MERGED", parent.Status)
	}

	// Verify: child fix tasks were created
	if !parent.TransitionsExecuted["integration-to-fix"] {
		t.Fatalf("Parent TransitionsExecuted missing 'integration-to-fix': %v", parent.TransitionsExecuted)
	}

	// Find children (should be 2, one per output entry)
	var children []*models.Task
	for i := range readState.Tasks {
		if readState.Tasks[i].ID != parentID {
			children = append(children, &readState.Tasks[i])
		}
	}
	if len(children) != 2 {
		t.Fatalf("Children count = %d, want 2", len(children))
	}

	for i, child := range children {
		if child.Status != "DRAFT_CODE" {
			t.Errorf("Child[%d] status = %q, want DRAFT_CODE", i, child.Status)
		}
		if child.RolePair != "coding-pair" {
			t.Errorf("Child[%d] role_pair = %q, want coding-pair", i, child.RolePair)
		}
		if !slices.Contains(readState.Sprint.Scope.Planned, child.ID) {
			t.Errorf("Child %q not in Sprint.Scope.Planned", child.ID)
		}
	}
}

// TestReviewerPreWork_DoesNotExecuteManualTransitions verifies that reviewer
// PreWork does NOT execute manual transitions. Manual transitions (e.g.,
// code-plan-to-coding) remain gated by the orchestrator checkpoint flow.
func TestReviewerPreWork_DoesNotExecuteManualTransitions(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2

	// Code-planning task: MERGED with output (manual transition available).
	reviewCommit := "abc123"
	mergeCommit := "def456"
	parentID := "planning-task-1"
	task := models.Task{
		ID:           parentID,
		Type:         models.TaskTypeCoding,
		RolePair:     "code-planning-pair",
		Description:  "Plan implementation of feature X",
		Status:       models.TaskStatusMerged,
		Priority:     1,
		Created:      now,
		SpecRef:      "specs/goals/test.md",
		DoneWhen:     "Plan approved",
		Scope:        "internal/pkg",
		ReviewCommit: &reviewCommit,
		MergeCommit:  &mergeCommit,
		Output: []models.OutputEntry{
			{Desc: "Implement feature X", DoneWhen: "Tests pass", Scope: "internal/pkg", SpecRef: "specs/goals/test.md"},
		},
		History: []models.TaskHistoryEntry{},
	}

	state.Tasks = []models.Task{task}
	state.Sprint.Scope.Planned = []string{parentID}
	testhelpers.WriteInitialState(t, statePath, state)

	bb := db.New(statePath)
	resolver := testResolver(t)
	s, err := NewRoleStrategy("code-reviewer", resolver)
	if err != nil {
		t.Fatalf("NewRoleStrategy() error = %v", err)
	}

	shouldContinue, err := s.PreWork(context.Background(), bb, SupervisorConfig{ProjectRoot: tmpDir})
	if err != nil {
		t.Fatalf("PreWork() error = %v", err)
	}
	if shouldContinue {
		t.Error("PreWork() shouldContinue = true, want false")
	}

	// Read final state
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	// Verify: parent is still MERGED
	parent := readState.FindTask(parentID)
	if parent == nil {
		t.Fatal("Parent task not found")
	}
	if parent.Status != models.TaskStatusMerged {
		t.Errorf("Parent status = %q, want MERGED", parent.Status)
	}

	// Verify: NO child tasks created (manual transitions not fired by reviewer)
	if len(readState.Tasks) != 1 {
		t.Errorf("Task count = %d, want 1 (no children should be created from manual transitions)", len(readState.Tasks))
	}

	// Verify: TransitionsExecuted should NOT include code-plan-to-coding
	if parent.TransitionsExecuted["code-plan-to-coding"] {
		t.Error("code-plan-to-coding should NOT be in TransitionsExecuted (manual transitions not fired by reviewer)")
	}
}

func TestReviewerClaimTask_InitialTaskClaimsSpecifiedReviewTask(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Agents["code-reviewer-1"] = testhelpers.RegisteredTestAgent(models.RoleCodeReviewer)

	lowPriority := testhelpers.BuildTaskByStatus("task-low", models.TaskStatusReadyForReview, now)
	lowPriority.Priority = 3
	lowPriority.Worktree = nil
	highPriority := testhelpers.BuildTaskByStatus("task-high", models.TaskStatusReadyForReview, now)
	highPriority.Priority = 1
	highPriority.Worktree = nil
	state.Tasks = []models.Task{lowPriority, highPriority}
	testhelpers.WriteInitialState(t, statePath, state)

	wtPath := filepath.Join(tmpDir, paths.WorktreesDirName, "task-low")
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", wtPath, err)
	}

	resolver := testResolver(t)
	strategy, err := NewRoleStrategy(models.RoleCodeReviewer, resolver)
	if err != nil {
		t.Fatalf("NewRoleStrategy() error = %v", err)
	}

	taskID, claimedTaskID, err := strategy.ClaimTask(SupervisorConfig{
		AgentID:     "code-reviewer-1",
		Role:        models.RoleCodeReviewer,
		ProjectRoot: tmpDir,
		StatePath:   statePath,
		InitialTask: "task-low",
	}, db.New(statePath))
	if err != nil {
		t.Fatalf("ClaimTask() error = %v", err)
	}
	if taskID != "task-low" || claimedTaskID != "" {
		t.Fatalf("ClaimTask() = (%q, %q), want (task-low, empty claimed ID)", taskID, claimedTaskID)
	}

	readState, err := db.New(statePath).Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	low := readState.FindTask("task-low")
	if low == nil {
		t.Fatal("task-low not found")
	}
	if low.Status != models.TaskStatusReviewing {
		t.Fatalf("task-low status = %s, want %s", low.Status, models.TaskStatusReviewing)
	}
	high := readState.FindTask("task-high")
	if high == nil {
		t.Fatal("task-high not found")
	}
	if high.Status != models.TaskStatusReadyForReview {
		t.Fatalf("task-high status = %s, want %s", high.Status, models.TaskStatusReadyForReview)
	}
}

func TestEnsureReviewerPromptClaimed_RejectsSubmittedUnclaimedTask(t *testing.T) {
	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-stale", models.TaskStatusReadyForReview, now),
	}

	resolver := testResolver(t)
	err := ensureReviewerPromptClaimed(state, "code-reviewer-1", "task-stale", resolver)
	if err == nil {
		t.Fatal("ensureReviewerPromptClaimed() error = nil, want stale submitted task rejection")
	}
	if !strings.Contains(err.Error(), "requires reviewing state") {
		t.Fatalf("ensureReviewerPromptClaimed() error = %v, want reviewing-state rejection", err)
	}
}
