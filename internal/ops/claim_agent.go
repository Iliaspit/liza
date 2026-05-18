package ops

import (
	"fmt"

	"github.com/liza-mas/liza/internal/models"
)

func requireRegisteredClaimAgent(state *models.State, agentID, expectedRole string) (models.Agent, error) {
	agent, exists := state.Agents[agentID]
	if !exists {
		return models.Agent{}, &PreconditionError{Reason: fmt.Sprintf("agent %s is not registered", agentID)}
	}
	if agent.Role == "" {
		return models.Agent{}, &PreconditionError{Reason: fmt.Sprintf("agent %s has no registered role", agentID)}
	}
	if expectedRole != "" && agent.Role != expectedRole {
		return models.Agent{}, &PreconditionError{Reason: fmt.Sprintf("agent %s role %q does not match expected role %q", agentID, agent.Role, expectedRole)}
	}
	if agent.Provider == "" {
		return models.Agent{}, &PreconditionError{Reason: fmt.Sprintf("agent %s has no registered provider", agentID)}
	}
	if agent.PID <= 0 {
		return models.Agent{}, &PreconditionError{Reason: fmt.Sprintf("agent %s has no registered process PID", agentID)}
	}
	if !IsProcessAlive(agent.PID) {
		return models.Agent{}, &PreconditionError{Reason: fmt.Sprintf("agent %s registered process PID %d is not running", agentID, agent.PID)}
	}
	return agent, nil
}

// TaskClaimsForAgent returns task-side ownership fields that point at agentID.
// It deliberately does not consult the agent row so recovery still works when
// agent metadata is missing or corrupt.
func TaskClaimsForAgent(state *models.State, agentID string) (doerTaskIDs, reviewerTaskIDs []string) {
	for i := range state.Tasks {
		task := &state.Tasks[i]
		if task.AssignedTo != nil && *task.AssignedTo == agentID {
			doerTaskIDs = append(doerTaskIDs, task.ID)
		}
		if task.ReviewingBy != nil && *task.ReviewingBy == agentID {
			reviewerTaskIDs = append(reviewerTaskIDs, task.ID)
		}
	}
	return doerTaskIDs, reviewerTaskIDs
}
