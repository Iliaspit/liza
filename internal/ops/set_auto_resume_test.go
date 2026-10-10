package ops

import (
	"reflect"
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

		if _, err := SetAutoResume(tmpDir, true, "operator[test]"); err != nil {
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

		if _, err := SetAutoResume(tmpDir, false, "operator[test]"); err != nil {
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
		if _, err := SetAutoResume(tmpDir, true, "operator[test]"); err != nil {
			t.Fatalf("first SetAutoResume(true) error: %v", err)
		}
		if _, err := SetAutoResume(tmpDir, true, "operator[test]"); err != nil {
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

func TestSetAutoResumeResultAndPreservation(t *testing.T) {
	root := t.TempDir()
	file, _ := testhelpers.SetupLizaDir(t, root)
	before := testhelpers.CreateValidState()
	before.Config.AutoResume = false
	testhelpers.WriteInitialState(t, file, before)
	result, err := SetAutoResume(root, true, "operator[governance-v20]")
	if err != nil {
		t.Fatal(err)
	}
	if result.Previous || !result.Enabled || result.ChangedBy != "operator[governance-v20]" {
		t.Fatalf("wrong result: %+v", result)
	}
	got, err := db.For(file).Read()
	if err != nil {
		t.Fatal(err)
	}
	before.Config.AutoResume = true
	if !reflect.DeepEqual(got, before) {
		t.Fatal("setter changed state beyond auto_resume")
	}
	result, err = SetAutoResume(root, true, "operator[governance-v20]")
	if err != nil || !result.Previous {
		t.Fatalf("idempotent result: %+v, %v", result, err)
	}
	if _, err := SetAutoResume(root, false, ""); err == nil {
		t.Fatal("missing actor accepted")
	}
}
