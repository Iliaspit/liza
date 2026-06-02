# Architecture Plan: Worktree Semble Ignore Safety

Status: review

## Goal

Prepare Liza-managed task and reviewer worktrees for safe Semble search by sharing linked-worktree private exclude handling with SCIP, generating Semble-visible task `.sembleignore` files, and wiring that preparation into worktree lifecycle points before MAS prompts can target those roots.

## Context

The Semble goal requires task worktrees to contain a physical `.sembleignore` because Semble reads ignore rules from the walked tree, while generated ignore files must stay out of task diffs. SCIP already creates generated `.liza/scip/` indexes and hides them with a linked-worktree private exclude. That SCIP-specific implementation currently owns `core.excludesFile` setup itself, so adding Semble directly beside it would risk competing `core.excludesFile` values and unsynchronized writes to the same private exclude file.

This plan narrows the parent Semble architecture to the worktree-safety slice. It separates a generic linked-worktree private exclude helper from Semble-specific ignore preparation, then wires Semble preparation into the task worktree and reviewer recovery paths that exist before MAS prompt rendering.

### References

- Goal spec: `specs/goals/20260601-integrate-semble.md`
- Parent plan: `specs/arch-plan/20260601-integrate-semble/20260601-130216-architecture-main-1.md`
- Dependency plan: `specs/arch-plan/20260601-integrate-semble/20260601-133505-architecture-main-1-architecture-0.md`
- Task: `architecture-main-1-architecture-2`
- Codebase:
  - `internal/ops/wt_create.go`
  - `internal/ops/claim_task.go`
  - `internal/ops/claim_task_strategy.go`
  - `internal/ops/scip_indexing.go`
  - `internal/ops/submit_review.go`
  - `internal/scipsearch/scipsearch.go`
  - `internal/scipsearch/scipsearch_test.go`
  - `internal/agent/worktree_check.go`
  - `internal/agent/worktree_check_test.go`
  - `INVARIANTS.md#5-concurrency--atomicity`
  - `INVARIANTS.md#7-worktree--integration`
  - `INVARIANTS.md#9-security`
  - `INVARIANTS.md#cross-reference-protection-matrix`
  - `specs/architecture/ADR/README.md`
  - `lessons/agents/worktree-file-path-consistency.md`
  - `lessons/agents/worktree-path-construction.md`
  - `lessons/agents/large-test-file-reads.md`
  - `lessons/agents/worktree-build-prerequisites.md`

### Constraints

- Semble ignore preparation depends on the `internal/semble` default ignore pattern contract from `architecture-main-1-architecture-0-code-planning-0`.
- No project-root `.sembleignore` is created or modified in this scope.
- Tracked operator-owned `.sembleignore` files must not be silently overwritten.
- Generated task-worktree `.sembleignore` files must remain visible to Semble while hidden from `git status --porcelain`.
- Semble and SCIP must share one linked-worktree private exclude file and one `core.excludesFile` target.
- Exclude setup must be serialized so concurrent SCIP/Semble lifecycle calls cannot corrupt `info/exclude` or race on `core.excludesFile`.
- Worktree lifecycle warnings remain bounded and must not log file contents or secrets.
- G1.1 applies: runtime behavior must not assume the target project is Liza, Go, or Make-based.
- The protection matrix intersects race conditions, lost work, scope creep, secret exposure, and silent failures; this plan routes writes through idempotent helpers, preserves worktree-specific config, reports bounded warnings, and keeps generated artifacts out of review diffs.
- `.pre-commit-config.yaml` exists at worktree `HEAD`; no `bootstrap-precommit` output entry is emitted.

### Assumptions

- None.

### Open Questions

- None for this scope.

---

## Components

### Shared Worktree Private Excludes (`internal/worktreeexclude/`)

**Responsibility:** Own linked-worktree private exclude setup for generated task-worktree artifacts across optional tools.

