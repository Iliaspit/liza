# 83 - Preserve-by-Default recover-task

## Status

ACCEPTED

## Context

ADR-0023 introduced `liza recover-task` as a crash-recovery shortcut that released
claims and removed the task worktree/branch. That was useful for early doer crash
recovery, but it became a data-loss footgun once submitted review candidates and
review-boundary metadata became first-class.

For `READY_FOR_REVIEW` and `REVIEWING` tasks, `task/<id>` plus `review_commit`
can be the only submitted candidate. Deleting the branch and recreating from the
integration branch makes the task mechanically recoverable but semantically
wrong: a reviewer may end up reviewing integration rather than the submitted
work. The same risk exists for integration-repair tasks whose worktree/branch is
the repair substrate.

## Decision

Change `liza recover-task` to preserve recoverable task work by default.

Default in-state recovery now:
- releases stale doer/reviewer claims and removes dead claiming agents from
  state
- preserves an existing healthy, clean worktree when its branch exists
- reattaches a missing worktree from an existing task branch, then validates it
- requires submitted/reviewing candidates to have `review_commit == worktree
  HEAD`
- fails closed for dirty worktrees, missing branches, missing submitted
  candidates, and missing integration-repair substrates
- keeps `BLOCKED` tasks blocked; `unblock-task` remains the guarded transition
  back to claimability

Destructive reset is available only through `liza recover-task --fresh`.

`--fresh` removes the task worktree/branch, creates a fresh worktree from the
integration branch, clears active claim/review/attempt metadata, and records a
`task_recovered_fresh` history event. For normal non-terminal workflow statuses,
the target status is the role-pair initial status. For `BLOCKED`, the target
status remains `BLOCKED` and `blocked_reason` / `blocked_questions` are
preserved.

If artifact cleanup fails, recovery returns an error and leaves task state
unchanged because the old worktree/branch may still exist. If fresh worktree
creation fails after successful destructive artifact cleanup, recovery must not
abort with stale blackboard pointers to the deleted branch/worktree. It records
a truthful repair state instead: status `BLOCKED`, claims cleared, and
worktree/base/review metadata cleared, with `blocked_reason` describing the
fresh-creation failure.

`--force` remains separate from `--fresh`: it bypasses live-PID checks and still
enables git-only cleanup for tasks absent from state. If a claimant PID is alive,
destructive reset requires both `--fresh --force`.

## Consequences

Positive:
- Submitted review candidates are not silently discarded by default.
- Reviewer recovery validates the review boundary instead of recreating a fresh
  integration-based worktree.
- Operators have an explicit destructive path when discarding task work is the
  intended repair.
- `BLOCKED` remains governed by `unblock-task`, avoiding accidental unblocking
  through recovery tooling.

Trade-offs:
- Some corrupt states now require an explicit `--fresh` decision or a supersede
  workflow instead of being automatically made claimable.
- Recovery has a status matrix instead of one universal postcondition.
- Operators must distinguish substrate repair from workflow unblocking.

## Relationship to Prior Decisions

Supersedes the `recover-task` deletion semantics described in ADR-0023 while
preserving ADR-0023's general crash-recovery command shape and PID-aware
operation. Complements ADR-0064 (Review Boundary Recovery), ADR-0078
(Repairable Review Boundary Metadata), and ADR-0080 (Claimable Rebase Unblock)
by treating task branches and `review_commit` as correctness boundaries rather
than disposable cleanup artifacts.
