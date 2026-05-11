// Package precommit provides repo-state detection helpers for the
// pre-commit bootstrap planning step: checking whether the integration
// branch already carries .pre-commit-config.yaml, and whether a
// bootstrap task is already in flight anywhere in the state.
package precommit

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/liza-mas/liza/internal/gitenv"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/taskkind"
)

// Kind is the typed marker value (see architecture-1 §2.1) that the
// architect emits on output[].kind and that proceed.go treats as the
// authoritative dedup key. Exported so Go-side callers cannot drift on
// the literal. Go templates cannot import constants — the architect
// prompt template reads this value via RoleContextData.PreCommitKind
// rather than by package import.
const Kind = taskkind.PreCommitBootstrap

// ErrContextBuild is the sentinel returned (wrapped) by configuration errors
// that require human intervention. Callers use errors.Is to distinguish those
// failures from transient git plumbing failures, which should follow normal
// retry behavior instead of task-local BLOCKED recovery.
var ErrContextBuild = errors.New("precommit context build failed")

// ConfigExistsOnIntegration reports whether .pre-commit-config.yaml
// exists at the tip of the integration branch in the given project
// root. Reads committed state via git plumbing, not the working tree,
// so uncommitted human drift or in-progress worktree changes do not
// produce a false positive.
//
// Returns (true, nil) when the file is tracked on the branch.
// Returns (false, nil) when the branch exists and the file is not
// tracked on it. Returns an ErrContextBuild-wrapped error only for
// configuration-correctness failures that require human intervention
// (empty inputs or missing integration branch). Other git plumbing errors
// are returned without the sentinel so the supervisor's normal retry path
// can handle transient failures.
func ConfigExistsOnIntegration(projectRoot, integrationBranch string) (bool, error) {
	if projectRoot == "" {
		return false, fmt.Errorf("precommit: projectRoot is empty: %w", ErrContextBuild)
	}
	if integrationBranch == "" {
		return false, fmt.Errorf("precommit: integrationBranch is empty: %w", ErrContextBuild)
	}

	// Step 1 — verify the ref exists. Isolates "branch invalid" as a
	// hard configuration error from "path absent on existing branch".
	_, err := gitenv.CombinedOutput(projectRoot, "rev-parse", "--verify", "--quiet",
		integrationBranch+"^{commit}")
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return false, fmt.Errorf(
				"precommit: integration branch %q not found in %s: %w",
				integrationBranch, projectRoot, ErrContextBuild)
		}
		return false, fmt.Errorf("precommit: rev-parse --verify %q: %v",
			integrationBranch, err)
	}

	// Step 2 — read the tree entry. Exit 0 with empty stdout means
	// "branch has no such path"; non-empty means tracked.
	output, err := gitenv.Output(projectRoot, "ls-tree", integrationBranch, "--",
		".pre-commit-config.yaml")
	if err != nil {
		var exitErr *exec.ExitError
		stderr := ""
		if errors.As(err, &exitErr) {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
		return false, fmt.Errorf(
			"precommit: ls-tree %q -- .pre-commit-config.yaml: %v (stdout: %s, stderr: %s)",
			integrationBranch, err, strings.TrimSpace(string(output)), stderr)
	}
	return len(output) > 0, nil
}

// BootstrapInFlight reports whether any task in the given state has
// Kind == precommit.Kind and a non-terminal status. "Non-terminal"
// means Task.Status.IsTerminal() returns false — MERGED, ABANDONED,
// SUPERSEDED are terminal; every other status (including BLOCKED) is
// in flight. A blocked bootstrap will eventually merge once rescoped,
// so it genuinely is in flight (goal spec §Q2 "Rescope invariant").
//
// Repo-wide: scans state.Tasks with no goal/sprint filter; cross-goal
// parallelism is covered by construction. Returns false if state is
// nil.
func BootstrapInFlight(state *models.State) bool {
	if state == nil {
		return false
	}
	for i := range state.Tasks {
		t := &state.Tasks[i]
		if t.Kind == "" {
			continue
		}
		if t.Kind != Kind {
			continue
		}
		if t.Status.IsTerminal() {
			continue
		}
		return true
	}
	return false
}
