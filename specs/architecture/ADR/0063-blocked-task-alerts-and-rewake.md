# 63 - Blocked Task Alerts and Re-Wake

## Context and Problem Statement

Blocked tasks could become queue dead weight. A task might block on missing dependencies, stale superseded references, unresolved questions, or repair-needed state, but the follow-up signal was not durable or visible enough for operators and agents to act reliably.

In practice, usability depended on manually inspecting state, watcher output, and blocked reasons. If blocked work was not surfaced clearly, the system appeared idle or stuck even when there was a specific next action.

## Considered Options

1. **Keep blocked state as task-local metadata** - simple, but operators must discover blocked work by polling and interpreting task state manually.
2. **Add canonical alerts and re-wake behavior for blocked tasks** - make blocked transitions and unresolved assessments visible as durable follow-up signals.

## Decision Outcome

Chose **Option 2**: blocked task lifecycle events write alerts and participate in wake/diagnostic paths.

### Architecture

- New `internal/alerts` package for canonical alert writes.
- `mark-blocked` writes alerts.
- unresolved blocked assessments raise follow-up alerts.
- alert write failures are surfaced as non-fatal warnings in human and JSON command paths.
- `unblock-task` and dependency validation include explicit blocked/dependency wake behavior.
- watch, TUI, prompt guidance, and support docs expose blocked-reason diagnostics.

### Rationale

Observability is essential to usability. A blocked task is not just a task status; it is a request for attention, dependency movement, or repair. If that signal is not durable and visible, the pipeline can appear to have capacity while making no progress.

Alerts are not the sole source of truth. State remains authoritative, and alert write failures are warnings rather than hard failures. The alert layer exists to make the lifecycle usable.

### Consequences

**Positive:**
- Blocked transitions produce visible follow-up signals.
- Operators can distinguish "no work" from "work is blocked and needs action."
- Dependency wake behavior becomes more explicit.
- Agent prompts and support docs can guide blocked-task repair using consistent diagnostics.

**Limitations accepted:**
- Alert writes can fail without failing the state transition.
- Alerting improves observability but does not replace state validation or dependency correctness.

**Extends:** ADR-0020 (Explicit Task Workflow Contract) - blocked transitions now have alert/wake semantics. ADR-0048 (Multi-Phase Planning) - blocked tasks in complex pipelines are easier to surface and resume.

---
*Reconstructed from commit 91e16578 and liza-run-issues.md (2026-05-18)*
