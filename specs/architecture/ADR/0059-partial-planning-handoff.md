# 59 - Partial Planning Handoff

## Context and Problem Statement

Planning output readiness was coupled to whole-sprint planning completion. `DetectOrchestratorWakeTriggers` emitted `PLANNING_COMPLETE` only when all planned tasks in the sprint were terminal.

That made a single unrelated active planner able to block every already-merged planning output in the same sprint. In the observed run, 12 merged code-planning tasks had unconsumed `output[]`, coder agents were idle, and no implementation tasks were generated because one planner remained active.

The issue was not raw capacity. The pipeline had ready work, but the wake condition waited for the wrong lifecycle boundary.

## Considered Options

1. **Keep planning handoff gated by all planned tasks terminal** - simple sprint-level gate, but one stalled or unrelated planned task can starve ready implementation work.
2. **Detect ready planning outputs independently** - allow each merged planning task with unconsumed output to trigger handoff while checkpoint/resume still controls execution.

## Decision Outcome

Chose **Option 2**: detect planning-output readiness independently from sprint-wide planning completion.

### Architecture

- `PLANNING_COMPLETE` wake detection runs for merged planning tasks with unconsumed output even when unrelated planned tasks remain active.
- `liza sprint-checkpoint` and `liza resume` remain the gate before transitions execute.
- `liza resume` can execute mid-sprint planning handoffs and clear the planning checkpoint trigger.
- `phase_handoff` status diagnostics expose:
  - ready planning tasks with unconsumed output
  - non-terminal planned tasks still active in the sprint
  - stale assigned agents that may explain lack of progress

### Rationale

This streamlines the pipeline. Whole-sprint planning completion and per-task output readiness are different facts. The pipeline should not wait for unrelated planning work before expanding already-approved plans into implementation children.

Checkpoint/resume remains the synchronization point, so partial handoff does not make transitions invisible or automatic in places where a gate is still required.

### Consequences

**Positive:**
- Ready planning outputs no longer starve behind unrelated active planners.
- Idle coder capacity can be used as soon as upstream approved output is available.
- Status diagnostics make partial handoff state explicit.

**Limitations accepted:**
- Wake detection is more granular than the previous all-planned-terminal check.
- Many-to-one readiness needed separate lifecycle handling so partial readiness does not create checkpoint/resume dead ends.
- If an inconsistency between planning tasks is discovered late, implementation tasks already generated from earlier planning outputs may need to be reworked.

**Extends:** ADR-0035 (Declarative Sub-Pipelines) - partial readiness within pipeline execution. ADR-0048 (Multi-Phase Planning) - planning outputs can advance independently inside a sprint.

---
*Reconstructed from commit bae0594d and GitHub issue #69 / PR #70 (2026-05-15)*
