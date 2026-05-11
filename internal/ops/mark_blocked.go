package ops

import (
	"fmt"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// MarkBlockedResult contains the outcome of marking a task as blocked.
type MarkBlockedResult struct {
	TaskID        string                `json:"task_id"`
	Reason        string                `json:"reason"`
	RepairRequest *models.RepairRequest `json:"repair_request,omitempty"`
}

// MarkBlockedOptions contains optional metadata for a block transition.
type MarkBlockedOptions struct {
	RepairRequest *models.RepairRequest
}

// MarkBlocked transitions a task from an executing status to BLOCKED. Only the
// assigned agent can block its own task. Requires reason and 1-3 clarifying
// questions per the blocking protocol. Uses pipeline-defined executing statuses.
func MarkBlocked(projectRoot, taskID, reason string, questions []string, agentID string) (*MarkBlockedResult, error) {
	return MarkBlockedWithOptions(projectRoot, taskID, reason, questions, agentID, MarkBlockedOptions{})
}

// MarkBlockedWithOptions transitions a task from an executing status to BLOCKED
// and optionally records a structured repair request for orchestrator follow-up.
func MarkBlockedWithOptions(projectRoot, taskID, reason string, questions []string, agentID string, opts MarkBlockedOptions) (*MarkBlockedResult, error) {
	if taskID == "" {
		return nil, &PreconditionError{Reason: "task ID is required"}
	}
	if reason == "" {
		return nil, &PreconditionError{Reason: "reason is required"}
	}
	if agentID == "" {
		return nil, &PreconditionError{Reason: "agent ID is required"}
	}
	if len(questions) == 0 {
		return nil, &PreconditionError{Reason: "at least 1 question is required"}
	}
	if len(questions) > 3 {
		return nil, &PreconditionError{Reason: "maximum 3 questions allowed per blocking protocol"}
	}
	repairRequest, err := normalizeRepairRequest(opts.RepairRequest)
	if err != nil {
		return nil, err
	}

	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())
	now := time.Now().UTC()

	// Load pipeline config for status checks and transitions.
	resolver, _, err := loadResolver(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to load pipeline config: %w", err)
	}
	var pipelineExecuting []models.TaskStatus
	for _, rpName := range resolver.RolePairNames() {
		if es, err := resolver.ExecutingStatus(rpName); err == nil {
			pipelineExecuting = append(pipelineExecuting, es)
		}
	}
	pipelineTransitions := BuildPipelineTransitions(resolver)

	err = bb.Modify(func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return &errors.NotFoundError{Entity: "task", ID: taskID}
		}

		if !isExecutingStatus(task.Status, pipelineExecuting) {
			return &PreconditionError{Reason: fmt.Sprintf("task must be in an executing status to be marked blocked, current status: %s", task.Status)}
		}

		if task.AssignedTo == nil || *task.AssignedTo != agentID {
			return &PreconditionError{Reason: "only the assigned agent can mark task as blocked"}
		}

		if err := task.TransitionWith(models.TaskStatusBlocked, pipelineTransitions); err != nil {
			return err
		}
		task.BlockedReason = &reason
		task.BlockedQuestions = questions
		task.RepairRequest = repairRequest
		releaseAgentsForTask(state, taskID)
		task.AssignedTo = nil
		task.LeaseExpires = nil

		task.History = append(task.History, models.TaskHistoryEntry{
			Time:   now,
			Event:  models.TaskEventBlocked,
			Agent:  &agentID,
			Reason: &reason,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to mark task as blocked: %w", err)
	}

	return &MarkBlockedResult{
		TaskID:        taskID,
		Reason:        reason,
		RepairRequest: repairRequest,
	}, nil
}

func normalizeRepairRequest(request *models.RepairRequest) (*models.RepairRequest, error) {
	if request == nil {
		return nil, nil
	}

	normalized := &models.RepairRequest{
		Operation:  strings.TrimSpace(request.Operation),
		Target:     strings.TrimSpace(request.Target),
		Command:    strings.TrimSpace(request.Command),
		Evidence:   compactNonEmpty(request.Evidence),
		Validation: compactNonEmpty(request.Validation),
	}
	if normalized.Operation == "" {
		return nil, &PreconditionError{Reason: "repair request operation is required"}
	}
	if normalized.Target == "" {
		return nil, &PreconditionError{Reason: "repair request target is required"}
	}
	if normalized.Command == "" {
		return nil, &PreconditionError{Reason: "repair request command is required"}
	}
	if len(normalized.Evidence) == 0 {
		return nil, &PreconditionError{Reason: "repair request evidence is required"}
	}
	if len(normalized.Validation) == 0 {
		return nil, &PreconditionError{Reason: "repair request validation is required"}
	}
	return normalized, nil
}

func compactNonEmpty(values []string) []string {
	var compacted []string
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			compacted = append(compacted, trimmed)
		}
	}
	return compacted
}
