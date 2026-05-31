package ops

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	gitpkg "github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

// setupPipelineTest creates a test directory with a frozen pipeline config and a valid state.
// Returns (projectRoot, stateFile) paths.
func setupPipelineTest(t *testing.T) (string, string) {
	t.Helper()
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	// Copy the valid pipeline YAML to .liza/pipeline.yaml (frozen config).
	src, err := os.ReadFile(filepath.Join(testhelpers.FindRepoRoot(t), "internal", "pipeline", "testdata", "valid-coding-subpipeline.yaml"))
	if err != nil {
		t.Fatalf("Failed to read pipeline testdata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".liza", "pipeline.yaml"), src, 0644); err != nil {
		t.Fatalf("Failed to write frozen pipeline config: %v", err)
	}

	return tmpDir, stateFile
}

func TestLoadDetectionContext_PipelineGoal(t *testing.T) {
	tmpDir, _ := setupPipelineTest(t)
	ctx, err := LoadDetectionContext(tmpDir)
	if err != nil {
		t.Fatalf("LoadDetectionContext() error: %v", err)
	}
	if len(ctx.SprintTerminals) == 0 {
		t.Error("expected non-empty SprintTerminals")
	}
	if len(ctx.PlanningPairs) == 0 {
		t.Error("expected non-empty PlanningPairs")
	}
}

func TestLoadDetectionContext_NoPipeline(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	testhelpers.SetupLizaDir(t, tmpDir)
	os.Remove(filepath.Join(tmpDir, ".liza", "pipeline.yaml"))
	_, err := LoadDetectionContext(tmpDir)
	if err == nil {
		t.Fatal("expected error for missing pipeline config")
	}
}

func TestLoadResolver_PipelineGoal(t *testing.T) {
	tmpDir, _ := setupPipelineTest(t)

	resolver, _, err := loadResolver(tmpDir)
	if err != nil {
		t.Fatalf("loadResolver() error: %v", err)
	}
	if resolver == nil {
		t.Fatal("expected non-nil resolver for pipeline goal")
	}
}

func TestBuildPipelineTransitions_BlockedCanReturnToExecutingStatuses(t *testing.T) {
	tmpDir, _ := setupPipelineTest(t)

	resolver, _, err := loadResolver(tmpDir)
	if err != nil {
		t.Fatalf("loadResolver() error: %v", err)
	}

	transitions := BuildPipelineTransitions(resolver)
	got := transitions[models.TaskStatusBlocked]
	for _, want := range []models.TaskStatus{
		models.TaskStatusCodePlanning,
		models.TaskStatusImplementing,
	} {
		if !testContainsStatus(got, want) {
			t.Fatalf("BLOCKED transitions = %v, want %s", got, want)
		}
	}
}

func TestBuildPipelineTransitions_AllowsOperatorCancelBeforeApproval(t *testing.T) {
	tmpDir, _ := setupPipelineTest(t)

	resolver, _, err := loadResolver(tmpDir)
	if err != nil {
		t.Fatalf("loadResolver() error: %v", err)
	}

	transitions := BuildPipelineTransitions(resolver)
	for _, tt := range []struct {
		from models.TaskStatus
	}{
		{from: models.TaskStatusImplementing},
		{from: models.TaskStatusReadyForReview},
		{from: models.TaskStatusReviewing},
		{from: models.TaskStatusReviewingCode2},
	} {
		t.Run(string(tt.from), func(t *testing.T) {
			if !testContainsStatus(transitions[tt.from], models.TaskStatusAbandoned) {
				t.Fatalf("%s transitions = %v, want ABANDONED", tt.from, transitions[tt.from])
			}
		})
	}
}

func TestBuildPipelineTransitions_DoesNotCancelApprovedState(t *testing.T) {
	tmpDir, _ := setupPipelineTest(t)

	resolver, _, err := loadResolver(tmpDir)
	if err != nil {
		t.Fatalf("loadResolver() error: %v", err)
	}

	transitions := BuildPipelineTransitions(resolver)
	if testContainsStatus(transitions[models.TaskStatusApproved], models.TaskStatusAbandoned) {
		t.Fatalf("CODE_APPROVED transitions = %v, want no ABANDONED edge", transitions[models.TaskStatusApproved])
	}
}

func testContainsStatus(values []models.TaskStatus, want models.TaskStatus) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestTransitionSourcePairs_PipelineGoal(t *testing.T) {
	tmpDir, _ := setupPipelineTest(t)

	pairs, err := TransitionSourcePairs(tmpDir)
	if err != nil {
		t.Fatalf("TransitionSourcePairs() error: %v", err)
	}
	// valid-coding-subpipeline.yaml has code-planning-pair as a transition source
	if !pairs["code-planning-pair"] {
		t.Error("expected code-planning-pair to be a transition source")
	}
	// coding-pair is not a transition source (it's the terminal pair)
	if pairs["coding-pair"] {
		t.Error("coding-pair should not be a transition source")
	}
}

func TestIsPlanningPair(t *testing.T) {
	pairs := map[string]bool{"code-planning-pair": true, "epic-planning-pair": true}

	// With explicit pairs map
	if !IsPlanningPair("code-planning-pair", pairs) {
		t.Error("IsPlanningPair(code-planning-pair, pairs) = false, want true")
	}
	if !IsPlanningPair("epic-planning-pair", pairs) {
		t.Error("IsPlanningPair(epic-planning-pair, pairs) = false, want true")
	}
	if IsPlanningPair("coding-pair", pairs) {
		t.Error("IsPlanningPair(coding-pair, pairs) = true, want false")
	}

	// With nil (legacy fallback)
	if !IsPlanningPair("code-planning-pair", nil) {
		t.Error("IsPlanningPair(code-planning-pair, nil) = false, want true")
	}
	if IsPlanningPair("epic-planning-pair", nil) {
		t.Error("IsPlanningPair(epic-planning-pair, nil) should be false in legacy mode")
	}
}

func TestTransitionSourcePairs_NoPipeline(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	testhelpers.SetupLizaDir(t, tmpDir)
	os.Remove(filepath.Join(tmpDir, ".liza", "pipeline.yaml"))

	_, err := TransitionSourcePairs(tmpDir)
	if err == nil {
		t.Fatal("expected error for missing pipeline config")
	}
}

func TestLoadResolver_NoPipeline(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	testhelpers.SetupLizaDir(t, tmpDir)
	os.Remove(filepath.Join(tmpDir, ".liza", "pipeline.yaml"))

	_, _, err := loadResolver(tmpDir)
	if err == nil {
		t.Fatal("expected error for missing pipeline config")
	}
}

// --- ClaimTask pipeline tests ---

func TestClaimTask_PipelineCodingPair(t *testing.T) {
	tmpDir, stateFile := setupPipelineTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Agents["coder-1"] = testhelpers.RegisteredTestAgent("coder")
	state.PipelineVersion = 2
	task := models.Task{
		ID:          "task-1",
		Type:        models.TaskTypeCoding,
		RolePair:    "coding-pair",
		Description: "Pipeline coding task",
		Status:      models.TaskStatus("DRAFT_CODE"),
		Priority:    1,
		Created:     now,
		SpecRef:     "README.md",
		DoneWhen:    "done",
		Scope:       "scope",
		History:     []models.TaskHistoryEntry{},
	}
	state.Tasks = []models.Task{task}
	state.Sprint.Scope.Planned = []string{"task-1"}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := ClaimTask(tmpDir, "task-1", "coder-1")
	if err != nil {
		t.Fatalf("ClaimTask() error: %v", err)
	}
	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}

	// Verify the task transitioned to the pipeline executing state, not hardcoded IMPLEMENTING
	bb := db.For(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	readTask := readState.FindTask("task-1")
	if readTask == nil {
		t.Fatal("Task not found")
	}
	if readTask.Status != models.TaskStatus("IMPLEMENTING_CODE") {
		t.Errorf("Task status = %v, want IMPLEMENTING_CODE", readTask.Status)
	}
}

func TestClaimTask_PipelineCodePlanningPair(t *testing.T) {
	tmpDir, stateFile := setupPipelineTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Agents["code-planner-1"] = testhelpers.RegisteredTestAgent("code-planner")
	state.PipelineVersion = 2
	task := models.Task{
		ID:          "plan-1",
		Type:        models.TaskTypeCoding,
		RolePair:    "code-planning-pair",
		Description: "Pipeline planning task",
		Status:      models.TaskStatus("DRAFT_CODING_PLAN"),
		Priority:    1,
		Created:     now,
		SpecRef:     "README.md",
		DoneWhen:    "done",
		Scope:       "scope",
		History:     []models.TaskHistoryEntry{},
	}
	state.Tasks = []models.Task{task}
	state.Sprint.Scope.Planned = []string{"plan-1"}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := ClaimTask(tmpDir, "plan-1", "code-planner-1")
	if err != nil {
		t.Fatalf("ClaimTask() error: %v", err)
	}
	if result.TaskID != "plan-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "plan-1")
	}

	// Verify pipeline executing state
	bb := db.For(stateFile)
	readState, _ := bb.Read()
	readTask := readState.FindTask("plan-1")
	if readTask.Status != models.TaskStatus("CODE_PLANNING") {
		t.Errorf("Task status = %v, want CODE_PLANNING", readTask.Status)
	}
}

