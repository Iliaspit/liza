# Code Planning: Document Setup and Init Indexing Activation

Task: `code-planning-main-1-code-planning-7`
Agent: `code-planner-1`

## Based On

- Goal spec: `specs/goals/20260602-indexing-activation.md`, read in full.
- Master plan: `specs/plans/20260602-indexing-activation/20260602-123514-code-planning-main-1.md`, especially Interface Contracts, Task 8, Dependency Graph, and Spec Compliance Matrix.
- Prior dependency plans:
  - `code-planning-main-1-code-planning-0` for setup default optional-index guidance.
  - `code-planning-main-1-code-planning-4` for pairing init project-local activation wiring.
  - `code-planning-main-1-code-planning-5` for MAS prompt gating and context boundaries.
- Existing support-doc structure in `support-docs/CONFIGURATION.md` and `support-docs/CUSTOMIZING_AGENT_TOOLS.md`, plus synced embedded support-doc copies.
- Software architecture review skill, applied as a planning lens for boundary ownership and documentation synchronization risk.

## Planning Boundary

This plan owns only user-facing support documentation and embedded support-doc synchronization for indexing activation. It does not change runtime behavior, command wiring, hook scripts, prompt templates, setup defaults, contract files, or provider-specific startup mechanisms.

Sibling boundary references used in the compliance matrix:

| Sibling | Responsibility |
|---|---|
| `code-planning-main-1-code-planning-0` | Setup default optional-index routing guidance in `AGENT_TOOLS.md`. |
| `code-planning-main-1-code-planning-1` | Pairing hook infrastructure and Stacklit lifecycle refresh. |
| `code-planning-main-1-code-planning-2` | Pairing SCIP root planning and ambiguity diagnostics. |
| `code-planning-main-1-code-planning-3` | Pairing Semble project-root safety activation. |
| `code-planning-main-1-code-planning-4` | Pairing init orchestration for project-local index activation. |
| `code-planning-main-1-code-planning-5` | MAS prompt gating and context boundaries. |
| `code-planning-main-1-code-planning-6` | End-to-end coverage for indexing activation. |

## Architecture Notes

The documentation boundary should preserve the setup/init split from the goal: `liza setup` owns global generic tool-routing guidance, while pairing `liza init` owns project-local activation artifacts such as hooks, ignored/private generated indexes, SCIP hook command plans, and Semble safety files.

`support-docs/CONFIGURATION.md` is the right place to distinguish activation modes because it already documents `LIZA_ENABLE_STACKLIT`, `LIZA_ENABLE_SEMBLE`, and `config.scip_search`. The downstream task should update those sections so readers can tell apart:

- pairing init env gates, which install or verify project-local artifacts for pairing sessions;
- MAS runtime activation, which remains prompt/context metadata based on env gates, target-specific readiness, and durable `config.scip_search` only for SCIP;
- disabled or unavailable optional-tool behavior, which must fall back to direct reads, `rg`, `ast-grep`, and policy-exposed semantic fallback tools rather than telling agents to invoke unavailable tools.

`support-docs/CUSTOMIZING_AGENT_TOOLS.md` should be updated only if the existing MAS-specific guidance needs clearer setup/init boundaries or disabled-tool fallback wording. If updated, examples must remain generic and must not include project-specific absolute paths, generated repo paths for a particular project, or claims that optional tools are installed.

Embedded support docs must stay synchronized with `support-docs/` copies. The downstream task may update or extend embedded consistency tests only to enforce that sync; it must not use tests to accept divergent documentation behavior.

## Planned Coding Tasks

### CP7 - Document setup and init indexing activation

desc: Document setup and init indexing activation.

done_when: `support-docs/CONFIGURATION.md`, `support-docs/CUSTOMIZING_AGENT_TOOLS.md` if needed, and synced embedded support docs explain that `liza setup` owns global generic guidance while pairing `liza init` owns project-local Stacklit/SCIP/Semble activation artifacts, distinguish pairing-init env-gate behavior from MAS runtime activation, document disabled-tool fallback behavior, and keep generated/project-specific paths out of global guidance examples.

scope: Owns `support-docs/CONFIGURATION.md`, `support-docs/CUSTOMIZING_AGENT_TOOLS.md`, `internal/embedded/support-docs/CONFIGURATION.md`, `internal/embedded/support-docs/CUSTOMIZING_AGENT_TOOLS.md`, and embedded consistency tests that enforce support-doc sync. Does not change runtime behavior, command wiring, hook scripts, prompt templates, setup defaults, contract files, or provider-specific startup mechanisms.

