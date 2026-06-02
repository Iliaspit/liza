# Code Plan: Semble Operator Documentation

Task ID: `architecture-main-1-architecture-4-code-planning-0`

Source artifacts:
- Goal spec: `specs/goals/20260601-integrate-semble.md`
- Architecture reference: `specs/arch-plan/20260601-integrate-semble/20260601-134006-architecture-main-1-architecture-4.md`
- Existing documentation surface: `README.md`, `support-docs/CONFIGURATION.md`, `support-docs/CUSTOMIZING_AGENT_TOOLS.md`, `support-docs/USAGE_MULTI_AGENTS.md`, `internal/embedded/support-docs/`

## Intent

Plan the documentation-only Semble operator update so one downstream coding task updates the public setup, routing, safety, and non-goal docs plus byte-identical embedded support-doc copies.

## Source Basis

Based on:
- `specs/goals/20260601-integrate-semble.md`
- `specs/arch-plan/20260601-integrate-semble/20260601-134006-architecture-main-1-architecture-4.md`
- `README.md`
- `support-docs/CONFIGURATION.md`
- `support-docs/CUSTOMIZING_AGENT_TOOLS.md`
- `support-docs/USAGE_MULTI_AGENTS.md`
- `internal/embedded/support-docs/`
- `Makefile`
- `specs/architecture/ADR/README.md`

No assumptions are required.

Doc Impact: update `README.md`, `support-docs/CONFIGURATION.md`, `support-docs/CUSTOMIZING_AGENT_TOOLS.md`, `support-docs/USAGE_MULTI_AGENTS.md`, and synced embedded support-doc copies.

Test Impact: no production behavior tests are added by this planning task; the downstream documentation task validates with declared `rg` documentation checks plus `go test ./internal/embedded` after embedded support-doc sync.

## Planned Coding Tasks

### Task 1: Semble Operator Documentation

**desc:** Semble Operator Documentation: update README.md, support-docs/CONFIGURATION.md, support-docs/CUSTOMIZING_AGENT_TOOLS.md, support-docs/USAGE_MULTI_AGENTS.md, and synced embedded support-doc copies to document Semble activation, offline behavior, .sembleignore safety, routing, and non-goals.

**done_when:** Documentation checks prove README.md lists Semble as an optional semantic discovery tool without making it a requirement; support-docs/CONFIGURATION.md documents LIZA_ENABLE_SEMBLE, truthy/false values, init-time prewarm, possible Hugging Face/model cache contact, offline validation, HF_HUB_OFFLINE=1, the named implementation constant's default 30-second timeout for prewarm and offline validation, bounded diagnostics, Semble non-goals, .sembleignore directory scope, and default runtime/generated/credential exclusions; support-docs/CUSTOMIZING_AGENT_TOOLS.md documents Semble routing relative to Stacklit, SCIP/scip-search, Morph MCP, rg, ast-grep, and direct reads; support-docs/USAGE_MULTI_AGENTS.md points MAS operators to Semble setup; and go test ./internal/embedded passes after embedded support-doc sync.

**scope:** In scope: update README.md, support-docs/CONFIGURATION.md, support-docs/CUSTOMIZING_AGENT_TOOLS.md, support-docs/USAGE_MULTI_AGENTS.md, and synced embedded support-doc copies to document Semble activation, offline behavior, .sembleignore safety, routing, and non-goals. Out of scope: implementation code, Pairing SessionStart shell code, generated runtime state under .liza/agent-outputs/, and converting docs/ stubs into canonical content.

**spec_ref:** specs/arch-plan/20260601-integrate-semble/20260601-134006-architecture-main-1-architecture-4.md#scope-1-semble-operator-documentation

**depends_on:** none

**task_depends_on:** none

**validation:**

