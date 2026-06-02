# Code Plan: MAS Semble Prompt Guidance

Task: `architecture-main-1-architecture-3-code-planning-0`
Status: draft

## Based On

- Assigned task JSON from `liza get architecture-main-1-architecture-3-code-planning-0 --json`.
- Goal spec: `specs/goals/20260601-integrate-semble.md`.
- Architecture reference: `specs/arch-plan/20260601-integrate-semble/20260601-144312-architecture-main-1-architecture-3.md`.
- Dependency outputs and concrete implementation task reads:
  - `architecture-main-1-architecture-0-code-planning-0-coding-2`
  - `architecture-main-1-architecture-2-code-planning-0-coding-1`
  - `architecture-main-1-architecture-2-code-planning-1-coding-0`
  - `architecture-main-1-architecture-2-code-planning-1-coding-1`
  - `architecture-main-1-architecture-2-code-planning-1-coding-2`
- Prior rejection feedback requiring implementation-level dependency edges for the consumed worktree safety contract.
- Stacklit orientation for the task worktree.

## Planning Decision

Create one downstream coding task. The assigned scope is a single prompt-guidance behavior change: MAS prompt assembly should render Semble guidance only when `internal/semble` returns a prompt-safe context for the exact role target root, while preserving existing tool-routing sections. Splitting prompt metadata, template rendering, and role target selection would force multiple tasks to edit the same prompt files and tests, increasing merge risk without separating independently shippable behavior.

This revision addresses the prior scheduling blocker by depending on the concrete implementation tasks that provide the consumed Semble prompt metadata API, shared worktree private exclude behavior, task worktree lifecycle preparation, and reviewer worktree recovery preparation. It intentionally does not depend on the merged planning-task IDs as substitutes for those implementation outputs.

## Task Graph

| Task | Intent | Sibling Dependencies | External Dependencies |
|------|--------|----------------------|-----------------------|
| Task 1 | Render and route safe MAS Semble prompt guidance for task, reviewer, and orchestrator prompts. | None | `architecture-main-1-architecture-0-code-planning-0-coding-2`, `architecture-main-1-architecture-2-code-planning-0-coding-1`, `architecture-main-1-architecture-2-code-planning-1-coding-0`, `architecture-main-1-architecture-2-code-planning-1-coding-1`, `architecture-main-1-architecture-2-code-planning-1-coding-2` |

## Task 1: MAS Semble Prompt Guidance

**Desc:** MAS Semble Prompt Guidance: extend prompt metadata and rendering so MAS prompts include a concise `=== SEMBLE SEARCH ===` section only when `internal/semble` reports enabled, offline-ready, and target-safe context for the exact role target root; wire task, reviewer, and safe orchestrator project-root target selection; preserve Stacklit/SCIP sections; update unified query routing to position Semble for conceptual discovery, content modes, Morph fallback, direct-read verification, and `rg`/`ast-grep` boundaries. Out of scope: init prewarm implementation, worktree `.sembleignore` generation internals, Pairing SessionStart shell hook changes, operator documentation, and Semble MCP.

**Done_when:** `go test ./internal/prompts ./internal/agent` proves Semble prompt sections are omitted when disabled, unavailable, offline-unready, or target-unsafe; task and reviewer prompts use shell-quoted absolute worktree roots and never parent project roots; orchestrator prompts use the project root only when root `.sembleignore` safety passes; rendered commands include `HF_HUB_OFFLINE=1`, `semble search`, `semble find-related`, and the one-line content-mode guidance; query routing preserves Stacklit/SCIP guidance and positions Morph as fallback only when Semble is unavailable; prompt text states Semble returns candidates, not proof; reviewer prompt assembly consumes the worktree safety contract without modifying reviewer recovery; and Pairing SessionStart tests remain unaffected.

**Scope:** In scope: extend `internal/prompts/builder.go`, `internal/prompts/templates/base_prompt.tmpl`, focused `internal/prompts` tests, `internal/agent/prompt.go`, focused `internal/agent` tests, and only the narrow role-strategy call sites needed to pass Semble prompt-safe metadata into existing base prompt construction; consume `internal/semble` prompt metadata, readiness, and target-safety contracts; choose the absolute task worktree root for task prompts, the reviewer candidate worktree root for reviewer prompts, and the project root for orchestrator prompts only when `internal/semble` reports project-root safety; preserve Stacklit/SCIP sections and update unified query routing text for Semble conceptual discovery, `--content` modes, Morph fallback, direct-read verification, and `rg`/`ast-grep` boundaries. Out of scope: editing `internal/semble` except for compile fixes to consume already-planned exported APIs, editing reviewer worktree recovery internals, changing Pairing SessionStart shell hooks or tests, implementing init prewarm, generating or hiding `.sembleignore` files, operator documentation, Semble MCP, remote Git URL guidance, automatic Semble/Python/model installation, durable `state.yaml` config, and `.liza/agent-outputs/`.

