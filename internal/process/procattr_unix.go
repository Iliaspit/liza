//go:build !windows

package process

import (
	"os/exec"
	"syscall"
)

// SetDetachedProcessGroup places the command in a new session with its own
// process group so the spawned agent does not inherit the parent's controlling
// terminal.
func SetDetachedProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Setsid:  true,
	}
}
