package ops

import (
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/procscan"
)

var agentProcessProcRoot = "/proc"

// SetAgentProcessProcRootForTest redirects process identity checks to a fake
// procfs tree and returns a cleanup function.
func SetAgentProcessProcRootForTest(procRoot string) func() {
	old := agentProcessProcRoot
	agentProcessProcRoot = procRoot
	return func() {
		agentProcessProcRoot = old
	}
}

func AgentProcessStatus(agentID string, agent models.Agent) procscan.AgentProcessStatus {
	return procscan.AgentProcessStatusForPID(agent.PID, agent.Role, agentID, agentProcessProcRoot)
}
