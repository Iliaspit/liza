# Code Plan: Task Worktree Semble Ignore Preparation and Lifecycle Wiring

Status: draft

Task: `architecture-main-1-architecture-2-code-planning-1`

Sources read:
- `specs/goals/20260601-integrate-semble.md`
- `specs/arch-plan/20260601-integrate-semble/20260601-135025-architecture-main-1-architecture-2.md`
- `architecture-main-1-architecture-2-code-planning-0` output summary and task detail
- `architecture-main-1-architecture-0-code-planning-0` output summary
- Targeted reads of `internal/ops/wt_create.go`, `internal/ops/claim_task.go`, `internal/ops/scip_indexing.go`, `internal/agent/worktree_check.go`, and `internal/semble/semble.go`
- `INVARIANTS.md` sections 5, 7, 8, 9, and the protection matrix

## Planning Decision

Split the assigned implementation into three coding tasks. The Semble ignore preparation helper is the shared behavior. Task lifecycle wiring and reviewer recovery wiring can then proceed in parallel after the helper lands because they touch different lifecycle packages.

This plan depends on the prior worktree private-exclude implementation and SCIP migration from `architecture-main-1-architecture-2-code-planning-0-coding-1`, and on the Semble default ignore pattern/source-of-truth contract from `architecture-main-1-architecture-0-code-planning-0-coding-2`.

## Task 1: Semble Task-Worktree Ignore Preparation Helper

**desc:** Semble task-worktree ignore preparation helper: add `internal/ops` preparation code that writes or appends the generated task-root `.sembleignore` from `internal/semble`, uses `internal/worktreeexclude` to hide only generated untracked `.sembleignore`, detects tracked operator-owned `.sembleignore` files, reports missing required patterns without modifying tracked content, and proves direct preparation is idempotent under repeated and concurrent calls. Out of scope: `CreateWorktree`, `ClaimTask`, reviewer recovery wiring, prompt rendering, init prewarm, operator documentation, project-root `.sembleignore` creation, SCIP internals, and `.liza/agent-outputs/`.

**scope:** In scope: create `internal/ops/semble_indexing.go` and focused `internal/ops/semble_indexing_test.go` coverage for Semble task-worktree `.sembleignore` preparation; consume `internal/semble.DefaultIgnorePatterns()` and `internal/semble.GeneratedWorktreeIgnorePayload()`; call `internal/worktreeexclude` for the generated `.sembleignore` private exclude entry; use Git tracked-file inspection to distinguish generated untracked files from tracked operator-owned `.sembleignore`; prove generated, partially generated, tracked-complete, tracked-incomplete, repeated, concurrent, and shared-private-exclude behavior. Out of scope: editing `internal/semble`, editing `internal/worktreeexclude` except for narrow caller-facing fixes required by this helper, editing `internal/scipsearch`, wiring `CreateWorktree`, wiring `ClaimTask`, wiring reviewer recovery, prompt rendering, init prewarm, operator documentation, project-root `.sembleignore` creation, and `.liza/agent-outputs/`.

**done_when:** `go test ./internal/ops` proves direct Semble task-worktree ignore preparation creates or updates a physical task-root `.sembleignore` whose non-empty lines exactly match every pattern returned by `internal/semble.DefaultIgnorePatterns()`; generated untracked `.sembleignore` is hidden through `internal/worktreeexclude` while remaining visible in the worktree for Semble traversal; generated `.sembleignore` and `.liza/scip/` entries share the same private worktree exclude without competing `core.excludesFile` values; repeated and concurrent preparation calls do not duplicate file content or private-exclude entries and keep `git status --porcelain` clean; tracked operator-owned `.sembleignore` files that already contain every default pattern are left tracked and unhidden; and tracked operator-owned `.sembleignore` files missing required patterns remain byte-for-byte unchanged while returning bounded missing-pattern diagnostics that do not include file contents.

**spec_ref:** `specs/arch-plan/20260601-integrate-semble/20260601-135025-architecture-main-1-architecture-2.md#semble-worktree-ignore-preparation-internalopssemble_indexinggo`

**plan_ref:** `specs/plans/20260601-integrate-semble/20260601-163042-architecture-main-1-architecture-2-code-planning-1.md`

**validation:** `go test ./internal/ops`

