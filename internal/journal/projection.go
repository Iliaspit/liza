package journal

// TaskStatusProjection is the journal-derived view of every known task's
// current status. It is the first projection used to verify that the shadow
// journal is complete: folding it over the journal must reproduce exactly
// the task statuses in state.yaml.
type TaskStatusProjection map[string]string

// ProjectTaskStatuses folds status-bearing events into the current status of
// every task that appears in the journal. Removed tasks are dropped, matching
// state.yaml semantics (archival/deletion removes the task entry).
//
// A journal.rotated snapshot event re-seeds the projection from its
// task_statuses field — the full fold of everything archived — so folding a
// rotated journal stays exactly equivalent to folding the full history.
func ProjectTaskStatuses(events []Event) TaskStatusProjection {
	proj := TaskStatusProjection{}
	for _, ev := range events {
		switch ev.Type {
		case EventJournalRotated:
			if seed, ok := taskStatusSeed(ev.Fields["task_statuses"]); ok {
				proj = seed
			}
		case EventTaskCreated:
			if status, ok := ev.Fields["status"].(string); ok {
				proj[ev.Task] = status
			}
		case EventTaskStatusChanged:
			if status, ok := ev.Fields["to"].(string); ok {
				proj[ev.Task] = status
			}
		case EventTaskRemoved:
			delete(proj, ev.Task)
		}
	}
	return proj
}

// taskStatusSeed extracts the task_statuses snapshot from a journal.rotated
// event. In-memory events carry map[string]string; events read back from the
// JSONL file carry map[string]any after the JSON round-trip — both forms are
// handled. Non-string statuses are skipped.
func taskStatusSeed(v any) (TaskStatusProjection, bool) {
	switch m := v.(type) {
	case map[string]string:
		seed := make(TaskStatusProjection, len(m))
		for id, status := range m {
			seed[id] = status
		}
		return seed, true
	case TaskStatusProjection:
		seed := make(TaskStatusProjection, len(m))
		for id, status := range m {
			seed[id] = status
		}
		return seed, true
	case map[string]any:
		seed := make(TaskStatusProjection, len(m))
		for id, status := range m {
			if s, ok := status.(string); ok {
				seed[id] = s
			}
		}
		return seed, true
	}
	return nil, false
}

// Diff returns, for each task where the projection disagrees with the given
// statuses, a pair [projected, actual] ("" marks a missing entry). An empty
// result means the journal fully explains the current task statuses.
func (p TaskStatusProjection) Diff(actual map[string]string) map[string][2]string {
	diff := map[string][2]string{}
	for id, status := range actual {
		if p[id] != status {
			diff[id] = [2]string{p[id], status}
		}
	}
	for id, status := range p {
		if _, ok := actual[id]; !ok {
			diff[id] = [2]string{status, ""}
		}
	}
	return diff
}