func TestClaimTask_NoPipelineReturnsError(t *testing.T) {
	// No pipeline.yaml → should fail now that pipeline is mandatory
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	os.Remove(filepath.Join(tmpDir, ".liza", "pipeline.yaml"))

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
	}
	state.Sprint.Scope.Planned = []string{"task-1"}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := ClaimTask(tmpDir, "task-1", "coder-1")
	if err == nil {
		t.Fatal("expected error when pipeline config is missing")
	}
}

func TestClaimTask_PipelineRejectedReclaim(t *testing.T) {
	tmpDir, stateFile := setupPipelineTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Agents["coder-2"] = testhelpers.RegisteredTestAgent("coder")
	state.PipelineVersion = 2

	// Create a CODE_REJECTED task with no assigned coder (recovered state)
	task := models.Task{
		ID:          "task-1",
		Type:        models.TaskTypeCoding,
		RolePair:    "coding-pair",
		Description: "Pipeline coding task after rejection",
		Status:      models.TaskStatus("CODE_REJECTED"),
		Priority:    1,
		Created:     now,
		SpecRef:     "README.md",
		DoneWhen:    "done",
		Scope:       "scope",
		History:     []models.TaskHistoryEntry{},
	}
	state.Tasks = []models.Task{task}
	state.Sprint.Scope.Planned = []string{"task-1"}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := ClaimTask(tmpDir, "task-1", "coder-2")
	if err != nil {
		t.Fatalf("ClaimTask() error: %v", err)
	}
	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}

	// Verify the task transitioned to IMPLEMENTING_CODE
	bb := db.For(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	readTask := readState.FindTask("task-1")
	if readTask == nil {
		t.Fatal("Task not found")
	}
	if readTask.Status != models.TaskStatus("IMPLEMENTING_CODE") {
		t.Errorf("Task status = %v, want IMPLEMENTING_CODE", readTask.Status)
	}
}

