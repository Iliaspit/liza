package commands

import (
	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/journal"
	"github.com/liza-mas/liza/internal/paths"
)

// JournalOptions controls JournalCommand filtering and verification.
type JournalOptions struct {
	// Since returns only events with Seq strictly greater than this value.
	Since int64
	// Task filters events to a single task ID ("" disables the filter).
	Task string
	// Limit keeps only the last N matching events (0 disables the limit).
	Limit int
	// Verify additionally checks that folding the journal reproduces the
	// task statuses currently in state.yaml.
	Verify bool
}

// JournalVerification reports the journal/state equivalence check.
type JournalVerification struct {
	Equivalent bool `json:"equivalent"`
	// Diff maps task ID → [projected status, actual status] for divergences.
	Diff map[string][2]string `json:"diff,omitempty"`
}

// JournalResult is the output of JournalCommand.
type JournalResult struct {
	Events       []journal.Event      `json:"events"`
	TotalEvents  int                  `json:"total_events"`
	Verification *JournalVerification `json:"verification,omitempty"`
}

// JournalCommand reads the shadow event journal, applies filters, and
// optionally verifies projection equivalence against state.yaml.
func JournalCommand(projectRoot string, opts JournalOptions) (*JournalResult, error) {
	lp := paths.New(projectRoot)
	store := journal.ForStatePath(lp.StatePath())

	all, err := store.ReadAll()
	if err != nil {
		return nil, err
	}

	result := &JournalResult{TotalEvents: len(all)}

	filtered := make([]journal.Event, 0, len(all))
	for _, ev := range all {
		if ev.Seq <= opts.Since {
			continue
		}
		if opts.Task != "" && ev.Task != opts.Task {
			continue
		}
		filtered = append(filtered, ev)
	}
	if opts.Limit > 0 && len(filtered) > opts.Limit {
		filtered = filtered[len(filtered)-opts.Limit:]
	}
	result.Events = filtered

	if opts.Verify {
		state, err := db.For(lp.StatePath()).Read()
		if err != nil {
			return nil, err
		}
		actual := map[string]string{}
		for _, task := range state.Tasks {
			actual[task.ID] = string(task.Status)
		}
		diff := journal.ProjectTaskStatuses(all).Diff(actual)
		result.Verification = &JournalVerification{
			Equivalent: len(diff) == 0,
		}
		if len(diff) > 0 {
			result.Verification.Diff = diff
		}
	}

	return result, nil
}
