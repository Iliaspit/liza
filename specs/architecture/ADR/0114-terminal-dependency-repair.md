# 114 - Terminal Dependency Repair

## Context and Problem Statement

Dependency direction is a persisted-state invariant for every task, including
terminal tasks. Supersession already canonicalized active consumers to the
replacement tasks, but it could leave the retiring task's own `depends_on`
edges untouched. A planning task that depended on downstream coding work could
therefore become `SUPERSEDED` while leaving the whole blackboard invalid.

The existing `retarget-dependency` operation is intentionally limited to one
edge on a non-terminal task. It cannot repair a `SUPERSEDED` task, and repairing
multiple illegal edges one at a time would fail full-state validation after the
first partial rewrite. Editing `.liza/state.yaml` directly would bypass locking,
authorization, audit history, and candidate-state validation.

## Decision Outcome

Prevent new terminal dependency corruption at the supersession boundary and
provide one narrow recovery operation for existing corruption.

When `supersede-task` retires a task, the same atomic mutation removes every
direct dependency whose role-pair is downstream of the retiring task's
role-pair. Legal same-pair, upstream, and unrelated dependencies are retained.
The removed IDs are recorded in the task's `superseded` history event, active
consumers are still rewritten to the declared replacements, and candidate-state
validation runs before any part of the mutation is persisted.

For an already-corrupted terminal task, an orchestrator runs:

```bash
liza repair-superseded-dependencies <task-id> --reason <reason>
```

The command is authorized only for the orchestrator role and accepts only a
task currently in `SUPERSEDED` status with at least one illegal downstream
direct dependency. In one locked transaction it removes all such edges,
retains legal dependencies, appends a `dependencies_rewritten` history event
with the operation, reason, caller, removed IDs, and retained IDs, and validates
the full candidate state before commit. Rejection or validation failure leaves
task state unchanged.

Recovery does not reactivate or otherwise rewrite the historical task:
`status`, `superseded_by`, `rescope_reason`, assignments, leases, worktree
metadata, outputs, and all unrelated task fields remain unchanged. After a
successful state commit, the operation appends the ordinary activity-log entry;
an activity-log failure is returned as a warning without rolling back the valid
state repair.

Operators and agents must use this command rather than edit the blackboard
directly.

## Consequences

**Positive:**

- Supported supersession cannot introduce this terminal dependency-direction
  violation.
- Existing multi-edge corruption is repaired atomically instead of passing
  through an invalid intermediate state.
- Authorization, task history, activity logging, and full validation make the
  recovery attributable and reviewable.
- Terminal history and replacement topology remain intact.

**Trade-offs:**

- Supersession now has a wider atomic mutation and validation surface.
- The recovery command is deliberately narrow: it does not repair active tasks,
  `MERGED` or `ABANDONED` tasks, or `output[].task_depends_on` metadata.
- Activity logging remains best-effort after the state commit, so callers must
  inspect returned warnings.

## Alternatives Considered

1. Ignore dependency-direction violations on terminal tasks.

Rejected because invalid historical metadata can feed later transitions and
because it would weaken the global dependency-direction invariant.

2. Allow `retarget-dependency` on terminal tasks.

Rejected because its single-edge contract cannot repair multiple illegal edges
while full-state validation remains fail-closed.

3. Edit `.liza/state.yaml` directly.

Rejected because direct edits bypass the state machine's locking, RBAC, audit,
and atomic validation guarantees.

## Relationship to Prior Decisions

- **Extends ADR-0077:** dependency canonicalization now includes pruning the
  retiring task's own illegal downstream edges at supersession time.
- **Complements ADR-0075:** active direct-edge retargeting remains separate from
  terminal dependency repair.
- **Preserves ADR-0065's semantic outcome:** active consumers continue to follow
  replacement work rather than treating supersession as dependency satisfaction.
