package ops

import (
	stderrors "errors"
	"fmt"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

const (
	AgentDegradedWorktreeCreateFailed     = "claim_worktree_create_failed"
	AgentDegradedWorktreeReadonly         = "claim_worktree_readonly"
	AgentDegradedWorktreePermissionDenied = "claim_worktree_permission_denied"
	AgentDegradedWorktreeSetupFailed      = "claim_worktree_setup_failed"
)

// MarkAgentDegradedInput contains the evidence used to mark an agent epoch as
// unable to provide effective claim capacity.
type MarkAgentDegradedInput struct {
	ProjectRoot    string
	AgentID        string
	Role           string
	Reason         string
	LastTask       string
	CandidateTasks []string
	LastError      string
	RecoverHint    string
	DegradedBy     string
}

// InfraClaimErrorClassification describes a claim failure that indicates the
// agent process cannot provision worktrees/refs for its role.
type InfraClaimErrorClassification struct {
	Reason      string
	RecoverHint string
	IsInfra     bool
}

// MarkAgentDegraded records current capacity health for an agent and appends an
// anomaly for durable history. It does not mutate task state or agent status.
func MarkAgentDegraded(input MarkAgentDegradedInput) error {
	if input.AgentID == "" {
		return &PreconditionError{Reason: "agent ID is required"}
	}
	if input.Role == "" {
		return &PreconditionError{Reason: "agent role is required"}
	}
	if input.Reason == "" {
		return &PreconditionError{Reason: "degraded reason is required"}
	}
	if input.LastError == "" {
		return &PreconditionError{Reason: "last error is required"}
	}

	bb := db.For(paths.New(input.ProjectRoot).StatePath())
	now := time.Now().UTC()
	return bb.Modify(func(state *models.State) error {
		agent, hasAgent := state.Agents[input.AgentID]
		if state.AgentHealth == nil {
			state.AgentHealth = make(map[string]models.AgentHealth)
		}

		var registeredAt *time.Time
		if hasAgent && !agent.RegisteredAt.IsZero() {
			value := agent.RegisteredAt
			registeredAt = &value
		}
		provider := ""
		pid := 0
		if hasAgent {
			provider = agent.Provider
			pid = agent.PID
		}

		state.AgentHealth[input.AgentID] = models.AgentHealth{
			State:          models.AgentHealthDegraded,
			Role:           input.Role,
			Provider:       provider,
			PID:            pid,
			RegisteredAt:   registeredAt,
			DegradedAt:     now,
			Reason:         input.Reason,
			LastTask:       input.LastTask,
			CandidateTasks: append([]string(nil), input.CandidateTasks...),
			LastError:      input.LastError,
			RecoverHint:    input.RecoverHint,
			DegradedBy:     input.DegradedBy,
		}

		state.Anomalies = append(state.Anomalies, models.Anomaly{
			Timestamp: now,
			Task:      input.LastTask,
			Reporter:  input.AgentID,
			Type:      "agent_degraded",
			Details: map[string]any{
				"agent_id":        input.AgentID,
				"role":            input.Role,
				"provider":        provider,
				"pid":             pid,
				"reason":          input.Reason,
				"candidate_tasks": append([]string(nil), input.CandidateTasks...),
				"last_error":      input.LastError,
				"recover_hint":    input.RecoverHint,
				"degraded_by":     input.DegradedBy,
			},
		})
		return nil
	})
}

// ClearAgentDegraded removes current degraded health for an agent ID. It is
// idempotent so callers can clear after a successful claim without pre-checks.
func ClearAgentDegraded(projectRoot, agentID string) error {
	if agentID == "" {
		return &PreconditionError{Reason: "agent ID is required"}
	}
	bb := db.For(paths.New(projectRoot).StatePath())
	return bb.Modify(func(state *models.State) error {
		if state.AgentHealth == nil {
			return nil
		}
		delete(state.AgentHealth, agentID)
		if len(state.AgentHealth) == 0 {
			state.AgentHealth = nil
		}
		return nil
	})
}

// ClassifyInfraClaimError recognizes narrow infrastructure failures that make a
// doer process unusable for claim capacity. Races and task precondition errors
// intentionally remain unclassified.
func ClassifyInfraClaimError(err error) InfraClaimErrorClassification {
	if err == nil {
		return InfraClaimErrorClassification{}
	}
	// The configured post_worktree_cmd failed. Every worktree this agent
	// provisions will hit the same project-level failure, so claiming again is
	// wasted capacity — degrade instead, which also bounds the claim retry loop.
	var setupErr *PostWorktreeSetupError
	if stderrors.As(err, &setupErr) {
		return InfraClaimErrorClassification{
			Reason: AgentDegradedWorktreeSetupFailed,
			RecoverHint: fmt.Sprintf(
				"run the configured post_worktree_cmd (%s) in %s and fix it, then clear agent health",
				setupErr.Cmd,
				setupErr.Dir,
			),
			IsInfra: true,
		}
	}

	msg := err.Error()
	lower := strings.ToLower(msg)

	switch {
	case strings.Contains(lower, "cannot lock ref") &&
		(strings.Contains(lower, "operation not permitted") || strings.Contains(lower, "permission denied")):
		return infraClaimClassification(AgentDegradedWorktreeCreateFailed)
	case strings.Contains(lower, "read-only file system"):
		return infraClaimClassification(AgentDegradedWorktreeReadonly)
	case strings.Contains(lower, "permission denied") &&
		(strings.Contains(lower, ".git/refs") || strings.Contains(lower, ".git/worktrees") || strings.Contains(lower, ".worktrees")):
		return infraClaimClassification(AgentDegradedWorktreePermissionDenied)
	case strings.Contains(lower, "operation not permitted") &&
		(strings.Contains(lower, ".git/refs") || strings.Contains(lower, ".git/worktrees") || strings.Contains(lower, ".worktrees")):
		return infraClaimClassification(AgentDegradedWorktreePermissionDenied)
	}

	return InfraClaimErrorClassification{}
}

func infraClaimClassification(reason string) InfraClaimErrorClassification {
	return InfraClaimErrorClassification{
		Reason:      reason,
		RecoverHint: fmt.Sprintf("restart the agent from a process context that can write project git refs and %s", paths.WorktreesDirName),
		IsInfra:     true,
	}
}
