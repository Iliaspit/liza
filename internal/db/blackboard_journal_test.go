package db

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/journal"
	"github.com/liza-mas/liza/internal/models"
)

// TestShadowJournalTracksModifications drives a realistic task lifecycle
// through Blackboard.Write/Modify and asserts that folding the shadow journal
// reproduces exactly the task statuses in state.yaml — the equivalence
// property the journal migration depends on.
func TestShadowJournalTracksModifications(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.yaml")
	bb := New(statePath)

	initial := &models.State{
		Version: 1,
		Tasks: []models.Task{
			{ID: "task-1", Status: "READY", Description: "first"},
		},
		Agents: map[string]models.Agent{},
	}
	if err := bb.Write(initial); err != nil {
		t.Fatalf("initial Write failed: %v", err)
	}

	// Register an agent and claim the task (history entry + status change).
	err := bb.Modify(func(state *models.State) error {
		state.Agents["coder-1"] = models.Agent{
			Role:   "coder",
			Status: models.AgentStatusIdle,
		}
		task := state.FindTask("task-1")
		task.Status = "IMPLEMENTING_CODE"
		agent := "coder-1"
		task.AssignedTo = &agent
		task.History = append(task.History, models.TaskHistoryEntry{
			Time:  time.Now(),
			Event: models.TaskEventClaimed,
			Agent: &agent,
		})
		return nil
	})
	if err != nil {
		t.Fatalf("claim Modify failed: %v", err)
	}

	// Add a second task and merge the first.
	err = bb.Modify(func(state *models.State) error {
		state.Tasks = append(state.Tasks, models.Task{
			ID: "task-2", Status: "DRAFT_CODE", Description: "second",
		})
		task := state.FindTask("task-1")
		task.Status = "MERGED"
		task.AssignedTo = nil
		return nil
	})
	if err != nil {
		t.Fatalf("merge Modify failed: %v", err)
	}

	// A pure lease-renewal write must not emit events (heartbeat noise).
	store := journal.ForStatePath(statePath)
	seqBeforeHeartbeat, err := store.LastSeq()
	if err != nil {
		t.Fatalf("LastSeq failed: %v", err)
	}
	err = bb.Modify(func(state *models.State) error {
		agent := state.Agents["coder-1"]
		agent.Heartbeat = time.Now()
		state.Agents["coder-1"] = agent
		return nil
	})
	if err != nil {
		t.Fatalf("heartbeat Modify failed: %v", err)
	}
	seqAfterHeartbeat, err := store.LastSeq()
	if err != nil {
		t.Fatalf("LastSeq failed: %v", err)
	}
	if seqAfterHeartbeat != seqBeforeHeartbeat {
		t.Errorf("heartbeat-only write emitted %d events; want 0",
			seqAfterHeartbeat-seqBeforeHeartbeat)
	}

	events, err := store.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected shadow journal events, got none")
	}

	var sawClaim, sawAgentRegistered bool
	for _, ev := range events {
		switch ev.Type {
		case "task.claimed":
			sawClaim = ev.Task == "task-1" && ev.Agent == "coder-1"
		case journal.EventAgentRegistered:
			sawAgentRegistered = ev.Agent == "coder-1"
		}
	}
	if !sawClaim || !sawAgentRegistered {
		t.Errorf("missing events: claim=%v registered=%v\n%+v", sawClaim, sawAgentRegistered, events)
	}

	// Equivalence: projection over the journal == statuses in state.yaml.
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	actual := map[string]string{}
	for _, task := range state.Tasks {
		actual[task.ID] = string(task.Status)
	}
	diff := journal.ProjectTaskStatuses(events).Diff(actual)
	if len(diff) != 0 {
		t.Errorf("journal projection diverges from state.yaml: %+v", diff)
	}
}

// TestShadowJournalOpProvenance verifies that events derived from a named
// operation carry that operation name, and that plain Modify falls back to
// the generic "modify" provenance.
func TestShadowJournalOpProvenance(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.yaml")
	bb := New(statePath)

	if err := bb.Write(&models.State{
		Version: 1,
		Tasks:   []models.Task{{ID: "task-1", Status: "READY"}},
		Agents:  map[string]models.Agent{},
	}); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	err := bb.ModifyOp("claim_task", func(state *models.State) error {
		state.FindTask("task-1").Status = "IMPLEMENTING_CODE"
		return nil
	})
	if err != nil {
		t.Fatalf("ModifyOp failed: %v", err)
	}
	err = bb.Modify(func(state *models.State) error {
		state.FindTask("task-1").Status = "MERGED"
		return nil
	})
	if err != nil {
		t.Fatalf("Modify failed: %v", err)
	}

	events, err := journal.ForStatePath(statePath).ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	ops := map[string]string{}
	for _, ev := range events {
		if ev.Type == journal.EventTaskStatusChanged {
			ops[ev.Fields["to"].(string)] = ev.Op
		} else if ev.Type == journal.EventTaskCreated {
			ops["created"] = ev.Op
		}
	}
	if ops["created"] != "write" {
		t.Errorf("initial write op = %q, want \"write\"", ops["created"])
	}
	if ops["IMPLEMENTING_CODE"] != "claim_task" {
		t.Errorf("named op = %q, want \"claim_task\"", ops["IMPLEMENTING_CODE"])
	}
	if ops["MERGED"] != "modify" {
		t.Errorf("generic op = %q, want \"modify\"", ops["MERGED"])
	}
}

// TestShadowJournalOnInitialWrite verifies that the very first Write (project
// init, no pre-existing state file) journals creation events from an empty
// before-state.
func TestShadowJournalOnInitialWrite(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.yaml")
	bb := New(statePath)

	if err := bb.Write(&models.State{
		Version: 1,
		Tasks:   []models.Task{{ID: "task-1", Status: "DRAFT_CODE"}},
		Agents:  map[string]models.Agent{},
	}); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	events, err := journal.ForStatePath(statePath).ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if len(events) != 1 || events[0].Type != journal.EventTaskCreated || events[0].Task != "task-1" {
		t.Fatalf("expected single task.created event, got %+v", events)
	}
}
