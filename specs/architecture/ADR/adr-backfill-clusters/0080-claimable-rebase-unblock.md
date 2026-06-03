# Cluster 0080 - Claimable Rebase Unblock

## Commit Set
- `b3d44346` - feat(unblock-task): support claimable rebase unblock

## Intent Hypothesis
Let repaired blocked tasks rebase preserved worktrees onto integration and return to normal claimability.

## Architectural Signals
- `unblock-task` can restore role-pair initial status without `--assign-to`
- optional unblock-time rebase for preserved worktrees
- `base_commit` updates after successful unblock-time rebase
- conflicts remain `BLOCKED` with repair metadata
- state-machine and worktree-management specs updated

## User-Confirmed Context
- Trigger: a blocked task can be waiting on a missing artifact; after the orchestrator creates a task to fill the gap, the blocked task must rebase to see the artifact introduced into the integration branch.
- Rationale: `--assign-to` is inconvenient for sandboxed agents because they cannot reliably inspect process state and get confused by that limitation.
- Tradeoff: unblock owns more preserved-worktree rebase behavior and conflict repair metadata.

## Candidate Decision Date
2026-06-02

## Status
ADR generated: `specs/architecture/ADR/0080-claimable-rebase-unblock.md`

## Confidence
0.90 (high)