**Boundaries:**
- Exposes: an idempotent helper that accepts a worktree root and one or more ignore entries, resolves the linked-worktree gitdir, appends missing entries to `info/exclude`, enables `extensions.worktreeConfig`, and sets worktree-specific `core.excludesFile` to that private exclude.
- Depends on: Git commands executed in the target worktree, filesystem reads/writes under the worktree gitdir, and a package-level lock for serialization.

**Key decisions:**
- Create `internal/worktreeexclude` instead of expanding `internal/scipsearch` because private exclude ownership is not SCIP-specific once Semble also needs it.
- Keep the helper entry-oriented: callers supply entries such as `.liza/scip/` or `.sembleignore`; the helper does not know SCIP or Semble semantics.
- Serialize the entire resolve/read/append/configure sequence with one package-level lock. The lock protects concurrent lifecycle calls within the Liza process and matches the current SCIP race surface.
- Reject an existing non-empty `core.excludesFile` that points somewhere else rather than overwriting it. This preserves operator-owned worktree config and makes competing exclude ownership explicit.
- Preserve idempotency by normalizing trimmed exclude lines and appending only missing entries, while preserving existing file content.

### SCIP Task Exclude Integration (`internal/scipsearch/`)

**Responsibility:** Keep generated task-worktree SCIP indexes prompt-local and out of diffs through the shared private exclude helper.

**Boundaries:**
- Exposes: unchanged SCIP refresh behavior and prompt-safe index refs.
- Depends on: `internal/worktreeexclude` for task-worktree private exclude setup.

**Key decisions:**
- Replace SCIP's local `ensureTaskWorktreeScipExclude`, `configureTaskWorktreeExclude`, and SCIP-specific mutex with the shared helper.
- Preserve the `.liza/scip/` exclude entry and existing task-worktree index behavior.
- Keep SCIP tests as regression coverage for idempotent private excludes, clean git status, and concurrent task-worktree refreshes.

### Semble Worktree Ignore Preparation (`internal/ops/semble_indexing.go`)

**Responsibility:** Prepare task/reviewer worktrees with Semble-visible generated ignore rules and hide generated `.sembleignore` files from Git status.

**Boundaries:**
- Exposes: an idempotent task-worktree preparation function used by ops and reviewer recovery paths.
- Depends on: `internal/semble` default ignore pattern source of truth, `internal/worktreeexclude`, Git tracked-file inspection for `.sembleignore`, and filesystem writes at `<worktree>/.sembleignore`.

**Key decisions:**
- Put lifecycle wiring in `internal/ops` because worktree preparation belongs with task worktree lifecycle, not prompt rendering.
- The preparation function handles absent or generated untracked `.sembleignore` by writing or appending missing default patterns, then adding `.sembleignore` to the shared private exclude.
- If `.sembleignore` is tracked and already contains every default pattern, leave it tracked and do not add it to the private exclude.
- If `.sembleignore` is tracked and missing required patterns, return a bounded warning/error that names the missing-pattern condition without dumping file contents. Do not modify the tracked file in this scope.
- Treat preparation as idempotent and safe to rerun on old worktrees, new worktrees, and reviewer-recovered worktrees.

### Task Worktree Lifecycle (`internal/ops/`)

**Responsibility:** Run Semble worktree preparation after a task worktree exists and before any spawned-agent prompt can safely advertise Semble for that root.

**Boundaries:**
- Exposes: warnings through existing `CreateWorktreeResult` and `ClaimResult` warning flows.
- Depends on: Semble preparation, existing post-worktree command sequencing, SCIP refresh, and Stacklit refresh.

**Key decisions:**
- Wire preparation into `CreateWorktree` for both existing and newly created worktrees because `liza wt-create` can repair or upgrade old worktrees.
- Wire preparation into `ClaimTask` after worktree provisioning and after any configured post-worktree command, because claims are the normal MAS path before agents are spawned.
- Run Semble preparation before SCIP/Stacklit prompt-index refresh warnings are returned so all generated artifact excludes share the same helper before generated files affect `git status`.
- Keep Semble preparation non-fatal at the lifecycle boundary: warnings are surfaced, and downstream prompt target-safety checks decide whether Semble guidance is omitted.
- Preserve existing state-machine and three-phase claim invariants; Semble preparation runs in the same outside-lock filesystem phase as post-worktree commands and optional index refresh.