- rg -q 'Semble.*optional|optional.*Semble' README.md
- rg -q 'semantic discovery|semantic repository search|repository-navigation' README.md
- rg -q 'LIZA_ENABLE_SEMBLE' support-docs/CONFIGURATION.md
- rg -q 'truthy' support-docs/CONFIGURATION.md
- rg -q 'false|disabled' support-docs/CONFIGURATION.md
- rg -q 'prewarm|prewarms|prewarming' support-docs/CONFIGURATION.md
- rg -q 'Hugging Face' support-docs/CONFIGURATION.md
- rg -q 'model cache|cache' support-docs/CONFIGURATION.md
- rg -q 'offline validation|offline-readiness|offline readiness' support-docs/CONFIGURATION.md
- rg -q 'HF_HUB_OFFLINE=1' support-docs/CONFIGURATION.md
- rg -q '30 seconds|30-second' support-docs/CONFIGURATION.md
- rg -q 'bounded diagnostic|bounded diagnostics' support-docs/CONFIGURATION.md
- rg -q 'automatic install|automatically install|does not install' support-docs/CONFIGURATION.md
- rg -q 'semble init' support-docs/CONFIGURATION.md
- rg -q 'Semble MCP' support-docs/CONFIGURATION.md
- rg -q 'remote Git URL|remote.*URL' support-docs/CONFIGURATION.md
- rg -q '\.sembleignore' support-docs/CONFIGURATION.md
- rg -q 'directory-scoped|directory scope' support-docs/CONFIGURATION.md
- rg -q '\.liza/' support-docs/CONFIGURATION.md
- rg -q '\.worktrees/' support-docs/CONFIGURATION.md
- rg -q 'stacklit\.json' support-docs/CONFIGURATION.md
- rg -q '\*\.scip' support-docs/CONFIGURATION.md
- rg -q '\.env' support-docs/CONFIGURATION.md
- rg -q '\*\.pem|credential' support-docs/CONFIGURATION.md
- rg -q 'Semble' support-docs/CUSTOMIZING_AGENT_TOOLS.md
- rg -q 'Stacklit' support-docs/CUSTOMIZING_AGENT_TOOLS.md
- rg -q 'SCIP|scip-search' support-docs/CUSTOMIZING_AGENT_TOOLS.md
- rg -q 'Morph MCP' support-docs/CUSTOMIZING_AGENT_TOOLS.md
- rg -q 'rg' support-docs/CUSTOMIZING_AGENT_TOOLS.md
- rg -q 'ast-grep' support-docs/CUSTOMIZING_AGENT_TOOLS.md
- rg -q 'direct read|direct source read|exact read' support-docs/CUSTOMIZING_AGENT_TOOLS.md
- rg -q 'Semble' support-docs/USAGE_MULTI_AGENTS.md
- rg -q 'LIZA_ENABLE_SEMBLE' support-docs/USAGE_MULTI_AGENTS.md
- rg -q 'CONFIGURATION|setup|offline' support-docs/USAGE_MULTI_AGENTS.md
- go test ./internal/embedded

## Dependency Graph

Task 1 has no sibling dependencies. The documentation surface is intentionally one task because the edited files repeat one operator contract and the embedded support-doc copies must be synced in the same change.

## Shared-File Audit

| File/Area | Task(s) | Dependency |
|---|---|---|
| `README.md` | Task 1 | None |
| `support-docs/CONFIGURATION.md` | Task 1 | None |
| `support-docs/CUSTOMIZING_AGENT_TOOLS.md` | Task 1 | None |
| `support-docs/USAGE_MULTI_AGENTS.md` | Task 1 | None |
| `internal/embedded/support-docs/CONFIGURATION.md` | Task 1 | None |
| `internal/embedded/support-docs/CUSTOMIZING_AGENT_TOOLS.md` | Task 1 | None |
| `internal/embedded/support-docs/USAGE_MULTI_AGENTS.md` | Task 1 | None |
| Implementation packages and prompt templates | No task | Out of scope for Task 1 |
| `.liza/agent-outputs/` | No task | Out of scope for Task 1 |

No file appears in more than one planned task, so no sibling `depends_on` chain is required.

## Spec Compliance Matrix

Rows below cover the goal-spec and architecture-plan requirements inside the assigned documentation scope. Implementation, prompt rendering, lifecycle wiring, Pairing SessionStart shell changes, and runtime-state changes are excluded by this task's explicit scope.