**task_depends_on:** `architecture-main-1-architecture-2-code-planning-0-coding-1`, `architecture-main-1-architecture-0-code-planning-0-coding-2`

## Task 2: Ops Worktree Lifecycle Wiring

**desc:** Ops worktree lifecycle Semble ignore wiring: call the Task 1 Semble preparation helper from `CreateWorktree` and `ClaimTask` after worktree provisioning and any configured post-worktree command, before optional SCIP/Stacklit refresh and before claim completion can make Semble prompt guidance eligible for the task root. Out of scope: reviewer recovery wiring, Semble offline validation internals, init prewarm, prompt template wording, operator documentation, project-root `.sembleignore` creation, SCIP internals, and modifying tracked operator-owned `.sembleignore` content.

**scope:** In scope: update `internal/ops/wt_create.go`, `internal/ops/claim_task.go`, and focused tests in `internal/ops/wt_create_test.go` and `internal/ops/claim_task_test.go` so existing and fresh `CreateWorktree` calls, fresh claims, rejected-task reclaims, and repeated or concurrent lifecycle calls run Semble task-worktree ignore preparation after worktree/post-worktree setup and before SCIP/Stacklit refresh or claim completion; append bounded Semble preparation warnings to existing warning flows; preserve three-phase claim revalidation and cleanup behavior. Narrow updates to `internal/ops/semble_indexing.go` are in scope only if needed to expose warning text or test seams from Task 1. Out of scope: reviewer recovery wiring, editing `internal/agent`, editing `internal/semble`, editing `internal/worktreeexclude`, editing `internal/scipsearch` internals, prompt rendering, init prewarm, operator documentation, project-root `.sembleignore` creation, and `.liza/agent-outputs/`.

**done_when:** `go test ./internal/ops` proves existing and fresh `CreateWorktree` calls, fresh claims, and rejected-task reclaims prepare a Semble-visible task-root `.sembleignore` containing every default runtime/generated/credential ignore pattern after worktree provisioning and any configured post-worktree command; Semble preparation runs before optional SCIP/Stacklit refresh and before successful claim completion can make prompt guidance eligible; generated `.sembleignore` remains hidden from task diffs and shares the private worktree exclude with `.liza/scip/`; repeated and concurrent lifecycle calls remain idempotent and keep `git status --porcelain` clean; Semble preparation warnings are bounded and appended to existing result warning flows without blocking worktree creation or claim; and tracked operator-owned `.sembleignore` files are left unmodified with explicit missing-pattern handling.

**spec_ref:** `specs/arch-plan/20260601-integrate-semble/20260601-135025-architecture-main-1-architecture-2.md#task-worktree-lifecycle-internalops`

**plan_ref:** `specs/plans/20260601-integrate-semble/20260601-163042-architecture-main-1-architecture-2-code-planning-1.md`

**validation:** `go test ./internal/ops`

**depends_on:** Task 1

## Task 3: Reviewer Worktree Recovery Wiring

**desc:** Reviewer worktree recovery Semble ignore wiring: call the Task 1 Semble preparation helper from reviewer worktree recovery after missing reviewer worktree reattachment and any configured post-worktree command, before SCIP/Stacklit refresh diagnostics and before reviewer prompt assembly can target the recovered candidate root. Out of scope: normal already-existing reviewer worktree preparation, `CreateWorktree`, `ClaimTask`, Semble offline validation internals, init prewarm, prompt template wording, operator documentation, project-root `.sembleignore` creation, SCIP internals, and modifying tracked operator-owned `.sembleignore` content.

**scope:** In scope: update `internal/agent/worktree_check.go` and focused `internal/agent/worktree_check_test.go` coverage so missing reviewer worktree recovery prepares a Semble-visible task-root `.sembleignore` after reattachment and any configured post-worktree command, before SCIP/Stacklit refresh diagnostics and recovery history recording; log bounded Semble preparation warnings without blocking review recovery; preserve the existing fast path for already-existing reviewer worktrees. Out of scope: editing `internal/ops` except consuming the Task 1 helper API, editing `internal/semble`, editing `internal/worktreeexclude`, editing `internal/scipsearch` internals, wiring `CreateWorktree`, wiring `ClaimTask`, prompt rendering, init prewarm, operator documentation, project-root `.sembleignore` creation, and `.liza/agent-outputs/`.

