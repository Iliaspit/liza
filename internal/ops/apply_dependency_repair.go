package ops

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/log"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/statevalidate"
)

// AppliedDependencyUpdate reports the committed canonical dependency state for
// one task in a declarative repair.
type AppliedDependencyUpdate struct {
	TaskID                       string                      `json:"task_id"`
	CanonicalDependencies        []string                    `json:"canonical_dependencies"`
	CanonicalDependencyContracts []models.DependencyContract `json:"canonical_dependency_contracts,omitempty"`
}

// ApplyDependencyRepairResult contains the committed declarative repair batch.
type ApplyDependencyRepairResult struct {
	SourceTaskID string                    `json:"source_task_id"`
	Updates      []AppliedDependencyUpdate `json:"updates"`
	Warnings     []string                  `json:"warnings,omitempty"`
}

func (r *ApplyDependencyRepairResult) GetWarnings() []string {
	if r == nil {
		return nil
	}
	return r.Warnings
}

type preparedDependencyUpdate struct {
	task               *models.Task
	requested          models.DependencyUpdate
	canonical          []string
	canonicalContracts []models.DependencyContract
	contractsSpecified bool
}

// ApplyDependencyRepair consumes one blocked task's declarative repair request
// and commits every requested dependency list in one validated transaction.
func ApplyDependencyRepair(projectRoot, sourceTaskID, reason, agentID string) (*ApplyDependencyRepairResult, error) {
	return applyDependencyRepairWithOptionalAuthority(projectRoot, sourceTaskID, reason, agentID, nil)
}

// ApplyDependencyRepairWithAuthority fences the complete repair batch with the
// orchestrator's registration generation.
func ApplyDependencyRepairWithAuthority(projectRoot, sourceTaskID, reason string, authority models.AgentAuthority) (*ApplyDependencyRepairResult, error) {
	return applyDependencyRepairWithOptionalAuthority(projectRoot, sourceTaskID, reason, authority.ID, &authority)
}

