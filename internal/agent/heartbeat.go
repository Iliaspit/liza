package agent

import (
	"context"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
)

const (
	// DefaultHeartbeatInterval derives from models to maintain a single source of truth.
	DefaultHeartbeatInterval = time.Duration(models.DefaultHeartbeatIntervalSec) * time.Second
	DefaultLeaseDuration     = time.Duration(models.DefaultLeaseDurationSeconds) * time.Second
)

type HeartbeatConfig struct {
	AgentID       string
	StatePath     string
	Interval      time.Duration
	LeaseDuration time.Duration
	State         *models.State // Optional: if provided, interval is read from state.Config.HeartbeatInterval
}

type Heartbeat struct {
	agentID       string
	bb            *db.Blackboard
	interval      time.Duration
	leaseDuration time.Duration
}

func NewHeartbeat(config HeartbeatConfig) *Heartbeat {
	interval := config.Interval

	if config.State != nil {
		interval = models.NormalizeHeartbeatInterval(config.State.Config.HeartbeatInterval)
	}

	if interval == 0 {
		interval = DefaultHeartbeatInterval
	}

	leaseDuration := config.LeaseDuration
	if leaseDuration == 0 {
		leaseDuration = DefaultLeaseDuration
	}

	return &Heartbeat{
		agentID:       config.AgentID,
		bb:            db.For(config.StatePath),
		interval:      interval,
		leaseDuration: leaseDuration,
	}
}

func (h *Heartbeat) Start(ctx context.Context) error {
	logger := GetLogger()
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := h.beat(); err != nil {
				if errors.IsNotFound(err) {
					return err
				}
				// Non-fatal: supervisors detect stale agents via watch command
				logger.Error("Heartbeat update failed", "error", err, "agent_id", h.agentID)
			}
		}
	}
}

func (h *Heartbeat) beat() error {
	now := time.Now().UTC()
	newLease := now.Add(h.leaseDuration)

	return h.bb.Modify(func(state *models.State) error {
		agent, exists := state.Agents[h.agentID]
		if !exists {
			return &errors.NotFoundError{Entity: "agent", ID: h.agentID}
		}

		agent.Heartbeat = now
		agent.LeaseExpires = &newLease
		state.Agents[h.agentID] = agent

		// Renew task lease if agent is actively assigned
		if agent.CurrentTask != nil {
			if task := state.FindTask(*agent.CurrentTask); task != nil {
				if task.AssignedTo != nil && *task.AssignedTo == h.agentID && task.LeaseExpires != nil {
					task.LeaseExpires = &newLease
					renewClaimLease(state, task.ID, models.ClaimKindDoer, h.agentID, newLease)
				}
				if task.ReviewingBy != nil && *task.ReviewingBy == h.agentID && task.ReviewLeaseExpires != nil {
					task.ReviewLeaseExpires = &newLease
					renewClaimLease(state, task.ID, models.ClaimKindReviewer, h.agentID, newLease)
				}
			}
		}

		return nil
	})
}

// renewClaimLease bumps the lease expiry of the matching claim record in
// lockstep with the legacy task lease fields (strangler dual-write). Old
// state files have no claims, so a missing claim is simply skipped.
func renewClaimLease(state *models.State, taskID, kind, agentID string, expires time.Time) {
	if c := state.FindClaim(taskID, kind); c != nil && c.AgentID == agentID {
		exp := expires
		c.ExpiresAt = &exp
	}
}
