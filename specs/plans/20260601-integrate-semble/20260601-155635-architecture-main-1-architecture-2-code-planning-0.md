# Code Plan: Shared Worktree Private Excludes and SCIP Migration

Status: draft

Task: `architecture-main-1-architecture-2-code-planning-0`

Spec reference: `specs/arch-plan/20260601-integrate-semble/20260601-135025-architecture-main-1-architecture-2.md#scope-1-shared-worktree-private-excludes-and-scip-migration`

## Source Inputs

- Goal spec: `specs/goals/20260601-integrate-semble.md`
- Architecture plan: `specs/arch-plan/20260601-integrate-semble/20260601-135025-architecture-main-1-architecture-2.md`
- Dependency plan output: `architecture-main-1-architecture-0-code-planning-0`
- Prior rejection: validation must prove the intended helper and SCIP tests actually ran, not merely that package-level `go test` exited 0.

## Planning Decision

Split the scope into two coding tasks. Task 1 creates the generic linked-worktree private exclude helper and its focused helper tests. Task 2 migrates SCIP task-worktree exclude setup onto that helper and proves SCIP generated-index cleanliness and conflict handling still hold. Task 2 depends on Task 1 because it imports the new helper and may require narrow caller-facing helper fixes to preserve the Task 1 contract.

This task is internal worktree/index plumbing. There is no user-visible documentation or e2e deliverable in this slice; operator docs, Semble `.sembleignore` generation, lifecycle wiring, prompt rendering, init prewarm, and project-root `.sembleignore` handling are owned by sibling tasks.

## Task 1: Shared Worktree Private Exclude Helper

**desc:** Shared worktree private exclude helper: create `internal/worktreeexclude/` with an idempotent linked-worktree private exclude API that resolves the target worktree gitdir, serializes exclude/config updates, appends missing relative entries without duplicates, enables `extensions.worktreeConfig`, sets or verifies the worktree-specific `core.excludesFile` for the private `info/exclude` file, and reports an existing conflicting `core.excludesFile` instead of overwriting it. Out of scope: SCIP refresh behavior, Semble `.sembleignore` generation, task/reviewer lifecycle wiring, prompt rendering, init prewarm, operator documentation, project-root `.sembleignore` handling, and `.liza/agent-outputs/`.

**done_when:** The self-verifying validation command `python3 -c 'import json, subprocess, sys; pattern = sys.argv[1]; required = set(sys.argv[2:]); cmd = ["go", "test", "-json", "-run", pattern, "./internal/worktreeexclude"]; p = subprocess.run(cmd, text=True, capture_output=True); print(p.stdout, end=""); print(p.stderr, end="", file=sys.stderr); ran = set(); [ran.add(event.get("Test")) for line in p.stdout.splitlines() if line.startswith("{") for event in [json.loads(line)] if event.get("Action") == "run" and event.get("Test")]; missing = required - ran; print("missing tests: " + ", ".join(sorted(missing)), file=sys.stderr) if missing else None; sys.exit(p.returncode if p.returncode else (1 if missing else 0))' 'TestEnsurePrivateExclude(Idempotent|ReportsConflictingCoreExcludesFile|ConcurrentSetup|EnablesWorktreeConfig)$' TestEnsurePrivateExcludeIdempotent TestEnsurePrivateExcludeReportsConflictingCoreExcludesFile TestEnsurePrivateExcludeConcurrentSetup TestEnsurePrivateExcludeEnablesWorktreeConfig` proves the helper creates or reuses the linked-worktree `info/exclude` file as the worktree-specific `core.excludesFile`; repeated calls preserve existing exclude content and add each requested relative entry exactly once; concurrent calls keep the private exclude file valid and non-duplicated; `extensions.worktreeConfig` is enabled for the worktree; and a pre-existing non-empty `core.excludesFile` pointing somewhere else returns a contextual conflict error without overwriting that value.

**scope:** In scope: create `internal/worktreeexclude/doc.go`, `internal/worktreeexclude/worktreeexclude.go`, and `internal/worktreeexclude/worktreeexclude_test.go` for linked-worktree gitdir resolution, package-level serialization, idempotent private exclude entry management, worktree-specific Git config setup, conflicting `core.excludesFile` detection, and helper tests for idempotent, conflicting, concurrent, and worktree-config setup. Out of scope: modifying `internal/scipsearch`, Semble `.sembleignore` generation, task/reviewer lifecycle wiring, prompt rendering, init prewarm, operator documentation, project-root `.sembleignore` handling, and `.liza/agent-outputs/`.

