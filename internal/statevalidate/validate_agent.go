package statevalidate

import (
	"fmt"
	"io"
	"time"

	"github.com/liza-mas/liza/internal/models"
)

// validateAgentInvariants checks that every WORKING agent has a current_task
// and a valid lease_expires timestamp. Warns (via warnWriter) when a lease has
// expired past the grace period, which may indicate a long-running operation
// rather than a stuck agent. Prevents orphaned agents that consume capacity
// without making progress.
func validateAgentInvariants(state *models.State, projectRoot string, skipSpecFileCheck bool, warnWriter io.Writer) error {
	now := time.Now().UTC()
	graceDeadline := now.Add(-models.LeaseExpiryGracePeriod)

	for agentID, agent := range state.Agents {
		if agent.LeaseExpires != nil && agent.LeaseExpires.After(now) {
			if agent.Role == "" {
				return fmt.Errorf("agent %s has active lease but no role", agentID)
			}
			if agent.Provider == "" {
				return fmt.Errorf("agent %s has active lease but no provider", agentID)
			}
			if agent.PID <= 0 {
				return fmt.Errorf("agent %s has active lease but no pid", agentID)
			}
		}

		// WORKING agent must have current_task
		if agent.Status == models.AgentStatusWorking && agent.CurrentTask == nil {
			return fmt.Errorf("agent %s has status WORKING but no current_task assigned", agentID)
		}

		// WORKING agent must have valid lease_expires
		if agent.Status == models.AgentStatusWorking {
			if agent.LeaseExpires == nil {
				return fmt.Errorf("agent %s has status WORKING but no lease_expires", agentID)
			}

			// Check lease expiry with grace period (warning only in original script)
			if agent.LeaseExpires.Before(graceDeadline) {
				// In bash this is a warning, but we'll treat it as an error for stricter validation
				// Could make this configurable if needed
				fmt.Fprintf(warnWriter, "WARNING: Agent %s has status WORKING but lease expired (may be long-running operation)\n", agentID)
			}
		}
	}

	return nil
}
