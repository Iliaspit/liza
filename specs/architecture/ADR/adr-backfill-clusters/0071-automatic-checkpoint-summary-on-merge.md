# Cluster 0071 - Automatic Checkpoint Summary on Merge

## Commit Set
- `cdc2981e` - fix(agent): round-2 reviewer claim + reviewer-1 self-claim guard + auto-emit checkpoint-summary on merge (#82)

## Intent Hypothesis
Emit a human-readable checkpoint summary automatically after successful merges without blocking the merge loop.

## Architectural Signals
- New `internal/agent/checkpoint_summary.go`
- New `auto_checkpoint_summary` config field
- Best-effort subprocess invocation of the checkpoint-summary skill
- Output constrained to `.liza/checkpoint-summary.md`
- Unexpected project mutation checks around the subprocess

## Mixed-Commit Note
This commit also fixes round-2 reviewer claimability and reviewer-1 self-claim regressions. Those look like bug fixes under ADR-0046 rather than separate ADRs. The candidate decision is only the automatic checkpoint-summary behavior.

## User Context Captured
- Trigger: checkpoint summaries are one of the most valuable features for keeping users able to steer as Liza becomes more autonomous.
- Alternatives: existing logs, but logs lose agent intent and decision context.
- Rationale: capturing events and decisions from the blackboard preserves more context than archaeological reconstruction from logs.
- Tradeoffs: the value of preserved steering context is greater than the implementation limitations and risks.
- Related direction: decision capture is intended to grow; manual checkpoint-summary was the first step, automatic post-merge summaries are the second, with more to come.

## Candidate Decision Date
2026-05-26

## Status
ADR generated: `specs/architecture/ADR/0071-automatic-checkpoint-summary-on-merge.md`

## Confidence
0.65 (medium)
