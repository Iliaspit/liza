package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/agent"
	"github.com/liza-mas/liza/internal/brand"
	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/statevalidate"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func setupHumanNoteCLIProject(t *testing.T) (string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testhelpers.SetupTestGitRepo(t, root)
	statePath, _ := testhelpers.SetupLizaDir(t, root)
	testhelpers.SetupPipelineConfig(t, root)
	state := testhelpers.CreateValidState()
	state.Goal.SpecRef = "README.md"
	now := time.Now().UTC().Add(-time.Hour)
	task := testhelpers.BuildTaskByStatus("planning-1", models.TaskStatusBlocked, now)
	task.Type, task.RolePair = models.TaskTypePlanning, "code-planning-pair"
	task.AssignedTo, task.Worktree = nil, nil
	task.History = []models.TaskHistoryEntry{{Time: now, Event: models.TaskEventOrchestratorAssessment}}
	state.Tasks = []models.Task{task}
	state.Sprint.Scope.Planned = []string{task.ID}
	if err := statevalidate.ValidateState(state, root, false, io.Discard); err != nil {
		t.Fatalf("invalid canonical CLI fixture: %v", err)
	}
	testhelpers.WriteInitialState(t, statePath, state)
	return root, statePath
}

// Exercise the registered CLI entrypoint without the agent-generation injection
// in other command-test helpers. Explicit root resolution works from any cwd.
func executeHumanNoteCLI(t *testing.T, root string, args ...string) error {
	t.Helper()
	resetRootCmdForTest(t)
	defer resetRootCmdForTest(t)
	rootCmd.SetArgs(append([]string{"--project-root", root, "add-human-note"}, args...))
	return rootCmd.Execute()
}

func TestAddHumanNoteCommandPreservesMessageAndUsesFixedHumanAttribution(t *testing.T) {
	root, path := setupHumanNoteCLIProject(t)
	for _, key := range []string{brand.EnvName("AGENT_ID"), "LIZA_AGENT_ID"} {
		t.Setenv(key, "ambient-worker")
	}
	for _, key := range []string{brand.EnvName("AGENT_GENERATION"), "LIZA_AGENT_GENERATION"} {
		t.Setenv(key, "stale-generation")
	}
	before, err := db.New(path).Read()
	if err != nil {
		t.Fatal(err)
	}
	message := " \n  Approved answer: λ.\nContinue the existing draft.  \n"
	if err := executeHumanNoteCLI(t, root, "planning-1", "--message", message); err != nil {
		t.Fatal(err)
	}
	after, err := db.New(path).Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(after.HumanNotes) != 1 {
		t.Fatalf("notes = %d, want 1", len(after.HumanNotes))
	}
	note := after.HumanNotes[0]
	if note.For != "planning-1" || note.Message != message || note.Timestamp.IsZero() || note.Timestamp.Location() != time.UTC || !reflect.DeepEqual(note.Extra, map[string]any{"changed_by": "human"}) {
		t.Fatalf("unexpected note: %#v", note)
	}
	after.HumanNotes = before.HumanNotes
	if !reflect.DeepEqual(after, before) {
		t.Fatal("CLI append changed other state")
	}
}

func TestAddHumanNoteCommandRejectsInputsWithoutMutation(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		contains string
	}{
		{"no task", []string{"--message", "answer"}, "accepts 1 arg"},
		{"extra task", []string{"planning-1", "other", "--message", "answer"}, "accepts 1 arg"},
		{"missing message", []string{"planning-1"}, "--message is required"},
		{"empty message", []string{"planning-1", "--message", ""}, "must not be blank"},
		{"blank message", []string{"planning-1", "--message", " \t\n"}, "must not be blank"},
		{"invalid ID", []string{"../planning-1", "--message", "answer"}, "invalid task ID"},
		{"unknown task", []string{"missing-1", "--message", "answer"}, "not found"},
		{"all", []string{"all", "--message", "answer"}, "one task ID"},
		{"consumed inherited flag", []string{"planning-1", "--message", "--verbose"}, "matches registered flag --verbose"},
		{"consumed root flag with value", []string{"planning-1", "--message", "--project-root=/tmp"}, "matches registered flag --project-root"},
		{"consumed shorthand", []string{"planning-1", "--message", "-v"}, "matches registered flag -v"},
		{"consumed help", []string{"planning-1", "--message", "--help"}, "matches registered flag --help"},
		{"consumed local flag", []string{"planning-1", "--message", "--message"}, "matches registered flag --message"},
		{"worker flag unsupported", []string{"planning-1", "--message", "answer", "--agent-id", "worker"}, "unknown flag"},
		{"attribution flag unsupported", []string{"planning-1", "--message", "answer", "--changed-by", "worker"}, "unknown flag"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, path := setupHumanNoteCLIProject(t)
			before := readHumanNoteCLIBytes(t, path)
			err := executeHumanNoteCLI(t, root, tt.args...)
			if err == nil || !strings.Contains(err.Error(), tt.contains) {
				t.Fatalf("error = %v, want %q", err, tt.contains)
			}
			if !bytes.Equal(before, readHumanNoteCLIBytes(t, path)) {
				t.Fatal("rejected CLI invocation mutated state")
			}
		})
	}
}

