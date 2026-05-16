// Package process provides shared subprocess management for agent spawning.
// Used by both the TUI and the HTTP API server.
package process

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/liza-mas/liza/internal/agent"
)

func buildSpawnCommand(projectRoot, role, cli string, extraArgs ...string) (*exec.Cmd, *os.File, error) {
	args := []string{"agent", role, "--cli", cli}
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
