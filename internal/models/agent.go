package models

import "time"

// AgentStatus represents the state of an agent
type AgentStatus string

const (
	AgentStatusStarting  AgentStatus = "STARTING"
	AgentStatusIdle      AgentStatus = "IDLE"
	AgentStatusWorking   AgentStatus = "WORKING"
	AgentStatusReviewing AgentStatus = "REVIEWING"
	AgentStatusWaiting   AgentStatus = "WAITING"
	AgentStatusHandoff   AgentStatus = "HANDOFF"
	AgentStatusPlanning  AgentStatus = "PLANNING"
)

// AgentHealthState represents whether a registered agent epoch counts as
// effective capacity for its role.
type AgentHealthState string

const (
	AgentHealthOK       AgentHealthState = "ok"
	AgentHealthDegraded AgentHealthState = "degraded"
)

// IsValid checks if the agent status is valid
func (as AgentStatus) IsValid() bool {
	switch as {
	case AgentStatusStarting, AgentStatusIdle, AgentStatusWorking,
		AgentStatusReviewing, AgentStatusWaiting, AgentStatusHandoff,
		AgentStatusPlanning:
		return true
	}
	return false
}

// IsValid checks if the agent health state is valid.
func (ahs AgentHealthState) IsValid() bool {
	switch ahs {
	case AgentHealthOK, AgentHealthDegraded:
		return true
	}
	return false
}

// Agent represents an agent (coder, reviewer, orchestrator) in the system
type Agent struct {
	Role            string         `yaml:"role"`
	Status          AgentStatus    `yaml:"status"`
	CurrentTask     *string        `yaml:"current_task,omitempty"`
	LeaseExpires    *time.Time     `yaml:"lease_expires,omitempty"`
	Heartbeat       time.Time      `yaml:"heartbeat"`
	RegisteredAt    time.Time      `yaml:"registered_at,omitempty"`
	Terminal        string         `yaml:"terminal"`
	Provider        string         `yaml:"provider,omitempty"`
	IterationsTotal int            `yaml:"iterations_total"`
	ContextPercent  int            `yaml:"context_percent"`
	PID             int            `yaml:"pid,omitempty"`
	Extra           map[string]any `yaml:",inline"`
}

// AgentHealth records current capacity health for a specific agent epoch.
type AgentHealth struct {
	State          AgentHealthState `yaml:"state"`
	Role           string           `yaml:"role"`
	Provider       string           `yaml:"provider,omitempty"`
	PID            int              `yaml:"pid,omitempty"`
	RegisteredAt   *time.Time       `yaml:"registered_at,omitempty"`
	DegradedAt     time.Time        `yaml:"degraded_at"`
	Reason         string           `yaml:"reason,omitempty"`
	LastTask       string           `yaml:"last_task,omitempty"`
	CandidateTasks []string         `yaml:"candidate_tasks,omitempty"`
	LastError      string           `yaml:"last_error,omitempty"`
	RecoverHint    string           `yaml:"recover_hint,omitempty"`
	DegradedBy     string           `yaml:"degraded_by,omitempty"`
}

// MatchesAgentEpoch reports whether this health record describes the supplied
// currently registered process generation.
func (h AgentHealth) MatchesAgentEpoch(agent Agent) bool {
	if h.PID != 0 && agent.PID != 0 && h.PID != agent.PID {
		return false
	}
	if h.Provider != "" && agent.Provider != "" && h.Provider != agent.Provider {
		return false
	}
	if h.RegisteredAt != nil && !agent.RegisteredAt.IsZero() && !h.RegisteredAt.Equal(agent.RegisteredAt) {
		return false
	}
	return true
}

// IsCurrentDegradedFor reports whether this health record currently removes the
// supplied registered agent epoch from effective role capacity.
func (h AgentHealth) IsCurrentDegradedFor(agent Agent) bool {
	return h.State == AgentHealthDegraded && h.MatchesAgentEpoch(agent)
}

// ReleaseAgent resets an agent to idle with no task assignment.
func (s *State) ReleaseAgent(agentID string) {
	if agent, ok := s.Agents[agentID]; ok {
		agent.Status = AgentStatusIdle
		agent.CurrentTask = nil
		agent.LeaseExpires = nil
		s.Agents[agentID] = agent
	}
}
