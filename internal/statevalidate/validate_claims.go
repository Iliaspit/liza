package statevalidate

import (
	"fmt"
	"io"

	"github.com/liza-mas/liza/internal/models"
)

// validateClaims checks first-class claim records (strangler phase 1).
//
// Errors: a claim must reference an existing task.
//
// Warnings (warnWriter): a claim that contradicts the legacy ownership fields
// it dual-writes (AssignedTo for doer claims, ReviewingBy for reviewer
// claims). Claims are new and old state files have none, so the absence of a
// claim for an owned task is expected and never warns — only contradiction
// does.
func validateClaims(state *models.State, warnWriter io.Writer) error {
	for _, c := range state.Claims {
		task := state.FindTask(c.TaskID)
		if task == nil {
			return fmt.Errorf("claim (kind: %s, agent: %s) references non-existent task %s", c.Kind, c.AgentID, c.TaskID)
		}

		switch c.Kind {
		case models.ClaimKindDoer:
			warnClaimLegacyMismatch(warnWriter, c, "assigned_to", task.AssignedTo)
		case models.ClaimKindReviewer:
			warnClaimLegacyMismatch(warnWriter, c, "reviewing_by", task.ReviewingBy)
		default:
			fmt.Fprintf(warnWriter, "WARNING: claim for task %s has unknown kind %q\n", c.TaskID, c.Kind)
		}
	}
	return nil
}

func warnClaimLegacyMismatch(warnWriter io.Writer, c models.Claim, legacyField string, legacyAgent *string) {
	if legacyAgent == nil {
		fmt.Fprintf(warnWriter, "WARNING: %s claim for task %s held by %s but legacy %s is unset\n",
			c.Kind, c.TaskID, c.AgentID, legacyField)
		return
	}
	if *legacyAgent != c.AgentID {
		fmt.Fprintf(warnWriter, "WARNING: %s claim for task %s held by %s but legacy %s is %s\n",
			c.Kind, c.TaskID, c.AgentID, legacyField, *legacyAgent)
	}
}
