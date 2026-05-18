// Package process provides shared subprocess management for agent spawning.
// Used by both the TUI and the HTTP API server.
package process

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/liza-mas/liza/internal/agent"
	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/paths"
)

func buildSpawnCommand(projectRoot, role, cli string, extraArgs ...string) (*exec.Cmd, *os.File, error) {
	args := []string{"agent", role, "--cli", cli}
	if goalID := readGoalID(projectRoot); goalID != "" && !hasFlag(extraArgs, "--goal-id") {
		args = append(args, "--goal-id", goalID)
	}
	args = append(args, extraArgs...)

	cmd := exec.Command("liza", args...)
	cmd.Dir = projectRoot
	SetDetachedProcessGroup(cmd)

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("open devnull: %w", err)
	}
	cmd.Stdin = devNull
	cmd.Stdout = devNull
	cmd.Stderr = devNull

	return cmd, devNull, nil
}

func readGoalID(projectRoot string) string {
	if projectRoot == "" {
		return ""
	}
	state, err := db.For(paths.New(projectRoot).StatePath()).Read()
	if err != nil {
		return ""
	}
	return state.Goal.ID
}

func hasFlag(args []string, name string) bool {
	for _, arg := range args {
		if arg == name || strings.HasPrefix(arg, name+"=") {
			return true
		}
	}
	return false
}

// SpawnAgent starts a detached `liza agent` subprocess with stdout/stderr
// redirected to /dev/null. The child process is placed in its own process
// group and a background goroutine reaps it to prevent zombie accumulation.
//
// Returns the started command and an error. The caller owns lifecycle
// management (the process is already started and will be reaped).
func SpawnAgent(projectRoot, role, cli string, extraArgs ...string) (*exec.Cmd, error) {
	if agent.CheckQuotaSignal(projectRoot, cli) {
		err := fmt.Errorf("provider quota exhausted for %s; refusing to spawn %s", cli, role)
		if alertErr := agent.LogQuotaSpawnBlockedAlert(projectRoot, cli, role); alertErr != nil {
			return nil, errors.Join(err, fmt.Errorf("write quota spawn-blocked alert: %w", alertErr))
		}
		return nil, err
	}
	if agent.CheckProviderUnavailableSignal(projectRoot, cli) {
		err := fmt.Errorf("provider unavailable for %s; refusing to spawn %s", cli, role)
		if alertErr := agent.LogProviderUnavailableSpawnBlockedAlert(projectRoot, cli, role); alertErr != nil {
			return nil, errors.Join(err, fmt.Errorf("write provider-unavailable spawn-blocked alert: %w", alertErr))
		}
		return nil, err
	}

	cmd, devNull, err := buildSpawnCommand(projectRoot, role, cli, extraArgs...)
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		devNull.Close()
		return nil, err
	}
	go func() {
		cmd.Wait()
		devNull.Close()
	}()

	return cmd, nil
}
