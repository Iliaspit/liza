# Code Planning: Preserve MAS Prompt Gating and Context Boundaries

Task ID: code-planning-main-1-code-planning-5
Agent ID: code-planner-2
Timestamp: 20260602-130539

## Based On

- Goal spec: `specs/goals/20260602-indexing-activation.md`
- Master plan: `specs/plans/20260602-indexing-activation/20260602-123514-code-planning-main-1.md`
- Assigned scope: MAS prompt metadata/rendering code and tests in `internal/prompts`, plus directly related prompt metadata builders in `internal/agent` or `internal/ops` if existing ownership requires it.
- Source reads: `internal/prompts/builder.go`, `internal/prompts/templates/base_prompt.tmpl`, `internal/prompts/role_context.go`, `internal/agent/prompt.go`, `internal/agent/strategy_orchestrator.go`, `internal/ops/scip_indexing.go`, `internal/ops/stacklit_indexing.go`, `internal/ops/semble_indexing.go`, and optional-tool readiness APIs in `internal/scipsearch`, `internal/stacklit`, and `internal/semble`.

## Architectural Approach

MAS prompt context must remain metadata-driven. Runtime index generation may refresh optional artifacts before prompts are built, but prompt rendering must only consume explicit prompt metadata for the current target root. Disabled env gates and readiness failures should remove only the affected optional-tool metadata, not fail the whole prompt and not cause fallback to pairing repo-root assumptions.

The split below keeps renderer wording separate from metadata production:

- CP6-1 owns the generic base prompt rendering contract and query-routing wording.
- CP6-2 owns MAS target selection, env/readiness gates, and failure isolation for task, reviewer, and orchestrator prompt metadata.

This avoids shared file writes across parallel coders: CP6-2 depends on CP6-1 because its prompt assertions consume CP6-1's rendered wording, but it does not write CP6-1's renderer files.

## Planned Coding Tasks

### CP6-1 - Render MAS optional-index prompt sections only from supplied prompt metadata

desc: Render MAS optional-index prompt sections only from supplied prompt metadata.

done_when: `BuildBasePrompt` omits Stacklit, SCIP, Semble, and QUERY ROUTING command sections when their corresponding metadata is absent; renders only supplied sections when one or more metadata inputs are present; and the rendered routing guidance distinguishes conceptual discovery, module orientation, dependency impact, symbol tracing, exact literal/path search, syntax-shaped search, and direct source reads as the evidence required for edits, reviews, and success claims.

scope: Owns `internal/prompts/templates/base_prompt.tmpl`, `internal/prompts/builder.go`, `internal/prompts/builder_test.go`, and prompt renderer helper tests needed for BasePromptConfig metadata filtering. Does not modify `AGENT_TOOLS.md`, pairing SessionStart hooks, pairing init activation, or runtime index generation.

spec_ref: specs/goals/20260602-indexing-activation.md

validation:

- `go test ./internal/prompts`

### CP6-2 - Build MAS prompt metadata from env-gated, target-specific readiness only

desc: Build MAS prompt metadata from env-gated, target-specific readiness only.

done_when: Task, reviewer, and orchestrator prompt construction populate Stacklit, SCIP, and Semble metadata only from the current task worktree, reviewer worktree, or safe orchestrator project root; disabled env gates suppress sections even when stale artifacts exist; Stacklit, SCIP, or Semble availability/readiness failures suppress only that tool's prompt metadata while preserving other available routing metadata and prompt construction.

scope: Owns `internal/agent/prompt.go`, `internal/agent/prompt_test.go`, `internal/agent/strategy_orchestrator.go`, `internal/agent/strategy_orchestrator_scip_test.go`, and directly related metadata calls in `internal/ops/scip_indexing.go`, `internal/ops/stacklit_indexing.go`, `internal/ops/semble_indexing.go`, and `internal/ops/semble_indexing_test.go` if prompt metadata failure isolation requires runtime-index cleanup or warning behavior changes. Does not modify `internal/prompts/templates/base_prompt.tmpl`, pairing SessionStart hooks, pairing init activation, global `AGENT_TOOLS.md`, or generated index locations.

spec_ref: specs/goals/20260602-indexing-activation.md

validation:

- `go test ./internal/agent ./internal/ops`

depends_on:

- CP6-1

## Dependency Graph

```text
CP6-1
  -> CP6-2
```

## Shared File Audit

| File or area | Writing task | Other planned task access | Dependency |
|---|---|---|---|
| `internal/prompts/templates/base_prompt.tmpl` | CP6-1 | CP6-2 consumes rendered output through public prompt builder only | CP6-2 depends on CP6-1 |
| `internal/prompts/builder.go` and `internal/prompts/builder_test.go` | CP6-1 | CP6-2 read-only through `prompts.BuildBasePrompt` | CP6-2 depends on CP6-1 |
| `internal/agent/prompt.go` and related agent tests | CP6-2 | none | none |
| `internal/ops/*_indexing.go` related metadata cleanup/warnings | CP6-2 | none | none |

