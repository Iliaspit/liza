# 58 - Output Entry Concrete Task Dependencies

## Status

ACCEPTED

## Context

`OutputEntry.depends_on` is sibling-local. Per ADR-0048, planners express ordering inside a single `output[]` payload with numeric indexes such as `"0"`, and `liza proceed` resolves those indexes to generated child task IDs.

This leaves no structured way for a planner to emit a child task that depends on an already-existing concrete task ID outside the current `output[]`. Using `depends_on` for both index references and task IDs would make the field context-dependent and would weaken the sibling-local contract.

## Decision

Add optional `task_depends_on []string` to `OutputEntry`.

- `depends_on` remains sibling output indexes only.
- `task_depends_on` names existing concrete task IDs.
- `liza set-task-output` validates `task_depends_on` entries as safe task IDs and rejects references to tasks not present in current state.
- `liza proceed` copies `task_depends_on` onto generated child `Task.DependsOn`, alongside resolved sibling dependencies and inherited phase-gate dependencies.

## Consequences

**Positive:**
- Planners can express dependencies on existing tasks without manual `add-tasks` replacement.
- `depends_on` keeps its ADR-0048 meaning and validation behavior.
- Generated child tasks continue using the single scheduler-facing `Task.DependsOn` field.

**Limitations accepted:**
- This is another additive `OutputEntry` schema field.
- The producing task must name tasks that already exist when `set-task-output` runs.
- Dependency cycles are still detected by state validation after generated child tasks exist.

**Extends:** ADR-0036 (Structured Task Output) — `task_depends_on` on `OutputEntry`. ADR-0048 (Multi-Phase Planning) — preserves sibling-index `depends_on` while adding concrete task dependency propagation for per-subtask output.

---
*Recorded 2026-05-11 for GitHub issue #31.*
