package ops

import (
	"fmt"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// validDispositionSentinels are the allowed non-task-ID disposition values.
var validDispositionSentinels = map[string]bool{
	"deferred":  true,
	"dismissed": true,
}

// SetDiscoveryDisposition sets the converted_to_task field on a discovery entry.
// Valid dispositions: a task ID (when converted to task), "deferred", or "dismissed".
func SetDiscoveryDisposition(projectRoot, discoveryID, disposition string) error {
	if discoveryID == "" {
		return &PreconditionError{Reason: "discovery_id is required"}
	}
	if disposition == "" {
		return &PreconditionError{Reason: "disposition is required"}
	}
	if !validDispositionSentinels[disposition] {
		if err := paths.ValidateTaskID(disposition); err != nil {
			return &PreconditionError{Reason: fmt.Sprintf("invalid disposition %q: must be a valid task ID, \"deferred\", or \"dismissed\"", disposition)}
		}
	}

	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())

	isTaskRef := !validDispositionSentinels[disposition]

	return bb.ModifyOp("set_discovery_disposition", func(state *models.State) error {
		// Verify referenced task exists when disposition is a task ID.
		if isTaskRef && state.FindTask(disposition) == nil {
			return &PreconditionError{Reason: fmt.Sprintf("disposition references task %q which does not exist", disposition)}
		}

		for i := range state.Discovered {
			if state.Discovered[i].ID == discoveryID {
				state.Discovered[i].ConvertedToTask = &disposition
				return nil
			}
		}
		return &PreconditionError{Reason: fmt.Sprintf("discovery %q not found", discoveryID)}
	})
}