**spec_ref:** specs/arch-plan/20260601-integrate-semble/20260601-135025-architecture-main-1-architecture-2.md#shared-worktree-private-excludes-internalworktreeexclude

**validation:**

```text
python3 -c 'import json, subprocess, sys; pattern = sys.argv[1]; required = set(sys.argv[2:]); cmd = ["go", "test", "-json", "-run", pattern, "./internal/worktreeexclude"]; p = subprocess.run(cmd, text=True, capture_output=True); print(p.stdout, end=""); print(p.stderr, end="", file=sys.stderr); ran = set(); [ran.add(event.get("Test")) for line in p.stdout.splitlines() if line.startswith("{") for event in [json.loads(line)] if event.get("Action") == "run" and event.get("Test")]; missing = required - ran; print("missing tests: " + ", ".join(sorted(missing)), file=sys.stderr) if missing else None; sys.exit(p.returncode if p.returncode else (1 if missing else 0))' 'TestEnsurePrivateExclude(Idempotent|ReportsConflictingCoreExcludesFile|ConcurrentSetup|EnablesWorktreeConfig)$' TestEnsurePrivateExcludeIdempotent TestEnsurePrivateExcludeReportsConflictingCoreExcludesFile TestEnsurePrivateExcludeConcurrentSetup TestEnsurePrivateExcludeEnablesWorktreeConfig
```

## Task 2: SCIP Task-Worktree Private Exclude Migration

**desc:** SCIP task-worktree private exclude migration: move `internal/scipsearch` task-worktree private exclude setup from SCIP-owned helper/config/mutex code onto `internal/worktreeexclude` while preserving `.liza/scip/` generated-index behavior, prompt-local index refs, clean task diffs, repeated-refresh idempotency, concurrent-refresh safety, and conflict reporting for an existing task-worktree `core.excludesFile`. Out of scope: Semble `.sembleignore` generation, task/reviewer lifecycle wiring, prompt rendering, init prewarm, operator documentation, project-root `.sembleignore` handling, and `.liza/agent-outputs/`.

**done_when:** The self-verifying validation command `python3 -c 'import json, subprocess, sys; pattern = sys.argv[1]; required = set(sys.argv[2:]); cmd = ["go", "test", "-json", "-run", pattern, "./internal/worktreeexclude", "./internal/scipsearch"]; p = subprocess.run(cmd, text=True, capture_output=True); print(p.stdout, end=""); print(p.stderr, end="", file=sys.stderr); ran = set(); [ran.add(event.get("Test")) for line in p.stdout.splitlines() if line.startswith("{") for event in [json.loads(line)] if event.get("Action") == "run" and event.get("Test")]; missing = required - ran; print("missing tests: " + ", ".join(sorted(missing)), file=sys.stderr) if missing else None; sys.exit(p.returncode if p.returncode else (1 if missing else 0))' 'TestEnsurePrivateExclude(Idempotent|ReportsConflictingCoreExcludesFile|ConcurrentSetup|EnablesWorktreeConfig)$|TestRefreshTaskWorktreeScip(UsesSharedExclude|HidesGeneratedIndexes|RepeatedRefreshIdempotent|ConcurrentExcludeSetup|ReportsConflictingCoreExcludesFile)$' TestEnsurePrivateExcludeIdempotent TestEnsurePrivateExcludeReportsConflictingCoreExcludesFile TestEnsurePrivateExcludeConcurrentSetup TestEnsurePrivateExcludeEnablesWorktreeConfig TestRefreshTaskWorktreeScipUsesSharedExclude TestRefreshTaskWorktreeScipHidesGeneratedIndexes TestRefreshTaskWorktreeScipRepeatedRefreshIdempotent TestRefreshTaskWorktreeScipConcurrentExcludeSetup TestRefreshTaskWorktreeScipReportsConflictingCoreExcludesFile` proves SCIP task-worktree refresh uses `internal/worktreeexclude` for the `.liza/scip/` private exclude entry; generated SCIP indexes remain prompt-local and hidden from `git status --porcelain`; repeated refreshes do not duplicate `.liza/scip/` exclude entries; concurrent refreshes keep the shared private exclude file valid; and an existing conflicting task-worktree `core.excludesFile` is reported instead of overwritten.

