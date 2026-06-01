# 71 - Automatic Checkpoint Summary on Merge

## Status

ACCEPTED

## Context

Checkpoint summaries are one of the most valuable features for keeping users able to steer as Liza becomes more autonomous. As agents make more decisions between human interventions, relying only on logs leaves too much intent and context behind. Logs record activity; they do not reliably preserve why agents chose a path or what decisions were made along the way.

Capturing events and decisions from the blackboard is more solid than reconstructing them later through log archaeology. Manual checkpoint-summary usage was the first step. Automatically producing a fresh summary after successful merges is the next step in making decision capture a built-in steering aid.

## Decision

After successful merges, Liza automatically invokes the configured agent CLI with the `checkpoint-summary` skill and writes the latest report to `.liza/checkpoint-summary.md`.

The operation is best-effort:
- merge success does not depend on summary generation
- failures are logged rather than rolled back
- `auto_checkpoint_summary: false` disables the behavior
- the subprocess may only mutate `.liza/checkpoint-summary.md`

The summary is intentionally generated from blackboard state rather than from raw logs.

## Consequences

Positive:
- Humans get fresh steering context after merge progress without remembering to run a manual summary.
- Decisions and agent intent are captured closer to the source of truth.
- The report lives under `.liza`, avoiding user-owned project docs.
- Summary failures do not block successful merges.
- The mechanism creates a foundation for richer decision capture later.

Trade-offs:
- Summary generation adds a subprocess dependency to the merge loop.
- The report is only as good as the blackboard and skill interpretation.
- Best-effort behavior means a missing summary does not halt the system.
- Mutation guarding is required to prevent the subprocess from changing project files.

## Alternatives Considered

1. Rely on logs.

Rejected because logs lose intent and decision context.

2. Keep checkpoint summaries manual.

Rejected because the feature is most useful when it appears at the natural steering point after merged progress.

3. Make summaries part of merge success.

Rejected because preserving user steering context is valuable, but a summary failure should not roll back or block already-successful merge work.

## Relationship to Prior Decisions

Extends ADR-0049 (Structured Handoff Events) by treating structured blackboard data as durable context for human steering. Complements ADR-0055 (Integration Sub-Pipeline) and later decision-capture work; automatic checkpoint summaries are a step toward richer built-in decision capture.

---
*Reconstructed from commit cdc2981e and user context (2026-05-26)*
