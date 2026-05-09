// Package process provides shared subprocess management for agent spawning.
// Used by both the TUI and the HTTP API server.
package process

import (
	"fmt"
	"os"
	"os/exec"
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
