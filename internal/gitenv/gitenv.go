// Package gitenv provides locale-safe environment for git subprocesses.
package gitenv

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// DefaultCommandTimeout bounds git subprocesses owned by Liza. It is deliberately
// conservative: most git plumbing commands complete in milliseconds, while local
// rebases and large-tree operations may need substantially longer.
var DefaultCommandTimeout = 5 * time.Minute

// DefaultCommandWaitDelay bounds how long waits may continue after cancellation
// when child processes still hold stdout/stderr pipes open.
var DefaultCommandWaitDelay = 5 * time.Second

// TimeoutError reports a git command killed after exceeding its deadline.
type TimeoutError struct {
	Args    []string
	Dir     string
	Timeout time.Duration
	Output  []byte
}

func (e *TimeoutError) Error() string {
	if e.Dir != "" {
		return fmt.Sprintf("git %v timed out after %s in %s", e.Args, e.Timeout, e.Dir)
	}
	return fmt.Sprintf("git %v timed out after %s", e.Args, e.Timeout)
}

// Env returns the current environment with LC_ALL=C forced, so git output
// is always in English regardless of the system locale.
func Env() []string {
	env := os.Environ()
	for i, e := range env {
		if strings.HasPrefix(e, "LC_ALL=") {
			env[i] = "LC_ALL=C"
			return env
		}
	}
	return append(env, "LC_ALL=C")
}

// Command creates an exec.Cmd for git with LC_ALL=C forced.
func Command(args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Env = Env()
	return cmd
}

// CombinedOutput runs git with the package default timeout and returns combined
// stdout/stderr. The timeout is applied to the subprocess, not to any caller-held
// locks; callers should still avoid invoking git while holding state locks.
func CombinedOutput(dir string, args ...string) ([]byte, error) {
	return CombinedOutputWithTimeout(DefaultCommandTimeout, dir, args...)
}

// CombinedOutputWithTimeout runs git with a caller-specified timeout. It exists
// primarily for tests and for operations with well-understood tighter bounds.
func CombinedOutputWithTimeout(timeout time.Duration, dir string, args ...string) ([]byte, error) {
	if timeout <= 0 {
		timeout = DefaultCommandTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = Env()
	cmd.WaitDelay = DefaultCommandWaitDelay

	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return output, &TimeoutError{
			Args:    append([]string(nil), args...),
			Dir:     dir,
			Timeout: timeout,
			Output:  append([]byte(nil), output...),
		}
	}
	return output, err
}

// Output runs git with the package default timeout and returns stdout only.
func Output(dir string, args ...string) ([]byte, error) {
	return OutputWithTimeout(DefaultCommandTimeout, dir, args...)
}

// OutputWithTimeout runs git with a caller-specified timeout and returns stdout
// only. Use this when command semantics depend on stdout rather than stderr.
func OutputWithTimeout(timeout time.Duration, dir string, args ...string) ([]byte, error) {
	if timeout <= 0 {
		timeout = DefaultCommandTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = Env()
	cmd.WaitDelay = DefaultCommandWaitDelay

	output, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return output, &TimeoutError{
			Args:    append([]string(nil), args...),
			Dir:     dir,
			Timeout: timeout,
			Output:  append([]byte(nil), output...),
		}
	}
	return output, err
}
