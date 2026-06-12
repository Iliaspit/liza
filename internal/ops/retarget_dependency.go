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

const retargetDependencyOperation = "retarget-dependency"

// RetargetDependencyResult contains the outcome of retargeting one task
// dependency edge.
type RetargetDependencyResult struct {
	TaskID                string   `json:"task_id"`
	OldDependency         string   `json:"old_dependency"`
	NewDependencies       []string `json:"new_dependencies"`
	CanonicalDependencies []string `json:"canonical_dependencies"`
	RepairRequestCleared  bool     `json:"repair_request_cleared"`
	Warnings              []string `json:"warnings,omitempty"`
}

func (r *RetargetDependencyResult) GetWarnings() []string {
	if r == nil {
		return nil
	}
	return r.Warnings
}

// RetargetDependency replaces one non-terminal task's direct depends_on edge
// with one or more direct dependencies. It is a metadata repair operation: it
// does not unblock the task or execute repair validation commands.
func RetargetDependency(projectRoot, taskID, oldDependency string, newDependencies []string, reason, agentID string) (*RetargetDependencyResult, error) {
	if taskID == "" {
		return nil, &PreconditionError{Reason: "task ID is required"}
	}
	if oldDependency == "" {
		return nil, &PreconditionError{Reason: "old dependency is required"}
	}
	if reason == "" {
		return nil, &PreconditionError{Reason: "reason is required"}
	}
	if agentID == "" {
		return nil, &PreconditionError{Reason: "orchestrator agent ID is required"}
	}

	normalizedNewDeps, err := normalizeRetargetNewDependencies(newDependencies)
	if err != nil {
		return nil, err
	}

	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())
	resolver, _, err := loadResolver(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to load pipeline config: %w", err)
	}

	var result RetargetDependencyResult
	now := time.Now().UTC()

	err = bb.ModifyOp("retarget_dependency", func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return &errors.NotFoundError{Entity: "task", ID: taskID}
		}
		if task.Status.IsTerminal() {
			return &PreconditionError{Reason: fmt.Sprintf("cannot retarget dependencies on terminal task %s (%s)", taskID, task.Status)}
		}
		if !slices.Contains(task.DependsOn, oldDependency) {
			return &PreconditionError{Reason: fmt.Sprintf("task %s does not depend on %s", taskID, oldDependency)}
		}
		for _, depID := range normalizedNewDeps {
			if depID == task.ID {
				return &PreconditionError{Reason: fmt.Sprintf("task %s cannot depend on itself", task.ID)}
			}
			if state.FindTask(depID) == nil {
				return &PreconditionError{Reason: fmt.Sprintf("new dependency %q does not exist", depID)}
			}
		}

		replaced := replaceDependency(task.DependsOn, oldDependency, normalizedNewDeps)
		canonical, _, err := canonicalizeConcreteDependencyList(state, resolver, task.ID, task.RolePair, replaced)
		if err != nil {
			return err
		}
		if len(canonical) == 0 {
			return &PreconditionError{Reason: "retarget-dependency cannot remove all dependencies; use an explicit dependency-removal operation instead"}
		}

		repairExtra := matchingRetargetRepairExtra(task.RepairRequest, taskID, oldDependency, normalizedNewDeps)
		repairRequestCleared := repairExtra != nil
		if repairRequestCleared {
			task.RepairRequest = nil
		}

		task.DependsOn = canonical
		extra := map[string]any{
			"manual":                 true,
			"operation":              retargetDependencyOperation,
			"old_dependency":         oldDependency,
			"new_dependencies":       append([]string(nil), normalizedNewDeps...),
			"canonical_dependencies": append([]string(nil), canonical...),
			"repair_request_cleared": repairRequestCleared,
		}
		for key, value := range repairExtra {
			extra[key] = value
		}
		note := fmt.Sprintf("retargeted dependency %s -> %s", oldDependency, strings.Join(normalizedNewDeps, ", "))
		task.History = append(task.History, models.TaskHistoryEntry{
			Time:   now,
			Event:  models.TaskEventDependenciesRewritten,
			Agent:  &agentID,
			Reason: &reason,
			Note:   &note,
			Extra:  extra,
		})

		if err := statevalidate.ValidateState(state, projectRoot, false, io.Discard); err != nil {
			return err
		}

		result = RetargetDependencyResult{
			TaskID:                taskID,
			OldDependency:         oldDependency,
			NewDependencies:       append([]string(nil), normalizedNewDeps...),
			CanonicalDependencies: append([]string(nil), canonical...),
			RepairRequestCleared:  repairRequestCleared,
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to retarget dependency: %w", err)
	}

	logger := log.New(lp.LogPath())
	logEntry := log.Entry{
		Timestamp: now,
		Agent:     agentID,
		Action:    retargetDependencyOperation,
		Task:      &taskID,
		Detail:    fmt.Sprintf("%s -> %s: %s", oldDependency, strings.Join(normalizedNewDeps, ","), reason),
	}
	if err := logger.Append(logEntry); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("activity log write failed: %v", err))
	}

	return &result, nil
}

func normalizeRetargetNewDependencies(values []string) ([]string, error) {
	var normalized []string
	seen := make(map[string]bool)
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			depID := strings.TrimSpace(part)
			if depID == "" {
				return nil, &PreconditionError{Reason: "new dependency entries cannot be empty"}
			}
			if seen[depID] {
				return nil, &PreconditionError{Reason: fmt.Sprintf("duplicate new dependency %q", depID)}
			}
			seen[depID] = true
			normalized = append(normalized, depID)
		}
	}
	if len(normalized) == 0 {
		return nil, &PreconditionError{Reason: "at least one new dependency is required"}
	}
	return normalized, nil
}

func replaceDependency(deps []string, oldDependency string, newDependencies []string) []string {
	replaced := make([]string, 0, len(deps)+len(newDependencies)-1)
	for _, depID := range deps {
		if depID == oldDependency {
			replaced = append(replaced, newDependencies...)
			continue
		}
		replaced = append(replaced, depID)
	}
	return dedupeStrings(replaced)
}

func matchingRetargetRepairExtra(request *models.RepairRequest, taskID, oldDependency string, newDependencies []string) map[string]any {
	if request == nil || request.Operation != retargetDependencyOperation {
		return nil
	}
	if !repairCommandMatchesRetarget(request.Command, taskID, oldDependency, newDependencies) {
		return nil
	}

	return map[string]any{
		"repair_operation":  request.Operation,
		"repair_target":     request.Target,
		"repair_command":    request.Command,
		"repair_evidence":   append([]string(nil), request.Evidence...),
		"repair_validation": append([]string(nil), request.Validation...),
	}
}

func repairCommandMatchesRetarget(command, taskID, oldDependency string, newDependencies []string) bool {
	fields := strings.Fields(command)
	for i := 0; i+3 < len(fields); i++ {
		if fields[i] != retargetDependencyOperation {
			continue
		}
		if fields[i+1] != taskID || fields[i+2] != oldDependency {
			continue
		}
		commandDeps, err := normalizeRetargetNewDependencies([]string{fields[i+3]})
		if err != nil {
			return false
		}
		return slices.Equal(commandDeps, newDependencies)
	}
	return false
}
