# Cluster 0070 - Active Task Cancellation

## Commit Set
- `ec0082c3` - fix(tasks): allow cancelling active tasks

## Intent Hypothesis
Give operators a first-class abort path for tasks that have already been claimed, submitted, or entered review.

## Architectural Signals
- `cancel-task` transition map expanded to executing, submitted, reviewing, and quorum-reviewing states
- Approved cancellation remains blocked
- State machine spec and support docs updated
- Cleanup and stale doer/reviewer command behavior covered by tests

## Reconstructed Context
- Trigger: operators needed a clean abort path after a task had already been claimed or submitted for review.
- Alternatives: manual state repair, superseding the task, waiting for review flow, or using crash recovery commands.
- Rationale: cancellation is a lifecycle operation, not a crash cleanup path; it should be encoded in the transition map and validated like other state changes.
- Tradeoffs: provider processes may continue briefly, cleanup must handle stale doer/reviewer ownership, and approved tasks remain protected because merge can move the integration ref.
- Related decisions: extends explicit lifecycle transitions and crash recovery without replacing either.

## Candidate Decision Date
2026-05-26

## Status
ADR generated: `specs/architecture/ADR/0070-active-task-cancellation.md`

## Confidence
0.80 (high)
