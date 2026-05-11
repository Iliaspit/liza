package ops

import "github.com/liza-mas/liza/internal/models"

func releaseAgentsForTask(state *models.State, taskID string) {
	for agentID, agent := range state.Agents {
		if agent.CurrentTask != nil && *agent.CurrentTask == taskID {
			// ReleaseAgent updates this map entry in place; mutating values during
			// map iteration is safe and keeps release atomic with task transition.
			state.ReleaseAgent(agentID)
		}
	}
}
