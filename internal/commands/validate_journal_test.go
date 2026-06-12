package commands

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/liza-mas/liza/internal/journal"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestWarnJournalDivergence_ReportsContradiction(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.yaml")

	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		{ID: "task-journaled", Status: "READY"},
		{ID: "task-pre-journal", Status: "MERGED"},
	}

	// Journal knows task-journaled with a contradicting status, and has never
	// seen task-pre-journal (upgraded project) — only the former may warn.
	store := journal.ForStatePath(statePath)
	err := store.Append([]journal.Event{
		{Type: journal.EventTaskCreated, Task: "task-journaled", Fields: map[string]any{"status": "IMPLEMENTING_CODE"}},
	})
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	var warnings strings.Builder
	warnJournalDivergence(statePath, state, &warnings)

	out := warnings.String()
	if !strings.Contains(out, "task-journaled") || !strings.Contains(out, `journal="IMPLEMENTING_CODE"`) {
		t.Errorf("expected divergence warning for task-journaled, got: %q", out)
	}
	if strings.Contains(out, "task-pre-journal") {
		t.Errorf("pre-journal task must not warn, got: %q", out)
	}
}

func TestWarnJournalDivergence_SilentWhenCoherent(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.yaml")

	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{{ID: "task-1", Status: "READY"}}

	store := journal.ForStatePath(statePath)
	err := store.Append([]journal.Event{
		{Type: journal.EventTaskCreated, Task: "task-1", Fields: map[string]any{"status": "READY"}},
	})
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	var warnings strings.Builder
	warnJournalDivergence(statePath, state, &warnings)
	if warnings.Len() != 0 {
		t.Errorf("expected no warnings for coherent journal, got: %q", warnings.String())
	}
}

func TestWarnJournalDivergence_SilentWithoutJournal(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.yaml")

	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{{ID: "task-1", Status: "READY"}}

	var warnings strings.Builder
	warnJournalDivergence(statePath, state, &warnings)
	if warnings.Len() != 0 {
		t.Errorf("expected no warnings without a journal file, got: %q", warnings.String())
	}
}
