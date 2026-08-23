package ops

import (
	"fmt"
	"strings"

	"github.com/liza-mas/liza/internal/secretmask"
)

// The command comes from user config and can be an inline script of any length;
// it is rendered into the recover hint and the agent-degraded record, so it is
// capped. With a filesystem-bounded Dir and a short exit status, this bounds the
// whole rendered diagnostic.
const postWorktreeCmdMaxBytes = 256

// PostWorktreeSetupError indicates the configured post_worktree_cmd failed for
// a worktree. The worktree is therefore not build-ready, so callers fail closed
// rather than hand it to a provider session (ADR-0117).
//
// The child process's own output is deliberately absent — not merely unlogged,
// but never captured. Masking cannot cover it: secretmask is built from this
// process's environment and recognizes a fixed set of credential-shaped keys,
// while a setup command can load a worktree env file, read another file, or
// generate a credential, then print it on failure. A value under a key like
// DATABASE_URL would pass through unmasked. Since no masking of child output is
// a guarantee, none is attempted and nothing is captured.
//
// The operator reproduces the failure by rerunning the named command in the
// named worktree, which the recover hint instructs.
type PostWorktreeSetupError struct {
	Cmd string // secret-masked, bounded
	Dir string
	Err error
}

// newPostWorktreeSetupError masks and bounds at capture, so every consumer —
// error text, logs, the agent-degraded record and its anomaly — gets state-safe
// values without repeating the treatment.
func newPostWorktreeSetupError(cmdStr, dir string, err error) *PostWorktreeSetupError {
	return &PostWorktreeSetupError{
		Cmd: boundPostWorktreeCmd(secretmask.New().MaskText(cmdStr)),
		Dir: dir,
		Err: err,
	}
}

func (e *PostWorktreeSetupError) Error() string {
	return fmt.Sprintf("post-worktree setup command failed in %s: %s: %v", e.Dir, e.Cmd, e.Err)
}

func (e *PostWorktreeSetupError) Unwrap() error {
	return e.Err
}

// boundPostWorktreeCmd caps the command head-first: the leading words identify
// which command failed, which is what the recover hint needs.
func boundPostWorktreeCmd(cmd string) string {
	if len(cmd) <= postWorktreeCmdMaxBytes {
		return cmd
	}
	return strings.ToValidUTF8(cmd[:postWorktreeCmdMaxBytes], "") + "... [truncated]"
}
