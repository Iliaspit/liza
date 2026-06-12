package ops

import (
	"time"

	"github.com/liza-mas/liza/internal/models"
)

// Claim records (strangler phase 1: dual-write).
//
// These helpers mirror legacy ownership fields (task.AssignedTo/LeaseExpires,
// task.ReviewingBy/ReviewLeaseExpires) into state.Claims. They mutate ONLY
// state.Claims — the legacy task/agent fields stay maintained by the existing
// mutation code surrounding each call. Claims do not yet drive any behavior.

// recordDoerClaim dual-writes the doer ownership of taskID into state.Claims.
func recordDoerClaim(state *models.State, taskID, agentID string, leaseExpires time.Time) {
	recordClaim(state, taskID, agentID, models.ClaimKindDoer, leaseExpires)
}

// recordReviewerClaim dual-writes the reviewer ownership of taskID into state.Claims.
func recordReviewerClaim(state *models.State, taskID, agentID string, leaseExpires time.Time) {
	recordClaim(state, taskID, agentID, models.ClaimKindReviewer, leaseExpires)
}

// recordClaim grants or renews a claim. A renewal (same task, kind, and agent)
// updates ExpiresAt in place and preserves GrantedAt; anything else replaces
// the claim for (taskID, kind) with a fresh grant.
func recordClaim(state *models.State, taskID, agentID, kind string, leaseExpires time.Time) {
	exp := leaseExpires
	if existing := state.FindClaim(taskID, kind); existing != nil && existing.AgentID == agentID {
		existing.ExpiresAt = &exp
		return
	}
	state.GrantClaim(models.Claim{
		TaskID:    taskID,
		AgentID:   agentID,
		Kind:      kind,
		GrantedAt: time.Now().UTC(),
		ExpiresAt: &exp,
	})
}

// releaseDoerClaimRecord removes the doer claim record for taskID, if any.
func releaseDoerClaimRecord(state *models.State, taskID string) {
	state.ReleaseClaimRecord(taskID, models.ClaimKindDoer)
}

// releaseReviewerClaimRecord removes the reviewer claim record for taskID, if any.
func releaseReviewerClaimRecord(state *models.State, taskID string) {
	state.ReleaseClaimRecord(taskID, models.ClaimKindReviewer)
}

// SweepExpiredClaims returns the claims whose lease expired more than the
// lease-expiry grace period before now. It does not mutate state — it is the
// read-only primitive for the future lease-sweep scheduler and is not wired
// into any command yet.
func SweepExpiredClaims(state *models.State, now time.Time) []models.Claim {
	cutoff := now.Add(-models.LeaseExpiryGracePeriod)
	var expired []models.Claim
	for _, c := range state.Claims {
		if c.ExpiresAt != nil && c.ExpiresAt.Before(cutoff) {
			expired = append(expired, c)
		}
	}
	return expired
}
