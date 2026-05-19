# Cluster 0059 - Partial Planning Handoff

## Commit Set
- `bae0594d` - unblock partial planning handoff (#70)

## Source Issues
- GitHub issue #69 - merged planning outputs can starve behind unrelated active sprint tasks
- GitHub pull request #70 - fix: unblock partial planning handoff

## Intent Hypothesis
Decouple planning-output readiness from whole-sprint planning completion so merged planning outputs can create implementation children while unrelated planned tasks remain active.

## Architectural Signals
- `PLANNING_COMPLETE` wake detection moved outside the all-planned-terminal branch
- `liza resume` executes mid-sprint planning handoffs after checkpoint/resume
- `phase_handoff` status diagnostics expose ready planning outputs, active planned tasks, and stale assigned agents
- Regression coverage for mid-sprint planning handoff

## User Context Captured
- Trigger: 12 merged code-planning outputs were ready, but one unrelated active planner prevented all coding children from being generated.
- Rationale: sprint completion and output readiness are different concepts; checkpoint/resume remains the gate for executing transitions.
- Tradeoffs: partial readiness logic is more complex, and many-to-one readiness needed separate lifecycle handling to avoid another stuck checkpoint path.

## Candidate Decision Date
2026-05-15

## Status
ADR generated: `specs/architecture/ADR/0059-partial-planning-handoff.md`

## Confidence
0.90 (high)
