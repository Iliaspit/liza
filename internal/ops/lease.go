package ops

import (
	"time"

	"github.com/liza-mas/liza/internal/models"
)

// renewLease sets a fresh lease expiry on task based on the configured duration.
// The matching doer claim record, if present, is renewed in lockstep (dual-write).
func renewLease(s *models.State, t *models.Task) {
	dur := s.Config.LeaseDuration
	if dur <= 0 {
		dur = models.DefaultLeaseDurationSeconds
	}
	exp := time.Now().UTC().Add(time.Duration(dur) * time.Second)
	t.LeaseExpires = &exp
	if t.AssignedTo != nil {
		recordClaim(s, t.ID, *t.AssignedTo, models.ClaimKindDoer, exp)
	}
}
