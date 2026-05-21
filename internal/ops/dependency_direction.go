package ops

import (
	"fmt"
	"slices"
	"strings"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/pipeline"
)

func validateDependencyDirection(state *models.State, resolver *pipeline.Resolver, dependentID, dependentRolePair string, deps []string) error {
	if state == nil || resolver == nil || dependentRolePair == "" {
		return nil
	}

	depResolver := models.NewDependencyResolver(state)
	for _, depID := range deps {
		result := depResolver.Resolve(depID)
		depTask := state.FindTask(depID)
		if depTask == nil || depTask.RolePair == "" {
			continue
		}
		downstream, err := resolver.IsRolePairDownstream(dependentRolePair, depTask.RolePair)
		if err != nil {
			return err
		}
		if downstream {
			return &PreconditionError{Reason: dependencyDirectionError(dependentID, dependentRolePair, depID, result.Path, depTask)}
		}
	}
	return nil
}

func dependencyDirectionError(dependentID, dependentRolePair, depID string, path []string, depTask *models.Task) string {
	if len(path) > 1 {
		return fmt.Sprintf("task %s dependency %s resolves through downstream task %s: role_pair %s is downstream of %s (path: %s)",
			dependentID, depID, depTask.ID, depTask.RolePair, dependentRolePair, strings.Join(path, " -> "))
	}
	return fmt.Sprintf("task %s has downstream dependency %s: role_pair %s is downstream of %s",
		dependentID, depTask.ID, depTask.RolePair, dependentRolePair)
}

func mergeInheritedDeps(current []string, inheritedDeps []string) []string {
	merged := append([]string(nil), current...)
	for _, dep := range inheritedDeps {
		if !slices.Contains(merged, dep) {
			merged = append(merged, dep)
		}
	}
	return merged
}