### Reviewer Worktree Recovery (`internal/agent/worktree_check.go`)

**Responsibility:** Prepare a recovered reviewer worktree for Semble before reviewer prompt guidance can target the recovered candidate root.

**Boundaries:**
- Exposes: bounded reviewer log warnings only.
- Depends on: Semble preparation from `internal/ops`, existing post-worktree command recovery, and SCIP/Stacklit refresh behavior.

**Key decisions:**
- Run Semble worktree preparation after a missing reviewer worktree is reattached and after the post-worktree command, before SCIP/Stacklit refresh diagnostics.
- For an already-existing reviewer worktree, keep the existing "nothing to do" fast path; this scope covers recovery. Regular prompt target-safety checks remain responsible for omitting Semble if an existing worktree lacks safety.
- Reviewer recovery warnings must not block review work, matching existing SCIP/Stacklit recovery behavior.

---

## Interfaces

### SCIP/Semble Callers -> Shared Worktree Private Excludes

**Contract:** Callers pass a target worktree root and relative ignore entries. The helper ensures the linked worktree uses its private `info/exclude` as `core.excludesFile`, appends missing entries idempotently, and returns an error on conflicting `core.excludesFile` ownership.

**Direction:** `internal/scipsearch` and `internal/ops` call `internal/worktreeexclude`; the helper has no dependency on either optional-tool package.

**Invariants:** One private exclude file and one `core.excludesFile` value per linked worktree; repeated and concurrent calls preserve entries and do not duplicate them.

### Semble Preparation -> Semble Capability

**Contract:** Semble preparation consumes the default `.sembleignore` pattern list from `internal/semble` and treats it as the source of truth for runtime/generated/credential exclusions.

**Direction:** `internal/ops` calls `internal/semble`; `internal/semble` does not know task lifecycle.

**Invariants:** Worktree `.sembleignore` content matches the default block from the spec through the Semble package contract, avoiding duplicated pattern literals across lifecycle code.

### Task Lifecycle -> Semble Preparation

**Contract:** Worktree creation/claim/recovery calls Semble preparation with the exact task or reviewer worktree root after the worktree exists. The preparation returns bounded warnings/errors only.

**Direction:** `internal/ops/wt_create.go`, `internal/ops/claim_task.go`, and `internal/agent/worktree_check.go` invoke preparation.

**Invariants:** Lifecycle code never prepares the parent project root, never hides tracked operator-owned `.sembleignore`, and never logs ignore file contents.

### Generated `.sembleignore` -> Git Status

**Contract:** Generated untracked `.sembleignore` is a physical file in the worktree root and an entry in the shared private exclude file.

**Direction:** Semble sees the file during traversal; Git status excludes it through worktree-specific config.

**Invariants:** `git status --porcelain` remains clean after lifecycle preparation and optional SCIP index generation; `core.excludesFile` is not contested between Semble and SCIP.

---

## Data Flow

Fresh task claim:

```text
ClaimTask -> create/validate task worktree -> optional PostWorktreeCmd -> prepare Semble `.sembleignore` -> refresh SCIP/Stacklit -> record claim -> spawned prompt can target task worktree only if prompt safety passes
```

Existing `liza wt-create` task worktree:

```text
CreateWorktree(existing) -> validate health -> provision config/hooks -> optional PostWorktreeCmd -> prepare Semble `.sembleignore` -> refresh SCIP/Stacklit -> return warnings
```

New `liza wt-create` task worktree:

```text
CreateWorktree(new/fresh) -> create worktree -> record base commit -> provision config/hooks -> optional PostWorktreeCmd -> prepare Semble `.sembleignore` -> refresh SCIP/Stacklit -> return warnings
```

Reviewer recovery:

```text
ensureReviewerWorktree -> reattach missing reviewer worktree -> optional PostWorktreeCmd -> prepare Semble `.sembleignore` -> refresh SCIP/Stacklit -> record recovery -> reviewer prompt assembly can evaluate target safety
```