**done_when:** `go test ./internal/agent` proves missing reviewer worktree recovery prepares a Semble-visible recovered task-root `.sembleignore` containing every default runtime/generated/credential ignore pattern after reattachment and any configured post-worktree command; Semble preparation runs before SCIP/Stacklit refresh diagnostics and before recovery history makes the reviewer root prompt-eligible; generated `.sembleignore` is hidden from reviewer task diffs through the same private worktree exclude used for `.liza/scip/`; Semble preparation warnings are bounded and logged without blocking recovery; tracked operator-owned `.sembleignore` files are left unmodified with explicit missing-pattern handling; and the existing already-present reviewer worktree fast path remains unchanged.

**spec_ref:** `specs/arch-plan/20260601-integrate-semble/20260601-135025-architecture-main-1-architecture-2.md#reviewer-worktree-recovery-internalagentworktree_checkgo`

**plan_ref:** `specs/plans/20260601-integrate-semble/20260601-163042-architecture-main-1-architecture-2-code-planning-1.md`

**validation:** `go test ./internal/agent`

**depends_on:** Task 1

## Dependency Graph

```text
architecture-main-1-architecture-2-code-planning-0-coding-1
architecture-main-1-architecture-0-code-planning-0-coding-2
        |
        v
Task 1: Semble task-worktree ignore preparation helper
        |------------------------------------|
        v                                    v
Task 2: Ops lifecycle wiring          Task 3: Reviewer recovery wiring
```

Task 2 and Task 3 can run in parallel after Task 1 because they do not modify the same lifecycle files.

## Shared-File Audit

| File/Area | Task(s) | Dependency |
|---|---|---|
| `internal/ops/semble_indexing.go` | Task 1; Task 2 only for narrow caller-facing fixes if needed | Task 2 depends on Task 1 |
| `internal/ops/semble_indexing_test.go` | Task 1 | None inside this plan |
| `internal/ops/wt_create.go` | Task 2 | None inside this plan |
| `internal/ops/claim_task.go` | Task 2 | None inside this plan |
| `internal/ops/wt_create_test.go` | Task 2 | None inside this plan |
| `internal/ops/claim_task_test.go` | Task 2 | None inside this plan |
| `internal/agent/worktree_check.go` | Task 3 | None inside this plan |
| `internal/agent/worktree_check_test.go` | Task 3 | None inside this plan |
| `internal/semble/*` | Prior dependency | This plan consumes the package; no edits planned |
| `internal/worktreeexclude/*` | Prior dependency | This plan consumes the package; no edits planned except narrow Task 1 caller-facing fixes if unavoidable |
| `internal/scipsearch/*` | Prior dependency | This plan verifies shared exclude interaction; no edits planned |

## Spec Compliance Matrix

