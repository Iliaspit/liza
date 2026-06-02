# Code Planning: Pairing Stacklit Hook Infrastructure

Task: `code-planning-main-1-code-planning-1`

## Based On

- Goal spec: `specs/goals/20260602-indexing-activation.md`
- Master plan: `specs/plans/20260602-indexing-activation/20260602-123514-code-planning-main-1.md`
- Prior dependency plan: `specs/plans/20260602-indexing-activation/20260602-130414-code-planning-main-1-code-planning-0.md`
- Assigned scope: `internal/pairingindex/**`, tests, and generated hook/script content contract.
- Current source orientation: Stacklit runtime command planning in `internal/stacklit`, Git command execution in `internal/gitenv`, and private generated-artifact exclusion patterns in `internal/worktreeexclude`.

## Planning Boundary

This plan owns the pairing-specific Stacklit Git hook infrastructure package only. It does not wire pairing init CLI behavior, implement SCIP root detection, edit SessionStart output, update global `AGENT_TOOLS.md`, change MAS worktree index generation, or update user-facing docs.

The prior setup guidance task remains the source for the Global Tool Guidance Contract. This plan consumes that contract only to preserve the separation between global routing text and project-local activation artifacts.

Sibling boundary references used in the compliance matrix:

| Sibling | Responsibility |
|---|---|
| `code-planning-main-1-code-planning-0` | Setup default optional-index routing guidance. |
| `code-planning-main-1-code-planning-2` | Pairing SCIP root planning and ambiguity diagnostics. |
| `code-planning-main-1-code-planning-3` | Pairing Semble project-root safety activation. |
| `code-planning-main-1-code-planning-4` | Pairing init orchestration for project-local index activation. |
| `code-planning-main-1-code-planning-5` | MAS prompt gating and context boundaries. |
| `code-planning-main-1-code-planning-6` | End-to-end coverage for indexing activation. |
| `code-planning-main-1-code-planning-7` | User-facing documentation and embedded support-doc sync. |

## Architecture Notes

`internal/pairingindex` should expose a small pairing-init-facing API whose behavior is fully testable without invoking real optional tools. The package should separate three concerns:

- effective Git hook location and lifecycle wrapper installation;
- generated `liza-index.sh` script content for Stacklit refresh;
- generated artifact cleanliness for repo-root `stacklit.json`.

The generated script is a contract boundary. Automatic lifecycle hooks must invoke it in the no-AI mode; only a direct manual `ai` argument may request AI-summary. Existing hook files are user state, so implementation must either recognize/idempotently preserve Liza-managed wrappers, chain safely, or return a clear collision diagnostic. Non-default `core.hooksPath` must be resolved from the effective Git configuration before writing hooks so `.git/hooks/*` files are not installed inertly.

## Planned Tasks

### Task 1 - Implement effective Git hook plumbing

desc: Implement effective Git hook plumbing for pairing index refresh.

done_when: `internal/pairingindex` can resolve the effective Git hooks directory for default and non-default `core.hooksPath`, install or verify Liza-managed wrappers for `post-commit`, `post-checkout`, `post-merge`, and `post-rewrite`, preserve idempotent Liza-managed hook state, and return explicit collision diagnostics instead of silently overwriting existing non-Liza hooks.

scope: Owns `internal/pairingindex` hook path resolution, lifecycle hook wrapper installation APIs, hook collision diagnostics, and tests/fixtures for default hooks, relative and absolute `core.hooksPath`, idempotent reinstall, missing or unsafe hook directories, and existing-hook collisions. Does not generate the `liza-index.sh` Stacklit script body, plan SCIP roots, wire pairing init CLI behavior, update global `AGENT_TOOLS.md`, or change MAS worktree index generation.

spec_ref: specs/goals/20260602-indexing-activation.md

validation:

- `go test ./internal/pairingindex`

task_depends_on:

- `code-planning-main-1-code-planning-0`

### Task 2 - Implement Stacklit liza-index script contract

desc: Implement Stacklit `liza-index.sh` script contract for pairing lifecycle refresh.

done_when: `internal/pairingindex` can render and install an executable repo-local `liza-index.sh` that automatic lifecycle hooks invoke without AI-summary, refreshes repo-root `stacklit.json` using the existing no-AI Stacklit generation command contract, and supports manual `liza-index.sh ai` invocation that includes Stacklit AI-summary behavior.

scope: Owns `internal/pairingindex` generated `liza-index.sh` content, script installation permissions, automatic no-AI Stacklit refresh command tests, manual `ai` argument tests, and integration points from lifecycle wrappers to the script entrypoint. Does not define SCIP indexer command construction, wire pairing init CLI behavior, edit SessionStart hooks, update global `AGENT_TOOLS.md`, or change MAS Stacklit refresh behavior in `internal/stacklit`.

