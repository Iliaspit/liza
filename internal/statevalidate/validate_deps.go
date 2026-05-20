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
// executing tasks must have all dependencies satisfied, and the dependency graph
// must be acyclic. Prevents agents from starting work on tasks whose
// prerequisites are incomplete and detects dependency cycles that would
// deadlock the scheduler.
func validateDependencies(state *models.State, projectRoot string, skipSpecFileCheck bool, resolver *pipeline.Resolver, cfg *pipeline.PipelineConfig, warnWriter io.Writer) error {
	taskIDs := buildTaskIDSet(state.Tasks)
	sc := newStatusClassifier(resolver, cfg)
	depResolver := models.NewDependencyResolver(state)

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

		var unmet []string
		for _, depID := range task.DependsOn {
			result := depResolver.Resolve(depID)
			if result.Invalid() {
				return fmt.Errorf("task %s has invalid dependency %s", task.ID, result.Summary())
			}
			if err := validateDependencyDirection(state, resolver, &task, depID, result); err != nil {
				return err
			}
			if result.Kind == models.DependencySatisfiedViaSupersession && warnWriter != nil {
				fmt.Fprintf(warnWriter, "WARNING: task %s dependency %s satisfied via supersession path: %s\n", task.ID, depID, strings.Join(result.Path, " -> "))
			}
			if !result.Satisfied() {
				unmet = append(unmet, result.Summary())
				if !sc.IsExecuting(task.Status) && warnWriter != nil && result.ViaSupersession() {
					fmt.Fprintf(warnWriter, "WARNING: task %s dependency %s is not satisfied via supersession path: %s; blocking: %s\n",
						task.ID, depID, strings.Join(result.Path, " -> "), strings.Join(result.BlockingIDs, ", "))
				}
			}
		}

		// Executing tasks must have all dependencies satisfied.
		if sc.IsExecuting(task.Status) {
			if len(unmet) > 0 {
				return fmt.Errorf("executing task %s has unmet dependencies: %s", task.ID, strings.Join(unmet, ", "))
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

func validateDependencyDirection(state *models.State, resolver *pipeline.Resolver, task *models.Task, depID string, result models.DependencySatisfaction) error {
	if state == nil || resolver == nil || task == nil || task.RolePair == "" {
		return nil
	}
	path := result.Path
	if len(path) == 0 {
		path = []string{depID}
	}
	for _, pathID := range path {
		depTask := state.FindTask(pathID)
		if depTask == nil || depTask.RolePair == "" {
			continue
		}
		downstream, err := resolver.IsRolePairDownstream(task.RolePair, depTask.RolePair)
		if err != nil {
			return err
		}
		if downstream {
			if len(path) > 1 {
				return fmt.Errorf("task %s dependency %s resolves through downstream task %s: role_pair %s is downstream of %s (path: %s)",
					task.ID, depID, depTask.ID, depTask.RolePair, task.RolePair, strings.Join(path, " -> "))
			}
			return fmt.Errorf("task %s has downstream dependency %s: role_pair %s is downstream of %s",
				task.ID, depTask.ID, depTask.RolePair, task.RolePair)
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