func TestClaimTask_PipelineRejectedIterationLimit(t *testing.T) {
	tmpDir, stateFile := setupPipelineTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Agents["coder-1"] = testhelpers.RegisteredTestAgent("coder")
	state.PipelineVersion = 2
	state.Config.MaxCoderIterations = 3

	agent := "coder-1"
	worktree := ".worktrees/task-1"
	baseCommit := "abc1234"
	leaseExpires := now.Add(30 * time.Minute)
	task := models.Task{
		ID:           "task-1",
		Type:         models.TaskTypeCoding,
		RolePair:     "coding-pair",
		Description:  "Pipeline coding task at iteration limit",
		Status:       models.TaskStatus("CODE_REJECTED"),
		Priority:     1,
		AssignedTo:   &agent,
		LeaseExpires: &leaseExpires,
		BaseCommit:   &baseCommit,
		Worktree:     &worktree,
		Iteration:    3, // at limit
		Attempt:      2, // attempt 2: iteration cap → BLOCKED
		Created:      now,
		SpecRef:      "README.md",
		DoneWhen:     "done",
		Scope:        "scope",
		History:      []models.TaskHistoryEntry{},
	}
	state.Tasks = []models.Task{task}
	state.Sprint.Scope.Planned = []string{"task-1"}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := ClaimTask(tmpDir, "task-1", "coder-1")
	if err == nil {
		t.Fatal("Expected error for iteration limit exceeded")
	}

	// Verify the task was transitioned to BLOCKED (not stuck in CODE_REJECTED)
	bb := db.For(stateFile)
	readState, readErr := bb.Read()
	if readErr != nil {
		t.Fatalf("Failed to read state: %v", readErr)
	}
	readTask := readState.FindTask("task-1")
	if readTask == nil {
		t.Fatal("Task not found")
	}
	if readTask.Status != models.TaskStatusBlocked {
		t.Errorf("Task status = %v, want BLOCKED", readTask.Status)
	}
}

