package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestSetDiscoveryDisposition(t *testing.T) {
	t.Parallel()

	t.Run("sets disposition on existing discovery", func(t *testing.T) {
		tmpDir := t.TempDir()
		stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

		state := testhelpers.CreateValidState()
		state.Tasks = []models.Task{
			{ID: "task-5", Status: "DRAFT_CODE", Description: "Target", DoneWhen: "Done", Scope: "Test"},
		}
		state.Discovered = []models.Discovery{
			{
				ID:          "disc-1",
				By:          "coder-1",
				Description: "Found unused API endpoint",
				Urgency:     "immediate",
				Created:     time.Now().UTC(),
			},
		}
		testhelpers.WriteInitialState(t, stateFile, state)

		err := SetDiscoveryDisposition(tmpDir, "disc-1", "task-5")
		if err != nil {
			t.Fatalf("SetDiscoveryDisposition() error: %v", err)
		}

		readState, err := db.New(stateFile).Read()
		if err != nil {
			t.Fatalf("Read state: %v", err)
		}
		if readState.Discovered[0].ConvertedToTask == nil {
			t.Fatal("ConvertedToTask should be set")
		}
		if *readState.Discovered[0].ConvertedToTask != "task-5" {
			t.Errorf("ConvertedToTask = %q, want %q", *readState.Discovered[0].ConvertedToTask, "task-5")
		}
	})

	t.Run("deferred disposition", func(t *testing.T) {
		tmpDir := t.TempDir()
		stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

		state := testhelpers.CreateValidState()
		state.Discovered = []models.Discovery{
			{ID: "disc-2", Description: "Low priority issue", Created: time.Now().UTC()},
		}
		testhelpers.WriteInitialState(t, stateFile, state)

		err := SetDiscoveryDisposition(tmpDir, "disc-2", "deferred")
		if err != nil {
			t.Fatalf("SetDiscoveryDisposition() error: %v", err)
		}

		readState, _ := db.New(stateFile).Read()
		if *readState.Discovered[0].ConvertedToTask != "deferred" {
			t.Errorf("ConvertedToTask = %q, want %q", *readState.Discovered[0].ConvertedToTask, "deferred")
		}
	})

	t.Run("discovery not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

		state := testhelpers.CreateValidState()
		testhelpers.WriteInitialState(t, stateFile, state)

		err := SetDiscoveryDisposition(tmpDir, "nonexistent", "dismissed")
		if err == nil {
			t.Fatal("Expected error for nonexistent discovery")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Error = %q, want to contain 'not found'", err.Error())
		}
	})

	t.Run("empty discovery_id rejected", func(t *testing.T) {
		tmpDir := t.TempDir()
		testhelpers.SetupLizaDir(t, tmpDir)

		err := SetDiscoveryDisposition(tmpDir, "", "dismissed")
		if err == nil {
			t.Fatal("Expected error for empty discovery_id")
		}
	})

	t.Run("empty disposition rejected", func(t *testing.T) {
		tmpDir := t.TempDir()
		testhelpers.SetupLizaDir(t, tmpDir)

		err := SetDiscoveryDisposition(tmpDir, "disc-1", "")
		if err == nil {
			t.Fatal("Expected error for empty disposition")
		}
	})

	t.Run("malformed disposition rejected", func(t *testing.T) {
		tmpDir := t.TempDir()
		testhelpers.SetupLizaDir(t, tmpDir)

		// "../escape" fails task ID validation (path traversal)
		err := SetDiscoveryDisposition(tmpDir, "disc-1", "../escape")
		if err == nil {
			t.Fatal("Expected error for invalid disposition")
		}
		if !strings.Contains(err.Error(), "invalid disposition") {
			t.Errorf("Error = %q, want to contain 'invalid disposition'", err.Error())
		}
	})

	t.Run("empty string disposition rejected", func(t *testing.T) {
		tmpDir := t.TempDir()
		testhelpers.SetupLizaDir(t, tmpDir)

		err := SetDiscoveryDisposition(tmpDir, "disc-1", " ")
		if err == nil {
			t.Fatal("Expected error for whitespace disposition")
		}
	})

	t.Run("valid task ID disposition accepted", func(t *testing.T) {
		tmpDir := t.TempDir()
		stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

		state := testhelpers.CreateValidState()
		state.Tasks = []models.Task{
			{ID: "task-5", Status: "DRAFT_CODE", Description: "Target task", DoneWhen: "Done", Scope: "Test"},
		}
		state.Discovered = []models.Discovery{
			{ID: "disc-3", Description: "Test", Created: time.Now().UTC()},
		}
		testhelpers.WriteInitialState(t, stateFile, state)

		err := SetDiscoveryDisposition(tmpDir, "disc-3", "task-5")
		if err != nil {
			t.Fatalf("SetDiscoveryDisposition() error: %v (valid task ID should be accepted)", err)
		}
	})

	t.Run("nonexistent task ID rejected", func(t *testing.T) {
		tmpDir := t.TempDir()
		stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

		state := testhelpers.CreateValidState()
		state.Discovered = []models.Discovery{
			{ID: "disc-4", Description: "Test", Created: time.Now().UTC()},
		}
		testhelpers.WriteInitialState(t, stateFile, state)

		err := SetDiscoveryDisposition(tmpDir, "disc-4", "task-9999")
		if err == nil {
			t.Fatal("Expected error for nonexistent task reference")
		}
		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("Error = %q, want to contain 'does not exist'", err.Error())
		}
	})
}