**scope:** In scope: `internal/scipsearch/scipsearch.go` and `internal/scipsearch/scipsearch_test.go` for replacing SCIP-specific private exclude setup with `internal/worktreeexclude`, preserving existing SCIP task-worktree index refresh semantics, and adding/adjusting regression tests for shared-helper usage, generated-index cleanliness, repeated refreshes, concurrent refreshes, and conflicting `core.excludesFile` behavior. Narrow caller-facing fixes in `internal/worktreeexclude/*` are in scope only when required to preserve the Task 1 helper contract during SCIP migration. Out of scope: Semble `.sembleignore` generation, task/reviewer lifecycle wiring, prompt rendering, init prewarm, operator documentation, project-root `.sembleignore` handling, and `.liza/agent-outputs/`.

**spec_ref:** specs/arch-plan/20260601-integrate-semble/20260601-135025-architecture-main-1-architecture-2.md#scip-task-exclude-integration-internalscipsearch

**depends_on:** Task 1

**validation:**

```text
python3 -c 'import json, subprocess, sys; pattern = sys.argv[1]; required = set(sys.argv[2:]); cmd = ["go", "test", "-json", "-run", pattern, "./internal/worktreeexclude", "./internal/scipsearch"]; p = subprocess.run(cmd, text=True, capture_output=True); print(p.stdout, end=""); print(p.stderr, end="", file=sys.stderr); ran = set(); [ran.add(event.get("Test")) for line in p.stdout.splitlines() if line.startswith("{") for event in [json.loads(line)] if event.get("Action") == "run" and event.get("Test")]; missing = required - ran; print("missing tests: " + ", ".join(sorted(missing)), file=sys.stderr) if missing else None; sys.exit(p.returncode if p.returncode else (1 if missing else 0))' 'TestEnsurePrivateExclude(Idempotent|ReportsConflictingCoreExcludesFile|ConcurrentSetup|EnablesWorktreeConfig)$|TestRefreshTaskWorktreeScip(UsesSharedExclude|HidesGeneratedIndexes|RepeatedRefreshIdempotent|ConcurrentExcludeSetup|ReportsConflictingCoreExcludesFile)$' TestEnsurePrivateExcludeIdempotent TestEnsurePrivateExcludeReportsConflictingCoreExcludesFile TestEnsurePrivateExcludeConcurrentSetup TestEnsurePrivateExcludeEnablesWorktreeConfig TestRefreshTaskWorktreeScipUsesSharedExclude TestRefreshTaskWorktreeScipHidesGeneratedIndexes TestRefreshTaskWorktreeScipRepeatedRefreshIdempotent TestRefreshTaskWorktreeScipConcurrentExcludeSetup TestRefreshTaskWorktreeScipReportsConflictingCoreExcludesFile
```

## Dependency Order

| Task | Depends on | Reason |
|------|------------|--------|
| Task 1 | - | Establishes the shared helper contract and tests. |
| Task 2 | Task 1 | Imports the helper, removes SCIP-owned private exclude code, and validates SCIP behavior on the shared helper. |

## Shared-File Audit

| File or area | Planned owner | Dependency handling |
|--------------|---------------|---------------------|
| `internal/worktreeexclude/*` | Task 1 | Task 2 may make only narrow caller-facing fixes required by SCIP migration, and depends on Task 1. |
| `internal/scipsearch/scipsearch.go` | Task 2 | Task 1 does not modify SCIP internals. |
| `internal/scipsearch/scipsearch_test.go` | Task 2 | Task 1 does not modify SCIP tests. |
| `.liza/agent-outputs/` | Out of scope | No task may create, edit, stage, or commit runtime log artifacts. |

## Validation Notes

- The validation commands intentionally use `go test -json` and fail if named top-level tests are missing. This prevents a package with no tests or missing intended regression coverage from satisfying the task by returning exit code 0.
- Task 1's required tests cover helper idempotency, conflicting `core.excludesFile`, concurrent setup, and worktree-specific config.
- Task 2's required tests cover the Task 1 helper contract plus SCIP shared-helper usage, generated-index cleanliness, repeated refresh idempotency, concurrent refresh safety, and SCIP conflict reporting.
- The parent canonical package sweep remains `go test ./internal/worktreeexclude ./internal/scipsearch`, but child validations are the self-verifying executable commands above.

