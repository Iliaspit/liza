package commands

import (
	"fmt"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/roles"
	"github.com/liza-mas/liza/internal/statehygiene"
	"gopkg.in/yaml.v3"
)

// MigrateCommand normalizes role names in state.yaml from underscore form
// to hyphenated form. Returns (changed, error) where changed indicates
// whether any modifications were made.
//
// Uses ReadRaw + manual unmarshal to bypass db.Read()'s read-path
// normalization, which would hide the underscore roles we need to detect.
func MigrateCommand(statePath string) (bool, error) {
	bb := db.For(statePath)
	raw, err := bb.ReadRaw()
	if err != nil {
		return false, fmt.Errorf("failed to read state file: %w", err)
	}

	var state models.State
	if err := yaml.Unmarshal(raw, &state); err != nil {
		return false, fmt.Errorf("failed to parse state file: %w", err)
	}

	changed := false
	for agentID, agent := range state.Agents {
		normalized := roles.NormalizeRoleName(agent.Role)
		if normalized != agent.Role {
			agent.Role = normalized
			state.Agents[agentID] = agent
			changed = true
		}
	}

	for i := range state.Tasks {
		if state.Tasks[i].MigrateAttemptedField() {
			changed = true
		}
	}
	if statehygiene.ScrubStateForMigration(&state) {
		changed = true
	}

	if !changed {
		return false, nil
	}

	if err := bb.Write(&state); err != nil {
		return false, fmt.Errorf("failed to write migrated state: %w", err)
	}

	return true, nil
}
