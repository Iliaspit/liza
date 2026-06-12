package journal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
)

func storeForTest(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	return ForStatePath(filepath.Join(dir, "state.yaml"))
}

func TestAppendAndReadRoundTrip(t *testing.T) {
	s := storeForTest(t)

	in := []Event{
		{Type: "task.created", Task: "task-1", Fields: map[string]any{"status": "READY"}},
		{Type: "task.claimed", Task: "task-1", Agent: "coder-1"},
	}
	if err := s.Append(in); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	out, err := s.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 events, got %d", len(out))
	}
	if out[0].Seq != 1 || out[1].Seq != 2 {
		t.Errorf("expected seq 1,2 got %d,%d", out[0].Seq, out[1].Seq)
	}
	if out[1].Agent != "coder-1" || out[1].Type != "task.claimed" {
		t.Errorf("unexpected event: %+v", out[1])
	}
	if out[0].Time.IsZero() {
		t.Error("expected timestamp to be assigned")
	}
}

func TestSeqContinuityAcrossStoreInstances(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.yaml")

	s1 := ForStatePath(statePath)
	if err := s1.Append([]Event{{Type: "a"}, {Type: "b"}}); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	s2 := ForStatePath(statePath)
	if err := s2.Append([]Event{{Type: "c"}}); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	out, err := s2.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if len(out) != 3 || out[2].Seq != 3 {
		t.Fatalf("expected 3 events ending at seq 3, got %d events, last seq %d", len(out), out[len(out)-1].Seq)
	}
}

func TestReadSince(t *testing.T) {
	s := storeForTest(t)
	if err := s.Append([]Event{{Type: "a"}, {Type: "b"}, {Type: "c"}}); err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	out, err := s.ReadSince(2)
	if err != nil {
		t.Fatalf("ReadSince failed: %v", err)
	}
	if len(out) != 1 || out[0].Type != "c" {
		t.Fatalf("expected only event c, got %+v", out)
	}
}

func TestFieldCapping(t *testing.T) {
	s := storeForTest(t)
	huge := strings.Repeat("x", maxFieldBytes*2)
	if err := s.Append([]Event{{Type: "a", Fields: map[string]any{"note": huge}}}); err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	out, err := s.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	note := out[0].Fields["note"].(string)
	if len(note) > maxFieldBytes+len(truncationSuffix) {
		t.Errorf("field not capped: %d bytes", len(note))
	}
	if !strings.HasSuffix(note, truncationSuffix) {
		t.Error("expected truncation marker suffix")
	}
}