**Spec_ref:** specs/arch-plan/20260601-integrate-semble/20260601-144312-architecture-main-1-architecture-3.md#scope-1-mas-semble-prompt-guidance

**Validation:** go test ./internal/prompts ./internal/agent

**Task_depends_on:** `architecture-main-1-architecture-0-code-planning-0-coding-2`, `architecture-main-1-architecture-2-code-planning-0-coding-1`, `architecture-main-1-architecture-2-code-planning-1-coding-0`, `architecture-main-1-architecture-2-code-planning-1-coding-1`, `architecture-main-1-architecture-2-code-planning-1-coding-2`

## Dependency Rationale

- `architecture-main-1-architecture-0-code-planning-0-coding-2` provides `internal/semble` target-safety and prompt metadata contracts used by Task 1.
- `architecture-main-1-architecture-2-code-planning-0-coding-1` provides the shared private exclude behavior consumed by Semble worktree preparation and avoids depending only on high-level planning artifacts.
- `architecture-main-1-architecture-2-code-planning-1-coding-0` provides the direct Semble task-worktree ignore preparation helper used by lifecycle and reviewer wiring.
- `architecture-main-1-architecture-2-code-planning-1-coding-1` provides `CreateWorktree` and `ClaimTask` lifecycle preparation before task prompts can safely advertise task-root Semble guidance.
- `architecture-main-1-architecture-2-code-planning-1-coding-2` provides reviewer worktree recovery preparation before reviewer prompts can safely advertise candidate-root Semble guidance.

## Shared-File Audit

| File/Area | Task(s) | Dependency Handling |
|-----------|---------|---------------------|
| `internal/prompts/builder.go` | Task 1 | Single downstream owner. |
| `internal/prompts/templates/base_prompt.tmpl` | Task 1 | Single downstream owner. |
| `internal/prompts/builder_test.go` and focused prompt tests | Task 1 | Single downstream owner; coder should read targeted regions because this test file is large. |
| `internal/agent/prompt.go` | Task 1 | Single downstream owner. |
| `internal/agent/prompt_test.go` and focused agent tests | Task 1 | Single downstream owner; coder should read targeted regions because this test file is large. |
| `internal/agent/strategy_orchestrator.go` | Task 1 | Narrow call-site updates only if existing prompt construction needs Semble metadata. |
| `internal/agent/strategy_reviewer.go` | Task 1 | Narrow call-site updates only; reviewer recovery internals remain out of scope. |
| `internal/agent/worktree_check.go` | None | Owned by dependency `architecture-main-1-architecture-2-code-planning-1-coding-2`; Task 1 must not modify reviewer recovery behavior. |
| `internal/semble/*` | None by default | Owned by dependency `architecture-main-1-architecture-0-code-planning-0-coding-2`; Task 1 consumes exported contracts and may only make compile fixes if the merged API requires them. |
| `internal/worktreeexclude/*` and `internal/scipsearch/*` | None | Owned by dependency `architecture-main-1-architecture-2-code-planning-0-coding-1`; Task 1 must not modify private-exclude or SCIP internals. |
| `internal/ops/semble_indexing.go` and task lifecycle files | None | Owned by dependencies `architecture-main-1-architecture-2-code-planning-1-coding-0` and `architecture-main-1-architecture-2-code-planning-1-coding-1`; Task 1 consumes prepared roots only. |

No file appears in more than one planned downstream task, so no sibling `depends_on` edge is needed.

## Validation Plan

- Validate JSON syntax for the structured output file with `jq`.
- Re-read this plan and verify the structured output `desc`, `done_when`, `scope`, `spec_ref`, `plan_ref`, and `validation` fields are copied verbatim.
- Verify `task_depends_on` contains concrete implementation tasks for `internal/semble` prompt metadata, shared private excludes, direct Semble ignore preparation, task worktree lifecycle wiring, and reviewer worktree recovery wiring.
- Run `go test ./internal/prompts ./internal/agent` from the task worktree as the canonical validation command for the assigned scope.
- Run pre-commit on the two planning artifacts.
- Run `liza set-task-output architecture-main-1-architecture-3-code-planning-0 --output <output-json> --agent-id code-planner-1 --json`.
- Stage only the two planning artifacts, commit them, verify `git status --short` is clean, and submit `HEAD` for review.

## Spec Compliance Matrix