Private exclude update:

```text
SCIP or Semble entry -> worktreeexclude lock -> git rev-parse --git-dir -> read info/exclude -> append missing entries -> enable worktreeConfig -> set/verify core.excludesFile -> release lock
```

---

## Cross-Cutting Concerns

| Concern | Approach |
|---------|----------|
| Error handling | Shared helper returns contextual errors; lifecycle callers convert them to bounded warnings where optional-tool failure must not block agent spawn. |
| Observability | Existing warning surfaces are reused: `CreateWorktreeResult.Warnings`, `ClaimResult.Warnings`, and reviewer recovery logger warnings. Diagnostics name the operation and condition without file contents. |
| Configuration | No new durable config. The helper uses worktree-specific Git config only. Semble enablement/offline readiness remains owned by the `internal/semble` dependency. |
| Concurrency | A package-level helper lock serializes private exclude file and `core.excludesFile` updates across SCIP and Semble callers in the process. |
| Security | Default ignore patterns come from `internal/semble` and include runtime/generated/credential patterns. Tracked operator-owned `.sembleignore` is not overwritten. |
| Testing | Unit tests cover helper idempotency/conflict/concurrency, SCIP migration, Semble generated-file cleanliness, lifecycle wiring, and reviewer recovery warning behavior. |

---

## Decomposition

Each scope becomes a code-planning child task. No bootstrap-precommit entry is emitted because `.pre-commit-config.yaml` exists at worktree `HEAD`.

### Scope 1: Shared Worktree Private Excludes and SCIP Migration

**Component(s):** Shared Worktree Private Excludes; SCIP Task Exclude Integration

**Boundary:** In scope: create `internal/worktreeexclude/` as the shared linked-worktree private exclude helper, move SCIP task-worktree private exclude setup onto that helper, preserve SCIP `.liza/scip/` generated-index cleanliness, and cover idempotent, conflicting, and concurrent exclude setup in helper/SCIP tests. Out of scope: Semble `.sembleignore` generation, task/reviewer lifecycle wiring, prompt rendering, init prewarm, operator documentation, and project-root `.sembleignore` handling.

**Output desc:** Shared Worktree Private Excludes and SCIP Migration: create `internal/worktreeexclude/` as the shared linked-worktree private exclude helper, move SCIP task-worktree private exclude setup onto that helper, preserve SCIP `.liza/scip/` generated-index cleanliness, and cover idempotent, conflicting, and concurrent exclude setup in helper/SCIP tests. Out of scope: Semble `.sembleignore` generation, task/reviewer lifecycle wiring, prompt rendering, init prewarm, operator documentation, and project-root `.sembleignore` handling.

**Output scope:** In scope: create `internal/worktreeexclude/` as the shared linked-worktree private exclude helper, move SCIP task-worktree private exclude setup onto that helper, preserve SCIP `.liza/scip/` generated-index cleanliness, and cover idempotent, conflicting, and concurrent exclude setup in helper/SCIP tests. Out of scope: Semble `.sembleignore` generation, task/reviewer lifecycle wiring, prompt rendering, init prewarm, operator documentation, and project-root `.sembleignore` handling.

**Output done_when:** `go test ./internal/worktreeexclude ./internal/scipsearch` proves the shared helper covers idempotent, conflicting, and concurrent linked-worktree private exclude setup; SCIP task-worktree refresh uses `internal/worktreeexclude` for `.liza/scip/` private excludes; generated SCIP indexes remain prompt-local and hidden from `git status --porcelain`; repeated refreshes do not duplicate exclude entries; concurrent refreshes keep the private exclude file valid; and an existing conflicting task-worktree `core.excludesFile` is reported instead of overwritten.

**Depends on:** None

**Validation:** `go test ./internal/worktreeexclude ./internal/scipsearch`

### Scope 2: Task Worktree Semble Ignore Preparation and Lifecycle Wiring