spec_ref: specs/goals/20260602-indexing-activation.md

validation:

- `go test ./internal/embedded`

task_depends_on:

- `code-planning-main-1-code-planning-0`
- `code-planning-main-1-code-planning-4`
- `code-planning-main-1-code-planning-5`

decomposition:

```yaml
owned_files:
  - support-docs/CONFIGURATION.md
  - support-docs/CUSTOMIZING_AGENT_TOOLS.md
  - internal/embedded/support-docs/CONFIGURATION.md
  - internal/embedded/support-docs/CUSTOMIZING_AGENT_TOOLS.md
  - internal/embedded/consistency_test.go
owned_modules:
  - support-docs
  - internal/embedded support-doc sync
interfaces_owned:
  - User documentation for indexing activation
interfaces_consumed:
  - Global Tool Guidance Contract
  - Init Orchestration Contract
  - Prompt Context Contract
depends_on: []
task_depends_on:
  - code-planning-main-1-code-planning-0
  - code-planning-main-1-code-planning-4
  - code-planning-main-1-code-planning-5
coverage_notes: "Cross-cutting documentation deliverable for user-visible setup/init behavior and embedded support-doc sync. Package validation checks embedded support-doc consistency."
```

## Dependency Graph

```text
Existing code-planning-main-1-code-planning-0
Existing code-planning-main-1-code-planning-4
Existing code-planning-main-1-code-planning-5
  -> CP7
```

CP7 runs after setup-default guidance, pairing init orchestration, and MAS prompt-boundary planning because it documents the user-visible contract those tasks define. It does not depend on sibling e2e coverage; that sibling validates behavior while CP7 documents it.

## Shared File Audit

No file appears in more than one planned output task because this plan has one output task. Across sibling tasks, support-doc files and embedded support-doc copies are owned by CP7; runtime, prompt, hook, command, setup default, and e2e implementation files remain out of CP7 scope.

## Validation Plan

Canonical validation for the planned downstream task:

- `go test ./internal/embedded`

The validation proves the embedded support-doc package checks run. Downstream coders should use the command exactly unless BASH constraints require an equivalent single-purpose command from the worktree.

## Spec Compliance Matrix

