# 70 - Active Task Cancellation

## Status

ACCEPTED

## Context

Operators can discover that a task is mis-framed after it has already been claimed, submitted, or entered review. Before this decision, the clean cancellation path was too narrow, pushing operators toward manual state edits, waiting for review flow, or using recovery commands for a non-crash situation.

Cancellation is a lifecycle decision, not merely cleanup. It must preserve ownership invariants, cleanup expectations, and merge boundaries.

## Decision

Allow `cancel-task` to abandon active tasks before approval.

The pipeline transition map now permits cancellation from executing, submitted, reviewing, and quorum-reviewing states. Approved cancellation remains blocked because an approved task can already be moving the integration ref through merge behavior before state update completes.

Cancellation cleanup handles stale doer and reviewer ownership paths, and the support documentation calls out provider-process caveats.

## Consequences

Positive:
- Operators get a first-class abort path for active but wrong work.
- Cancellation becomes an invariant-checked lifecycle operation instead of manual state repair.
- Review-flow and quorum states can be abandoned without inventing a false rejection or supersession.
- Approved task boundaries remain protected.

Trade-offs:
- Provider subprocesses can outlive the state transition briefly.
- Cleanup must handle more ownership states.
- Cancellation still cannot undo work that has crossed the approval/merge boundary.

## Alternatives Considered

1. Require task supersession.

Rejected because supersession represents replacement work, not necessarily a desire to abort the active task entirely.

2. Use crash recovery commands.

Rejected because cancellation is intentional operator control, not recovery from an inconsistent or crashed process.

3. Allow cancellation after approval.

Rejected because approval can trigger merge activity and integration-ref movement; cancellation at that point risks lying about merged state.

## Relationship to Prior Decisions

Extends ADR-0020 (Explicit Task Workflow Contract) by adding more valid terminal transitions. Complements ADR-0023 (Crash Recovery Commands) while keeping intentional cancellation separate from crash cleanup.

---
*Reconstructed from commit ec0082c3 (2026-05-26)*
