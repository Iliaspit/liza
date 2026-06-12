package journal

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// buildLifecycleEvents returns events exercising create / status-change /
// remove so the projection has non-trivial content.
func buildLifecycleEvents() []Event {
	return []Event{
		{Type: EventTaskCreated, Task: "t1", Fields: map[string]any{"status": "READY"}},
		{Type: EventTaskCreated, Task: "t2", Fields: map[string]any{"status": "READY"}},
		{Type: EventTaskCreated, Task: "t3", Fields: map[string]any{"status": "DRAFT_CODE"}},
		{Type: EventTaskStatusChanged, Task: "t1", Fields: map[string]any{"from": "READY", "to": "IMPLEMENTING_CODE"}},
		{Type: EventTaskStatusChanged, Task: "t2", Fields: map[string]any{"from": "READY", "to": "MERGED"}},
		{Type: EventTaskRemoved, Task: "t3"},
		{Type: EventTaskStatusChanged, Task: "t1", Fields: map[string]any{"from": "IMPLEMENTING_CODE", "to": "MERGED"}},
	}
}

func TestRotateNoOpUnderThreshold(t *testing.T) {
	s := storeForTest(t)
	if err := s.Append([]Event{{Type: "a"}, {Type: "b"}, {Type: "c"}}); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	res, err := s.Rotate(10)
	if err != nil {
		t.Fatalf("Rotate failed: %v", err)
	}
	if res != nil {
		t.Fatalf("expected no-op rotation under threshold, got %+v", res)
	}
	// Exactly at the threshold is also a no-op (rotation requires MORE than threshold).
	if res, err = s.Rotate(3); err != nil || res != nil {
		t.Fatalf("expected no-op at exact threshold, got res=%+v err=%v", res, err)
	}

	events, err := s.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("journal modified by no-op rotation: %d events", len(events))
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(s.Path()), ArchiveDirName)); !os.IsNotExist(err) {
		t.Errorf("expected no archive directory after no-op, stat err: %v", err)
	}
}

func TestRotatePreservesProjectionEquivalence(t *testing.T) {
	s := storeForTest(t)
	if err := s.Append(buildLifecycleEvents()); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	beforeEvents, err := s.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	before := ProjectTaskStatuses(beforeEvents)

	res, err := s.Rotate(2)
	if err != nil {
		t.Fatalf("Rotate failed: %v", err)
	}
	if res == nil {
		t.Fatal("expected rotation above threshold")
	}

	// The rotated journal (snapshot event only, read back from disk so
	// task_statuses went through the JSON round-trip as map[string]any)
	// must project identically.
	afterEvents, err := s.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll after rotate failed: %v", err)
	}
	if len(afterEvents) != 1 || afterEvents[0].Type != EventJournalRotated {
		t.Fatalf("expected fresh journal with single snapshot event, got %+v", afterEvents)
	}
	if _, isAny := afterEvents[0].Fields["task_statuses"].(map[string]any); !isAny {
		t.Fatalf("expected JSON round-tripped task_statuses to be map[string]any, got %T",
			afterEvents[0].Fields["task_statuses"])
	}
	after := ProjectTaskStatuses(afterEvents)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("projection diverged across rotation:\nbefore: %+v\nafter:  %+v", before, after)
	}

	// Post-rotation appends keep folding correctly on top of the seed.
	post := []Event{
		{Type: EventTaskStatusChanged, Task: "t2", Fields: map[string]any{"from": "MERGED", "to": "ARCHIVED"}},
		{Type: EventTaskCreated, Task: "t4", Fields: map[string]any{"status": "READY"}},
		{Type: EventTaskRemoved, Task: "t1"},
	}
	if err := s.Append(post); err != nil {
		t.Fatalf("Append after rotation failed: %v", err)
	}

	liveEvents, err := s.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	fullEvents, err := s.ReadAllIncludingArchives()
	if err != nil {
		t.Fatalf("ReadAllIncludingArchives failed: %v", err)
	}

	liveProj := ProjectTaskStatuses(liveEvents)
	fullProj := ProjectTaskStatuses(fullEvents)
	want := TaskStatusProjection{"t2": "ARCHIVED", "t4": "READY"}
	if !reflect.DeepEqual(liveProj, want) {
		t.Errorf("live projection mismatch: got %+v want %+v", liveProj, want)
	}
	if !reflect.DeepEqual(fullProj, want) {
		t.Errorf("full-history projection mismatch: got %+v want %+v", fullProj, want)
	}
}

