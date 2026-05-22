package commands

import (
	"fmt"

	"github.com/liza-mas/liza/internal/ops"
)

type MarkAgentDegradedOptions struct {
	ProjectRoot    string
	AgentID        string
	Role           string
	Reason         string
	LastTask       string
	CandidateTasks []string
	LastError      string
	RecoverHint    string
}

func MarkAgentDegradedCommand(opts MarkAgentDegradedOptions) error {
	if err := ops.MarkAgentDegraded(ops.MarkAgentDegradedInput{
		ProjectRoot:    opts.ProjectRoot,
		AgentID:        opts.AgentID,
		Role:           opts.Role,
		Reason:         opts.Reason,
		LastTask:       opts.LastTask,
		CandidateTasks: opts.CandidateTasks,
		LastError:      opts.LastError,
		RecoverHint:    opts.RecoverHint,
		DegradedBy:     "operator",
	}); err != nil {
		return err
	}
	fmt.Printf("Marked agent %s degraded: %s\n", opts.AgentID, opts.Reason)
	return nil
}

func ClearAgentDegradedCommand(projectRoot, agentID string) error {
	if err := ops.ClearAgentDegraded(projectRoot, agentID); err != nil {
		return err
	}
	fmt.Printf("Cleared degraded health for agent %s\n", agentID)
	return nil
}