## Spec Compliance Matrix

| # | Requirement | Source | Task(s) | Status |
|---|-------------|--------|---------|--------|
| 1 | Create a shared helper for linked-worktree private exclude setup instead of expanding SCIP-specific ownership. | Architecture plan, Components, lines 66-79 | Task 1 | Covered |
| 2 | Helper accepts a worktree root plus relative entries, resolves the linked-worktree gitdir, appends missing entries, enables `extensions.worktreeConfig`, and sets or verifies `core.excludesFile`. | Architecture plan, Components, lines 70-72 and Interfaces, lines 141-147 | Task 1 | Covered |
| 3 | Helper has no SCIP or Semble semantics; callers supply entries such as `.liza/scip/` or `.sembleignore`. | Architecture plan, Components, lines 75-76 | Task 1 | Covered |
| 4 | Exclude setup is serialized so concurrent lifecycle calls cannot corrupt `info/exclude` or race on `core.excludesFile`. | Goal spec, Worktree Safety Requirements, lines 426-430; Architecture plan, Components, lines 77-78 | Task 1, Task 2 | Covered |
| 5 | Existing conflicting non-empty `core.excludesFile` is reported instead of overwritten. | Architecture plan, Components, line 78; Interfaces, lines 143-147 | Task 1, Task 2 | Covered |
| 6 | Repeated and concurrent calls preserve entries and avoid duplicate exclude lines. | Architecture plan, Components, lines 77-79; Interfaces, line 147 | Task 1, Task 2 | Covered |
| 7 | Move SCIP task-worktree private exclude setup from SCIP-owned helper/config/mutex code onto `internal/worktreeexclude`. | Architecture plan, SCIP Task Exclude Integration, lines 81-92 | Task 2 | Covered |
| 8 | Preserve the `.liza/scip/` exclude entry and existing task-worktree SCIP index behavior. | Architecture plan, SCIP Task Exclude Integration, lines 89-92 | Task 2 | Covered |
| 9 | Generated SCIP indexes remain prompt-local and hidden from `git status --porcelain`. | Goal spec, Worktree Safety Requirements, lines 414-415 and 420-433; Architecture plan, Scope 1 done_when, lines 236-240 | Task 2 | Covered |
| 10 | Repeated SCIP refreshes do not duplicate exclude entries. | Assigned task done_when; Architecture plan, Scope 1 done_when, lines 236-240 | Task 2 | Covered |
| 11 | Concurrent SCIP refreshes keep the shared private exclude file valid. | Assigned task done_when; Architecture plan, Cross-Cutting Concerns, lines 216-218 | Task 2 | Covered |
| 12 | Validation must prove helper idempotent, conflicting, concurrent setup and SCIP cleanliness/conflict tests actually ran. | Prior rejection feedback; Architecture plan, Testing concern, line 218 | Task 1, Task 2 | Covered |
| E2E | e2e test coverage for new behavior | Cross-cutting | N/A: internal helper and SCIP generated-index plumbing is covered by package-level regression tests; no user-facing workflow or CLI surface changes in this slice. | N/A |
| DOC | Documentation updates for changed behavior | Cross-cutting | N/A: operator documentation and Semble-facing behavior are explicitly owned by sibling documentation/lifecycle tasks, while this slice is internal refactoring plus SCIP regression coverage. | N/A |

## Pre-Submit Checklist

- Output JSON contains exactly two entries, one for each planned task.
- Output entry `desc`, `done_when`, `scope`, `spec_ref`, and `validation` values are copied verbatim from this plan.
- Task 2 uses `depends_on: ["0"]` because it consumes and may narrowly adjust `internal/worktreeexclude/*`.
- No output entry covers Semble `.sembleignore` generation, lifecycle wiring, prompt rendering, init prewarm, operator documentation, project-root `.sembleignore` handling, or `.liza/agent-outputs/`.
- Review diff must contain only this plan and its matching output JSON; superseded architecture-2 plan artifacts are removed from the candidate diff.