**Component(s):** Semble Worktree Ignore Preparation; Task Worktree Lifecycle; Reviewer Worktree Recovery

**Boundary:** In scope: add Semble task-worktree `.sembleignore` preparation using `internal/semble` default ignore patterns and `internal/worktreeexclude`; wire it into `CreateWorktree`, `ClaimTask`, and reviewer worktree recovery after worktree/post-worktree setup and before Semble prompt guidance can target those roots; prove generated `.sembleignore` files contain every default runtime/generated/credential pattern, are hidden from task diffs, share the private exclude with `.liza/scip/`, remain idempotent under repeated and concurrent lifecycle calls, and treat tracked operator-owned `.sembleignore` files explicitly rather than silently overwriting them. Out of scope: Semble offline validation internals, init prewarm, prompt template wording, operator documentation, project-root `.sembleignore` creation, and modifying tracked operator-owned `.sembleignore` content without explicit task scope.

**Output desc:** Task Worktree Semble Ignore Preparation and Lifecycle Wiring: add Semble task-worktree `.sembleignore` preparation using `internal/semble` default ignore patterns and `internal/worktreeexclude`; wire it into `CreateWorktree`, `ClaimTask`, and reviewer worktree recovery after worktree/post-worktree setup and before Semble prompt guidance can target those roots; prove generated `.sembleignore` files contain every default runtime/generated/credential pattern, are hidden from task diffs, share the private exclude with `.liza/scip/`, remain idempotent under repeated and concurrent lifecycle calls, and treat tracked operator-owned `.sembleignore` files explicitly rather than silently overwriting them. Out of scope: Semble offline validation internals, init prewarm, prompt template wording, operator documentation, project-root `.sembleignore` creation, and modifying tracked operator-owned `.sembleignore` content without explicit task scope.

**Output scope:** In scope: add Semble task-worktree `.sembleignore` preparation using `internal/semble` default ignore patterns and `internal/worktreeexclude`; wire it into `CreateWorktree`, `ClaimTask`, and reviewer worktree recovery after worktree/post-worktree setup and before Semble prompt guidance can target those roots; prove generated `.sembleignore` files contain every default runtime/generated/credential pattern, are hidden from task diffs, share the private exclude with `.liza/scip/`, remain idempotent under repeated and concurrent lifecycle calls, and treat tracked operator-owned `.sembleignore` files explicitly rather than silently overwriting them. Out of scope: Semble offline validation internals, init prewarm, prompt template wording, operator documentation, project-root `.sembleignore` creation, and modifying tracked operator-owned `.sembleignore` content without explicit task scope.

**Output done_when:** `go test ./internal/ops ./internal/agent` proves fresh claims, rejected-task reclaims, existing and fresh `CreateWorktree` calls, and reviewer worktree recovery prepare a Semble-visible task-root `.sembleignore` with every default runtime/generated/credential ignore pattern; generated `.sembleignore` and `.liza/scip/` entries share the same private worktree exclude without competing `core.excludesFile` values; repeated and concurrent lifecycle calls are idempotent and keep `git status --porcelain` clean; preparation runs after post-worktree setup and before prompt-eligible recovery/claim completion; warnings are bounded; and tracked operator-owned `.sembleignore` files are left unmodified with explicit missing-pattern handling instead of silent overwrite.

**Depends on:** Scope 1; `architecture-main-1-architecture-0-code-planning-0`

**Validation:** `go test ./internal/ops ./internal/agent`

### Spec Coverage

