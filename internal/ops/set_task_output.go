package ops

import (
	"fmt"
	"strings"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/statevalidate"
)

// SetTaskOutputInput contains the parameters for setting output entries on a task.
type SetTaskOutputInput struct {
	TaskID  string
	AgentID string
	Output  []models.OutputEntry
}

// SetTaskOutput sets the output[] entries on a task. The task must exist, be assigned
// to the given agent, and be in an executing state (IMPLEMENTING, CODE_PLANNING, or
// a pipeline-defined executing status). Overwrites any existing output (idempotent).
func SetTaskOutput(projectRoot string, input *SetTaskOutputInput) error {
	if input.TaskID == "" {
		return &PreconditionError{Reason: "task_id is required"}
	}
	if input.AgentID == "" {
		return &PreconditionError{Reason: "agent_id is required"}
	}
	for i, entry := range input.Output {
		if entry.Desc == "" {
			return &PreconditionError{Reason: fmt.Sprintf("output[%d].desc is required", i)}
		}
		if entry.DoneWhen == "" {
			return &PreconditionError{Reason: fmt.Sprintf("output[%d].done_when is required", i)}
		}
		if entry.Scope == "" {
			return &PreconditionError{Reason: fmt.Sprintf("output[%d].scope is required", i)}
		}
		if err := models.ValidateKind(entry.Kind); err != nil {
			return &PreconditionError{Reason: fmt.Sprintf("output[%d].%s", i, err.Error())}
		}
		if err := models.ValidateDependsOn(entry.DependsOn, i, len(input.Output)); err != nil {
			return &PreconditionError{Reason: err.Error()}
		}
		if err := validateTaskDependsOn(entry.TaskDependsOn, i); err != nil {
			return &PreconditionError{Reason: err.Error()}
		}
		for _, ref := range []struct {
			field string
			value string
		}{
			{field: "spec_ref", value: entry.SpecRef},
			{field: "epic_ref", value: entry.EpicRef},
			{field: "plan_ref", value: entry.PlanRef},
			{field: "arch_ref", value: entry.ArchRef},
		} {
			if err := statevalidate.ValidateArtifactRefScalar(fmt.Sprintf("output[%d].%s", i, ref.field), ref.value, input.TaskID); err != nil {
				return &PreconditionError{Reason: err.Error()}
			}
		}
	}

	// Normalize spec_ref and plan_ref on each output entry to strip worktree prefixes.
	// Mutates input.Output in-place; callers do not reuse the slice.
	for i := range input.Output {
		input.Output[i].SpecRef = paths.NormalizeSpecRef(input.Output[i].SpecRef)
		input.Output[i].EpicRef = paths.NormalizeSpecRef(input.Output[i].EpicRef)
		input.Output[i].PlanRef = paths.NormalizeSpecRef(input.Output[i].PlanRef)
		input.Output[i].ArchRef = paths.NormalizeSpecRef(input.Output[i].ArchRef)
		input.Output[i].TaskDependsOn = normalizeTaskDependsOn(input.Output[i].TaskDependsOn)
	}

	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())

	// Collect pipeline executing statuses
	resolver, _, err := loadResolver(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to load pipeline config: %w", err)
	}
	var pipelineExecuting []models.TaskStatus
	for _, rpName := range resolver.RolePairNames() {
		if es, err := resolver.ExecutingStatus(rpName); err == nil {
			pipelineExecuting = append(pipelineExecuting, es)
		}
	}

	return bb.Modify(func(state *models.State) error {
		task := state.FindTask(input.TaskID)
		if task == nil {
			return fmt.Errorf("task %s not found", input.TaskID)
		}

		if !isExecutingStatus(task.Status, pipelineExecuting) {
			return &PreconditionError{Reason: fmt.Sprintf("task %s is not in an executing state (current status: %s)", input.TaskID, task.Status)}
		}

		if task.AssignedTo == nil || *task.AssignedTo != input.AgentID {
			currentAgent := "none"
			if task.AssignedTo != nil {
				currentAgent = *task.AssignedTo
			}
			return &PreconditionError{Reason: fmt.Sprintf("task %s is not assigned to agent %s (currently assigned to: %s)", input.TaskID, input.AgentID, currentAgent)}
		}

		for i, entry := range input.Output {
			for _, depID := range entry.TaskDependsOn {
				if state.FindTask(depID) == nil {
					return &PreconditionError{Reason: fmt.Sprintf("output[%d].task_depends_on references non-existent task %q", i, depID)}
				}
			}
		}

		consumerRolePairs, err := resolver.OutputConsumerRolePairs(task.RolePair)
		if err != nil {
			return err
		}
		for _, consumerRolePair := range consumerRolePairs {
			for i, entry := range input.Output {
				if err := validateDependencyDirection(state, resolver, fmt.Sprintf("%s output[%d]", task.ID, i), consumerRolePair, entry.TaskDependsOn); err != nil {
					return err
				}
			}
		}

		task.Output = input.Output
		return nil
	})
}

func validateTaskDependsOn(deps []string, entryIndex int) error {
	for _, depID := range deps {
		trimmed := strings.TrimSpace(depID)
		if trimmed == "" {
			continue
		}
		if err := paths.ValidateTaskID(trimmed); err != nil {
			return fmt.Errorf("output[%d].task_depends_on contains invalid task ID %q: %w", entryIndex, depID, err)
		}
	}
	return nil
}

func normalizeTaskDependsOn(deps []string) []string {
	normalized := make([]string, 0, len(deps))
	for _, depID := range deps {
		trimmed := strings.TrimSpace(depID)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	return normalized
}
