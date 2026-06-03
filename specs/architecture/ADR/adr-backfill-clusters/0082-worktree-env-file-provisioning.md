# Cluster 0082 - Worktree Env-File Provisioning

## Commit Set
- `b1d3ed79` - feat(worktree): copy ignored env files into worktrees

## Intent Hypothesis
Allow post-worktree setup to run with required ignored project-root environment files present in task worktrees.

## Architectural Signals
- new `internal/envgate` helper
- config field for env-file copy behavior
- init CLI/config wiring and `LIZA_ENABLE_COPY_ENV_FILES`
- create, claim, and reviewer recovery paths provision env files before post-worktree commands
- source and destination ignore-status validation
- worktree-management and support-doc updates

## Reconstructed Context
- Trigger: ignored local env files are absent from linked worktrees, but some post-worktree setup commands need them.
- Alternatives: manual copy, tracked env files, no env files, reading parent-root env state.
- Rationale: opt-in provisioning preserves worktree isolation while making setup reproducible enough.
- Tradeoffs: credential-sensitive copy path; stale copies and ignore-status edge cases.

## Candidate Decision Date
2026-06-03

## Status
ADR generated: `specs/architecture/ADR/0082-worktree-env-file-provisioning.md`

## Confidence
0.90 (high)
