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
	"github.com/liza-mas/liza/internal/journal"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
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
	Repair            bool
	WarnWriter        io.Writer
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
	warnings := opts.WarnWriter
	if warnings == nil {
		warnings = warnWriter
	}

	projectRoot := filepath.Dir(filepath.Dir(statePath))
	logPath := filepath.Join(filepath.Dir(statePath), "log.yaml")
	if opts.Repair {
		if !opts.SkipProcessChecks {
			repaired, err := ops.RepairInvalidDoerOwnership(statePath, projectRoot, logPath, "validate --repair")
			if repaired > 0 {
				fmt.Fprintf(warnings, "REPAIRED: invalid active doer ownership cleared for %d task(s); worktrees remain on disk for inspection but may be removed by a later reclaim\n", repaired)
			}
			if err != nil {
				var refused *ops.DoerRepairRefusedError
				if stderrors.As(err, &refused) {
					fmt.Fprintf(warnings, "WARNING: invalid active doer ownership not repaired: %s\n", refused.Error())
				} else {
					return fmt.Errorf("repair invalid doer ownership: %w", err)
				}
			}
		}
		repaired, err := ops.RepairInvalidReviewOwnership(statePath, projectRoot, logPath, "validate --repair")
		if err != nil {
			return fmt.Errorf("repair invalid review ownership: %w", err)
		}
		if repaired > 0 {
			fmt.Fprintf(warnings, "REPAIRED: invalid active review ownership cleared for %d task(s)\n", repaired)
		}
	}

	state, err := db.For(statePath).Read()
	if err != nil {
		schemaErr := &lizaerrors.StateSchemaError{Operation: "validate", Err: err}
		return &lizaerrors.ValidationError{Message: schemaErr.Error(), Err: schemaErr}
	}

	if err := statevalidate.ValidateState(state, projectRoot, opts.SkipSpecFileCheck, warnings); err != nil {
		return &lizaerrors.ValidationError{Message: err.Error(), Err: err}
	}
	if !opts.SkipProcessChecks {
		if err := validateNoZombieAgents(state, projectRoot, warnings); err != nil {
			return &lizaerrors.ValidationError{Message: err.Error(), Err: err}
		}
	}
	warnJournalDivergence(statePath, state, warnings)
	return nil
}

// warnJournalDivergence reports where the journal-reconstructed view
// contradicts state.yaml across the dimensions the journal fully captures:
// task statuses, claim holders, and the system singletons (sprint, circuit
// breaker, goal, mode). Warning-level while the journal is a shadow log —
// state.yaml remains authoritative. Reads the full history (including rotated
// archives) so the claim/singleton folds, which have no rotation snapshot,
// stay complete. Dimensions the journal has no opinion on (pre-journal tasks,
// unseen singletons) are intentionally silent.
func warnJournalDivergence(statePath string, state *models.State, warnings io.Writer) {
	events, err := journal.ForStatePath(statePath).ReadAllIncludingArchives()
	if err != nil {
		fmt.Fprintf(warnings, "WARNING: journal unreadable, skipping coherence check: %v\n", err)
		return
	}
	if len(events) == 0 {
		return
	}

	recon := journal.Reconstruct(events)

	for i := range state.Tasks {
		task := &state.Tasks[i]
		projected, journaled := recon.TaskStatuses[task.ID]
		if !journaled {
			continue
		}
		if projected != string(task.Status) {
			fmt.Fprintf(warnings,
				"WARNING: journal/state divergence for task %s: journal=%q state=%q (investigate with `liza journal --task %s`)\n",
				task.ID, projected, task.Status, task.ID)
		}
	}

	for _, d := range recon.DiffClaims(state) {
		fmt.Fprintf(warnings, "WARNING: journal/state divergence: %s\n", d)
	}
	for _, d := range recon.DiffSingletons(state) {
		fmt.Fprintf(warnings, "WARNING: journal/state divergence: %s\n", d)
	}
}

func validateNoZombieAgents(state *models.State, projectRoot string, warnings io.Writer) error {
	zombies, err := findZombieAgents(procscan.ZombieScanOptions{
		ProjectRoot:    projectRoot,
		GoalID:         state.Goal.ID,
		RegisteredPIDs: registeredAgentPIDs(state),
	})
	if stderrors.Is(err, procscan.ErrProcessScanUnavailable) {
		fmt.Fprintln(warnings, "WARNING: Live liza agent process scan skipped (procfs unavailable on this host)")
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
