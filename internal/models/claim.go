package models

import "time"

// Claim kinds. A claim records one ownership relation between an agent and a
// task: "doer" mirrors AssignedTo/LeaseExpires, "reviewer" mirrors
// ReviewingBy/ReviewLeaseExpires.
const (
	ClaimKindDoer     = "doer"
	ClaimKindReviewer = "reviewer"
)

// Claim is a first-class ownership record (strangler phase 1). During the
// dual-write phase claims are written alongside the legacy task/agent
// ownership fields; they do not yet drive any behavior. Old state files have
// no claims, so absence of a claim for an owned task is expected and valid.
type Claim struct {
	TaskID    string     `yaml:"task_id"`
	AgentID   string     `yaml:"agent_id"`
	Kind      string     `yaml:"kind"`
	GrantedAt time.Time  `yaml:"granted_at"`
	ExpiresAt *time.Time `yaml:"expires_at,omitempty"`
}

// FindClaim returns a pointer to the claim for (taskID, kind), or nil if no
// such claim exists. The pointer refers to the element within s.Claims, so
// mutations are reflected in the state (useful inside Blackboard.Modify
// closures, e.g. lease renewals updating ExpiresAt in place).
func (s *State) FindClaim(taskID, kind string) *Claim {
	for i := range s.Claims {
		if s.Claims[i].TaskID == taskID && s.Claims[i].Kind == kind {
			return &s.Claims[i]
		}
	}
	return nil
}

// ClaimsForAgent returns copies of all claims held by agentID.
func (s *State) ClaimsForAgent(agentID string) []Claim {
	var claims []Claim
	for _, c := range s.Claims {
		if c.AgentID == agentID {
			claims = append(claims, c)
		}
	}
	return claims
}

// GrantClaim records a claim, replacing any existing claim with the same
// TaskID+Kind. There is at most one claim per (task, kind).
func (s *State) GrantClaim(c Claim) {
	if existing := s.FindClaim(c.TaskID, c.Kind); existing != nil {
		*existing = c
		return
	}
	s.Claims = append(s.Claims, c)
}

// ReleaseClaimRecord removes the claim for (taskID, kind). Returns true if a
// claim was removed. Releasing an absent claim is a no-op (old state files
// have no claims), so callers may release unconditionally.
func (s *State) ReleaseClaimRecord(taskID, kind string) bool {
	for i := range s.Claims {
		if s.Claims[i].TaskID == taskID && s.Claims[i].Kind == kind {
			s.Claims = append(s.Claims[:i], s.Claims[i+1:]...)
			return true
		}
	}
	return false
}