| # | Requirement | Source | Task(s) | Status |
|---|-------------|--------|---------|--------|
| 1 | Document Semble as an optional Liza repository-navigation tool for semantic chunk discovery, without making it a hard dependency. | Goal spec lines 57-84, 137-145, 300-305, 548; architecture lines 37-47, 96-100 | Task 1 | Covered |
| 2 | Document `LIZA_ENABLE_SEMBLE` strict opt-in activation and truthy/false disabled value semantics. | Goal spec lines 139-147, 277-284; architecture lines 48-59, 138-143 | Task 1 | Covered |
| 3 | Document init-time model prewarm/download behavior during `liza init --spec`, including possible Hugging Face/model cache contact. | Goal spec lines 148-155, 306-313, 480-489, 506-507; architecture lines 48-59, 126-134 | Task 1 | Covered |
| 4 | Document offline validation, `HF_HUB_OFFLINE=1`, and MAS behavior that injects Semble only when offline-ready. | Goal spec lines 153-166, 312-315, 494-502, 588-591; architecture lines 48-59, 126-134, 138-143 | Task 1 | Covered |
| 5 | Document the named implementation constant's default 30-second timeout for prewarm and offline validation. | Goal spec lines 503-505; architecture lines 50-53, 140-142 | Task 1 | Covered |
| 6 | Document bounded diagnostics, non-blocking MAS spawn behavior, and distinct unavailable/offline-model failure cases without asking agents to troubleshoot Semble during unrelated work. | Goal spec lines 163-164, 261-262, 314-315, 508-514, 576-577, 607; architecture lines 138-143 | Task 1 | Covered |
| 7 | Document `.sembleignore` directory-scoped behavior, root/worktree scoping, and why `.worktrees/` root exclusion matters. | Goal spec lines 110-111, 172-187, 390-407, 411-420, 553-554; architecture lines 48-59, 136-145 | Task 1 | Covered |
| 8 | Document the default `.sembleignore` runtime, generated-index, and credential exclusion patterns, including `.liza/`, `.worktrees/`, `stacklit.json`, `*.scip`, `.env`, and credential/key patterns. | Goal spec lines 177-187, 436-469, 567-573; architecture lines 50-54, 136-145 | Task 1 | Covered |
| 9 | Document Semble routing relative to Stacklit, SCIP/scip-search, Morph MCP, `rg`, `ast-grep`, and direct reads, including that Semble results are candidates, not proof. | Goal spec lines 60-80, 188-197, 247-268, 316-317, 519-544, 551-561, 597-602; architecture lines 60-71, 102-107 | Task 1 | Covered |
| 10 | Document MAS operator setup pointers in `support-docs/USAGE_MULTI_AGENTS.md` that direct operators to Semble setup, configuration, and offline/safety details. | Goal spec lines 546-563; architecture lines 72-82 | Task 1 | Covered |
| 11 | Document Semble non-goals: no automatic install, no vendoring, no `semble init`, no Semble MCP, no remote Git URL indexing, no replacement of existing tools, and no semantic-result relevance guarantee. | Goal spec lines 81-84, 326-335, 612-628; architecture lines 48-59, 136-145 | Task 1 | Covered |
| 12 | Keep README altitude concise and link readers to configuration details rather than duplicating setup instructions. | Architecture lines 37-47, 96-100 | Task 1 | Covered |
| 13 | Sync changed support docs into `internal/embedded/support-docs/` and validate embedded support docs. | Architecture lines 83-93, 108-113, 114-124 | Task 1 | Covered |
| 14 | Keep implementation code, Pairing SessionStart shell code, generated runtime state under `.liza/agent-outputs/`, and conversion of `docs/` stubs out of scope. | Assigned scope; architecture lines 19-25, 150-155 | Task 1 | Covered |
| E2E | e2e test coverage for new behavior | Cross-cutting | N/A: this is a documentation-only task; it does not change runtime behavior. | N/A |
| DOC | Documentation updates for changed behavior | Cross-cutting | Task 1 | Covered |

## Validation Plan

The downstream coding task should:

1. Update canonical docs and sync support docs into `internal/embedded/support-docs/`.
2. Run the declared `rg` commands above from the worktree root.
3. Run `go test ./internal/embedded` from the worktree root after embedded sync.
4. Run pre-commit on touched documentation and embedded-copy files before submission.

This planning task validates by checking plan/output parity, `jq` JSON validity, `liza set-task-output`, and a clean committed worktree before review submission.

## Pre-Submit Self-Check

- Task decomposition: one coding task, one observable documentation intent.
- Falsifiability: `done_when` is validated by explicit `rg` checks and `go test ./internal/embedded`.
- Scope boundaries: no implementation code, Pairing SessionStart shell code, runtime logs, or docs-stub conversion are planned.
- Shared-file audit: no file is shared across multiple planned tasks.
- Cross-reference consistency: every Task 1 responsibility named in this plan is claimed by the Task 1 heading.