// --- AddTask pipeline tests ---

func TestInitialTaskStatus_PipelineGoal(t *testing.T) {
	tmpDir, _ := setupPipelineTest(t)

	resolver, _, err := loadResolver(tmpDir)
	if err != nil {
		t.Fatalf("loadResolver error: %v", err)
	}

	// Pipeline goal: coding-pair → DRAFT_CODE
	status, err := resolver.InitialStatus("coding-pair")
	if err != nil {
		t.Fatalf("resolver.InitialStatus(coding-pair) error: %v", err)
	}
	if status != models.TaskStatus("DRAFT_CODE") {
		t.Errorf("InitialStatus(coding-pair) = %v, want DRAFT_CODE", status)
	}

	// Pipeline goal: code-planning-pair → DRAFT_CODING_PLAN
	status, err = resolver.InitialStatus("code-planning-pair")
	if err != nil {
		t.Fatalf("resolver.InitialStatus(code-planning-pair) error: %v", err)
	}
	if status != models.TaskStatus("DRAFT_CODING_PLAN") {
		t.Errorf("InitialStatus(code-planning-pair) = %v, want DRAFT_CODING_PLAN", status)
	}
}

func TestInitialTaskStatus_UnknownRolePair(t *testing.T) {
	tmpDir, _ := setupPipelineTest(t)

	resolver, _, err := loadResolver(tmpDir)
	if err != nil {
		t.Fatalf("loadResolver error: %v", err)
	}

	_, err = resolver.InitialStatus("nonexistent-pair")
	if err == nil {
		t.Fatal("expected error for unknown role-pair")
	}
}

// --- SubmitForReview pipeline tests ---