| Spec Requirement | Scope |
|------------------|-------|
| Liza-managed task worktrees receive Semble-visible ignore rules before agents are prompted to use Semble | Scope 2 |
| Task worktree Semble searches exclude `.liza/`, `.worktrees/`, `stacklit.json`, `*.scip`, and credential-file patterns | Scope 2, consuming `internal/semble` |
| Create a physical `.sembleignore` in each Liza-managed task worktree before Semble prompt guidance | Scope 2 |
| Hide generated task-worktree `.sembleignore` from `git status` using SCIP-style private exclude | Scope 1, Scope 2 |
| Semble and SCIP exclusion handling share the same private exclude file | Scope 1, Scope 2 |
| Serialize Semble/SCIP exclude updates | Scope 1 |
| Generated Semble-related files leave task diffs clean | Scope 2 |
| Concurrent task worktrees can use Semble without cache-path or exclude-file collisions | Scope 1, Scope 2 |
| Tracked operator-owned `.sembleignore` files are not silently overwritten | Scope 2 |
| Reviewer worktree recovery prepares Semble ignore safety | Scope 2 |
| Semble offline validation, init prewarm, prompt text, docs, project-root `.sembleignore`, and Semble MCP remain out of this task | Out of scope here; covered by sibling/dependency tasks where applicable |

## Shared-File Audit

| File/Area | Owner | Readers / Ordering |
|-----------|-------|--------------------|
| `internal/worktreeexclude/*` | Scope 1 | Scope 2 consumes the helper; Scope 2 depends on Scope 1 |
| `internal/scipsearch/scipsearch.go` | Scope 1 | Scope 2 verifies shared exclude interaction through lifecycle tests but does not own SCIP internals |
| `internal/scipsearch/scipsearch_test.go` | Scope 1 | Scope 2 may rely on SCIP behavior but should not edit SCIP tests unless integration needs an added assertion |
| `internal/ops/semble_indexing.go` | Scope 2 | New Semble lifecycle preparation owner |
| `internal/ops/wt_create.go` | Scope 2 | Existing CLI worktree creation lifecycle wiring |
| `internal/ops/claim_task.go` | Scope 2 | Normal MAS claim lifecycle wiring before spawned prompts |
| `internal/ops/wt_create_test.go` | Scope 2 | Existing worktree creation test home; read targeted regions first |
| `internal/ops/claim_task_test.go` | Scope 2 | Claim lifecycle coverage if code-planner chooses to assert normal MAS claims directly |
| `internal/agent/worktree_check.go` | Scope 2 | Reviewer worktree recovery wiring |
| `internal/agent/worktree_check_test.go` | Scope 2 | Reviewer recovery coverage |
| `internal/semble/*` | Dependency task | Scope 2 consumes default ignore patterns; dependency is `architecture-main-1-architecture-0-code-planning-0` |

## Validation Plan

- Confirm the decomposition maps all Worktree Safety Requirements from `specs/goals/20260601-integrate-semble.md` to Scope 1 or Scope 2.
- Confirm `Output desc`, `Output scope`, `Output done_when`, dependency, and validation fields above are copied verbatim into the structured output JSON.
- Confirm each output entry has `arch_ref` set to `specs/arch-plan/20260601-integrate-semble/20260601-135025-architecture-main-1-architecture-2.md`.
- Confirm no `bootstrap-precommit` output entry is emitted because `.pre-commit-config.yaml` exists at worktree `HEAD`.
- Run the canonical validation command for this architecture task: `go test ./internal/ops ./internal/worktreeexclude ./internal/scipsearch ./internal/agent`.
- Run `liza set-task-output architecture-main-1-architecture-2 --output specs/arch-plan/20260601-integrate-semble/20260601-135025-architecture-main-1-architecture-2-output.json --agent-id architect-1 --json`.
- Run pre-commit on the touched architecture artifacts, commit only those artifacts, verify `git status --short` is clean, and submit `HEAD` for review.

## Self-Review Notes

- The plan is implementation planning only; no runtime code is changed in this architecture task.
- Scope 1 owns the generic helper and SCIP migration to prevent two packages from competing over `core.excludesFile`.
- Scope 2 owns Semble-specific worktree preparation and lifecycle wiring because prompt safety depends on the exact worktree root being prepared before agent launch or reviewer recovery.
- `ClaimTask` is included even though the parent file list emphasized `wt_create.go`; it is the normal MAS path that creates a task worktree before a spawned agent receives prompt guidance.
- The tracked `.sembleignore` rule is explicit: generated files can be modified and hidden, but tracked operator content is never silently overwritten.