func TestAddHumanNoteCommandRejectsNonBlockedTarget(t *testing.T) {
	root, path := setupHumanNoteCLIProject(t)
	if err := db.New(path).Modify(func(state *models.State) error {
		state.FindTask("planning-1").Status = models.TaskStatusMerged
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	before := readHumanNoteCLIBytes(t, path)
	if err := executeHumanNoteCLI(t, root, "planning-1", "--message", "answer"); err == nil || !strings.Contains(err.Error(), "must be BLOCKED") {
		t.Fatalf("error = %v", err)
	}
	if !bytes.Equal(before, readHumanNoteCLIBytes(t, path)) {
		t.Fatal("nonblocked rejection mutated state")
	}
}

func TestAddHumanNoteCommandUnknownFlagShapedMessageIsAccepted(t *testing.T) {
	root, path := setupHumanNoteCLIProject(t)
	message := "--domain-answer=keep"
	if err := executeHumanNoteCLI(t, root, "planning-1", "--message", message); err != nil {
		t.Fatal(err)
	}
	state, err := db.New(path).Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.HumanNotes) != 1 || state.HumanNotes[0].Message != message {
		t.Fatal("flag-shaped free text changed")
	}
}

func TestAddHumanNoteCommandMessageFlagDoesNotLeak(t *testing.T) {
	root, path := setupHumanNoteCLIProject(t)
	if err := executeHumanNoteCLI(t, root, "planning-1", "--message", "answer"); err != nil {
		t.Fatal(err)
	}
	before := readHumanNoteCLIBytes(t, path)
	if addHumanNoteCmd.Flags().Changed("message") {
		t.Fatal("message flag Changed leaked")
	}
	value, err := addHumanNoteCmd.Flags().GetString("message")
	if err != nil || value != "" {
		t.Fatalf("message flag = %q, error = %v", value, err)
	}
	if err := executeHumanNoteCLI(t, root, "planning-1"); err == nil {
		t.Fatal("missing message reused previous value")
	}
	if !bytes.Equal(before, readHumanNoteCLIBytes(t, path)) {
		t.Fatal("leaked message added another note")
	}
}

func TestAddHumanNoteEndToEndPreservesDirtyPlanningDraftAndWakes(t *testing.T) {
	root, statePath := setupHumanNoteCLIProject(t)
	worktree := filepath.Join(root, ".worktrees", "planning-1")
	testhelpers.MustGit(t, root, "worktree", "add", "-b", "task/planning-1", worktree, "HEAD")
	draft := filepath.Join(worktree, "planning.md")
	if err := os.WriteFile(draft, []byte("# Planning draft\nStaged decision\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, worktree, "add", "planning.md")
	if err := os.WriteFile(draft, []byte("# Planning draft\nStaged decision\nUnstaged detail\n"), 0644); err != nil {
		t.Fatal(err)
	}
	untracked := filepath.Join(worktree, "questions.txt")
	if err := os.WriteFile(untracked, []byte("Preserve unresolved notes\n"), 0644); err != nil {
		t.Fatal(err)
	}
	owner, reviewer, relative := "code-planner-1", "code-plan-reviewer-1", ".worktrees/planning-1"
	head := testhelpers.MustGit(t, worktree, "rev-parse", "HEAD")
	lease := time.Now().UTC().Add(time.Hour)
	if err := db.New(statePath).Modify(func(state *models.State) error {
		task := state.FindTask("planning-1")
		task.AssignedTo, task.ReviewingBy, task.Worktree, task.BaseCommit = &owner, &reviewer, &relative, &head
		task.LeaseExpires, task.ReviewLeaseExpires, task.Attempt = &lease, &lease, 2
		task.Extra = map[string]any{"draft_marker": "preserve"}
		state.Agents[owner] = models.Agent{Role: models.RoleCodePlanner, Status: models.AgentStatusHandoff, Generation: "planner-generation", CurrentTask: &task.ID, LeaseExpires: &lease, Heartbeat: time.Now().UTC(), Provider: "codex", PID: os.Getpid()}
		state.Agents[reviewer] = models.Agent{Role: models.RoleCodePlanReviewer, Status: models.AgentStatusHandoff, Generation: "reviewer-generation", CurrentTask: &task.ID, LeaseExpires: &lease, Heartbeat: time.Now().UTC(), Provider: "codex", PID: os.Getpid()}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	before, err := db.New(statePath).Read()
	if err != nil {
		t.Fatal(err)
	}
	if err := statevalidate.ValidateState(before, root, false, io.Discard); err != nil {
		t.Fatalf("invalid canonical E2E fixture: %v", err)
	}
	if wake := agent.DetectOrchestratorWakeTriggers(before, nil, nil, nil); wake.Trigger != agent.WakeTriggerNone {
		t.Fatalf("before note wake = %#v", wake)
	}
	statusBefore := testhelpers.MustGit(t, worktree, "status", "--porcelain=v1", "--untracked-files=all")
	if !strings.Contains(statusBefore, "AM planning.md") || !strings.Contains(statusBefore, "?? questions.txt") {
		t.Fatalf("fixture must contain staged, dirty, and untracked draft: %q", statusBefore)
	}
	indexPath := testhelpers.MustGit(t, worktree, "rev-parse", "--git-path", "index")
	if !filepath.IsAbs(indexPath) {
		indexPath = filepath.Join(worktree, indexPath)
	}
	files := []string{draft, untracked, filepath.Join(worktree, ".git"), indexPath}
	snapshots := make(map[string][]byte)
	for _, path := range files {
		snapshots[path] = readHumanNoteCLIBytes(t, path)
	}
	mainHead := testhelpers.MustGit(t, root, "rev-parse", "HEAD")
	branch := testhelpers.MustGit(t, worktree, "symbolic-ref", "HEAD")
	worktreesBefore := testhelpers.MustGit(t, root, "worktree", "list", "--porcelain")
	t.Setenv(brand.EnvName("AGENT_ID"), owner)
	t.Setenv(brand.EnvName("AGENT_GENERATION"), "stale-worker-generation")
	message := "Human decision: retain and continue the staged planning draft."
	if err := executeHumanNoteCLI(t, root, "planning-1", "--message", message); err != nil {
		t.Fatal(err)
	}
	// Read index before git status can refresh its bookkeeping.
	for _, path := range files {
		if !bytes.Equal(snapshots[path], readHumanNoteCLIBytes(t, path)) {
			t.Fatalf("draft/index/link changed: %s", path)
		}
	}
	if got := testhelpers.MustGit(t, worktree, "status", "--porcelain=v1", "--untracked-files=all"); got != statusBefore {
		t.Fatalf("worktree status changed: %q -> %q", statusBefore, got)
	}
	if testhelpers.MustGit(t, worktree, "rev-parse", "HEAD") != head || testhelpers.MustGit(t, root, "rev-parse", "HEAD") != mainHead || testhelpers.MustGit(t, worktree, "symbolic-ref", "HEAD") != branch || testhelpers.MustGit(t, root, "worktree", "list", "--porcelain") != worktreesBefore {
		t.Fatal("worktree/branch/HEAD identity changed")
	}
	after, err := db.New(statePath).Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(after.HumanNotes) != 1 || after.HumanNotes[0].For != "planning-1" || after.HumanNotes[0].Message != message || after.HumanNotes[0].Extra["changed_by"] != "human" {
		t.Fatalf("unexpected persisted human note: %#v", after.HumanNotes)
	}
	if wake := agent.DetectOrchestratorWakeTriggers(after, nil, nil, nil); wake.Trigger != agent.WakeTriggerBlocked || wake.Count != 1 {
		t.Fatalf("new human decision did not wake blocked task: %#v", wake)
	}
	after.HumanNotes = before.HumanNotes
	if !reflect.DeepEqual(after, before) {
		t.Fatal("human note changed task lifecycle, claims, agents, or unrelated state")
	}
}

func readHumanNoteCLIBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
