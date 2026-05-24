package ops

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/pipeline"
)

func rewriteActiveDependents(state *models.State, resolver *pipeline.Resolver, targetID string, replacements []string, agentID string, now time.Time) error {
	for i := range state.Tasks {
		task := &state.Tasks[i]
		if task.ID == targetID {
			continue
		}

		depsChanged := false
		if !task.Status.IsTerminal() {
			rewritten, changed, err := canonicalizeConcreteDependencyList(state, resolver, task.ID, task.RolePair, task.DependsOn)
			if err != nil {
				return err
			}
			if changed {
				task.DependsOn = rewritten
				depsChanged = true
			}
		}

		outputChanged := false
		if operationalOutputMayBeConsumed(state, resolver, task) {
			changed, err := canonicalizeOutputTaskDependsOnForConsumers(state, resolver, task)
			if err != nil {
				return err
			}
			outputChanged = changed
		}

		if !depsChanged && !outputChanged {
			continue
		}

		note := fmt.Sprintf("rewrote dependencies after %s became terminal", targetID)
		task.History = append(task.History, models.TaskHistoryEntry{
			Time:  now,
			Event: models.TaskEventDependenciesRewritten,
			Agent: &agentID,
			Note:  &note,
			Extra: map[string]any{
				"removed_dependency":       targetID,
				"replacement_dependencies": replacements,
				"rewrote_depends_on":       depsChanged,
				"rewrote_task_depends_on":  outputChanged,
			},
		})
	}
	return nil
}

func canonicalizeTaskDependsOnForTransition(state *models.State, resolver *pipeline.Resolver, task *models.Task) error {
	if task == nil || len(task.DependsOn) == 0 {
		return nil
	}
	rewritten, changed, err := canonicalizeConcreteDependencyList(state, resolver, task.ID, task.RolePair, task.DependsOn)
	if err != nil {
		return err
	}
	if changed {
		task.DependsOn = rewritten
	}
	return nil
}

func canonicalizedOutputTaskDependsOnForTarget(state *models.State, resolver *pipeline.Resolver, task *models.Task, targetRolePair string) ([]models.OutputEntry, bool, error) {
	if task == nil || len(task.Output) == 0 {
		return nil, false, nil
	}
	output := append([]models.OutputEntry(nil), task.Output...)
	changedAny := false
	for i := range output {
		dependentID := fmt.Sprintf("%s output[%d]", task.ID, i)
		rewritten, changed, err := canonicalizeConcreteDependencyList(state, resolver, dependentID, targetRolePair, output[i].TaskDependsOn)
		if err != nil {
			return nil, false, err
		}
		if changed {
			output[i].TaskDependsOn = rewritten
			changedAny = true
		}
	}
	return output, changedAny, nil
}

func canonicalizeOutputTaskDependsOnForConsumers(state *models.State, resolver *pipeline.Resolver, task *models.Task) (bool, error) {
	consumerRolePairs, err := resolver.OutputConsumerRolePairs(task.RolePair)
	if err != nil {
		return false, err
	}
	changedAny := false
	for i := range task.Output {
		dependentID := fmt.Sprintf("%s output[%d]", task.ID, i)
		rewritten := task.Output[i].TaskDependsOn
		entryChanged := false
		for _, consumerRolePair := range consumerRolePairs {
			var changed bool
			var err error
			rewritten, changed, err = canonicalizeConcreteDependencyList(state, resolver, dependentID, consumerRolePair, rewritten)
			if err != nil {
				return false, err
			}
			entryChanged = entryChanged || changed
		}
		if entryChanged {
			for _, consumerRolePair := range consumerRolePairs {
				if err := validateDependencyDirection(state, resolver, dependentID, consumerRolePair, rewritten); err != nil {
					return false, err
				}
			}
			task.Output[i].TaskDependsOn = rewritten
			changedAny = true
		}
	}
	return changedAny, nil
}

func canonicalizeConcreteDependencyList(state *models.State, resolver *pipeline.Resolver, dependentID, dependentRolePair string, deps []string) ([]string, bool, error) {
	if len(deps) == 0 {
		return deps, false, nil
	}
	rewritten := make([]string, 0, len(deps))
	changed := false
	for _, dep := range deps {
		canonical, depChanged, err := canonicalizeDependencyID(state, dep, nil)
		if err != nil {
			return nil, false, &PreconditionError{Reason: fmt.Sprintf("cannot rewrite dependency for %s: %v", dependentID, err)}
		}
		changed = changed || depChanged
		rewritten = append(rewritten, canonical...)
	}
	if !changed {
		return deps, false, nil
	}
	rewritten = dedupeStrings(rewritten)
	if err := validateDependencyDirection(state, resolver, dependentID, dependentRolePair, rewritten); err != nil {
		return nil, false, err
	}
	return rewritten, true, nil
}

func canonicalizeDependencyID(state *models.State, depID string, visiting map[string]bool) ([]string, bool, error) {
	if state == nil {
		return []string{depID}, false, nil
	}
	depTask := state.FindTask(depID)
	if depTask == nil {
		return []string{depID}, false, nil
	}
	switch depTask.Status {
	case models.TaskStatusAbandoned:
		return nil, true, nil
	case models.TaskStatusSuperseded:
		if visiting == nil {
			visiting = make(map[string]bool)
		}
		if visiting[depID] {
			return nil, false, fmt.Errorf("supersession cycle at %s", depID)
		}
		if len(depTask.SupersededBy) == 0 {
			return nil, true, nil
		}
		visiting[depID] = true
		var rewritten []string
		for _, replacementID := range depTask.SupersededBy {
			if state.FindTask(replacementID) == nil {
				return nil, false, fmt.Errorf("replacement dependency %s does not exist", replacementID)
			}
			canonical, _, err := canonicalizeDependencyID(state, replacementID, visiting)
			if err != nil {
				return nil, false, err
			}
			rewritten = append(rewritten, canonical...)
		}
		delete(visiting, depID)
		return dedupeStrings(rewritten), true, nil
	default:
		return []string{depID}, false, nil
	}
}

func operationalOutputMayBeConsumed(state *models.State, resolver *pipeline.Resolver, task *models.Task) bool {
	if task == nil || len(task.Output) == 0 || task.Status == models.TaskStatusSuperseded || task.Status == models.TaskStatusAbandoned {
		return false
	}
	if task.Status != models.TaskStatusMerged {
		return true
	}
	for _, transition := range resolver.AllTransitions() {
		if transition.Cardinality != "per-subtask" {
			continue
		}
		sourceRolePair, err := resolver.TransitionSourceRolePair(transition.Name)
		if err != nil || sourceRolePair != task.RolePair {
			continue
		}
		if !task.TransitionsExecuted[transition.Name] {
			return true
		}
		slug := transition.TaskSlugOrName()
		for i := range task.Output {
			if state.FindTask(perSubtaskChildID(task.ID, slug, i)) == nil {
				return true
			}
		}
	}
	return false
}
