# Code Planning: Pairing SCIP Root Planning

Task: `code-planning-main-1-code-planning-2`
Agent: `code-planner-3`
Spec: `specs/goals/20260602-indexing-activation.md`

## Based On

- Goal spec: `specs/goals/20260602-indexing-activation.md`, read in full.
- Master plan: `specs/plans/20260602-indexing-activation/20260602-123514-code-planning-main-1.md`.
- Prior dependency plan: `specs/plans/20260602-indexing-activation/20260602-130414-code-planning-main-1-code-planning-0.md`.
- Assigned task JSON for `code-planning-main-1-code-planning-2`.
- `internal/scipsearch/doc.go` and `internal/scipsearch/scipsearch.go`, read to verify current runtime planning ownership.
- Targeted reads of `internal/scipsearch/scipsearch_test.go` around runtime planning coverage.
- Stacklit module summary for `internal/scipsearch`.
- `specs/architecture/ADR/README.md`.

## Planning Boundary

This plan owns only pairing-specific SCIP root planning inside `internal/scipsearch`. It does not plan hook installation, pairing init CLI wiring, SessionStart rendering, MAS prompt rendering, e2e coverage, or user-facing documentation beyond assigning those responsibilities to existing sibling planning tasks.

Sibling boundary references used in the compliance matrix:

| Sibling | Responsibility |
|---|---|
| `code-planning-main-1-code-planning-0` | Generic optional-index routing in setup defaults; already merged dependency. |
| `code-planning-main-1-code-planning-1` | Pairing hook infrastructure and Stacklit lifecycle refresh. |
| `code-planning-main-1-code-planning-3` | Pairing Semble project-root safety activation. |
| `code-planning-main-1-code-planning-4` | Pairing init orchestration for project-local index activation. |
| `code-planning-main-1-code-planning-5` | MAS prompt gating and context boundaries. |
| `code-planning-main-1-code-planning-6` | End-to-end coverage for indexing activation. |
| `code-planning-main-1-code-planning-7` | User-facing documentation and embedded support-doc sync. |

## Architecture Notes

`internal/scipsearch` already owns supported SCIP language vocabulary, MAS runtime init validation, runtime command planning, and index discovery. Adding pairing planning here keeps language/root inference in one package while giving pairing init a stricter API that can reject ambiguous monorepos before hook scripts are generated.

The pairing API should not change MAS runtime behavior by accident. MAS runtime planning currently intersects configured languages with detected target-root languages and uses deterministic/fallback inference for Go, TypeScript, and Python. Pairing planning should document and test that this runtime inference remains intentionally different unless the downstream coder explicitly elects to align both paths in scope and tests.

## Planned Tasks

### Task 1 - Implement pairing SCIP root planning with ambiguity diagnostics

desc: Add pairing SCIP root planning with ambiguity diagnostics.

done_when: `internal/scipsearch` exposes pairing-specific SCIP planning that respects repeated `--scip-search <language>` filters, selects exactly one confident root per enabled Go, TypeScript, or Python language, emits concrete indexer commands with required root/cwd arguments, fails with a diagnostic listing candidate roots and unresolved language when root selection is ambiguous, and states whether MAS runtime inference remains intentionally different.

scope: Owns `internal/scipsearch` pairing planning APIs and tests for Go, TypeScript, Python, explicit language filtering, confident single-root planning, and monorepo ambiguity. Does not install Git hooks, does not wire pairing init CLI behavior, and does not change MAS runtime planning unless the task explicitly documents and tests that choice.

spec_ref: specs/goals/20260602-indexing-activation.md

validation:

- `go test ./internal/scipsearch`

task_depends_on:

- `code-planning-main-1-code-planning-0`

decomposition:

```yaml
owned_files:
  - internal/scipsearch/scipsearch.go
  - internal/scipsearch/scipsearch_test.go
owned_modules:
  - internal/scipsearch
interfaces_owned:
  - Pairing SCIP Planning Contract
interfaces_consumed:
  - Global Tool Guidance Contract
depends_on: []
task_depends_on:
  - code-planning-main-1-code-planning-0
coverage_notes: "Covers SCIP single-root command construction, repeated language filters, ambiguity diagnostics, and the FT-003 divergence note for MAS runtime inference."
```

## Dependency Graph

`Task 1` has no sibling output dependencies within this plan. It depends on already-merged `code-planning-main-1-code-planning-0` for the generic optional-index routing contract and will be consumed by sibling `code-planning-main-1-code-planning-4` when pairing init wiring is planned.

## Shared-File Audit

No file appears in more than one output task in this plan because this plan has one output task. Shared-file ordering with sprint siblings is declared in the master plan: `internal/scipsearch/**` is written by this task, consumed by pairing init wiring, and not written by hook, Semble, prompt, e2e, or docs tasks.

## Spec Compliance Matrix

