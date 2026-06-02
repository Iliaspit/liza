# Code Planning: Pairing Semble Project-Root Safety Activation

Task: `code-planning-main-1-code-planning-3`
Agent: `code-planner-1`
Spec: `specs/goals/20260602-indexing-activation.md`
Parent plan: `specs/plans/20260602-indexing-activation/20260602-123514-code-planning-main-1.md`

## Based On

- `specs/goals/20260602-indexing-activation.md` read in full.
- `specs/plans/20260602-indexing-activation/20260602-123514-code-planning-main-1.md` read in full, including Task 4 and shared file ownership.
- `liza get code-planning-main-1-code-planning-3 --json` for assigned task scope, done_when, validation, and dependency metadata.
- `liza get code-planning-main-1-code-planning-0 --output-summary --json` confirming the dependency task is merged and owns only generic optional-index routing defaults.
- Stacklit module summary for `internal/semble`.
- `internal/semble/doc.go` lines 1-10.
- `internal/semble/semble.go` lines 1-220, 300-460, and 560-660.
- `internal/semble/semble_test.go` lines 120-180 and 340-470.
- `specs/architecture/ADR/README.md` for architectural decision context.

## Architectural Approach

Keep the Semble project-root safety rule inside `internal/semble`, next to the existing Semble target-safety and default ignore-pattern source of truth.

Existing `ValidateTargetSafety(TargetKindProjectRoot)` already proves that SessionStart metadata should be omitted unless a physical `.sembleignore` exists with every required sensitive/runtime ignore pattern. This task should add a pairing-init-facing helper that can establish that prerequisite at the physical project root before downstream init wiring attempts to advertise Semble.

The helper should be conservative with user-owned files:

- when `.sembleignore` is absent, create it with the default ordered payload from `DefaultIgnorePatterns`;
- when `.sembleignore` already contains all required patterns, report success without rewriting or duplicating content;
- when `.sembleignore` exists but is incomplete or unreadable, report a bounded diagnostic listing the safety failure rather than silently overwriting user content;
- when creation fails, report a clear diagnostic that downstream pairing init can surface;
- keep task-worktree/MAS safety semantics unchanged by leaving `TargetKindTaskWorktree`, `GeneratedWorktreeIgnorePayload`, and prompt metadata gating behavior intact except for shared default payload reuse.

This preserves the parent plan's Pairing Semble Safety Contract: `internal/semble` owns the default physical `.sembleignore` payload and project-root safety check used before pairing SessionStart advertises Semble; pairing init consumes the API but does not define the payload.

## Planned Tasks

### Task 1 - Add pairing Semble project-root safety activation

desc: Add pairing Semble project-root safety activation.

done_when: Pairing Semble activation can create or verify a safe physical project-root `.sembleignore` using the default sensitive/runtime ignore patterns before SessionStart can advertise Semble, reports a clear diagnostic when safety cannot be established, and leaves MAS Semble worktree safety semantics intact.

scope: Owns `internal/semble` project-root ignore helpers and tests for default payload, idempotent verification, unsafe/missing ignore handling, and diagnostics. Does not wire pairing init CLI behavior, does not edit SessionStart hook output, and does not add durable Semble config to state.

spec_ref: specs/goals/20260602-indexing-activation.md

validation:

- `go test ./internal/semble`

depends_on:

- Existing task `code-planning-main-1-code-planning-0`

decomposition:

```yaml
owned_files:
  - internal/semble/semble.go
  - internal/semble/semble_test.go
owned_modules:
  - internal/semble
read_only_depends_on: []
read_only_task_depends_on:
  - code-planning-main-1-code-planning-0
interfaces_owned:
  - Pairing Semble Safety Contract
interfaces_consumed:
  - Global Tool Guidance Contract
coverage_notes: "Covers physical .sembleignore creation/verification, default sensitive/runtime ignore payload reuse, idempotent success on already-safe files, diagnostics for incomplete/unreadable/uncreatable safety artifacts, and non-interference with MAS task-worktree Semble gating."
```

## Dependency Graph

```text
Existing code-planning-main-1-code-planning-0
  -> Task 1
```

Task 1 is a single downstream coding task because implementation and unit tests are colocated in one package and one observable behavior: project-root Semble safety establishment for pairing init consumers. Splitting tests from behavior would violate the TDD colocation rule, and splitting default payload from ensure/verify behavior would create shared-file contention in `internal/semble/semble.go`.