## Validation Plan

Canonical validation for the assigned scope remains:

- `go test ./internal/prompts ./internal/agent ./internal/ops`

Child tasks use narrower package commands listed above. The final sprint e2e task remains responsible for cross-component init/prompt coverage beyond this scope.

## Spec Compliance Matrix

| # | Requirement | Source | Task(s) | Status |
|---|-------------|--------|---------|--------|
| 1 | Generic routing guidance must be safe when Stacklit, SCIP, or Semble are disabled, missing, or not advertised. | General NFR-000-1, lines 45-46 | CP6-1, CP6-2; sibling code-planning-main-1-code-planning-0 | Covered |
| 2 | Project-specific paths, generated index locations, and readiness state must not be written into global `AGENT_TOOLS.md`. | General NFR-000-2, lines 47-48 | Sibling code-planning-main-1-code-planning-0; CP6-1 excludes `AGENT_TOOLS.md` | Covered |
| 3 | Optional indexing failures must omit unavailable prompt/context sections instead of making agents troubleshoot unrelated tooling. | General NFR-000-3, lines 49-51 | CP6-2; sibling code-planning-main-1-code-planning-6 | Covered |
| 4 | `liza setup` installs generic Stacklit, SCIP, and Semble routing guidance into default `AGENT_TOOLS.md`. | FR-001-1, lines 96-97 | Sibling code-planning-main-1-code-planning-0 | Covered |
| 5 | Guidance is conditional on explicit Stacklit path, explicit SCIP path, or supplied Semble target/readiness. | FR-001-2, lines 98-102 | CP6-1, CP6-2; sibling code-planning-main-1-code-planning-0 | Covered |
| 6 | Guidance describes fallbacks for disabled or unavailable tools. | FR-001-3, lines 103-106 | CP6-1; sibling code-planning-main-1-code-planning-0 | Covered |
| 7 | Guidance contains no project-specific absolute paths, generated repo locations, or installed-tool claims. | FR-001-4, lines 107-109 | Sibling code-planning-main-1-code-planning-0; CP6-1 for MAS template wording | Covered |
| 8 | Installed default guidance remains concise. | NFR-001-1, lines 113-115 | Sibling code-planning-main-1-code-planning-0 | Covered |
| 9 | Fresh setup installs generic Stacklit/SCIP/Semble routing guidance. | AC-001-1, lines 119-121 | Sibling code-planning-main-1-code-planning-0 | Covered |
| 10 | With optional tools disabled, guidance still routes to valid fallback tools. | AC-001-2, lines 122-124 | CP6-1 for MAS prompt fallback wording; sibling code-planning-main-1-code-planning-0 | Covered |
| 11 | User-customized `AGENT_TOOLS.md` keeps setup customization protection. | AC-001-3, lines 125-126 | Sibling code-planning-main-1-code-planning-0 | Covered |
| 12 | Pairing init flows install project-local provider hooks as today. | FR-002-1, lines 156-157 | Sibling code-planning-main-1-code-planning-4 | Covered |
| 13 | Stacklit truthy pairing init installs or verifies Git hook plumbing for repo-root `stacklit.json` refresh. | FR-002-2, lines 158-160 | Sibling code-planning-main-1-code-planning-1 and code-planning-main-1-code-planning-4 | Covered |
| 14 | Automatic Git lifecycle refresh must not run `stacklit ai-summary`; manual `ai` may. | FR-002-2a, lines 161-163 | Sibling code-planning-main-1-code-planning-1 | Covered |
| 15 | SCIP truthy pairing init autodetects repo-specific SCIP plan and hook plumbing for enabled languages. | FR-002-3, lines 164-167 | Sibling code-planning-main-1-code-planning-2 and code-planning-main-1-code-planning-4 | Covered |
| 16 | Pairing SCIP autodetection plans concrete Go, TypeScript, and Python indexer commands with root/cwd arguments. | FR-002-3a, lines 168-170 | Sibling code-planning-main-1-code-planning-2 | Covered |
| 17 | Repeated `--scip-search` flags restrict languages but are not sufficient root/cwd selection. | FR-002-3b, lines 171-173 | Sibling code-planning-main-1-code-planning-2 and code-planning-main-1-code-planning-4 | Covered |
| 18 | Ambiguous monorepo SCIP roots fail with candidate-root diagnostic instead of guessed hook command. | FR-002-3c, lines 174-177 | Sibling code-planning-main-1-code-planning-2 and code-planning-main-1-code-planning-4 | Covered |
| 19 | Confident single-root SCIP detection generates concrete repo-specific SCIP commands. | FR-002-3d, lines 178-180 | Sibling code-planning-main-1-code-planning-2 and code-planning-main-1-code-planning-4 | Covered |
| 20 | Pairing init keeps generated indexes ignored, privately excluded, or out of accidental task diffs unless intentionally tracked. | FR-002-4, lines 181-183 | Sibling code-planning-main-1-code-planning-1 and code-planning-main-1-code-planning-4 | Covered |
| 21 | Semble truthy pairing init ensures safe physical `.sembleignore` before SessionStart advertises Semble. | FR-002-5, lines 184-186 | Sibling code-planning-main-1-code-planning-3 and code-planning-main-1-code-planning-4 | Covered |
| 22 | Pairing init must not modify `~/.liza/AGENT_TOOLS.md`. | FR-002-6, lines 187-188 | Sibling code-planning-main-1-code-planning-4 | Covered |
| 23 | Pairing init preserves existing project Git hooks and does not silently overwrite lifecycle hooks. | FR-002-7, lines 189-191 | Sibling code-planning-main-1-code-planning-1 and code-planning-main-1-code-planning-4 | Covered |
| 24 | Pairing init detects non-default `core.hooksPath` and installs safely or reports diagnostic. | FR-002-8, lines 192-195 | Sibling code-planning-main-1-code-planning-1 and code-planning-main-1-code-planning-4 | Covered |
| 25 | Docs distinguish pairing-init env gates from MAS runtime activation. | FR-002-9, lines 196-199 | Sibling code-planning-main-1-code-planning-7 | Covered |
| 26 | Pairing index activation preserves SessionStart concrete path/readiness rule. | NFR-002-1, lines 203-205 | CP6-2 for MAS separation; siblings code-planning-main-1-code-planning-4 and code-planning-main-1-code-planning-6 | Covered |
| 27 | Pairing activation does not assume Liza's own build system or target project stack. | NFR-002-2, lines 206-207 | Siblings code-planning-main-1-code-planning-1, code-planning-main-1-code-planning-2, and code-planning-main-1-code-planning-4 | Covered |
| 28 | Disabled pairing env vars activate no optional indexing hook behavior. | AC-002-1, lines 211-212 | Sibling code-planning-main-1-code-planning-4 and code-planning-main-1-code-planning-6 | Covered |
| 29 | Stacklit-enabled pairing lifecycle refresh works without editing `AGENT_TOOLS.md`. | AC-002-2, lines 213-215 | Sibling code-planning-main-1-code-planning-1, code-planning-main-1-code-planning-4, and code-planning-main-1-code-planning-6 | Covered |
| 30 | Semble-enabled pairing init creates or reports missing safety artifact before advertisement. | AC-002-3, lines 216-218 | Sibling code-planning-main-1-code-planning-3, code-planning-main-1-code-planning-4, and code-planning-main-1-code-planning-6 | Covered |
| 31 | Generated indexes do not appear as accidental untracked files after init and refresh. | AC-002-4, lines 219-221 | Sibling code-planning-main-1-code-planning-1, code-planning-main-1-code-planning-4, and code-planning-main-1-code-planning-6 | Covered |
| 32 | Single-root Go, TypeScript, or Python pairing SCIP init generates concrete commands. | AC-002-5, lines 222-225 | Sibling code-planning-main-1-code-planning-2, code-planning-main-1-code-planning-4, and code-planning-main-1-code-planning-6 | Covered |
| 33 | Monorepo ambiguous SCIP roots fail clearly instead of writing guessed hook. | AC-002-6, lines 226-228 | Sibling code-planning-main-1-code-planning-2, code-planning-main-1-code-planning-4, and code-planning-main-1-code-planning-6 | Covered |
| 34 | `--scip-search go` restricts detection to Go while still requiring confident Go root selection. | AC-002-7, lines 229-231 | Sibling code-planning-main-1-code-planning-2, code-planning-main-1-code-planning-4, and code-planning-main-1-code-planning-6 | Covered |
| 35 | Existing project Git hooks are preserved, chained, or explicitly diagnosed. | AC-002-8, lines 232-234 | Sibling code-planning-main-1-code-planning-1, code-planning-main-1-code-planning-4, and code-planning-main-1-code-planning-6 | Covered |
| 36 | Non-default `core.hooksPath` installs into effective path or reports unsafe diagnostic. | AC-002-9, lines 235-237 | Sibling code-planning-main-1-code-planning-1, code-planning-main-1-code-planning-4, and code-planning-main-1-code-planning-6 | Covered |
| 37 | Automatic Stacklit lifecycle refresh skips AI-summary. | AC-002-10, lines 238-240 | Sibling code-planning-main-1-code-planning-1 and code-planning-main-1-code-planning-6 | Covered |
| 38 | Manual `liza-index.sh ai` includes Stacklit AI-summary behavior. | AC-002-11, lines 241-242 | Sibling code-planning-main-1-code-planning-1 and code-planning-main-1-code-planning-6 | Covered |
| 39 | Docs explain pairing-init env gates separately from MAS runtime activation. | AC-002-12, lines 243-245 | Sibling code-planning-main-1-code-planning-7 | Covered |
| 40 | MAS init preserves Stacklit/Semble process-local gates and SCIP env gate plus durable config. | FR-003-1, lines 282-284 | CP6-2 | Covered |
| 41 | MAS prompts include optional tool sections only when corresponding metadata is present. | FR-003-2, lines 285-286 | CP6-1, CP6-2 | Covered |
| 42 | MAS prompt routing stays role/target specific. | FR-003-3, lines 287-289 | CP6-2 | Covered |
| 43 | MAS init behavior does not depend on pairing Git hooks or repo-root pairing indexes. | FR-003-4, lines 290-291 | CP6-2; sibling code-planning-main-1-code-planning-6 | Covered |
| 44 | Preserve or explicitly supersede current MAS SCIP runtime planning semantics. | FR-003-5, lines 292-296 | CP6-2; sibling code-planning-main-1-code-planning-2 | Covered |
| 45 | Plan documents divergence or aligns MAS if pairing SCIP ambiguity behavior is stricter. | FR-003-6, lines 297-300 | Sibling code-planning-main-1-code-planning-2 | Covered |
| 46 | Disabled SCIP env gate omits MAS SCIP prompt even with config. | AC-003-1, lines 304-305 | CP6-2 | Covered |
| 47 | Disabled Stacklit env gate omits MAS Stacklit prompt unless documented fallback exists. | AC-003-2, lines 306-308 | CP6-2 | Covered |
| 48 | Disabled Semble env gate omits MAS Semble prompt. | AC-003-3, lines 309-310 | CP6-2 | Covered |
| 49 | Enabled tool readiness/index failure omits failed MAS section while other guidance remains usable. | AC-003-4, lines 311-313 | CP6-2 | Covered |
| 50 | Pairing SCIP autodetection task states MAS inference choice. | AC-003-5, lines 314-317 | Sibling code-planning-main-1-code-planning-2 | Covered |
| 51 | Pairing SessionStart continues emitting concrete repo-root optional tool instructions only when artifacts/readiness exist. | FR-004-1, lines 345-347 | Sibling code-planning-main-1-code-planning-4 and code-planning-main-1-code-planning-6; CP6-2 preserves MAS separation | Covered |
| 52 | MAS prompts continue using structured task/reviewer/orchestrator metadata rather than pairing SessionStart assumptions. | FR-004-2, lines 348-350 | CP6-2 | Covered |
| 53 | Prompt and SessionStart guidance avoid duplicated long-form routing text when shorter shared wording is sufficient. | FR-004-3, lines 351-352 | CP6-1; sibling code-planning-main-1-code-planning-0 and code-planning-main-1-code-planning-7 | Covered |
| 54 | Generic routing text states indexes/search results are navigation aids and direct source reads remain evidence. | FR-004-4, lines 353-355 | CP6-1 | Covered |
| 55 | Pairing with no indexes and Semble disabled emits initialization guidance without optional tool blocks. | AC-004-1, lines 359-361 | Sibling code-planning-main-1-code-planning-6 | Covered |
| 56 | MAS mode with generated worktree indexes uses worktree-specific paths. | AC-004-2, lines 362-364 | CP6-2; sibling code-planning-main-1-code-planning-6 | Covered |
| 57 | Full routing guidance distinguishes conceptual discovery, module orientation, symbol navigation, exact search, syntax search, and direct reads. | AC-004-3, lines 365-368 | CP6-1 | Covered |
| 58 | Unavailable optional tools are not instructed before fallback search/read tools. | AC-004-4, lines 369-371 | CP6-1, CP6-2 | Covered |
| E2E | e2e test coverage for new behavior | Cross-cutting | Sibling code-planning-main-1-code-planning-6 owns end-to-end coverage; CP6 tasks include package-level tests only | Covered |
| DOC | Documentation updates for changed behavior | Cross-cutting | Sibling code-planning-main-1-code-planning-7 owns docs; CP6 is internal MAS prompt behavior | Covered |

## Pre-Submit Self-Check Notes

- Atomicity: CP6-1 is renderer-only; CP6-2 is metadata-only.
- Shared files: no file is written by more than one CP6 task.
- Scope exclusions: pairing SessionStart hooks, pairing init activation, global `AGENT_TOOLS.md`, and generated index locations are explicitly excluded.
- Validation commands are single-purpose, worktree-executable Go package tests.