func TestRotateSeqMonotonicity(t *testing.T) {
	s := storeForTest(t)
	if err := s.Append(buildLifecycleEvents()); err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	lastBefore, err := s.LastSeq()
	if err != nil {
		t.Fatalf("LastSeq failed: %v", err)
	}

	res, err := s.Rotate(1)
	if err != nil {
		t.Fatalf("Rotate failed: %v", err)
	}
	if res.ArchivedThroughSeq != lastBefore {
		t.Errorf("expected ArchivedThroughSeq %d, got %d", lastBefore, res.ArchivedThroughSeq)
	}

	events, err := s.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if events[0].Seq != lastBefore+1 {
		t.Errorf("snapshot event seq: got %d, want %d (no reset)", events[0].Seq, lastBefore+1)
	}

	if err := s.Append([]Event{{Type: "post-rotate"}}); err != nil {
		t.Fatalf("Append after rotation failed: %v", err)
	}
	lastAfter, err := s.LastSeq()
	if err != nil {
		t.Fatalf("LastSeq failed: %v", err)
	}
	if lastAfter != lastBefore+2 {
		t.Errorf("expected seq to continue at %d, got %d", lastBefore+2, lastAfter)
	}
}

func TestReadAllIncludingArchivesOrdering(t *testing.T) {
	s := storeForTest(t)

	// Two rotation cycles so multiple archive segments exist.
	for cycle := 0; cycle < 2; cycle++ {
		if err := s.Append(buildLifecycleEvents()); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
		if res, err := s.Rotate(1); err != nil || res == nil {
			t.Fatalf("Rotate failed: res=%+v err=%v", res, err)
		}
	}
	if err := s.Append([]Event{{Type: "tail"}}); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	all, err := s.ReadAllIncludingArchives()
	if err != nil {
		t.Fatalf("ReadAllIncludingArchives failed: %v", err)
	}
	// 7 lifecycle + snapshot + 7 lifecycle + snapshot + tail = 17, seq 1..17.
	if len(all) != 17 {
		t.Fatalf("expected 17 events across archives + live, got %d", len(all))
	}
	for i, ev := range all {
		if ev.Seq != int64(i+1) {
			t.Fatalf("ordering broken at index %d: seq %d (full: %+v)", i, ev.Seq, all)
		}
	}

	archiveDir := filepath.Join(filepath.Dir(s.Path()), ArchiveDirName)
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		t.Fatalf("reading archive dir failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 archive segments, got %d", len(entries))
	}
}

func TestMaybeRotateSizeGate(t *testing.T) {
	s := storeForTest(t)

	// Missing journal: cheap no-op.
	if res, err := s.MaybeRotate(DefaultRotateThreshold); err != nil || res != nil {
		t.Fatalf("expected no-op on missing journal, got res=%+v err=%v", res, err)
	}

	if err := s.Append(buildLifecycleEvents()); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Default threshold: file is far below the size gate — no rotation.
	if res, err := s.MaybeRotate(DefaultRotateThreshold); err != nil || res != nil {
		t.Fatalf("expected size gate to skip rotation, got res=%+v err=%v", res, err)
	}

	// Tiny threshold: gate opens and the event count (7 > 2) triggers rotation.
	res, err := s.MaybeRotate(2)
	if err != nil {
		t.Fatalf("MaybeRotate failed: %v", err)
	}
	if res == nil {
		t.Fatal("expected rotation once size gate opens and count exceeds threshold")
	}
	if res.ArchivedEvents != 7 {
		t.Errorf("expected 7 archived events, got %d", res.ArchivedEvents)
	}
}
