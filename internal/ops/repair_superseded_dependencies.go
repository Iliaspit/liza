package ops

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/log"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/statevalidate"
)

const repairSupersededDependenciesOperation = "repair-superseded-dependencies"

// RepairSupersededDependenciesResult contains the audited dependency cleanup.
type RepairSupersededDependenciesResult struct {
	TaskID               string   `json:"task_id"`
	RemovedDependencies  []string `json:"removed_dependencies"`
	RetainedDependencies []string `json:"retained_dependencies"`
	Warnings             []string `json:"warnings,omitempty"`
}

func (r *RepairSupersededDependenciesResult) GetWarnings() []string {
	if r == nil {
		return nil
	}
	return r.Warnings
}

// RepairSupersededDependencies removes all downstream-role dependency edges
// from one SUPERSEDED task in a single validated state transaction.
func RepairSupersededDependencies(projectRoot, taskID, reason, agentID string) (*RepairSupersededDependenciesResult, error) {
	return repairSupersededDependenciesWithOptionalAuthority(projectRoot, taskID, reason, agentID, nil)
}

// RepairSupersededDependenciesWithAuthority fences the terminal dependency
// repair with the orchestrator's registration generation.
func RepairSupersededDependenciesWithAuthority(projectRoot, taskID, reason string, authority models.AgentAuthority) (*RepairSupersededDependenciesResult, error) {
	return repairSupersededDependenciesWithOptionalAuthority(projectRoot, taskID, reason, authority.ID, &authority)
}

func repairSupersededDependenciesWithOptionalAuthority(projectRoot, taskID, reason, agentID string, authority *models.AgentAuthority) (*RepairSupersededDependenciesResult, error) {
	if taskID == "" {
		return nil, &PreconditionError{Reason: "task ID is required"}
	}
	if reason == "" {
		return nil, &PreconditionError{Reason: "reason is required"}
	}
	if agentID == "" {
		return nil, &PreconditionError{Reason: "orchestrator agent ID is required"}
	}

	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())
	resolver, _, err := loadResolver(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to load pipeline config: %w", err)
	}

	var result RepairSupersededDependenciesResult
	now := time.Now().UTC()
	err = lifecycleMutation(bb, authority)(func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return &errors.NotFoundError{Entity: "task", ID: taskID}
		}
		if task.Status != models.TaskStatusSuperseded {
			return &PreconditionError{Reason: fmt.Sprintf("cannot repair dependencies on task %s in status %s (must be SUPERSEDED)", taskID, task.Status)}
		}

		retained, removed, err := pruneDownstreamDependencies(state, resolver, task)
		if err != nil {
			return err
		}
		if len(removed) == 0 {
			return &PreconditionError{Reason: fmt.Sprintf("task %s has no illegal downstream dependencies", taskID)}
		}

		task.DependsOn = retained
		task.DependencyContracts = removeDependencyContracts(task.DependencyContracts, removed)
		note := fmt.Sprintf("removed downstream dependencies: %s", strings.Join(removed, ", "))
		task.History = append(task.History, models.TaskHistoryEntry{
			Time:   now,
			Event:  models.TaskEventDependenciesRewritten,
			Agent:  &agentID,
			Reason: &reason,
			Note:   &note,
			Extra: map[string]any{
				"manual":                true,
				"operation":             repairSupersededDependenciesOperation,
				"removed_dependencies":  append([]string(nil), removed...),
				"retained_dependencies": append([]string(nil), retained...),
			},
		})

		if err := statevalidate.ValidateState(state, projectRoot, false, io.Discard); err != nil {
			return err
		}

		result = RepairSupersededDependenciesResult{
			TaskID:               taskID,
			RemovedDependencies:  append([]string(nil), removed...),
			RetainedDependencies: append([]string(nil), retained...),
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to repair superseded dependencies: %w", err)
	}

	logEntry := log.Entry{
		Timestamp: now,
		Agent:     agentID,
		Action:    repairSupersededDependenciesOperation,
		Task:      &taskID,
		Detail: fmt.Sprintf(
			"removed=%s retained=%s: %s",
			strings.Join(result.RemovedDependencies, ","),
			strings.Join(result.RetainedDependencies, ","),
			reason,
		),
	}
	if err := log.New(lp.LogPath()).Append(logEntry); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("activity log write failed: %v", err))
	}

	return &result, nil
}