spec_ref: specs/goals/20260602-indexing-activation.md

validation:

- `go test ./internal/pairingindex`

depends_on:

- Task 1

### Task 3 - Implement generated Stacklit artifact cleanliness

desc: Implement generated Stacklit artifact cleanliness for pairing index refresh.

done_when: `internal/pairingindex` keeps generated repo-root Stacklit artifacts out of accidental task diffs by verifying `stacklit.json` is intentionally tracked, ignored, or privately excluded before activation succeeds, reports a clear diagnostic when no safe status strategy exists, and proves with Git status fixtures that generated Stacklit artifacts do not appear as accidental untracked files.

scope: Owns `internal/pairingindex` generated artifact status checks, private exclude or ignore coordination for repo-root `stacklit.json`, diagnostics for unsafe generated-artifact state, and tests/fixtures for tracked, ignored, privately excluded, and unsafe untracked Stacklit artifact cases. Does not modify worktree-local MAS artifact handling, wire pairing init CLI behavior, update global `AGENT_TOOLS.md`, or document user-facing env-gate behavior.

spec_ref: specs/goals/20260602-indexing-activation.md

validation:

- `go test ./internal/pairingindex`

depends_on:

- Task 2

## Dependency Graph

```text
Existing dependency: code-planning-main-1-code-planning-0
  -> Task 1
Task 1
  -> Task 2
Task 2
  -> Task 3
```

Task 1 must precede Task 2 because lifecycle wrappers need an installed script entrypoint. Task 2 must precede Task 3 because artifact-cleanliness tests should validate the generated Stacklit refresh path rather than a placeholder script. This serializes work inside `internal/pairingindex`, avoiding shared-package merge conflicts while keeping each task's intent falsifiable.

## Shared-File Audit

All planned tasks write under `internal/pairingindex/**`. Because that package is shared, Task 2 depends on Task 1 and Task 3 depends on Task 2. No task in this plan writes files owned by sibling plans. Sibling `code-planning-main-1-code-planning-4` consumes this package after this planning output is merged and generated implementation tasks complete.

## Spec Compliance Matrix

