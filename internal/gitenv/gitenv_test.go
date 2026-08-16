package gitenv

import (
	"context"
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

func TestContextCommands_CancelHungGit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake git is Unix-specific")
	}

	binDir := t.TempDir()
	fakeGit := filepath.Join(binDir, "git")
	if err := os.WriteFile(fakeGit, []byte("#!/bin/sh\nexec sleep 30\n"), 0755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	originalWaitDelay := DefaultCommandWaitDelay
	DefaultCommandWaitDelay = 50 * time.Millisecond
	t.Cleanup(func() { DefaultCommandWaitDelay = originalWaitDelay })

	tests := []struct {
		name string
		run  func(context.Context) ([]byte, error)
	}{
		{name: "combined output", run: func(ctx context.Context) ([]byte, error) {
			return CombinedOutputContext(ctx, "", "status")
		}},
		{name: "stdout only", run: func(ctx context.Context) ([]byte, error) {
			return OutputContext(ctx, "", "status")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			timer := time.AfterFunc(50*time.Millisecond, cancel)
			defer timer.Stop()

			start := time.Now()
			_, err := tt.run(ctx)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("context-aware Git error = %v, want context.Canceled", err)
			}
			if elapsed := time.Since(start); elapsed >= time.Second {
				t.Fatalf("context-aware Git cancellation took %s, want under 1s", elapsed)
			}
		})
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
