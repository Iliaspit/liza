package ops

import (
	"testing"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestSetAutoResume(t *testing.T) {
	t.Parallel()

	t.Run("sets AutoResume to true", func(t *testing.T) {
		tmpDir := t.TempDir()
		stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

		state := testhelpers.CreateValidState()
		state.Config.AutoResume = false
		testhelpers.WriteInitialState(t, stateFile, state)

		if err := SetAutoResume(tmpDir, true); err != nil {
			t.Fatalf("SetAutoResume(true) error: %v", err)
		}

		bb := db.New(stateFile)
		got, err := bb.Read()
		if err != nil {
			t.Fatalf("Read() error: %v", err)
		}
		if !got.Config.AutoResume {
			t.Error("expected AutoResume=true, got false")
		}
	})

	t.Run("sets AutoResume to false", func(t *testing.T) {
		tmpDir := t.TempDir()
		stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

		state := testhelpers.CreateValidState()
		state.Config.AutoResume = true
		testhelpers.WriteInitialState(t, stateFile, state)

		if err := SetAutoResume(tmpDir, false); err != nil {
			t.Fatalf("SetAutoResume(false) error: %v", err)
		}

		bb := db.New(stateFile)
		got, err := bb.Read()
		if err != nil {
			t.Fatalf("Read() error: %v", err)
		}
		if got.Config.AutoResume {
			t.Error("expected AutoResume=false, got true")
		}
	})

	t.Run("idempotent", func(t *testing.T) {
		tmpDir := t.TempDir()
		stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

		state := testhelpers.CreateValidState()
		state.Config.AutoResume = true
		testhelpers.WriteInitialState(t, stateFile, state)

		// Call twice with same value — should succeed both times.
		if err := SetAutoResume(tmpDir, true); err != nil {
			t.Fatalf("first SetAutoResume(true) error: %v", err)
		}
		if err := SetAutoResume(tmpDir, true); err != nil {
			t.Fatalf("second SetAutoResume(true) error: %v", err)
		}

		bb := db.New(stateFile)
		got, err := bb.Read()
		if err != nil {
			t.Fatalf("Read() error: %v", err)
		}
		if !got.Config.AutoResume {
			t.Error("expected AutoResume=true after idempotent calls, got false")
		}
	})
}
