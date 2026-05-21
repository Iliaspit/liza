package ops

import (
	"fmt"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/identity"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/roles"
)

// UnblockTaskResult contains the outcome of restoring a repaired BLOCKED task.
type UnblockTaskResult struct {
	TaskID       string            `json:"task_id"`
	FromStatus   models.TaskStatus `json:"from_status"`
	ToStatus     models.TaskStatus `json:"to_status"`
	AssignedTo   string            `json:"assigned_to"`
	LeaseExpires time.Time         `json:"lease_expires"`
}

// UnblockTask restores a repaired BLOCKED task to its role-pair executing state
// and assigns it back to an existing doer so normal submit-for-review flow can
// continue. The orchestrator must run any repair_request.validation commands
// before calling this operation; this op records the prior repair request in
// history for audit but intentionally does not execute arbitrary command text.
func UnblockTask(projectRoot, taskID, assignTo, reason, agentID string) (*UnblockTaskResult, error) {
	if taskID == "" {
		return nil, &PreconditionError{Reason: "task ID is required"}
	}
	if assignTo == "" {
		return nil, &PreconditionError{Reason: "assign-to agent ID is required"}
	}
	if reason == "" {
		return nil, &PreconditionError{Reason: "reason is required"}
	}
	if agentID == "" {
		return nil, &PreconditionError{Reason: "agent ID is required"}
	}
	if err := identity.ValidateRole(agentID, roles.Orchestrator); err != nil {
		return nil, &PreconditionError{Reason: fmt.Sprintf("only orchestrator agents can unblock tasks: %v", err)}
	}

	assignRole, err := identity.ExtractRole(assignTo)
	if err != nil {
		return nil, &PreconditionError{Reason: fmt.Sprintf("invalid assign-to agent ID %s: %v", assignTo, err)}
	}

	resolver, _, err := loadResolver(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to load pipeline config: %w", err)
	}
	pipelineTransitions := BuildPipelineTransitions(resolver)

	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())
	now := time.Now().UTC()

	var result UnblockTaskResult
	err = bb.Modify(func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return &errors.NotFoundError{Entity: "task", ID: taskID}
		}
		if task.Status != models.TaskStatusBlocked {
			return &PreconditionError{Reason: fmt.Sprintf("task must be BLOCKED to unblock, current status: %s", task.Status)}
		}
		if task.RolePair == "" {
			return &PreconditionError{Reason: fmt.Sprintf("task %s has no role_pair set", taskID)}
		}
		expectedDoer, err := resolver.DoerRole(task.RolePair)
		if err != nil {
			return &PreconditionError{Reason: fmt.Sprintf("unrecognized role-pair %q: %v", task.RolePair, err)}
		}
		if assignRole != expectedDoer {
			return &PreconditionError{Reason: fmt.Sprintf("assign-to agent role %q does not match task doer role %q", assignRole, expectedDoer)}
		}
		targetStatus, err := resolver.ExecutingStatus(task.RolePair)
		if err != nil {
			return &PreconditionError{Reason: fmt.Sprintf("unrecognized role-pair %q: %v", task.RolePair, err)}
		}
		if task.Worktree == nil {
			return &PreconditionError{Reason: fmt.Sprintf("task %s has no worktree to resume", taskID)}
		}
		var unmet []string
		for _, dep := range unmetDependencies(task, state) {
			if dep.Invalid() {
				return &PreconditionError{Reason: fmt.Sprintf("task %s has invalid dependency: %s", taskID, dep.Summary())}
			}
			unmet = append(unmet, dep.Summary())
		}
		if len(unmet) > 0 {
			return &PreconditionError{Reason: fmt.Sprintf("task %s has unmet dependencies: %s", taskID, strings.Join(unmet, ", "))}
		}

		agent, exists := state.Agents[assignTo]
		if !exists {
			return &PreconditionError{Reason: fmt.Sprintf("assign-to agent %s is not registered", assignTo)}
		}
		if agent.Role != expectedDoer {
			return &PreconditionError{Reason: fmt.Sprintf("assign-to agent %s has role %q, want %q", assignTo, agent.Role, expectedDoer)}
		}
		if !IsProcessAlive(agent.PID) {
			return &PreconditionError{Reason: fmt.Sprintf("assign-to agent %s has no live process (pid %d)", assignTo, agent.PID)}
		}
		if agent.CurrentTask != nil && *agent.CurrentTask != "" && *agent.CurrentTask != taskID {
			return &PreconditionError{Reason: fmt.Sprintf("agent %s is already working on task %s", assignTo, *agent.CurrentTask)}
		}

		leaseDuration := state.Config.LeaseDuration
		if leaseDuration == 0 {
			leaseDuration = models.DefaultLeaseDurationSeconds
		}
		leaseExpires := now.Add(time.Duration(leaseDuration) * time.Second)

		fromStatus := task.Status
		if err := task.TransitionWith(targetStatus, pipelineTransitions); err != nil {
			return err
		}
		historyExtra := map[string]any{
			"assigned_to":   assignTo,
			"target_status": string(targetStatus),
		}
		if task.RepairRequest != nil {
			historyExtra["repair_operation"] = task.RepairRequest.Operation
			historyExtra["repair_target"] = task.RepairRequest.Target
			historyExtra["repair_command"] = task.RepairRequest.Command
			historyExtra["repair_evidence"] = append([]string(nil), task.RepairRequest.Evidence...)
			historyExtra["repair_validation"] = append([]string(nil), task.RepairRequest.Validation...)
		}

		task.AssignedTo = &assignTo
		task.LeaseExpires = &leaseExpires
		task.BlockedReason = nil
		task.BlockedQuestions = nil
		task.RepairRequest = nil
		task.History = append(task.History, models.TaskHistoryEntry{
			Time:   now,
			Event:  models.TaskEventUnblocked,
			Agent:  &agentID,
			Reason: &reason,
			Extra:  historyExtra,
		})

		agent.Status = models.AgentStatusWorking
		agent.CurrentTask = &taskID
		agent.LeaseExpires = &leaseExpires
		agent.Heartbeat = now
		state.Agents[assignTo] = agent

		result = UnblockTaskResult{
			TaskID:       taskID,
			FromStatus:   fromStatus,
			ToStatus:     targetStatus,
			AssignedTo:   assignTo,
			LeaseExpires: leaseExpires,
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to unblock task: %w", err)
	}

	return &result, nil
}
