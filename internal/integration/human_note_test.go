package integration

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/statevalidate"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestHumanNoteConcurrentAppendPreservesUpdates(t *testing.T) {
	root := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, root)
	testhelpers.SetupPipelineConfig(t, root)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Concurrent human note fixture\n"), 0644); err != nil {
		t.Fatal(err)
	}
	state := testhelpers.CreateValidState()
	state.Goal.SpecRef = "README.md"
	state.Tasks = []models.Task{testhelpers.BuildTaskByStatus("blocked-1", models.TaskStatusBlocked, time.Now().UTC())}
	state.Tasks[0].AssignedTo, state.Tasks[0].Worktree = nil, nil
	state.Sprint.Scope.Planned = []string{"blocked-1"}
	state.HumanNotes = []models.HumanNote{{Timestamp: time.Now().UTC().Add(-time.Hour), For: "blocked-1", Message: "existing"}}
	if err := statevalidate.ValidateState(state, root, false, io.Discard); err != nil {
		t.Fatalf("invalid canonical concurrency fixture: %v", err)
	}
	bb := testhelpers.WriteInitialState(t, statePath, state)
	before, err := bb.Read()
	if err != nil {
		t.Fatal(err)
	}
	const count = 12
	start := make(chan struct{})
	errors := make(chan error, count+1)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		independent := db.New(statePath)
		message := fmt.Sprintf("answer-%d", i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errors <- ops.AddHumanNote(independent, "blocked-1", message)
		}()
	}
	metadataBB := db.New(statePath)
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		errors <- metadataBB.Modify(func(state *models.State) error {
			state.Goal.Description = "concurrent metadata update"
			return nil
		})
	}()
	close(start)
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Errorf("concurrent update failed: %v", err)
		}
	}
	if t.Failed() {
		t.FailNow()
	}
	after, err := db.New(statePath).Read()
	if err != nil {
		t.Fatal(err)
	}
	if err := statevalidate.ValidateState(after, root, false, io.Discard); err != nil {
		t.Fatalf("concurrent updates left invalid state: %v", err)
	}
	if len(after.HumanNotes) != count+1 {
		t.Fatalf("notes = %d, want %d", len(after.HumanNotes), count+1)
	}
	if !reflect.DeepEqual(after.HumanNotes[0], before.HumanNotes[0]) {
		t.Fatal("existing note changed")
	}
	seen := make(map[string]int)
	for _, note := range after.HumanNotes[1:] {
		seen[note.Message]++
		if note.For != "blocked-1" || note.Timestamp.IsZero() || note.Timestamp.Location() != time.UTC || !reflect.DeepEqual(note.Extra, map[string]any{"changed_by": "human"}) {
			t.Fatalf("invalid appended note: %#v", note)
		}
	}
	for i := 0; i < count; i++ {
		if seen[fmt.Sprintf("answer-%d", i)] != 1 {
			t.Fatalf("answer-%d count = %d", i, seen[fmt.Sprintf("answer-%d", i)])
		}
	}
	if after.Goal.Description != "concurrent metadata update" {
		t.Fatal("concurrent metadata update lost")
	}
	after.Goal.Description = before.Goal.Description
	after.HumanNotes = after.HumanNotes[:1]
	if !reflect.DeepEqual(after, before) {
		t.Fatal("concurrent append changed unrelated canonical state")
	}
}