func TestSubmitForReview_PipelineCodingPairTransition(t *testing.T) {
	tmpDir, stateFile := setupPipelineTest(t)

	// Create a real git worktree so SubmitForReview can complete the full flow
	g := gitpkg.New(tmpDir)
	if _, err := g.CreateWorktree("task-1", "integration"); err != nil {
		t.Fatalf("CreateWorktree() error: %v", err)
	}
	wtPath := g.GetWorktreePath("task-1")

	// Add a test file to satisfy TDD enforcement, then commit
	testFile := filepath.Join(wtPath, "feature_test.go")
	if err := os.WriteFile(testFile, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, wtPath, "add", "feature_test.go")
	testhelpers.MustGit(t, wtPath, "commit", "-m", "Add feature with test")

	commitSHA := testhelpers.MustGit(t, wtPath, "rev-parse", "HEAD")
	baseCommit := testhelpers.MustGit(t, tmpDir, "rev-parse", "integration")

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2

	agent := "coder-1"
	leaseExpires := now.Add(30 * time.Minute)
	worktree := ".worktrees/task-1"
	task := models.Task{
		ID:           "task-1",
		Type:         models.TaskTypeCoding,
		RolePair:     "coding-pair",
		Description:  "Pipeline coding task",
		Status:       models.TaskStatus("IMPLEMENTING_CODE"),
		Priority:     1,
		AssignedTo:   &agent,
		LeaseExpires: &leaseExpires,
		BaseCommit:   &baseCommit,
		Worktree:     &worktree,
		Created:      now,
		SpecRef:      "README.md",
		DoneWhen:     "done",
		Scope:        "scope",
		History: []models.TaskHistoryEntry{
			{Time: now, Event: models.TaskEventPreExecutionCheckpoint, Agent: &agent},
		},
	}
	state.Tasks = []models.Task{task}
	state.Sprint.Scope.Planned = []string{"task-1"}
	state.Agents = map[string]models.Agent{
		"coder-1": {
			Role:         "coder",
			Status:       models.AgentStatusWorking,
			CurrentTask:  &task.ID,
			LeaseExpires: &leaseExpires,
			Heartbeat:    now,
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := SubmitForReview(tmpDir, "task-1", commitSHA, "coder-1")
	if err != nil {
		t.Fatalf("SubmitForReview() error: %v", err)
	}
	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}

	// Verify the task transitioned to the pipeline submitted state
	bb := db.For(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	readTask := readState.FindTask("task-1")
	if readTask == nil {
		t.Fatal("Task not found")
	}
	if readTask.Status != models.TaskStatusReadyForReview {
		t.Errorf("Task status = %v, want %s", readTask.Status, models.TaskStatusReadyForReview)
	}
}

// --- SubmitVerdict pipeline tests ---

func TestSubmitVerdict_PipelineApproved(t *testing.T) {
	tmpDir, stateFile := setupPipelineTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2

	reviewingBy := "code-plan-reviewer-1"
	reviewLeaseExpires := now.Add(30 * time.Minute)
	reviewCommit := "review123"
	agent := "code-planner-1"
	worktree := ".worktrees/plan-1"
	task := models.Task{
		ID:                 "plan-1",
		Type:               models.TaskTypeCoding,
		RolePair:           "code-planning-pair",
		Description:        "Pipeline planning task",
		Status:             models.TaskStatus("REVIEWING_CODING_PLAN"),
		Priority:           1,
		AssignedTo:         &agent,
		ReviewingBy:        &reviewingBy,
		ReviewLeaseExpires: &reviewLeaseExpires,
		ReviewCommit:       &reviewCommit,
		Worktree:           &worktree,
		Created:            now,
		SpecRef:            "README.md",
		DoneWhen:           "done",
		Scope:              "scope",
		History:            []models.TaskHistoryEntry{},
	}
	state.Tasks = []models.Task{task}
	state.Sprint.Scope.Planned = []string{"plan-1"}
	state.Agents = map[string]models.Agent{
		"code-plan-reviewer-1": {
			Role:   "code-plan-reviewer",
			Status: models.AgentStatusReviewing,
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := SubmitVerdict(tmpDir, "plan-1", "APPROVED", "", "code-plan-reviewer-1", "")
	if err != nil {
		t.Fatalf("SubmitVerdict() error: %v", err)
	}
	if result.Verdict != "APPROVED" {
		t.Errorf("Verdict = %q, want APPROVED", result.Verdict)
	}

	bb := db.For(stateFile)
	readState, _ := bb.Read()
	readTask := readState.FindTask("plan-1")
	if readTask.Status != models.TaskStatus("CODING_PLAN_APPROVED") {
		t.Errorf("Task status = %v, want CODING_PLAN_APPROVED", readTask.Status)
	}
}

func TestSubmitVerdict_PipelineCodingPairApproved(t *testing.T) {
	tmpDir, stateFile := setupPipelineTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2

	reviewingBy := "code-reviewer-1"
	reviewLeaseExpires := now.Add(30 * time.Minute)
	reviewCommit := "review123"
	agent := "coder-1"
	worktree := ".worktrees/task-1"
	task := models.Task{
		ID:                 "task-1",
		Type:               models.TaskTypeCoding,
		RolePair:           "coding-pair",
		Description:        "Pipeline coding task",
		Status:             models.TaskStatus("REVIEWING_CODE"),
		Priority:           1,
		AssignedTo:         &agent,
		ReviewingBy:        &reviewingBy,
		ReviewLeaseExpires: &reviewLeaseExpires,
		ReviewCommit:       &reviewCommit,
		Worktree:           &worktree,
		Created:            now,
		SpecRef:            "README.md",
		DoneWhen:           "done",
		Scope:              "scope",
		History:            []models.TaskHistoryEntry{},
	}
	state.Tasks = []models.Task{task}
	state.Sprint.Scope.Planned = []string{"task-1"}
	state.Agents = map[string]models.Agent{
		"code-reviewer-1": {
			Role:   "code-reviewer",
			Status: models.AgentStatusReviewing,
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := SubmitVerdict(tmpDir, "task-1", "APPROVED", "", "code-reviewer-1", "")
	if err != nil {
		t.Fatalf("SubmitVerdict() error: %v", err)
	}
	if result.Verdict != "APPROVED" {
		t.Errorf("Verdict = %q, want APPROVED", result.Verdict)
	}

	bb := db.For(stateFile)
	readState, _ := bb.Read()
	readTask := readState.FindTask("task-1")
	if readTask.Status != models.TaskStatus("CODE_APPROVED") {
		t.Errorf("Task status = %v, want CODE_APPROVED", readTask.Status)
	}
}

// --- ResumeHandoff pipeline tests ---

func TestResumeHandoff_PipelineExecutingState(t *testing.T) {
	tmpDir, stateFile := setupPipelineTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2

	agent := "coder-1"
	leaseExpires := now.Add(30 * time.Minute)
	worktree := ".worktrees/task-1"
	baseCommit := "abc1234"
	task := models.Task{
		ID:             "task-1",
		Type:           models.TaskTypeCoding,
		RolePair:       "coding-pair",
		Description:    "Pipeline coding task",
		Status:         models.TaskStatus("IMPLEMENTING_CODE"),
		Priority:       1,
		AssignedTo:     &agent,
		LeaseExpires:   &leaseExpires,
		BaseCommit:     &baseCommit,
		Worktree:       &worktree,
		HandoffPending: true,
		Created:        now,
		SpecRef:        "README.md",
		DoneWhen:       "done",
		Scope:          "scope",
		History:        []models.TaskHistoryEntry{},
	}
	state.Tasks = []models.Task{task}
	state.Sprint.Scope.Planned = []string{"task-1"}
	state.Agents = map[string]models.Agent{
		"coder-1": {
			Role:         "coder",
			Status:       models.AgentStatusHandoff,
			CurrentTask:  &task.ID,
			LeaseExpires: &leaseExpires,
			Heartbeat:    now,
		},
	}

	// Create the worktree directory on disk
	testhelpers.CreateTestWorktree(t, tmpDir, "task-1")

	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := ResumeHandoff(ResumeHandoffInput{
		ProjectRoot: tmpDir,
		AgentID:     "coder-1",
	})
	if err != nil {
		t.Fatalf("ResumeHandoff() error: %v", err)
	}
	if !result.Found {
		t.Fatal("Expected to find resumable handoff")
	}
	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}
}
