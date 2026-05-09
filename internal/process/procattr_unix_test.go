//go:build !windows

package process

import (
	"os/exec"
	"testing"
)

func TestSetDetachedProcessGroup_ConfiguresNewSession(t *testing.T) {
	cmd := exec.Command("true")

	SetDetachedProcessGroup(cmd)

	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr = nil, want detached attributes")
	}
	if !cmd.SysProcAttr.Setpgid {
		t.Fatal("Setpgid = false, want true")
	}
	if !cmd.SysProcAttr.Setsid {
		t.Fatal("Setsid = false, want true")
	}
}
