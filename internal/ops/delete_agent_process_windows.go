//go:build windows

package ops

import "os"

func signalAgentProcessTree(pid int) error {
	return killAgentProcessTree(pid)
}

func killAgentProcessTree(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}