func applyDependencyRepairWithOptionalAuthority(projectRoot, sourceTaskID, reason, agentID string, authority *models.AgentAuthority) (*ApplyDependencyRepairResult, error) {
	if sourceTaskID == "" {
		return nil, &PreconditionError{Reason: "blocked task ID is required"}
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

	var result ApplyDependencyRepairResult
	now := time.Now().UTC()
	err = lifecycleMutation(bb, authority)(func(state *models.State) error {
		source := state.FindTask(sourceTaskID)
		if source == nil {
			return &errors.NotFoundError{Entity: "task", ID: sourceTaskID}
		}
		if source.Status != models.TaskStatusBlocked {
			return &PreconditionError{Reason: fmt.Sprintf("cannot apply dependency repair from task %s in status %s (must be BLOCKED)", sourceTaskID, source.Status)}
		}
		if source.RepairRequest == nil {
			return &PreconditionError{Reason: fmt.Sprintf("blocked task %s has no repair request", sourceTaskID)}
		}

		request, err := normalizeRepairRequest(source.RepairRequest, sourceTaskID)
		if err != nil {
			return err
		}
		if request.Operation != models.RepairOperationApplyDependencyRepair {
			return &PreconditionError{Reason: fmt.Sprintf("blocked task %s repair request operation is %q, want %q", sourceTaskID, request.Operation, models.RepairOperationApplyDependencyRepair)}
		}

		prepared := make([]preparedDependencyUpdate, 0, len(request.DependencyUpdates))
		for _, update := range request.DependencyUpdates {
			task := state.FindTask(update.TaskID)
			if task == nil {
				return &errors.NotFoundError{Entity: "dependency repair task", ID: update.TaskID}
			}
			if task.Status.IsTerminal() {
				return &PreconditionError{Reason: fmt.Sprintf("cannot apply dependency repair to terminal task %s (%s)", task.ID, task.Status)}
			}
			if !slices.Equal(task.DependsOn, update.ExpectedDependsOn) {
				return &PreconditionError{Reason: fmt.Sprintf("task %s dependencies changed since the repair request was created: got %v, expected %v", task.ID, task.DependsOn, update.ExpectedDependsOn)}
			}
			contractsSpecified := state.DependencyContractVersion >= models.DependencyContractVersion || update.ExpectedDependencyContracts != nil || update.DesiredDependencyContracts != nil
			if contractsSpecified && !slices.Equal(task.DependencyContracts, update.ExpectedDependencyContracts) {
				return &PreconditionError{Reason: fmt.Sprintf("task %s dependency contracts changed since the repair request was created", task.ID)}
			}
			for _, dependencyID := range update.DesiredDependsOn {
				if dependencyID == task.ID {
					return &PreconditionError{Reason: fmt.Sprintf("task %s cannot depend on itself", task.ID)}
				}
				if state.FindTask(dependencyID) == nil {
					return &PreconditionError{Reason: fmt.Sprintf("desired dependency %q for task %s does not exist", dependencyID, task.ID)}
				}
			}

			canonical, _, err := canonicalizeConcreteDependencyList(state, resolver, task.ID, task.RolePair, update.DesiredDependsOn)
			if err != nil {
				return err
			}
			canonicalContracts := slices.Clone(task.DependencyContracts)
			if contractsSpecified {
				canonicalContracts, _, err = canonicalizeConcreteDependencyContracts(state, slices.Clone(update.DesiredDependencyContracts))
				if err != nil {
					return err
				}
			}
			prepared = append(prepared, preparedDependencyUpdate{
				task:               task,
				requested:          update,
				canonical:          append([]string{}, canonical...),
				canonicalContracts: canonicalContracts,
				contractsSpecified: contractsSpecified,
			})
		}

		affectedTaskIDs := make([]string, len(prepared))
		for i, update := range prepared {
			affectedTaskIDs[i] = update.task.ID
		}

		updates := make([]AppliedDependencyUpdate, 0, len(prepared))
		sourceUpdated := false
		for _, update := range prepared {
			update.task.DependsOn = append([]string{}, update.canonical...)
			if update.contractsSpecified {
				update.task.DependencyContracts = slices.Clone(update.canonicalContracts)
			}
			sourceUpdated = sourceUpdated || update.task.ID == sourceTaskID
			note := fmt.Sprintf("applied dependency repair requested by %s", sourceTaskID)
			extra := map[string]any{
				"manual":                 true,
				"operation":              models.RepairOperationApplyDependencyRepair,
				"repair_source_task":     sourceTaskID,
				"affected_task_ids":      append([]string{}, affectedTaskIDs...),
				"expected_dependencies":  append([]string{}, update.requested.ExpectedDependsOn...),
				"desired_dependencies":   append([]string{}, update.requested.DesiredDependsOn...),
				"canonical_dependencies": append([]string{}, update.canonical...),
				"repair_evidence":        append([]string{}, request.Evidence...),
				"repair_validation":      append([]string{}, request.Validation...),
				"repair_request_cleared": true,
			}
			if update.contractsSpecified {
				extra["expected_dependency_contracts"] = slices.Clone(update.requested.ExpectedDependencyContracts)
				extra["desired_dependency_contracts"] = slices.Clone(update.requested.DesiredDependencyContracts)
				extra["canonical_dependency_contracts"] = slices.Clone(update.canonicalContracts)
			}
			update.task.History = append(update.task.History, models.TaskHistoryEntry{
				Time:   now,
				Event:  models.TaskEventDependenciesRewritten,
				Agent:  &agentID,
				Reason: &reason,
				Note:   &note,
				Extra:  extra,
			})
			updates = append(updates, AppliedDependencyUpdate{
				TaskID:                       update.task.ID,
				CanonicalDependencies:        append([]string{}, update.canonical...),
				CanonicalDependencyContracts: slices.Clone(update.canonicalContracts),
			})
		}
		if !sourceUpdated {
			note := "applied dependency repair without changing the source task dependencies"
			source.History = append(source.History, models.TaskHistoryEntry{
				Time:   now,
				Event:  models.TaskEventDependencyRepairApplied,
				Agent:  &agentID,
				Reason: &reason,
				Note:   &note,
				Extra: map[string]any{
					"manual":                 true,
					"operation":              models.RepairOperationApplyDependencyRepair,
					"affected_task_ids":      append([]string{}, affectedTaskIDs...),
					"repair_evidence":        append([]string{}, request.Evidence...),
					"repair_validation":      append([]string{}, request.Validation...),
					"repair_request_cleared": true,
				},
			})
		}
		source.RepairRequest = nil

		if err := statevalidate.ValidateState(state, projectRoot, false, io.Discard); err != nil {
			return err
		}

		result = ApplyDependencyRepairResult{
			SourceTaskID: sourceTaskID,
			Updates:      updates,
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to apply dependency repair: %w", err)
	}

	updated := make([]string, 0, len(result.Updates))
	for _, update := range result.Updates {
		updated = append(updated, fmt.Sprintf("%s=[%s]", update.TaskID, strings.Join(update.CanonicalDependencies, ",")))
	}
	logEntry := log.Entry{
		Timestamp: now,
		Agent:     agentID,
		Action:    models.RepairOperationApplyDependencyRepair,
		Task:      &sourceTaskID,
		Detail:    fmt.Sprintf("updated=%s: %s", strings.Join(updated, " "), reason),
	}
	if err := log.New(lp.LogPath()).Append(logEntry); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("activity log write failed: %v", err))
	}

	return &result, nil
}
