package commands

import (
	stderrors "errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/liza-mas/liza/internal/db"
	lizaerrors "github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/procscan"
	"github.com/liza-mas/liza/internal/statevalidate"
)

// warnWriter is the destination for non-fatal validation warnings.
// Defaults to os.Stderr; tests override it to capture output without
// monkey-patching the global stderr (which is not goroutine-safe).
var warnWriter io.Writer = os.Stderr

// SetWarnWriter sets the destination for non-fatal validation warnings.
func SetWarnWriter(w io.Writer) {
	warnWriter = w
}

// ValidateOptions controls live and offline validation behavior.
type ValidateOptions struct {
	SkipSpecFileCheck bool
	SkipProcessChecks bool
}

// ValidateCommand validates the state.yaml file against all schema rules.
// Returns an error with detailed description if validation fails.
func ValidateCommand(statePath string, skipSpecFileCheck bool) error {
	return ValidateCommandWithOptions(statePath, ValidateOptions{SkipSpecFileCheck: skipSpecFileCheck})
}

// ValidateCommandWithOptions validates state.yaml and, by default, verifies
// that no live liza agent supervisor for this project/goal is missing from
// state.yaml. Process validation is host-local and intentionally skippable for
// archived/offline state validation.
func ValidateCommandWithOptions(statePath string, opts ValidateOptions) error {
	projectRoot := filepath.Dir(filepath.Dir(statePath))
	state, err := db.For(statePath).Read()
	if err != nil {
		schemaErr := &lizaerrors.StateSchemaError{Operation: "validate", Err: err}
		return &lizaerrors.ValidationError{Message: schemaErr.Error(), Err: schemaErr}
	}

	if err := statevalidate.ValidateState(state, projectRoot, opts.SkipSpecFileCheck, warnWriter); err != nil {
		return &lizaerrors.ValidationError{Message: err.Error(), Err: err}
	}
	if !opts.SkipProcessChecks {
		if err := validateNoZombieAgents(state, projectRoot); err != nil {
			return &lizaerrors.ValidationError{Message: err.Error(), Err: err}
		}
	}
	return nil
}

func validateNoZombieAgents(state *models.State, projectRoot string) error {
	zombies, err := findZombieAgents(procscan.ZombieScanOptions{
		ProjectRoot:    projectRoot,
		GoalID:         state.Goal.ID,
		RegisteredPIDs: registeredAgentPIDs(state),
	})
	if stderrors.Is(err, procscan.ErrProcessScanUnavailable) {
		fmt.Fprintln(warnWriter, "WARNING: Live liza agent process scan skipped (procfs unavailable on this host)")
		return nil
	}
	if err != nil {
		return fmt.Errorf("scan liza agent processes: %w", err)
	}
	if len(zombies) == 0 {
		return nil
	}

	parts := make([]string, 0, len(zombies))
	for _, zombie := range zombies {
		role := zombie.Role
		if role == "" {
			role = "unknown"
		}
		parts = append(parts, fmt.Sprintf("pid %d role %s", zombie.PID, role))
	}
	return fmt.Errorf("zombie liza agent process detected: %s not registered in state.yaml (use 'liza get agents --zombies' to inspect, or 'liza validate --skip-process-checks' for offline validation)", strings.Join(parts, ", "))
}

func validateAgentInvariants(state *models.State, projectRoot string, skipSpecFileCheck bool) error {
	return statevalidate.ValidateAgentInvariants(state, projectRoot, skipSpecFileCheck, warnWriter)
}

func validateAnomalies(state *models.State, projectRoot string, skipSpecFileCheck bool) error {
	return statevalidate.ValidateAnomalies(state, projectRoot, skipSpecFileCheck)
}