| # | Requirement | Source | Task(s) | Status |
|---|-------------|--------|---------|--------|
| 1 | Semble prompt guidance is strict opt-in and omitted when disabled, unavailable, offline-unready, or target-unsafe. | Goal spec lines 144-164, 211-262, 588-591; architecture lines 42-43, 151-155 | Task 1 | Covered |
| 2 | MAS prompt guidance targets exactly one role-appropriate root: task worktree for task agents, reviewer candidate worktree for reviewers, project root for orchestrators only when root safety passes. | Goal spec lines 238-245, 318-325, 592-596; architecture lines 44-47, 93-130 | Task 1 | Covered |
| 3 | Task and reviewer prompts must never substitute or mention the parent project root as the Semble target. | Goal spec lines 238-245, 318-325; architecture lines 44-45, 115-117, 173-179 | Task 1 | Covered |
| 4 | Rendered Semble commands include `HF_HUB_OFFLINE=1`, `semble search`, `semble find-related`, and shell-quoted absolute target roots. | Goal spec lines 165-166, 222-227, 312-313; architecture lines 47, 91, 167-172 | Task 1 | Covered |
| 5 | Prompt content-mode guidance stays to one line and covers `code`, `docs`, `config`, and `all`, with `code` as default. | Goal spec lines 264-268, 639-640; architecture lines 88-90, 260-263 | Task 1 | Covered |
| 6 | Semble is positioned as candidate discovery, not proof, and direct file reads remain required before editing or evidence claims. | Goal spec lines 228-235, 316-317, 537-538, 597-598, 625-626; architecture lines 48, 89-90, 221-225 | Task 1 | Covered |
| 7 | Stacklit and SCIP prompt guidance remain first-class and additive, with distinct routing for module orientation and exact symbol/reference tracing. | Goal spec lines 69-72, 190-191, 247-258, 529-531, 599; architecture lines 49, 87-89, 203-213, 265 | Task 1 | Covered |
| 8 | Morph MCP semantic/codebase search is only a fallback when Semble is unavailable and current tool/MCP policy exposes Morph. | Goal spec lines 73-75, 167-171, 256-257, 534, 600-601; architecture lines 50, 203-213, 266 | Task 1 | Covered |
| 9 | `rg` is retained for exhaustive literal/exact search and not routed as broad conceptual fallback; `ast-grep` remains for syntax-shaped search. | Goal spec lines 76-79, 192-195, 530-544, 602; architecture lines 51, 203-213, 267-268 | Task 1 | Covered |
| 10 | MAS receives Semble through spawned prompts, not Pairing SessionStart repo-root guidance; Pairing SessionStart tests remain unaffected. | Goal spec lines 198-202, 341-384, 603-606; architecture lines 43, 132-143, 269-270 | Task 1 | Covered |
| 11 | Agent prompt assembly consumes `internal/semble` readiness, prompt metadata, and target-safety APIs; `internal/prompts` renders already-authorized metadata and must not call `internal/semble` directly. | Architecture lines 66-78, 149-169 | Task 1 | Covered |
| 12 | Reviewer prompt assembly consumes the worktree safety contract without modifying reviewer recovery behavior. | Architecture lines 106-117, 173-179, 284-285; dependency tasks `architecture-main-1-architecture-2-code-planning-1-coding-0`, `architecture-main-1-architecture-2-code-planning-1-coding-2` | Task 1 | Covered |
| 13 | Orchestrator prompt assembly does not pre-index or refresh Semble; it only renders project-root guidance when `internal/semble` reports root safety. | Architecture lines 119-130, 197-201 | Task 1 | Covered |
| 14 | Semble readiness or safety failures are non-blocking for agent spawn and should result in omission, with only bounded diagnostics where existing seams support them. | Goal spec lines 163-164, 205, 607; architecture lines 101-105, 217-227 | Task 1 | Covered |
| 15 | The downstream validation command proves omission states, role target selection, command rendering, routing text, Stacklit/SCIP preservation, reviewer recovery non-modification, and Pairing SessionStart non-regression. | Assigned task done_when; architecture lines 132-143, 245, 288-294 | Task 1 | Covered |
| 16 | Prompt-guidance implementation must wait for concrete implementations of Semble prompt metadata, worktree private excludes, task lifecycle `.sembleignore` preparation, and reviewer recovery `.sembleignore` preparation. | Prior rejection feedback; dependency output summaries for `architecture-main-1-architecture-0-code-planning-0`, `architecture-main-1-architecture-2-code-planning-0`, and `architecture-main-1-architecture-2-code-planning-1` | Task 1 | Covered |
| E2E | e2e test coverage for new behavior | Cross-cutting | N/A: assigned acceptance is package-level prompt rendering and prompt assembly validation through `go test ./internal/prompts ./internal/agent`; no separate end-to-end layer is required for this internal spawned-prompt text change. | N/A |
| DOC | Documentation updates for changed behavior | Cross-cutting | N/A: operator documentation is explicitly out of this scope and covered by sibling `architecture-main-1-architecture-4-code-planning-0`; Task 1 updates only runtime prompt text and tests. | N/A |

## Pre-Submit Self-Check

- Task decomposition: one cohesive behavior task; no sibling overlap.
- Falsifiability: Task 1 `done_when` names concrete prompt outputs, omission states, role roots, and validation command.
- Output parity: structured output must copy Task 1 fields verbatim.
- Dependency parity: structured output must use the concrete implementation `task_depends_on` list from Task 1, not merged planning-task IDs.
- Shared-file audit: no shared file across multiple planned tasks.
- Scope boundaries: no init prewarm, `.sembleignore` generation internals, SessionStart hook changes, operator docs, Semble MCP, or `.liza/agent-outputs/`.
