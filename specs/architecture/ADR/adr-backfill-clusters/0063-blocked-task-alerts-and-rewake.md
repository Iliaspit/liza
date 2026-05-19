# Cluster 0063 - Blocked Task Alerts and Re-Wake

## Commit Set
- `91e16578` - alert and re-wake blocked tasks

## Source Material
- `liza-run-issues.md` sections 4 and 9

## Intent Hypothesis
Make blocked-task state transitions visible and resumable by writing canonical alerts, preserving dependency wake behavior, and surfacing unresolved blocked assessments as follow-up work.

## Architectural Signals
- New `internal/alerts` package
- `mark-blocked`, `assess-blocked`, `unblock-task`, watch, TUI, and JSON command paths updated around alerts
- `depends_on` wiring and dependency validation added for blocked/unblock flows
- Prompt/support docs updated with blocked-reason diagnostics and wake behavior

## User Context Captured
- Trigger: dependency sequencing, superseded references, and blocked tasks could require manual repair without a durable alert/wake signal.
- Rationale: blocked state should not become invisible queue dead weight; operators and agents need canonical follow-up signals.
- Tradeoffs: alert write failures are non-fatal warnings, so alerts improve observability but are not the sole source of truth.

## Candidate Decision Date
2026-05-18

## Status
ADR generated: `specs/architecture/ADR/0063-blocked-task-alerts-and-rewake.md`

## Confidence
0.75 (medium, user confirmed ADR-worthy)
