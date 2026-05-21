# Re-Wake Orchestrator for Cross-Architect BLOCKED Ordering

## Problem Statement

`internal/agent/workdetection.go:135-199` computes orchestrator wake-worthiness for BLOCKED tasks. A task becomes actionable again after an `orchestrator_assessment` only when one of:

1. New non-assessment history event on the task itself.
2. New non-assessment history event on any task in `task.depends_on`.
3. Human note targeting this task (by ID or `"all"`) timestamped after the last assessment.

None of these fire when a BLOCKED task is waiting on a **sibling** whose relationship is not captured in `depends_on` — e.g. cross-architect ordering where an architect's plan forbids setting `depends_on` and instructs the coder to mark BLOCKED until the sibling merges.

Concrete instance (observed 2026-04-17 in-sprint):

- `architecture-3-architecture-to-code-plan-1-code-plan-to-coding-0` is BLOCKED on sibling `architecture-2-architecture-to-code-plan-0-code-plan-to-coding-0` (currently IMPLEMENTING_CODE).
- `blocked_reason` explicitly states: *"cross-architect ordering enforced by orchestrator phase gating, not by sibling depends_on"*.
- Orchestrator assessed the block twice (17:02 and 17:19); no `depends_on` → no re-wake signal when the sibling progresses or merges.
- "Orchestrator phase gating" is not implemented as a wake trigger; it exists in intent only.

Consequence: the blocked task is permanently parked until a human intervenes (human note, direct state edit, or supersede). The sprint cannot advance through this branch without manual prodding.

The framing is not a one-off: the project pattern `architect plan §4.7` that produces this situation will recur every time a coding task needs a sibling's struct-field landing.

## Design

Add a re-wake signal that matches the actual blocking relationship. Three candidate mechanisms, in increasing order of ambition:

### Option A — Expand the re-wake scan to `parent_tasks` siblings

When checking whether a BLOCKED task is actionable, also walk the task's parent(s) and scan sibling tasks (other `output[]` of the same parent) for non-assessment history after the last assessment.

- Pro: captures the common "sibling of the same architect/planner output" case with zero spec or UX change.
- Con: over-broad — unrelated siblings progressing could wake the orchestrator when no real signal has arrived. Risk of wake spam.

### Option B — Require `depends_on` for ordering blocks, enforce at `mark-blocked`

Change contract: a coder marking BLOCKED for ordering reasons must set `depends_on` on the blocking task. `liza mark-blocked` grows a `--depends-on` flag; absence of `depends_on` on a BLOCKED task with an ordering-shaped reason is a validation warning.

- Pro: preserves the single explicit signal the wake logic already honors. No wake-logic change.
- Con: overrides the architect plan's "not by sibling depends_on" directive. Also requires coders to know sibling task IDs at mark-blocked time (usually yes, since `blocked_reason` cites them).

### Option C — Named blocker references, separate from `depends_on`

Add a `blocked_on: [<task-id>, …]` field distinct from `depends_on`. Wake logic unions `depends_on` and `blocked_on` when scanning for re-wake. `liza mark-blocked --blocked-on <id>` populates it. `blocked_on` does not affect task scheduling or DAG semantics — purely a wake hint.

- Pro: clean separation. Keeps `depends_on` semantics (scheduling DAG) distinct from wake hints. Matches the "phase gating" intent of the original architect plan without forcing scheduling semantics onto it.
- Con: new field. Migration of existing BLOCKED tasks optional.

### Recommendation

**Option B** as the primary fix. `depends_on` is not just a wake hint: it is the existing scheduling dependency that prevents a task from being claimed until upstream work is terminal. That is acceptable here because the blocker is an actual ordering constraint — the task should not resume until the named sibling has landed.

The architect plan's "not by sibling depends_on" directive should be revisited because it suppresses the only existing structural representation of this ordering constraint. If future cases need wake-only behavior without scheduling impact, that is Option C's `blocked_on` field, not an overload of `depends_on`.

Option A is tempting for its simplicity but the wake-spam risk is real — parent-task siblings frequently make unrelated progress.

## Implementation Surface

- `internal/models/task.go` — no schema change (Option B uses existing `depends_on`).
- `internal/commands/mark_blocked.go` (or equivalent) — add `--depends-on` repeatable flag; populate `task.DependsOn`.
- `internal/statevalidate/` — new lint: BLOCKED task whose `blocked_reason` names an existing task ID and whose `depends_on` is empty → warning, surfaced in `liza validate`. Text heuristics such as `waiting on`, `sibling.*not.*merged`, and `cross-.*ordering` may provide secondary hints, but the primary signal should be structural.
- `internal/prompts/templates/blocks/coder_tools.tmpl` (or wherever the coder's BLOCKED protocol is documented) — instruct coders to pass `--depends-on` when marking blocked on a specific other task.
- Architect-plan guidance: the phrase *"enforced by orchestrator phase gating, not by sibling depends_on"* is obsolete under this spec. An ADR amendment or a note in the architect protocol should clarify that `depends_on` is both the scheduling dependency and the re-wake signal for true cross-architect ordering constraints.

## Immediate Mitigation (before this spec lands)

For in-flight blocked tasks waiting on siblings:

- Human pauses agents, backs up `state.yaml`, edits narrowly to add `depends_on: [<sibling-task-id>]` on each affected BLOCKED task, runs `liza validate`, then resumes.
- Or: human adds `HumanNote{For: "all", Timestamp: now()}` to force a one-time wake.

No CLI exists today for either. The second is cheaper and non-destructive; the first is the correct permanent form once the spec lands.

## Acceptance Criteria

1. `liza mark-blocked --depends-on <task-id>` sets `task.DependsOn` and transitions to BLOCKED atomically.
2. `liza validate` emits a warning for any BLOCKED task whose `blocked_reason` names an existing task ID and whose `depends_on` is empty.
3. When a task listed in a blocked task's `depends_on` transitions (any non-assessment event) after the block's last `orchestrator_assessment`, `countActionableBlockedTasks` counts the blocked task as actionable.
4. An integration test reproduces the cross-architect scenario: task A BLOCKED with `depends_on: [B]`, orchestrator assessed, B transitions → next wake detection returns `BLOCKED_TASKS` with count ≥ 1.
5. Coder-role documentation instructs the coder to pass `--depends-on` when the BLOCKED reason names a specific sibling.

## Out of Scope

- Option C's `blocked_on` field — revisit only if future cases need wake-only behavior without scheduling impact.
- Auto-unblocking (moving BLOCKED → READY without orchestrator round-trip). The orchestrator-round-trip is valuable: it forces re-assessment rather than assumed-good resumption.
- Bulk retrofit of existing BLOCKED tasks' `depends_on`. Migration is optional; the immediate-mitigation path covers the transient.

## Status

Implemented 2026-05-18 via Option B: `liza mark-blocked --depends-on <task-id>` records explicit blocker dependencies, validation warns when a blocked reason names another task without `depends_on`, and `unblock-task` refuses to resume until dependencies are directly MERGED. The out-of-scope `blocked_on` field remains deferred.
