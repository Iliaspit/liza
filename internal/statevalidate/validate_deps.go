package statevalidate

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/pipeline"
)

// validateDependencies checks referential integrity and ordering constraints
// for task dependencies: every depends_on entry must reference an existing task,
// executing tasks must have all dependencies in MERGED or SUPERSEDED status, and
// the dependency graph must be acyclic. Prevents agents from starting work on
// tasks whose prerequisites are incomplete and detects dependency cycles that
// would deadlock the scheduler.
func validateDependencies(state *models.State, projectRoot string, skipSpecFileCheck bool, resolver *pipeline.Resolver, cfg *pipeline.PipelineConfig) error {
	taskIDs := buildTaskIDSet(state.Tasks)
	sc := newStatusClassifier(resolver, cfg)

	for _, task := range state.Tasks {
		if len(task.DependsOn) == 0 {
			continue
		}

		seenDeps := make(map[string]bool, len(task.DependsOn))
		// All dependencies must reference existing tasks
		for _, depID := range task.DependsOn {
			if strings.TrimSpace(depID) != depID || depID == "" {
				return fmt.Errorf("task %s has invalid depends_on entry %q (must be non-empty and trimmed)", task.ID, depID)
			}
			if depID == task.ID {
				return fmt.Errorf("task %s has depends_on referencing itself", task.ID)
			}
			if seenDeps[depID] {
				return fmt.Errorf("task %s has duplicate depends_on entry %q", task.ID, depID)
			}
			seenDeps[depID] = true
			if !taskIDs[depID] {
				return fmt.Errorf("task %s has depends_on referencing non-existent task '%s'", task.ID, depID)
			}
		}

		// Executing tasks must have all dependencies satisfied (MERGED or SUPERSEDED)
		if sc.IsExecuting(task.Status) {
			var unmet []string
			for _, depID := range task.DependsOn {
				depTask := state.FindTask(depID)
				if depTask != nil && depTask.Status != models.TaskStatusMerged && depTask.Status != models.TaskStatusSuperseded {
					unmet = append(unmet, depID)
				}
			}
			if len(unmet) > 0 {
				return fmt.Errorf("executing task %s has unmet dependencies: %s (must be MERGED or SUPERSEDED)", task.ID, strings.Join(unmet, ", "))
			}
		}
	}

	for _, task := range state.Tasks {
		if len(task.DependsOn) == 0 {
			continue
		}

		visited := make(map[string]bool)
		if err := checkCircular(task.ID, task.ID, visited, state); err != nil {
			return err
		}
	}

	return nil
}

func warnBlockedReasonMissingDependsOn(state *models.State, warnWriter io.Writer) {
	if warnWriter == nil {
		return
	}
	var taskIDs []string
	for _, task := range state.Tasks {
		if task.ID != "" {
			taskIDs = append(taskIDs, task.ID)
		}
	}
	slices.Sort(taskIDs)
	for _, task := range state.Tasks {
		if task.Status != models.TaskStatusBlocked || len(task.DependsOn) > 0 || task.BlockedReason == nil {
			continue
		}
		reason := *task.BlockedReason
		for _, id := range taskIDs {
			if id == "" || id == task.ID {
				continue
			}
			if containsTaskID(reason, id) {
				fmt.Fprintf(warnWriter, "WARNING: BLOCKED task %s blocked_reason references task %s but depends_on is empty; add depends_on so orchestrator can re-wake when the blocker changes\n", task.ID, id)
				break
			}
		}
	}
}

func containsTaskID(text, id string) bool {
	if id == "" {
		return false
	}
	start := 0
	for {
		idx := strings.Index(text[start:], id)
		if idx < 0 {
			return false
		}
		matchStart := start + idx
		matchEnd := matchStart + len(id)
		if hasTaskIDBoundary(text, matchStart-1) && hasTaskIDBoundary(text, matchEnd) {
			return true
		}
		start = matchStart + 1
	}
}

func hasTaskIDBoundary(text string, index int) bool {
	if index < 0 || index >= len(text) {
		return true
	}
	return !isTaskIDChar(text[index])
}

func isTaskIDChar(ch byte) bool {
	return ch >= 'a' && ch <= 'z' ||
		ch >= 'A' && ch <= 'Z' ||
		ch >= '0' && ch <= '9' ||
		ch == '.' ||
		ch == '_' ||
		ch == '-'
}

// checkCircular performs a depth-first traversal of the dependency graph
// starting from 'start', detecting if any path leads back to it. Returns an
// error describing the cycle when one is found.
func checkCircular(start, current string, visited map[string]bool, state *models.State) error {
	task := state.FindTask(current)
	if task == nil || len(task.DependsOn) == 0 {
		return nil
	}

	for _, depID := range task.DependsOn {
		if depID == start {
			return fmt.Errorf("circular dependency detected: %s eventually depends on itself", start)
		}
		if !visited[depID] {
			visited[depID] = true
			if err := checkCircular(start, depID, visited, state); err != nil {
				return err
			}
		}
	}

	return nil
}