| # | Requirement | Source | Task(s) | Status |
|---|-------------|--------|---------|--------|
| 1 | Generic routing guidance must be safe when Stacklit, SCIP, or Semble are disabled, missing, or not advertised. | General NFR-000-1, lines 45-46 | CP7 for docs; sibling `code-planning-main-1-code-planning-0`; sibling `code-planning-main-1-code-planning-5` | Covered |
| 2 | Project-specific paths, generated index locations, and readiness state must not be written into global `AGENT_TOOLS.md`. | General NFR-000-2, lines 47-48 | CP7 for global-guidance examples; sibling `code-planning-main-1-code-planning-0` | Covered |
| 3 | Optional indexing failures must omit unavailable prompt/context sections instead of making agents troubleshoot unrelated tooling. | General NFR-000-3, lines 49-51 | CP7 for disabled-tool fallback docs; sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 4 | `liza setup` installs generic Stacklit, SCIP, and Semble routing guidance into default `AGENT_TOOLS.md`. | FR-001-1, lines 96-97 | Sibling `code-planning-main-1-code-planning-0`; CP7 documents setup ownership | Covered |
| 5 | Guidance is conditional on explicit Stacklit path, explicit SCIP path, or supplied Semble target/readiness. | FR-001-2, lines 98-102 | Sibling `code-planning-main-1-code-planning-0`; sibling `code-planning-main-1-code-planning-5`; CP7 documents conditional usage | Covered |
| 6 | Guidance describes fallbacks for disabled or unavailable tools. | FR-001-3, lines 103-106 | CP7; sibling `code-planning-main-1-code-planning-0` | Covered |
| 7 | Guidance contains no project-specific absolute paths, generated repo locations, or installed-tool claims. | FR-001-4, lines 107-109 | CP7 for support-doc examples; sibling `code-planning-main-1-code-planning-0` | Covered |
| 8 | Installed default guidance remains concise. | NFR-001-1, lines 113-115 | Sibling `code-planning-main-1-code-planning-0`; CP7 keeps support docs from expanding global guidance | Covered |
| 9 | Fresh setup installs generic Stacklit/SCIP/Semble routing guidance. | AC-001-1, lines 119-121 | Sibling `code-planning-main-1-code-planning-0`; CP7 documents setup result | Covered |
| 10 | With optional tools disabled, guidance still routes to valid fallback tools. | AC-001-2, lines 122-124 | CP7; sibling `code-planning-main-1-code-planning-0` | Covered |
| 11 | User-customized `AGENT_TOOLS.md` keeps setup customization protection. | AC-001-3, lines 125-126 | Sibling `code-planning-main-1-code-planning-0` | Covered |
| 12 | Pairing init flows install project-local provider hooks as today. | FR-002-1, lines 156-157 | Sibling `code-planning-main-1-code-planning-4`; CP7 documents pairing init responsibility | Covered |
| 13 | Stacklit truthy pairing init installs or verifies Git hook plumbing for repo-root `stacklit.json` refresh. | FR-002-2, lines 158-160 | Sibling `code-planning-main-1-code-planning-1`; sibling `code-planning-main-1-code-planning-4`; CP7 documents activation artifact ownership | Covered |
| 14 | Automatic Git lifecycle refresh must not run `stacklit ai-summary`; manual `ai` may. | FR-002-2a, lines 161-163 | Sibling `code-planning-main-1-code-planning-1`; sibling `code-planning-main-1-code-planning-6`; CP7 documents lifecycle behavior if support docs mention refresh | Covered |
| 15 | SCIP truthy pairing init autodetects repo-specific SCIP plan and hook plumbing for enabled languages. | FR-002-3, lines 164-167 | Sibling `code-planning-main-1-code-planning-2`; sibling `code-planning-main-1-code-planning-4`; CP7 documents pairing-init SCIP activation | Covered |
| 16 | Pairing SCIP autodetection plans concrete Go, TypeScript, and Python indexer commands with root/cwd arguments. | FR-002-3a, lines 168-170 | Sibling `code-planning-main-1-code-planning-2`; CP7 documents concrete command plans without project-specific global examples | Covered |
| 17 | Repeated `--scip-search` flags restrict languages but are not sufficient root/cwd selection. | FR-002-3b, lines 171-173 | Sibling `code-planning-main-1-code-planning-2`; sibling `code-planning-main-1-code-planning-4`; CP7 documents pairing init flag behavior if support docs mention it | Covered |
| 18 | Ambiguous monorepo SCIP roots fail with candidate-root diagnostic instead of guessed hook command. | FR-002-3c, lines 174-177 | Sibling `code-planning-main-1-code-planning-2`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 19 | Confident single-root SCIP detection generates concrete repo-specific SCIP commands. | FR-002-3d, lines 178-180 | Sibling `code-planning-main-1-code-planning-2`; sibling `code-planning-main-1-code-planning-4`; CP7 documents pairing init output at a generic level | Covered |
| 20 | Pairing init keeps generated indexes ignored, privately excluded, or out of accidental task diffs unless intentionally tracked. | FR-002-4, lines 181-183 | Sibling `code-planning-main-1-code-planning-1`; sibling `code-planning-main-1-code-planning-4`; CP7 documents generated-artifact cleanliness | Covered |
| 21 | Semble truthy pairing init ensures safe physical `.sembleignore` before SessionStart advertises Semble. | FR-002-5, lines 184-186 | Sibling `code-planning-main-1-code-planning-3`; sibling `code-planning-main-1-code-planning-4`; CP7 documents Semble safety artifact | Covered |
| 22 | Pairing init must not modify `~/.liza/AGENT_TOOLS.md`. | FR-002-6, lines 187-188 | Sibling `code-planning-main-1-code-planning-4`; CP7 documents setup/init ownership split | Covered |
| 23 | Pairing init preserves existing project Git hooks and does not silently overwrite lifecycle hooks. | FR-002-7, lines 189-191 | Sibling `code-planning-main-1-code-planning-1`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 24 | Pairing init detects non-default `core.hooksPath` and installs safely or reports diagnostic. | FR-002-8, lines 192-195 | Sibling `code-planning-main-1-code-planning-1`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 25 | Docs distinguish pairing-init env gates from MAS runtime activation. | FR-002-9, lines 196-199 | CP7 | Covered |
| 26 | Pairing index activation preserves SessionStart concrete path/readiness rule. | NFR-002-1, lines 203-205 | Sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-5`; CP7 documents readiness-specific guidance | Covered |
| 27 | Pairing activation does not assume Liza's own build system or target project stack. | NFR-002-2, lines 206-207 | Sibling `code-planning-main-1-code-planning-1`; sibling `code-planning-main-1-code-planning-2`; sibling `code-planning-main-1-code-planning-4`; CP7 avoids Liza-specific global examples | Covered |
| 28 | Disabled pairing env vars activate no optional indexing hook behavior. | AC-002-1, lines 211-212 | Sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6`; CP7 documents disabled gates | Covered |
| 29 | Stacklit-enabled pairing lifecycle refresh works without editing `AGENT_TOOLS.md`. | AC-002-2, lines 213-215 | Sibling `code-planning-main-1-code-planning-1`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6`; CP7 documents setup/init split | Covered |
| 30 | Semble-enabled pairing init creates or reports missing safety artifact before advertisement. | AC-002-3, lines 216-218 | Sibling `code-planning-main-1-code-planning-3`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6`; CP7 documents Semble safety | Covered |
| 31 | Generated indexes do not appear as accidental untracked files after init and refresh. | AC-002-4, lines 219-221 | Sibling `code-planning-main-1-code-planning-1`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6`; CP7 documents generated artifact cleanliness | Covered |
| 32 | Single-root Go, TypeScript, or Python pairing SCIP init generates concrete commands. | AC-002-5, lines 222-225 | Sibling `code-planning-main-1-code-planning-2`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 33 | Monorepo ambiguous SCIP roots fail clearly instead of writing guessed hook. | AC-002-6, lines 226-228 | Sibling `code-planning-main-1-code-planning-2`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 34 | `--scip-search go` restricts detection to Go while still requiring confident Go root selection. | AC-002-7, lines 229-231 | Sibling `code-planning-main-1-code-planning-2`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6`; CP7 documents flag behavior if included | Covered |
| 35 | Existing project Git hooks are preserved, chained, or explicitly diagnosed. | AC-002-8, lines 232-234 | Sibling `code-planning-main-1-code-planning-1`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 36 | Non-default `core.hooksPath` installs into effective path or reports unsafe diagnostic. | AC-002-9, lines 235-237 | Sibling `code-planning-main-1-code-planning-1`; sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6` | Covered |
| 37 | Automatic Stacklit lifecycle refresh skips AI-summary. | AC-002-10, lines 238-240 | Sibling `code-planning-main-1-code-planning-1`; sibling `code-planning-main-1-code-planning-6`; CP7 documents if support docs mention refresh commands | Covered |
| 38 | Manual `liza-index.sh ai` includes Stacklit AI-summary behavior. | AC-002-11, lines 241-242 | Sibling `code-planning-main-1-code-planning-1`; sibling `code-planning-main-1-code-planning-6`; CP7 documents if support docs mention manual refresh | Covered |
| 39 | Docs explain pairing-init env gates separately from MAS runtime activation. | AC-002-12, lines 243-245 | CP7 | Covered |
| 40 | MAS init preserves Stacklit/Semble process-local gates and SCIP env gate plus durable config. | FR-003-1, lines 282-284 | Sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6`; CP7 documents MAS runtime activation | Covered |
| 41 | MAS prompts include optional tool sections only when corresponding metadata is present. | FR-003-2, lines 285-286 | Sibling `code-planning-main-1-code-planning-5`; CP7 documents metadata-driven availability | Covered |
| 42 | MAS prompt routing stays role/target specific. | FR-003-3, lines 287-289 | Sibling `code-planning-main-1-code-planning-5`; CP7 documents target-specific MAS context if support docs mention MAS prompts | Covered |
| 43 | MAS init behavior does not depend on pairing Git hooks or repo-root pairing indexes. | FR-003-4, lines 290-291 | Sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6`; CP7 documents separation from pairing init | Covered |
| 44 | Preserve or explicitly supersede current MAS SCIP runtime planning semantics. | FR-003-5, lines 292-296 | Sibling `code-planning-main-1-code-planning-2`; sibling `code-planning-main-1-code-planning-5`; CP7 documents current MAS SCIP config semantics | Covered |
| 45 | Plan documents divergence or aligns MAS if pairing SCIP ambiguity behavior is stricter. | FR-003-6, lines 297-300 | Sibling `code-planning-main-1-code-planning-2`; CP7 documents behavior only after sibling choice | Covered |
| 46 | Disabled SCIP env gate omits MAS SCIP prompt even with config. | AC-003-1, lines 304-305 | Sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6`; CP7 documents env-gate precedence | Covered |
| 47 | Disabled Stacklit env gate omits MAS Stacklit prompt unless documented fallback exists. | AC-003-2, lines 306-308 | Sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6`; CP7 documents disabled behavior | Covered |
| 48 | Disabled Semble env gate omits MAS Semble prompt. | AC-003-3, lines 309-310 | Sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6`; CP7 documents disabled behavior | Covered |
| 49 | Enabled tool readiness/index failure omits failed MAS section while other guidance remains usable. | AC-003-4, lines 311-313 | Sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6`; CP7 documents graceful degradation | Covered |
| 50 | Pairing SCIP autodetection task states MAS inference choice. | AC-003-5, lines 314-317 | Sibling `code-planning-main-1-code-planning-2`; CP7 documents resulting divergence only if needed | Covered |
| 51 | Pairing SessionStart continues emitting concrete repo-root optional tool instructions only when artifacts/readiness exist. | FR-004-1, lines 345-347 | Sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6`; CP7 documents readiness-based SessionStart guidance | Covered |
| 52 | MAS prompts continue using structured task/reviewer/orchestrator metadata rather than pairing SessionStart assumptions. | FR-004-2, lines 348-350 | Sibling `code-planning-main-1-code-planning-5`; CP7 documents MAS/pairing distinction | Covered |
| 53 | Prompt and SessionStart guidance avoid duplicated long-form routing text when shorter shared wording is sufficient. | FR-004-3, lines 351-352 | CP7; sibling `code-planning-main-1-code-planning-0`; sibling `code-planning-main-1-code-planning-5` | Covered |
| 54 | Generic routing text states indexes/search results are navigation aids and direct source reads remain evidence. | FR-004-4, lines 353-355 | CP7; sibling `code-planning-main-1-code-planning-0`; sibling `code-planning-main-1-code-planning-5` | Covered |
| 55 | Pairing with no indexes and Semble disabled emits initialization guidance without optional tool blocks. | AC-004-1, lines 359-361 | Sibling `code-planning-main-1-code-planning-4`; sibling `code-planning-main-1-code-planning-6`; CP7 documents absent-section behavior | Covered |
| 56 | MAS mode with generated worktree indexes uses worktree-specific paths. | AC-004-2, lines 362-364 | Sibling `code-planning-main-1-code-planning-5`; sibling `code-planning-main-1-code-planning-6`; CP7 documents target-specific paths without global examples | Covered |
| 57 | Full routing guidance distinguishes conceptual discovery, module orientation, symbol navigation, exact search, syntax search, and direct reads. | AC-004-3, lines 365-368 | CP7; sibling `code-planning-main-1-code-planning-0`; sibling `code-planning-main-1-code-planning-5` | Covered |
| 58 | Unavailable optional tools are not instructed before fallback search/read tools. | AC-004-4, lines 369-371 | CP7; sibling `code-planning-main-1-code-planning-0`; sibling `code-planning-main-1-code-planning-5` | Covered |
| E2E | e2e test coverage for new behavior | Cross-cutting | Sibling `code-planning-main-1-code-planning-6`: cross-cutting end-to-end coverage for setup/init/prompt behavior | Covered |
| DOC | Documentation updates for changed behavior | Cross-cutting | CP7: user-facing support docs and embedded support-doc sync | Covered |

## Pre-Submit Self-Check Notes

- Atomicity: CP7 has one observable intent: document the setup/init/MAS optional-index activation contract and keep embedded support docs synchronized.
- Falsifiability: CP7 can be falsified by reading the named support docs for setup/init ownership, pairing-vs-MAS env-gate distinctions, disabled-tool fallbacks, and absence of project-specific global guidance examples, then running `go test ./internal/embedded`.
- Output parity: the structured output entry must copy CP7 `desc`, `done_when`, `scope`, `spec_ref`, and validation command character-for-character from this plan.
- Shared files: no shared files across outputs in this plan because there is one output task.
- Cross-references: every sibling referenced above is declared in the boundary table and appears in the parent plan's task graph.
- Scope protection: implementation documentation files named in CP7 are downstream child-task scope, not modified by this code-planning task.