| # | Requirement | Source | Task(s) | Status |
|---|-------------|--------|---------|--------|
| 1 | Generic routing guidance must be safe when Stacklit, SCIP, or Semble are disabled, missing, or not advertised. | General NFR-000-1, lines 45-46 | Sibling `code-planning-main-1-code-planning-0`; sibling `code-planning-main-1-code-planning-5` | Covered |
| 2 | Project-specific paths, generated index locations, and readiness state must not be written into global `AGENT_TOOLS.md`. | General NFR-000-2, lines 47-48 | Task 1; Task 2; Task 3; sibling `code-planning-main-1-code-planning-0`; sibling `code-planning-main-1-code-planning-7` | Covered |
| 3 | Optional indexing failures must omit unavailable prompt/context sections instead of making agents troubleshoot unrelated tooling. | General NFR-000-3, lines 49-51 | Sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 4 | `liza setup` installs generic Stacklit, SCIP, and Semble routing guidance into default `AGENT_TOOLS.md`. | FR-001-1, lines 96-97 | Sibling `code-planning-main-1-code-planning-0` | Covered |
| 5 | Guidance is conditional on explicit Stacklit path, explicit SCIP path, or supplied Semble target/readiness. | FR-001-2, lines 98-102 | Sibling `code-planning-main-1-code-planning-0`; sibling `code-planning-main-1-code-planning-5` | Covered |
| 6 | Guidance describes fallbacks for disabled or unavailable tools. | FR-001-3, lines 103-106 | Sibling `code-planning-main-1-code-planning-0`; sibling `code-planning-main-1-code-planning-7` | Covered |
| 7 | Guidance contains no project-specific absolute paths, generated repo locations, or installed-tool claims. | FR-001-4, lines 107-109 | Sibling `code-planning-main-1-code-planning-0`; sibling `code-planning-main-1-code-planning-7` | Covered |
| 8 | Installed default guidance remains concise. | NFR-001-1, lines 113-115 | Sibling `code-planning-main-1-code-planning-0` | Covered |
| 9 | Fresh setup installs generic Stacklit/SCIP/Semble routing guidance. | AC-001-1, lines 119-121 | Sibling `code-planning-main-1-code-planning-0` | Covered |
| 10 | With optional tools disabled, guidance still routes to valid fallback tools. | AC-001-2, lines 122-124 | Sibling `code-planning-main-1-code-planning-0` | Covered |
| 11 | User-customized `AGENT_TOOLS.md` keeps setup customization protection. | AC-001-3, lines 125-126 | Sibling `code-planning-main-1-code-planning-0` | Covered |
| 12 | Pairing init flows install project-local provider hooks as today. | FR-002-1, lines 156-157 | Sibling `code-planning-main-1-code-planning-4` | Covered |
| 13 | Stacklit truthy pairing init installs or verifies Git hook plumbing for repo-root `stacklit.json` refresh. | FR-002-2, lines 158-160 | Task 1; Task 2; sibling `code-planning-main-1-code-planning-4` | Covered |
| 14 | Automatic Git lifecycle refresh must not run `stacklit ai-summary`; manual `ai` may. | FR-002-2a, lines 161-163 | Task 2; sibling `code-planning-main-1-code-planning-6` | Covered |
| 15 | SCIP truthy pairing init autodetects repo-specific SCIP plan and hook plumbing for enabled languages. | FR-002-3, lines 164-167 | Task 1; Task 2; sibling `code-planning-main-1-code-planning-2`; sibling `code-planning-main-1-code-planning-4` | Covered |
| 16 | Pairing SCIP autodetection plans concrete Go, TypeScript, and Python indexer commands with root/cwd arguments. | FR-002-3a, lines 168-170 | Sibling `code-planning-main-1-code-planning-2` | Covered |
| 17 | Repeated `--scip-search` flags restrict languages but are not sufficient root/cwd selection. | FR-002-3b, lines 171-173 | Sibling `code-planning-main-1-code-planning-2`; sibling `code-planning-main-1-code-planning-4` | Covered |
| 18 | Ambiguous monorepo SCIP roots fail with candidate-root diagnostic instead of guessed hook command. | FR-002-3c, lines 174-177 | Sibling `code-planning-main-1-code-planning-2`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 19 | Confident single-root SCIP detection generates concrete repo-specific SCIP commands. | FR-002-3d, lines 178-180 | Sibling `code-planning-main-1-code-planning-2`; sibling `code-planning-main-1-code-planning-4` | Covered |
| 20 | Pairing init keeps generated indexes ignored, privately excluded, or out of accidental task diffs unless intentionally tracked. | FR-002-4, lines 181-183 | Task 3; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 21 | Semble truthy pairing init ensures safe physical `.sembleignore` before SessionStart advertises Semble. | FR-002-5, lines 184-186 | Sibling `code-planning-main-1-code-planning-3`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 22 | Pairing init must not modify `~/.liza/AGENT_TOOLS.md`. | FR-002-6, lines 187-188 | Task 1; Task 2; Task 3; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 23 | Pairing init preserves existing project Git hooks and does not silently overwrite lifecycle hooks. | FR-002-7, lines 189-191 | Task 1; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 24 | Pairing init detects non-default `core.hooksPath` and installs safely or reports diagnostic. | FR-002-8, lines 192-195 | Task 1; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 25 | Docs distinguish pairing-init env gates from MAS runtime activation. | FR-002-9, lines 196-199 | Sibling `code-planning-main-1-code-planning-7` | Covered |
| 26 | Pairing index activation preserves SessionStart concrete path/readiness rule. | NFR-002-1, lines 203-205 | Task 2; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-5` | Covered |
| 27 | Pairing activation does not assume Liza's own build system or target project stack. | NFR-002-2, lines 206-207 | Task 1; Task 2; Task 3; sibling `code-planning-main-1-code-planning-2`; sibling `code-planning-main-1-code-planning-4` | Covered |
| 28 | Disabled pairing env vars activate no optional indexing hook behavior. | AC-002-1, lines 211-212 | Sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 29 | Stacklit-enabled pairing lifecycle refresh works without editing `AGENT_TOOLS.md`. | AC-002-2, lines 213-215 | Task 1; Task 2; Task 3; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 30 | Semble-enabled pairing init creates or reports missing safety artifact before advertisement. | AC-002-3, lines 216-218 | Sibling `code-planning-main-1-code-planning-3`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 31 | Generated indexes do not appear as accidental untracked files after init and refresh. | AC-002-4, lines 219-221 | Task 3; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 32 | Single-root Go, TypeScript, or Python pairing SCIP init generates concrete commands. | AC-002-5, lines 222-225 | Sibling `code-planning-main-1-code-planning-2`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 33 | Monorepo ambiguous SCIP roots fail clearly instead of writing guessed hook. | AC-002-6, lines 226-228 | Sibling `code-planning-main-1-code-planning-2`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 34 | `--scip-search go` restricts detection to Go while still requiring confident Go root selection. | AC-002-7, lines 229-231 | Sibling `code-planning-main-1-code-planning-2`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 35 | Existing project Git hooks are preserved, chained, or explicitly diagnosed. | AC-002-8, lines 232-234 | Task 1; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 36 | Non-default `core.hooksPath` installs into effective path or reports unsafe diagnostic. | AC-002-9, lines 235-237 | Task 1; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 37 | Automatic Stacklit lifecycle refresh skips AI-summary. | AC-002-10, lines 238-240 | Task 2; sibling `code-planning-main-1-code-planning-6` | Covered |
| 38 | Manual `liza-index.sh ai` includes Stacklit AI-summary behavior. | AC-002-11, lines 241-242 | Task 2; sibling `code-planning-main-1-code-planning-6` | Covered |
| 39 | Docs explain pairing-init env gates separately from MAS runtime activation. | AC-002-12, lines 243-245 | Sibling `code-planning-main-1-code-planning-7` | Covered |
| 40 | MAS init preserves Stacklit/Semble process-local gates and SCIP env gate plus durable config. | FR-003-1, lines 282-284 | Sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 41 | MAS prompts include optional tool sections only when corresponding metadata is present. | FR-003-2, lines 285-286 | Sibling `code-planning-main-1-code-planning-5` | Covered |
| 42 | MAS prompt routing stays role/target specific. | FR-003-3, lines 287-289 | Sibling `code-planning-main-1-code-planning-5` | Covered |
| 43 | MAS init behavior does not depend on pairing Git hooks or repo-root pairing indexes. | FR-003-4, lines 290-291 | Task 1; Task 2; Task 3; sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 44 | Preserve or explicitly supersede current MAS SCIP runtime planning semantics. | FR-003-5, lines 292-296 | Sibling `code-planning-main-1-code-planning-2`; sibling `code-planning-main-1-code-planning-5` | Covered |
| 45 | Plan documents divergence or aligns MAS if pairing SCIP ambiguity behavior is stricter. | FR-003-6, lines 297-300 | Sibling `code-planning-main-1-code-planning-2` | Covered |
| 46 | Disabled SCIP env gate omits MAS SCIP prompt even with config. | AC-003-1, lines 304-305 | Sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 47 | Disabled Stacklit env gate omits MAS Stacklit prompt unless documented fallback exists. | AC-003-2, lines 306-308 | Sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 48 | Disabled Semble env gate omits MAS Semble prompt. | AC-003-3, lines 309-310 | Sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 49 | Enabled tool readiness/index failure omits failed MAS section while other guidance remains usable. | AC-003-4, lines 311-313 | Sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 50 | Pairing SCIP autodetection task states MAS inference choice. | AC-003-5, lines 314-317 | Sibling `code-planning-main-1-code-planning-2` | Covered |
| 51 | Pairing SessionStart continues emitting concrete repo-root optional tool instructions only when artifacts/readiness exist. | FR-004-1, lines 345-347 | Task 2; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 52 | MAS prompts continue using structured task/reviewer/orchestrator metadata rather than pairing SessionStart assumptions. | FR-004-2, lines 348-350 | Sibling `code-planning-main-1-code-planning-5` | Covered |
| 53 | Prompt and SessionStart guidance avoid duplicated long-form routing text when shorter shared wording is sufficient. | FR-004-3, lines 351-352 | Sibling `code-planning-main-1-code-planning-0`; sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-7` | Covered |
| 54 | Generic routing text states indexes/search results are navigation aids and direct source reads remain evidence. | FR-004-4, lines 353-355 | Sibling `code-planning-main-1-code-planning-0`; sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-7` | Covered |
| 55 | Pairing with no indexes and Semble disabled emits initialization guidance without optional tool blocks. | AC-004-1, lines 359-361 | Sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 56 | MAS mode with generated worktree indexes uses worktree-specific paths. | AC-004-2, lines 362-364 | Sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 57 | Full routing guidance distinguishes conceptual discovery, module orientation, symbol navigation, exact search, syntax search, and direct reads. | AC-004-3, lines 365-368 | Sibling `code-planning-main-1-code-planning-0`; sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-7` | Covered |
| 58 | Unavailable optional tools are not instructed before fallback search/read tools. | AC-004-4, lines 369-371 | Sibling `code-planning-main-1-code-planning-0`; sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-7` | Covered |
| E2E | e2e test coverage for new behavior | Cross-cutting | Sibling `code-planning-main-1-code-planning-6`: cross-cutting end-to-end coverage for setup/init/prompt behavior | Covered |
| DOC | Documentation updates for changed behavior | Cross-cutting | Sibling `code-planning-main-1-code-planning-7`: cross-cutting documentation for setup/init behavior | Covered |

## Pre-Submit Self-Check Notes

- Atomicity: Task 1 owns hook location/wrapper plumbing, Task 2 owns the generated Stacklit script contract, and Task 3 owns generated-artifact cleanliness.
- Falsifiability: each task has a concrete observable outcome backed by `go test ./internal/pairingindex`.
- Output parity: structured output entries must copy each task's `desc`, `done_when`, `scope`, `spec_ref`, and validation command character-for-character from this plan.
- Shared files: all tasks share `internal/pairingindex/**`, so Task 2 depends on Task 1 and Task 3 depends on Task 2.
- Scope protection: no planned task writes pairing init CLI wiring, SCIP root detection, global setup guidance, MAS index generation, or docs.
