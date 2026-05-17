package ops

import (
	stderrors "errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// DeleteAgentResult contains the outcome of deleting an agent.
type DeleteAgentResult struct {
	AgentID string
	PID     int // PID of the deleted agent's process (0 if unknown).
}

// SignalProcess sends SIGTERM to the deleted agent's process if it had a known PID.
// Verifies the process is a liza agent via /proc/<pid>/cmdline before signaling,
// preventing accidental kills from PID reuse. Safe to call unconditionally.
func (r *DeleteAgentResult) SignalProcess() bool {
	if r.PID <= 0 {
		return false
	}
	if !isLizaAgentProcess(r.PID) {
		return false
	}
	proc, err := os.FindProcess(r.PID)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.SIGTERM) == nil
}

// isLizaAgentProcess checks if the process with the given PID is a liza agent
// by reading /proc/<pid>/cmdline. Returns false if the process doesn't exist,
// is unreadable, or isn't a liza agent.
// Linux-only: returns false on platforms without procfs (documented no-op).
func isLizaAgentProcess(pid int) bool {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return false
	}
	return matchLizaAgentCmdline(string(data))
}

// matchLizaAgentCmdline checks if a null-separated cmdline string matches
// a liza agent process (argv[0] basename == "liza", argv[1] == "agent").
func matchLizaAgentCmdline(cmdline string) bool {
	args := strings.Split(cmdline, "\x00")
	if len(args) < 2 {
		return false
	}
	base := args[0]
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	return base == "liza" && args[1] == "agent"
}

// IsProcessAlive checks if a process with the given PID is running.
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil || stderrors.Is(err, syscall.EPERM)
}

// validateAgentDeletion checks whether an agent can be safely deleted based on
// lease and task state. Does not check PID liveness (callers handle that separately).
func validateAgentDeletion(agent models.Agent, agentID string) error {
	now := time.Now().UTC()
	if agent.LeaseExpires != nil && agent.LeaseExpires.After(now) {
		return fmt.Errorf("agent %s has active lease (expires %v), use --force to delete", agentID, agent.LeaseExpires.Format(time.RFC3339))
	}
	if agent.CurrentTask != nil {
		return fmt.Errorf("agent %s is working on task %s, use --force to delete", agentID, *agent.CurrentTask)
	}
	return nil
}

// DeleteAgent removes an agent from state. Without force, refuses if the agent
// has an active lease, current task, or running process. The allowRunningPID
// flag bypasses only the PID liveness check (for interactive CLI confirmation)
// without bypassing lease/task safety checks. Callers should check
// IsAgentProcessRunning for interactive confirmation first. No terminal I/O.
func DeleteAgent(projectRoot, agentID string, force, allowRunningPID bool, reason string) (*DeleteAgentResult, error) {
	if agentID == "" {
		return nil, fmt.Errorf("agent ID required")
	}

	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())

	state, err := bb.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read state: %w", err)
	}

	agent, exists := state.Agents[agentID]
	if !exists {
		return nil, &errors.NotFoundError{Entity: "agent", ID: agentID}
	}

	if !force {
		if err := validateAgentDeletion(agent, agentID); err != nil {
			return nil, err
		}
		if !allowRunningPID && agent.PID != 0 && IsProcessAlive(agent.PID) {
			return nil, fmt.Errorf("agent %s is still running with PID %d, use --force to delete or confirm interactively via CLI", agentID, agent.PID)
		}
	}

	err = bb.Modify(func(state *models.State) error {
		agent, exists := state.Agents[agentID]
		if !exists {
			return &errors.NotFoundError{Entity: "agent", ID: agentID}
		}

		if !force {
			if err := validateAgentDeletion(agent, agentID); err != nil {
				return err
			}
		}

		delete(state.Agents, agentID)

		humanNote := models.HumanNote{
			Timestamp: time.Now().UTC(),
			Message:   fmt.Sprintf("Agent %s deleted: %s", agentID, reason),
			For:       agentID,
		}
		state.HumanNotes = append(state.HumanNotes, humanNote)

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to delete agent: %w", err)
	}

	return &DeleteAgentResult{
		AgentID: agentID,
		PID:     agent.PID,
	}, nil
}

// IsAgentProcessRunning checks if the agent's registered PID is alive. Use before
// DeleteAgent to prompt for interactive confirmation.
func IsAgentProcessRunning(projectRoot, agentID string) (bool, int, error) {
	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())

	state, err := bb.Read()
	if err != nil {
		return false, 0, fmt.Errorf("failed to read state: %w", err)
	}

	agent, exists := state.Agents[agentID]
	if !exists {
		return false, 0, &errors.NotFoundError{Entity: "agent", ID: agentID}
	}

	if agent.PID != 0 && IsProcessAlive(agent.PID) {
		return true, agent.PID, nil
	}

	return false, agent.PID, nil
}