## Scope Boundaries

- In scope: `internal/semble` API and unit tests for a pairing-init-facing project-root `.sembleignore` create/verify helper.
- In scope: retaining `DefaultIgnorePatterns` as the single ordered source of truth for `.liza/`, `.worktrees/`, generated index files, environment files, credentials, keys, and secret directories.
- In scope: diagnostics that downstream init code can display when the safety artifact cannot be established.
- Out of scope: invoking the helper from `liza init`, editing `internal/commands/init.go`, editing provider SessionStart hook output, changing MAS prompt metadata construction, and adding durable Semble state/config.

## Spec Compliance Matrix

Rows are scoped to the CP3-owned Pairing Semble Safety Contract. Sibling-owned requirements from the full goal spec remain governed by the merged master plan and are not re-planned here.

| # | Requirement | Source | Task(s) | Status |
|---|-------------|--------|---------|--------|
| 1 | Optional indexing failures must degrade by omitting unavailable prompt/context sections rather than making agents troubleshoot unrelated tooling. CP3 contributes by returning clear safety diagnostics and preserving prompt omission unless target safety passes. | General NFR-000-3, lines 49-51 | Task 1 | Covered |
| 2 | Pairing init must ensure generated pairing indexes are ignored, privately excluded, or otherwise kept out of accidental task diffs unless already intentionally tracked. CP3 contributes the Semble default ignore payload that excludes generated/runtime artifacts. | FR-002-4, lines 181-183 | Task 1 | Covered |
| 3 | When `LIZA_ENABLE_SEMBLE` is truthy, pairing init must ensure the project root has a safe physical `.sembleignore` before SessionStart advertises Semble. | FR-002-5, lines 184-186 | Task 1 | Covered |
| 4 | Pairing index activation must preserve the existing rule that SessionStart only supplies concrete paths/readiness that exist for the current repository. | NFR-002-1, lines 203-205 | Task 1 | Covered |
| 5 | Pairing index activation must not assume Liza's own build system or language stack applies to target projects. | NFR-002-2, lines 206-207 | Task 1 | Covered |
| 6 | Given pairing init with Semble enabled but no safe `.sembleignore`, init creates or reports the missing safety artifact before Semble is advertised. | AC-002-3, lines 216-218 | Task 1 | Covered |
| 7 | MAS init behavior must not depend on pairing Git hooks or repo-root pairing indexes. CP3 preserves this by leaving MAS task-worktree safety semantics unchanged. | FR-003-4, lines 290-291 | Task 1 | Covered |
| 8 | Disabled Semble env gate omits MAS Semble prompt section. CP3 must not weaken existing env-gated metadata behavior while adding pairing safety helpers. | AC-003-3, lines 309-310 | Task 1 | Covered |
| 9 | Pairing SessionStart continues emitting concrete repo-root optional tool instructions only when artifacts/readiness exist. CP3 contributes the physical `.sembleignore` safety prerequisite used by that readiness path. | FR-004-1, lines 345-347 | Task 1 | Covered |
| E2E | e2e test coverage for new behavior | Cross-cutting | N/A: assigned CP3 scope is package-level helper behavior; parent Task 7 owns cross-component setup/init/SessionStart integration coverage after CP3 and init wiring merge. | N/A |
| DOC | Documentation updates for changed behavior | Cross-cutting | N/A: assigned CP3 scope is internal helper API; parent Task 8 owns user-facing docs after pairing init wiring defines the CLI behavior surface. | N/A |

## Pre-Submit Self-Check Notes

- Atomicity: one coding task, one intent, one package, tests colocated with behavior.
- Falsifiability: `done_when` requires observable creation, verification, diagnostics, and MAS non-interference, validated by `go test ./internal/semble`.
- Shared-file audit: only Task 1 writes `internal/semble/semble.go` and `internal/semble/semble_test.go`; no sibling output in this plan shares those files.
- Cross-reference consistency: Task 1 explicitly owns the Pairing Semble Safety Contract, consumes only the Global Tool Guidance Contract from existing task `code-planning-main-1-code-planning-0`, and excludes init wiring, SessionStart hook output, MAS prompt changes, docs, and durable config.