| # | Requirement | Source | Task(s) | Status |
|---|---|---|---|---|
| 1 | Liza-managed task worktrees receive Semble-visible ignore rules before agents are prompted to use Semble. | Goal spec: Worktree Safety Requirements; Architecture Plan: Goal | Task 1, Task 2, Task 3 | Covered |
| 2 | Task worktree Semble searches must not include sibling task worktrees. | Goal spec: Worktree Safety Requirements | Task 1 | Covered |
| 3 | Task worktree Semble searches must not include `.liza/` runtime state, prompts, outputs, alerts, SCIP indexes, Stacklit indexes, or credential-file patterns. | Goal spec: Worktree Safety Requirements; Security and Safety Requirements | Task 1 | Covered |
| 4 | Liza creates a physical `.sembleignore` in each Liza-managed task worktree before Semble prompt guidance is injected. | Goal spec: Worktree Safety Requirements | Task 1, Task 2, Task 3 | Covered |
| 5 | Generated task-worktree `.sembleignore` is hidden from Git status using the SCIP-style private worktree exclude while remaining visible to Semble. | Goal spec: Worktree Safety Requirements; Architecture Plan: Generated `.sembleignore` -> Git Status | Task 1, Task 2, Task 3 | Covered |
| 6 | Semble and SCIP exclusion handling share the same task-worktree private exclude file instead of competing over `core.excludesFile`. | Goal spec: Worktree Safety Requirements; Architecture Plan: SCIP/Semble Callers -> Shared Worktree Private Excludes | Task 1, Task 2, Task 3 | Covered |
| 7 | Semble and SCIP task-worktree exclude setup must be serialized so concurrent lifecycle hooks cannot corrupt `info/exclude` or race while setting `core.excludesFile`. | Goal spec: Worktree Safety Requirements; INVARIANTS.md: Concurrency & Atomicity | Task 1, Task 2 | Covered |
| 8 | Generated Semble-related ignore/config files leave task `git status --porcelain` clean unless explicitly approved project config. | Goal spec: Worktree Safety Requirements; Success Criteria 16 | Task 1, Task 2, Task 3 | Covered |
| 9 | Concurrent task worktrees can use Semble without cache-path or exclude-file collision. | Goal spec: Worktree Safety Requirements; Risks: Worktree contamination | Task 1, Task 2 | Covered |
| 10 | Default generated task-worktree `.sembleignore` content includes every listed runtime, generated-index, and credential pattern. | Goal spec: Default generated task-worktree `.sembleignore` content; Security and Safety Requirements | Task 1, Task 2, Task 3 | Covered |
| 11 | Existing and fresh `CreateWorktree` calls prepare Semble ignore safety after worktree/post-worktree setup. | Assigned done_when; Architecture Plan: Task Worktree Lifecycle | Task 2 | Covered |
| 12 | Fresh claims and rejected-task reclaims prepare Semble ignore safety after worktree/post-worktree setup and before claim completion can make the task prompt-eligible. | Assigned done_when; Architecture Plan: Task Worktree Lifecycle; INVARIANTS.md: three-phase claim | Task 2 | Covered |
| 13 | Reviewer worktree recovery prepares Semble ignore safety after reattachment/post-worktree setup and before reviewer prompt guidance can target the recovered root. | Assigned done_when; Architecture Plan: Reviewer Worktree Recovery | Task 3 | Covered |
| 14 | Warnings are bounded and do not dump file contents or secrets. | Assigned done_when; Goal spec: Security and Safety Requirements; Architecture Plan: Cross-Cutting Concerns | Task 1, Task 2, Task 3 | Covered |
| 15 | Tracked operator-owned `.sembleignore` files are left unmodified; missing required patterns are handled explicitly instead of silently overwritten. | Assigned done_when; Architecture Plan: Constraints and Semble Worktree Ignore Preparation | Task 1, Task 2, Task 3 | Covered |
| 16 | Semble preparation remains non-fatal at lifecycle boundaries; prompt target-safety checks decide whether Semble guidance is omitted. | Architecture Plan: Task Worktree Lifecycle; Reviewer Worktree Recovery | Task 2, Task 3 | Covered |
| 17 | No project-root `.sembleignore` is created or modified in this scope. | Assigned scope; Architecture Plan: Constraints | Task 1, Task 2, Task 3 | Covered |
| 18 | Semble offline validation internals, init prewarm, prompt template wording, operator documentation, project-root `.sembleignore`, and Semble MCP stay out of this task. | Assigned scope; Goal spec: Non-Goals | Task 1, Task 2, Task 3 | Covered |
| E2E | e2e test coverage for new behavior | Cross-cutting | N/A: this is internal lifecycle safety for task/reviewer worktrees; end-to-end prompt guidance is owned by sibling MAS Semble Prompt Guidance, and this plan requires package tests exercising lifecycle entry points directly. | N/A |
| DOC | Documentation updates for changed behavior | Cross-cutting | N/A: operator documentation is explicitly out of scope here and owned by the Semble Operator Documentation sibling; this plan creates internal implementation tasks only. | N/A |

## Validation Plan

- Re-read this plan and the structured output JSON and verify each output entry's `desc`, `done_when`, `scope`, `spec_ref`, `plan_ref`, `validation`, `depends_on`, and `task_depends_on` match this plan.
- Run `python3 -m json.tool specs/plans/20260601-integrate-semble/20260601-163042-architecture-main-1-architecture-2-code-planning-1-output.json`.
- Verify `git diff -- specs/plans/20260601-integrate-semble/20260601-163042-architecture-main-1-architecture-2-code-planning-1.md specs/plans/20260601-integrate-semble/20260601-163042-architecture-main-1-architecture-2-code-planning-1-output.json` contains only planning artifacts.
- Stage the plan and output JSON, commit them, verify `git status --short` is clean, then submit `HEAD` with `liza submit-for-review`.