func TestTornTrailingLineTolerated(t *testing.T) {
	s := storeForTest(t)
	if err := s.Append([]Event{{Type: "a"}}); err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	// Simulate a crash mid-append: partial JSON with no newline.
	f, err := os.OpenFile(s.Path(), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	if _, err := f.WriteString(`{"seq":2,"type":"tor`); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	f.Close()

	out, err := s.ReadAll()
	if err != nil {
		t.Fatalf("expected torn tail to be tolerated, got: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 event, got %d", len(out))
	}

	// The next append must continue from the last well-formed seq.
	if err := s.Append([]Event{{Type: "b"}}); err != nil {
		t.Fatalf("Append after torn tail failed: %v", err)
	}
}

func TestMalformedMiddleLineIsError(t *testing.T) {
	s := storeForTest(t)
	if err := os.WriteFile(s.Path(), []byte("not json\n{\"seq\":1,\"type\":\"a\"}\n"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if _, err := s.ReadAll(); err == nil {
		t.Fatal("expected integrity error for malformed non-tail line")
	}
}

func strPtr(s string) *string { return &s }

func TestDeriveTaskLifecycle(t *testing.T) {
	before := &models.State{
		Tasks: []models.Task{
			{ID: "task-1", Status: "READY"},
		},
		Agents: map[string]models.Agent{},
	}
	after := &models.State{
		Tasks: []models.Task{
			{
				ID:     "task-1",
				Status: "IMPLEMENTING_CODE",
				History: []models.TaskHistoryEntry{
					{Time: time.Now(), Event: models.TaskEventClaimed, Agent: strPtr("coder-1")},
				},
			},
			{ID: "task-2", Status: "DRAFT_CODE", Type: "coding", RolePair: "code-pair"},
		},
		Agents: map[string]models.Agent{
			"coder-1": {Role: "coder", Status: models.AgentStatusIdle},
		},
	}

	events := Derive(before, after)

	wantTypes := map[string]bool{
		"task.claimed":         false,
		EventTaskStatusChanged: false,
		EventTaskCreated:       false,
		EventAgentRegistered:   false,
	}
	for _, ev := range events {
		if _, ok := wantTypes[ev.Type]; ok {
			wantTypes[ev.Type] = true
		}
	}
	for typ, seen := range wantTypes {
		if !seen {
			t.Errorf("expected derived event %q, got %+v", typ, events)
		}
	}

	for _, ev := range events {
		if ev.Type == EventTaskStatusChanged {
			if ev.Fields["from"] != "READY" || ev.Fields["to"] != "IMPLEMENTING_CODE" {
				t.Errorf("unexpected status change fields: %+v", ev.Fields)
			}
		}
		if ev.Type == "task.claimed" && ev.Agent != "coder-1" {
			t.Errorf("expected claim agent coder-1, got %q", ev.Agent)
		}
	}
}

func TestDeriveRemovalAndAppendOnly(t *testing.T) {
	before := &models.State{
		Tasks:  []models.Task{{ID: "task-1", Status: "MERGED"}},
		Agents: map[string]models.Agent{"coder-1": {Role: "coder"}},
	}
	after := &models.State{
		Tasks:  []models.Task{},
		Agents: map[string]models.Agent{},
		Anomalies: []models.Anomaly{
			{Task: "task-1", Reporter: "coder-1", Type: "retry_loop"},
		},
	}

	events := Derive(before, after)

	var sawRemoved, sawUnregistered, sawAnomaly bool
	for _, ev := range events {
		switch ev.Type {
		case EventTaskRemoved:
			sawRemoved = ev.Task == "task-1"
		case EventAgentUnregistered:
			sawUnregistered = ev.Agent == "coder-1"
		case EventAnomalyLogged:
			sawAnomaly = ev.Fields["anomaly_type"] == "retry_loop"
		}
	}
	if !sawRemoved || !sawUnregistered || !sawAnomaly {
		t.Errorf("missing derived events: removed=%v unregistered=%v anomaly=%v\n%+v",
			sawRemoved, sawUnregistered, sawAnomaly, events)
	}
}

func TestProjectionFoldsToCurrentStatuses(t *testing.T) {
	events := []Event{
		{Type: EventTaskCreated, Task: "t1", Fields: map[string]any{"status": "READY"}},
		{Type: EventTaskStatusChanged, Task: "t1", Fields: map[string]any{"from": "READY", "to": "IMPLEMENTING_CODE"}},
		{Type: EventTaskCreated, Task: "t2", Fields: map[string]any{"status": "DRAFT_CODE"}},
		{Type: EventTaskStatusChanged, Task: "t1", Fields: map[string]any{"from": "IMPLEMENTING_CODE", "to": "MERGED"}},
		{Type: EventTaskRemoved, Task: "t2"},
	}
	proj := ProjectTaskStatuses(events)
	if proj["t1"] != "MERGED" {
		t.Errorf("expected t1=MERGED, got %q", proj["t1"])
	}
	if _, ok := proj["t2"]; ok {
		t.Error("expected t2 removed from projection")
	}

	diff := proj.Diff(map[string]string{"t1": "MERGED"})
	if len(diff) != 0 {
		t.Errorf("expected empty diff, got %+v", diff)
	}
	diff = proj.Diff(map[string]string{"t1": "BLOCKED"})
	if len(diff) != 1 {
		t.Errorf("expected 1 diff entry, got %+v", diff)
	}
}

func TestDeriveClaimEvents(t *testing.T) {
	now := time.Now().UTC()
	exp := now.Add(time.Hour)
	renewed := now.Add(2 * time.Hour)

	tasks := []models.Task{
		{ID: "task-1", Status: "IMPLEMENTING_CODE"},
		{ID: "task-2", Status: "REVIEWING_CODE"},
		{ID: "task-3", Status: "IMPLEMENTING_CODE"},
		{ID: "task-4", Status: "IMPLEMENTING_CODE"},
	}
	before := &models.State{
		Tasks:  tasks,
		Agents: map[string]models.Agent{},
		Claims: []models.Claim{
			// Released in after (claim.released expected).
			{TaskID: "task-2", AgentID: "reviewer-1", Kind: models.ClaimKindReviewer, GrantedAt: now, ExpiresAt: &exp},
			// Pure lease renewal in after (no event expected).
			{TaskID: "task-3", AgentID: "coder-2", Kind: models.ClaimKindDoer, GrantedAt: now, ExpiresAt: &exp},
			// Agent changes in after (release + grant expected).
			{TaskID: "task-4", AgentID: "coder-3", Kind: models.ClaimKindDoer, GrantedAt: now, ExpiresAt: &exp},
		},
	}
	after := &models.State{
		Tasks:  tasks,
		Agents: map[string]models.Agent{},
		Claims: []models.Claim{
			// Newly granted (claim.granted expected).
			{TaskID: "task-1", AgentID: "coder-1", Kind: models.ClaimKindDoer, GrantedAt: now, ExpiresAt: &exp},
			// Same claim, renewed lease only.
			{TaskID: "task-3", AgentID: "coder-2", Kind: models.ClaimKindDoer, GrantedAt: now, ExpiresAt: &renewed},
			// Same task+kind, different agent.
			{TaskID: "task-4", AgentID: "coder-4", Kind: models.ClaimKindDoer, GrantedAt: renewed, ExpiresAt: &renewed},
		},
	}

	events := Derive(before, after)

	var claimEvents []Event
	for _, ev := range events {
		if ev.Type == EventClaimGranted || ev.Type == EventClaimReleased {
			claimEvents = append(claimEvents, ev)
		}
	}
	if len(claimEvents) != 4 {
		t.Fatalf("expected 4 claim events, got %d: %+v", len(claimEvents), claimEvents)
	}

	type want struct {
		eventType string
		agent     string
		kind      string
	}
	wants := map[string][]want{
		"task-1": {{EventClaimGranted, "coder-1", models.ClaimKindDoer}},
		"task-2": {{EventClaimReleased, "reviewer-1", models.ClaimKindReviewer}},
		"task-4": {
			{EventClaimReleased, "coder-3", models.ClaimKindDoer},
			{EventClaimGranted, "coder-4", models.ClaimKindDoer},
		},
	}
	got := map[string][]want{}
	for _, ev := range claimEvents {
		if ev.Task == "task-3" {
			t.Errorf("lease renewal must not emit claim events, got %+v", ev)
			continue
		}
		got[ev.Task] = append(got[ev.Task], want{ev.Type, ev.Agent, ev.Fields["kind"].(string)})
	}
	for taskID, wantEvents := range wants {
		gotEvents := got[taskID]
		if len(gotEvents) != len(wantEvents) {
			t.Errorf("task %s: got %+v, want %+v", taskID, gotEvents, wantEvents)
			continue
		}
		for i := range wantEvents {
			if gotEvents[i] != wantEvents[i] {
				t.Errorf("task %s event %d: got %+v, want %+v", taskID, i, gotEvents[i], wantEvents[i])
			}
		}
	}
}
