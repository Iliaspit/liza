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
		if task.ID == targetID || task.Status.IsTerminal() {
			continue
		}

		rewritten, changed := rewriteDependencyList(task.DependsOn, targetID, replacements)
		if !changed {
			continue
		}
		if len(replacements) > 0 {
			for _, depID := range rewritten {
				if state.FindTask(depID) == nil {
					return &PreconditionError{Reason: fmt.Sprintf("cannot rewrite dependency for task %s: replacement dependency %s does not exist", task.ID, depID)}
				}
			}
			if err := validateDependencyDirection(state, resolver, task.ID, task.RolePair, rewritten); err != nil {
				return err
			}
		}

		task.DependsOn = rewritten
		note := fmt.Sprintf("rewrote depends_on after %s became terminal", targetID)
		task.History = append(task.History, models.TaskHistoryEntry{
			Time:  now,
			Event: models.TaskEventDependenciesRewritten,
			Agent: &agentID,
			Note:  &note,
			Extra: map[string]any{
				"removed_dependency":       targetID,
				"replacement_dependencies": replacements,
			},
		})
	}
	return nil
}

func rewriteDependencyList(deps []string, targetID string, replacements []string) ([]string, bool) {
	if len(deps) == 0 {
		return deps, false
	}
	rewritten := make([]string, 0, len(deps)+len(replacements))
	changed := false
	for _, dep := range deps {
		if dep != targetID {
			rewritten = append(rewritten, dep)
			continue
		}
		changed = true
		rewritten = append(rewritten, replacements...)
	}
	if !changed {
		return deps, false
	}
	return dedupeStrings(rewritten), true
}