| # | Requirement | Source | Task(s) | Status |
|---|-------------|--------|---------|--------|
| 1 | Generic routing guidance must be safe when Stacklit, SCIP, or Semble are disabled, missing, or not advertised. | General NFR-000-1, lines 45-46 | Sibling `code-planning-main-1-code-planning-0`; sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-7` | Covered |
| 2 | Project-specific paths, generated index locations, and readiness state must not be written into global `AGENT_TOOLS.md`. | General NFR-000-2, lines 47-48 | Sibling `code-planning-main-1-code-planning-0`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-7` | Covered |
| 3 | Optional indexing failures must omit unavailable prompt/context sections instead of making agents troubleshoot unrelated tooling. | General NFR-000-3, lines 49-51 | Sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 4 | `liza setup` installs generic Stacklit, SCIP, and Semble routing guidance into default `AGENT_TOOLS.md`. | FR-001-1, lines 96-97 | Sibling `code-planning-main-1-code-planning-0` | Covered |
| 5 | Guidance is conditional on explicit Stacklit path, explicit SCIP path, or supplied Semble target/readiness. | FR-001-2, lines 98-102 | Sibling `code-planning-main-1-code-planning-0`; sibling `code-planning-main-1-code-planning-5` | Covered |
| 6 | Guidance describes fallbacks for disabled or unavailable tools. | FR-001-3, lines 103-106 | Sibling `code-planning-main-1-code-planning-0`; sibling `code-planning-main-1-code-planning-7` | Covered |
| 7 | Guidance contains no project-specific absolute paths, generated repo locations, or installed-tool claims. | FR-001-4, lines 107-109 | Sibling `code-planning-main-1-code-planning-0`; sibling `code-planning-main-1-code-planning-7` | Covered |
| 8 | Installed default guidance remains concise. | NFR-001-1, lines 113-115 | Sibling `code-planning-main-1-code-planning-0` | Covered |
| 9 | Fresh setup installs generic Stacklit/SCIP/Semble routing guidance. | AC-001-1, lines 119-121 | Sibling `code-planning-main-1-code-planning-0` | Covered |
| 10 | With optional tools disabled, guidance still routes to valid fallback tools. | AC-001-2, lines 122-124 | Sibling `code-planning-main-1-code-planning-0` | Covered |
| 11 | User-customized `AGENT_TOOLS.md` keeps setup customization protection. | AC-001-3, lines 125-126 | Sibling `code-planning-main-1-code-planning-0` | Covered |
| 12 | Pairing init flows install project-local provider hooks as today. | FR-002-1, lines 156-157 | Sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 13 | Stacklit truthy pairing init installs or verifies Git hook plumbing for repo-root `stacklit.json` refresh. | FR-002-2, lines 158-160 | Sibling `code-planning-main-1-code-planning-1`; sibling `code-planning-main-1-code-planning-4` | Covered |
| 14 | Automatic Git lifecycle refresh must not run `stacklit ai-summary`; manual `ai` may. | FR-002-2a, lines 161-163 | Sibling `code-planning-main-1-code-planning-1`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 15 | SCIP truthy pairing init autodetects repo-specific SCIP plan and hook plumbing for enabled languages. | FR-002-3, lines 164-167 | Task 1; sibling `code-planning-main-1-code-planning-4` | Covered |
| 16 | Pairing SCIP autodetection plans concrete Go, TypeScript, and Python indexer commands with root/cwd arguments. | FR-002-3a, lines 168-170 | Task 1 | Covered |
| 17 | Repeated `--scip-search` flags restrict languages but are not sufficient root/cwd selection. | FR-002-3b, lines 171-173 | Task 1; sibling `code-planning-main-1-code-planning-4` | Covered |
| 18 | Ambiguous monorepo SCIP roots fail with candidate-root diagnostic instead of guessed hook command. | FR-002-3c, lines 174-177 | Task 1; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 19 | Confident single-root SCIP detection generates concrete repo-specific SCIP commands. | FR-002-3d, lines 178-180 | Task 1; sibling `code-planning-main-1-code-planning-4` | Covered |
| 20 | Pairing init keeps generated indexes ignored, privately excluded, or out of accidental task diffs unless intentionally tracked. | FR-002-4, lines 181-183 | Sibling `code-planning-main-1-code-planning-1`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 21 | Semble truthy pairing init ensures safe physical `.sembleignore` before SessionStart advertises Semble. | FR-002-5, lines 184-186 | Sibling `code-planning-main-1-code-planning-3`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 22 | Pairing init must not modify `~/.liza/AGENT_TOOLS.md`. | FR-002-6, lines 187-188 | Sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 23 | Pairing init preserves existing project Git hooks and does not silently overwrite lifecycle hooks. | FR-002-7, lines 189-191 | Sibling `code-planning-main-1-code-planning-1`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 24 | Pairing init detects non-default `core.hooksPath` and installs safely or reports diagnostic. | FR-002-8, lines 192-195 | Sibling `code-planning-main-1-code-planning-1`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 25 | Docs distinguish pairing-init env gates from MAS runtime activation. | FR-002-9, lines 196-199 | Sibling `code-planning-main-1-code-planning-7` | Covered |
| 26 | Pairing index activation preserves SessionStart concrete path/readiness rule. | NFR-002-1, lines 203-205 | Sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 27 | Pairing activation does not assume Liza's own build system or target project stack. | NFR-002-2, lines 206-207 | Task 1; sibling `code-planning-main-1-code-planning-1`; sibling `code-planning-main-1-code-planning-4` | Covered |
| 28 | Disabled pairing env vars activate no optional indexing hook behavior. | AC-002-1, lines 211-212 | Sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 29 | Stacklit-enabled pairing lifecycle refresh works without editing `AGENT_TOOLS.md`. | AC-002-2, lines 213-215 | Sibling `code-planning-main-1-code-planning-1`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 30 | Semble-enabled pairing init creates or reports missing safety artifact before advertisement. | AC-002-3, lines 216-218 | Sibling `code-planning-main-1-code-planning-3`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 31 | Generated indexes do not appear as accidental untracked files after init and refresh. | AC-002-4, lines 219-221 | Sibling `code-planning-main-1-code-planning-1`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 32 | Single-root Go, TypeScript, or Python pairing SCIP init generates concrete commands. | AC-002-5, lines 222-225 | Task 1; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 33 | Monorepo ambiguous SCIP roots fail clearly instead of writing guessed hook. | AC-002-6, lines 226-228 | Task 1; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 34 | `--scip-search go` restricts detection to Go while still requiring confident Go root selection. | AC-002-7, lines 229-231 | Task 1; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 35 | Existing project Git hooks are preserved, chained, or explicitly diagnosed. | AC-002-8, lines 232-234 | Sibling `code-planning-main-1-code-planning-1`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 36 | Non-default `core.hooksPath` installs into effective path or reports unsafe diagnostic. | AC-002-9, lines 235-237 | Sibling `code-planning-main-1-code-planning-1`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 37 | Automatic Stacklit lifecycle refresh skips AI-summary. | AC-002-10, lines 238-240 | Sibling `code-planning-main-1-code-planning-1`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 38 | Manual `liza-index.sh ai` includes Stacklit AI-summary behavior. | AC-002-11, lines 241-242 | Sibling `code-planning-main-1-code-planning-1`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 39 | Docs explain pairing-init env gates separately from MAS runtime activation. | AC-002-12, lines 243-245 | Sibling `code-planning-main-1-code-planning-7` | Covered |
| 40 | MAS init preserves Stacklit/Semble process-local gates and SCIP env gate plus durable config. | FR-003-1, lines 282-284 | Sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 41 | MAS prompts include optional tool sections only when corresponding metadata is present. | FR-003-2, lines 285-286 | Sibling `code-planning-main-1-code-planning-5` | Covered |
| 42 | MAS prompt routing stays role/target specific. | FR-003-3, lines 287-289 | Sibling `code-planning-main-1-code-planning-5` | Covered |
| 43 | MAS init behavior does not depend on pairing Git hooks or repo-root pairing indexes. | FR-003-4, lines 290-291 | Sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 44 | Preserve or explicitly supersede current MAS SCIP runtime planning semantics. | FR-003-5, lines 292-296 | Task 1; sibling `code-planning-main-1-code-planning-5` | Covered |
| 45 | Plan documents divergence or aligns MAS if pairing SCIP ambiguity behavior is stricter. | FR-003-6, lines 297-300 | Task 1 | Covered |
| 46 | Disabled SCIP env gate omits MAS SCIP prompt even with config. | AC-003-1, lines 304-305 | Sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 47 | Disabled Stacklit env gate omits MAS Stacklit prompt unless documented fallback exists. | AC-003-2, lines 306-308 | Sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 48 | Disabled Semble env gate omits MAS Semble prompt. | AC-003-3, lines 309-310 | Sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 49 | Enabled tool readiness/index failure omits failed MAS section while other guidance remains usable. | AC-003-4, lines 311-313 | Sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 50 | Pairing SCIP autodetection task states MAS inference choice. | AC-003-5, lines 314-317 | Task 1 | Covered |
| 51 | Pairing SessionStart continues emitting concrete repo-root optional tool instructions only when artifacts/readiness exist. | FR-004-1, lines 345-347 | Sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6` | Covered |
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

- Atomicity: Task 1 has one observable behavior change, pairing-specific SCIP planning and diagnostics in `internal/scipsearch`.
- Falsifiability: Task 1 can be falsified by package tests that inspect selected languages, exact command plans, ambiguity diagnostics, and the explicit MAS divergence statement.
- Output parity: The structured output entry must copy Task 1 `desc`, `done_when`, `scope`, `spec_ref`, and validation command character-for-character from this plan.
- Shared files: no shared files across outputs in this plan.
- Cross-references: every sibling referenced above is declared in the boundary table and already appears in the master plan's task graph.
- Scope protection: implementation files named in Task 1 are downstream child-task scope, not modified by this code-planning task.
