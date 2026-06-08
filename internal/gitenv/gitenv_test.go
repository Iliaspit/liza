package gitenv

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestCombinedOutputWithTimeout_KillsHungGit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake git is Unix-specific")
	}

	binDir := t.TempDir()
	fakeGit := filepath.Join(binDir, "git")
	if err := os.WriteFile(fakeGit, []byte("#!/bin/sh\nsleep 2\necho late\n"), 0755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	originalWaitDelay := DefaultCommandWaitDelay
	DefaultCommandWaitDelay = 50 * time.Millisecond
	t.Cleanup(func() { DefaultCommandWaitDelay = originalWaitDelay })

	start := time.Now()
	output, err := CombinedOutputWithTimeout(50*time.Millisecond, "", "status")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("CombinedOutputWithTimeout() error = nil, want timeout")
	}
	var timeoutErr *TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("CombinedOutputWithTimeout() error = %T %v, want *TimeoutError", err, err)
	}
	if elapsed >= time.Second {
		t.Fatalf("CombinedOutputWithTimeout() elapsed = %s, want under 1s", elapsed)
	}
	if len(output) != 0 {
		t.Fatalf("output = %q, want empty before fake git sleeps", string(output))
	}
	if timeoutErr.Timeout != 50*time.Millisecond {
		t.Errorf("Timeout = %s, want 50ms", timeoutErr.Timeout)
	}
	if len(timeoutErr.Args) != 1 || timeoutErr.Args[0] != "status" {
		t.Errorf("Args = %#v, want [status]", timeoutErr.Args)
	}
}

func TestOutputWithTimeout_ReturnsStdoutOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake git is Unix-specific")
	}

	binDir := t.TempDir()
	fakeGit := filepath.Join(binDir, "git")
	if err := os.WriteFile(fakeGit, []byte("#!/bin/sh\necho noisy >&2\necho tracked\n"), 0755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	output, err := OutputWithTimeout(5*time.Second, "", "ls-tree")
	if err != nil {
		t.Fatalf("OutputWithTimeout() error = %v, want nil", err)
	}
	if string(output) != "tracked\n" {
		t.Fatalf("output = %q, want stdout only", string(output))
	}
}
