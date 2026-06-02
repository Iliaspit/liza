# Architecture Plan: MAS Semble Prompt Guidance

Status: draft

## Goal

Define how MAS prompt assembly consumes Semble readiness and target-safety contracts so spawned prompts render concise Semble search guidance only for the exact safe target root.

## Context

The Semble goal adds optional semantic chunk discovery to Liza without replacing Stacklit, SCIP, `rg`, `ast-grep`, or direct reads. This architecture task narrows the master prompt-guidance scope to the MAS prompt path: prompt metadata, base prompt rendering, role-specific target-root selection, and regression coverage for task, reviewer, and orchestrator prompts.

Upstream architecture separates the Semble capability package from worktree safety. This plan consumes those contracts instead of redefining them. `internal/semble` owns activation, offline readiness, target safety, prompt-safe command metadata, shell-quoted examples, and default ignore rules. Worktree-safety planning owns generated task/reviewer `.sembleignore` preparation before prompts can target those roots. This scope wires those results into existing prompt assembly only.

### References

- Goal spec: `specs/goals/20260601-integrate-semble.md`
- Parent plan: `specs/arch-plan/20260601-integrate-semble/20260601-130216-architecture-main-1.md`
- Dependency plan: `specs/arch-plan/20260601-integrate-semble/20260601-133505-architecture-main-1-architecture-0.md`
- Dependency plan: `specs/arch-plan/20260601-integrate-semble/20260601-135025-architecture-main-1-architecture-2.md`
- Task: `architecture-main-1-architecture-3`
- Codebase:
  - `internal/prompts/builder.go`
  - `internal/prompts/templates/base_prompt.tmpl`
  - `internal/prompts/role_context.go`
  - `internal/prompts/builder_test.go`
  - `internal/agent/prompt.go`
  - `internal/agent/prompt_test.go`
  - `internal/agent/strategy_orchestrator.go`
  - `internal/agent/strategy_reviewer.go`
  - `internal/agent/worktree_check.go`
  - `internal/agent/strategy_orchestrator_scip_test.go`
  - `specs/architecture/ADR/README.md`
  - `GUARDRAILS.md`
  - `lessons/agents/worktree-file-path-consistency.md`
  - `lessons/agents/worktree-path-construction.md`
  - `lessons/agents/large-test-file-reads.md`
  - `lessons/agents/worktree-build-prerequisites.md`

### Constraints

- Semble prompt guidance is strict opt-in and must be omitted when Semble is disabled, unavailable, offline-unready, or target-unsafe.
- MAS Semble context belongs in spawned prompts, not Pairing SessionStart. Pairing SessionStart shell-hook changes are out of scope and must remain unaffected.
- Task and planning/doer roles must target the absolute task worktree root, never the parent project root.
- Reviewer roles must target the reviewer candidate worktree root after reviewer worktree recovery has established the candidate root. This scope consumes the worktree safety contract and does not modify reviewer recovery behavior.
- Orchestrator prompts may target `ProjectRoot` only when `internal/semble` reports project-root safety, including root `.sembleignore` coverage for `.worktrees/`, `.liza/`, generated index files, and credential patterns.
- Semble prompt commands must include `HF_HUB_OFFLINE=1`, `semble search`, `semble find-related`, and shell-quoted absolute target roots.
- Semble guidance must state that Semble returns candidate chunks, not proof, and require direct file reads for evidence.
- Existing Stacklit and SCIP prompt sections remain first-class and additive. The unified routing section must preserve their distinct roles.
- Morph MCP semantic/codebase search is positioned only as fallback when Semble is unavailable and current tool/MCP policy exposes Morph.
- `rg` remains for exhaustive literal/exact matching and must not be routed as the broad conceptual fallback.
- `.pre-commit-config.yaml` exists at worktree `HEAD`; no `bootstrap-precommit` output entry is emitted.

### Assumptions

- None.

### Open Questions

- None for this scope.

---

## Components

### Prompt Metadata (`internal/prompts/builder.go`)

**Responsibility:** Carry and normalize prompt-safe Semble search metadata beside existing SCIP and Stacklit prompt metadata.

**Boundaries:**
- Exposes: a Semble metadata field on `BasePromptConfig`, normalized template data, shell-quoted command fields, and omission behavior when metadata is empty.
- Depends on: prompt-safe Semble context from `internal/semble` and existing prompt shell-quoting conventions.

**Key decisions:**
- Add Semble to `BasePromptConfig` rather than role templates because Semble is cross-role tool guidance like Stacklit and SCIP.
- Normalize Semble data in builder code before template execution so the template remains declarative and can omit the whole section when context is absent.
- Do not have `internal/prompts` call `internal/semble`; prompt rendering consumes already-authorized metadata from agent prompt assembly.

### Base Prompt Template (`internal/prompts/templates/base_prompt.tmpl`)

**Responsibility:** Render the concise `=== SEMBLE SEARCH ===` section and update unified query routing only when Semble metadata is present.

**Boundaries:**
- Exposes: rendered Semble prompt text and routing guidance for spawned MAS agents.
- Depends on: normalized prompt metadata containing shell-quoted absolute target root and command examples.

**Key decisions:**
- Place the Semble section after Stacklit/SCIP availability sections and before unified query routing, preserving existing sections and making routing additive.
- Keep prompt text concise by using the goal-spec one-line content-mode guidance: `Use --content with one of: code, docs, config, all; code is the default.`
- Render the proof boundary explicitly: Semble returns candidates; direct reads verify source-of-truth behavior.
- Include `HF_HUB_OFFLINE=1` in every rendered command example so unattended MAS prompts do not suggest model downloads.

### Task Prompt Assembly (`internal/agent/prompt.go`)

**Responsibility:** Resolve the task-agent Semble target root and pass Semble metadata into the base prompt.

**Boundaries:**
- Exposes: task-role prompt configuration with optional Semble metadata.
- Depends on: task `Worktree`, existing Stacklit/SCIP availability collection, and `internal/semble` readiness plus target-safety API.

**Key decisions:**
- Use the same absolute `data.Worktree` root already used for task Stacklit/SCIP lookup as the Semble task target root.
- Call Semble readiness/target-safety only after `data.Worktree` is resolved, and treat empty context as normal prompt omission.
- Do not make Semble failures fatal to prompt creation; bounded diagnostics may be logged by assembly code, but prompt guidance is omitted.

### Reviewer Prompt Assembly (`internal/agent/prompt.go`, `internal/agent/strategy_reviewer.go`)

**Responsibility:** Pass reviewer candidate worktree Semble metadata into reviewer prompts without changing reviewer recovery semantics.

**Boundaries:**
- Exposes: reviewer-role prompt configuration with optional Semble metadata for the candidate root.
- Depends on: the reviewer worktree root used by `ensureReviewerWorktree`, the worktree safety contract planned by `architecture-main-1-architecture-2`, and `internal/semble` prompt-safe context.

**Key decisions:**
- Use the review task's absolute worktree root after reviewer claim/recovery as the Semble target. Do not derive or mention parent project roots.
- Keep `ensureReviewerWorktree` behavior owned by the worktree-safety scope; this prompt scope only consumes the guarantee that recovered worktrees are prepared before prompt assembly can advertise Semble.
- Cover reviewer prompts with tests that prove worktree-root targeting and omission for unsafe or unavailable contexts.

### Orchestrator Prompt Assembly (`internal/agent/strategy_orchestrator.go`, `internal/agent/prompt.go`)

**Responsibility:** Add project-root Semble guidance to orchestrator prompts only when the root is safe.

**Boundaries:**
- Exposes: orchestrator base prompt config with optional Semble metadata.
- Depends on: `config.ProjectRoot`, root `.sembleignore` safety checks from `internal/semble`, and existing orchestrator Stacklit/SCIP project-root refresh flow.

**Key decisions:**
- Do not pre-index or refresh Semble from orchestrator `PreExecution`; Semble remains CLI-on-demand guidance gated by offline readiness.
- Reuse the same prompt-metadata path as task/reviewer prompts, with `TargetKindProjectRoot` so `internal/semble` enforces root `.sembleignore` safety.
- Omit Semble when root safety fails, even if Stacklit/SCIP project-root indexes are available.

### Prompt Tests (`internal/prompts/*_test.go`, `internal/agent/*_test.go`)

**Responsibility:** Prove rendering, target-root isolation, omission paths, and routing text without depending on a real Semble install or model cache.

**Boundaries:**
- Exposes: focused unit tests in `internal/prompts` and `internal/agent`.
- Depends on: fakeable Semble readiness/context seams, temporary worktree paths, and existing prompt-rendering helpers.

**Key decisions:**
- Builder tests cover raw rendering: omitted empty context, command text, content-mode one-liner, candidate-not-proof wording, and Stacklit/SCIP routing preservation.
- Agent tests cover role target selection: task and reviewer worktree roots, project-root-only orchestrator safety, and no parent-root leakage into task/reviewer prompts.
- Pairing SessionStart tests are not edited in this scope; package validation includes `internal/agent` and `internal/prompts` to catch prompt assembly regressions.

---

## Interfaces

### Agent Prompt Assembly -> Semble Capability

**Contract:** Agent prompt assembly passes exactly one absolute target root plus a target kind to `internal/semble`. The Semble package returns prompt-safe metadata only when enabled, executable-ready, offline-ready, and target-safe. Otherwise it returns empty context and bounded diagnostics suitable for logs.

**Direction:** `internal/agent` calls `internal/semble`; `internal/semble` does not know agent roles or prompt templates.

**Invariants:** Disabled or unready Semble never yields rendered command guidance. Task/reviewer callers never substitute `ProjectRoot` for a task/reviewer worktree root.

### Agent Prompt Assembly -> Prompt Builder

**Contract:** `baseConfigFrom` accepts Semble prompt metadata beside SCIP and Stacklit index refs and passes it through to `prompts.BuildBasePrompt`.

**Direction:** `internal/agent` constructs `prompts.BasePromptConfig`; `internal/prompts` renders the base prompt.

**Invariants:** Base prompt generation remains additive: absent Semble metadata omits only the Semble section and does not suppress SCIP or Stacklit sections.

### Prompt Builder -> Base Prompt Template

**Contract:** Builder-normalized Semble template data contains the display target root, shell-quoted absolute target root, offline environment prefix, example commands, content-mode guidance, and discovery-not-proof wording needed by the template.

**Direction:** `internal/prompts/builder.go` prepares template data; `base_prompt.tmpl` renders text.

**Invariants:** Rendered examples include `HF_HUB_OFFLINE=1`, `semble search`, `semble find-related`, and the same shell-quoted absolute target root in each target-root command.

### Worktree Safety -> Reviewer Prompt Assembly

**Contract:** Reviewer claim/recovery provides a candidate worktree path that prompt assembly can pass to Semble target-safety checks. Worktree safety preparation is owned by the upstream worktree scope; prompt assembly consumes the result rather than rewriting recovery.

**Direction:** Reviewer strategy ensures/reuses worktree before prompt build; prompt assembly evaluates Semble context for that root.

**Invariants:** Prompt assembly does not hide missing safety by falling back to project root. Unsafe reviewer roots omit Semble guidance.

---

## Data Flow

Task prompt:

```text
Task state -> resolve task Worktree -> Semble prompt-context check for task worktree -> BasePromptConfig.SembleSearch -> base_prompt.tmpl renders section or omits it -> agent uses Semble for conceptual candidates and direct reads for proof
```

Reviewer prompt:

```text
Reviewer claim -> ensure/reuse reviewer candidate worktree -> resolve review task Worktree -> Semble prompt-context check for reviewer worktree -> BasePromptConfig.SembleSearch -> reviewer prompt renders safe candidate-root guidance or omits Semble
```

Orchestrator prompt:

```text
Orchestrator PreExecution refreshes project-root SCIP/Stacklit only -> build orchestrator context -> Semble prompt-context check for project root and root .sembleignore safety -> BasePromptConfig.SembleSearch -> orchestrator prompt renders project-root guidance only when safe
```

Unified routing:

```text
Available tool metadata -> base prompt sections -> QUERY ROUTING
  -> Semble for broad conceptual discovery and docs/config content modes
  -> Stacklit for module/dependency orientation
  -> scip-search for exact symbol/reference/implementation tracing
  -> Morph MCP semantic fallback only when Semble is unavailable and policy exposes Morph
  -> rg for literals and ast-grep for syntax-shaped search
  -> direct reads for proof
```

---

## Cross-Cutting Concerns

| Concern | Approach |
|---------|----------|
| Error handling | Treat Semble readiness/safety failures as omission, not agent-spawn failure. Log only bounded diagnostics from `internal/semble` where existing prompt assembly has a logger seam. |
| Observability | Tests should assert omission states rather than relying on logs. Runtime logs may name Semble unavailability or unsafe target, but must not include file contents, cache paths, or raw Semble output. |
| Configuration | Prompt assembly reads no durable Semble config. `internal/semble` owns `LIZA_ENABLE_SEMBLE` and cache/model environment handling. |
| Testing | Use fake Semble prompt-context providers and temporary absolute roots to prove role targeting, shell quoting, omission paths, routing text, and Stacklit/SCIP preservation. |
| Security | Prompt guidance never encourages remote Git URL search, never targets sibling/parent worktrees for task/reviewer prompts, and requires direct file reads before evidence claims. |
| Concurrency | Prompt assembly only reads readiness/safety metadata. Any Semble cache synchronization is owned by `internal/semble`; worktree ignore serialization is owned by the worktree-safety scope. |
| Reversibility | Changes are localized to prompt metadata, rendering, and prompt assembly tests. Disabling Semble or failing safety checks removes prompt guidance without changing existing Stacklit/SCIP behavior. |

---

## Decomposition

Each scope becomes a code-planning child task. No `bootstrap-precommit` entry is emitted because `.pre-commit-config.yaml` exists at worktree `HEAD`.

### Scope 1: MAS Semble Prompt Guidance

**Component(s):** Prompt Metadata; Base Prompt Template; Task Prompt Assembly; Reviewer Prompt Assembly; Orchestrator Prompt Assembly; Prompt Tests

**Boundary:** In scope: extend prompt metadata and rendering so MAS prompts include a concise `=== SEMBLE SEARCH ===` section only when `internal/semble` reports enabled, offline-ready, and target-safe context for the exact role target root; wire task, reviewer, and safe orchestrator project-root target selection; preserve Stacklit/SCIP sections; update unified query routing to position Semble for conceptual discovery, content modes, Morph fallback, direct-read verification, and `rg`/`ast-grep` boundaries. Out of scope: init prewarm implementation, worktree `.sembleignore` generation internals, Pairing SessionStart shell hook changes, operator documentation, and Semble MCP.

**Output desc:** MAS Semble Prompt Guidance: extend prompt metadata and rendering so MAS prompts include a concise `=== SEMBLE SEARCH ===` section only when `internal/semble` reports enabled, offline-ready, and target-safe context for the exact role target root; wire task, reviewer, and safe orchestrator project-root target selection; preserve Stacklit/SCIP sections; update unified query routing to position Semble for conceptual discovery, content modes, Morph fallback, direct-read verification, and `rg`/`ast-grep` boundaries. Out of scope: init prewarm implementation, worktree `.sembleignore` generation internals, Pairing SessionStart shell hook changes, operator documentation, and Semble MCP.

**Output scope:** In scope: extend prompt metadata and rendering so MAS prompts include a concise `=== SEMBLE SEARCH ===` section only when `internal/semble` reports enabled, offline-ready, and target-safe context for the exact role target root; wire task, reviewer, and safe orchestrator project-root target selection; preserve Stacklit/SCIP sections; update unified query routing to position Semble for conceptual discovery, content modes, Morph fallback, direct-read verification, and `rg`/`ast-grep` boundaries. Out of scope: init prewarm implementation, worktree `.sembleignore` generation internals, Pairing SessionStart shell hook changes, operator documentation, and Semble MCP.

**Output done_when:** `go test ./internal/prompts ./internal/agent` proves Semble prompt sections are omitted when disabled, unavailable, offline-unready, or target-unsafe; task and reviewer prompts use shell-quoted absolute worktree roots and never parent project roots; orchestrator prompts use the project root only when root `.sembleignore` safety passes; rendered commands include `HF_HUB_OFFLINE=1`, `semble search`, `semble find-related`, and the one-line content-mode guidance; query routing preserves Stacklit/SCIP guidance and positions Morph as fallback only when Semble is unavailable; prompt text states Semble returns candidates, not proof; reviewer prompt assembly consumes the worktree safety contract without modifying reviewer recovery; and Pairing SessionStart tests remain unaffected.

**Depends on:** Existing tasks `architecture-main-1-architecture-0-code-planning-0` and `architecture-main-1-architecture-2-code-planning-1`.

**Validation:** `go test ./internal/prompts ./internal/agent`

### Spec Coverage

| Spec Requirement | Scope |
|------------------|-------|
| MAS prompts include Semble guidance only when installed, offline-ready, enabled, and target-safe | Scope 1 |
| MAS prompts omit Semble guidance when disabled, unavailable, offline-unready, or target-unsafe | Scope 1 |
| Task agents search only their task worktree root | Scope 1 |
| Reviewer agents search the reviewer candidate worktree root | Scope 1 |
| Orchestrator agents search the project root only when root `.sembleignore` safety passes | Scope 1 |
| MAS Semble guidance uses `HF_HUB_OFFLINE=1` | Scope 1 |
| Rendered commands include `semble search` and `semble find-related` with shell-quoted absolute roots | Scope 1 |
| Content-specific Semble guidance covers `code`, `docs`, `config`, and `all` in one line | Scope 1 |
| Semble guidance positions results as candidate discovery, not proof | Scope 1 |
| Direct file reads remain required before editing or evidence claims | Scope 1 |
| Stacklit and SCIP guidance remain distinct and preserved | Scope 1 |
| Semble is preferred for conceptual discovery when available; Morph MCP is fallback only when Semble is unavailable and policy exposes Morph | Scope 1 |
| `rg` is retained for exhaustive literal/exact search and not broad conceptual fallback | Scope 1 |
| `ast-grep` remains for syntax-shaped search | Scope 1 |
| MAS receives Semble through spawned prompts, not Pairing SessionStart | Scope 1 |
| Pairing SessionStart support remains unaffected | Scope 1 through regression awareness; no SessionStart shell-hook edits |
| Init prewarm, worktree `.sembleignore` generation, docs, remote Git URL indexing, Semble MCP, and Semble install flows remain out of scope | Out of scope here; covered by sibling/dependency tasks where applicable |

## Shared-File Audit

| File/Area | Owner | Readers / Ordering |
|-----------|-------|--------------------|
| `internal/prompts/builder.go` | Scope 1 | Adds Semble metadata beside SCIP/Stacklit metadata. |
| `internal/prompts/templates/base_prompt.tmpl` | Scope 1 | Renders Semble and unified routing text; must preserve existing SCIP/Stacklit sections. |
| `internal/prompts/builder_test.go` | Scope 1 | Rendering and routing tests; read targeted regions because the file is large. |
| `internal/prompts/role_context.go` | Scope 1 | May carry Semble refs if the code-planner chooses role-context propagation; otherwise base config can consume directly from agent prompt data. |
| `internal/agent/prompt.go` | Scope 1 | Central target-root selection and base prompt config construction for task, reviewer, and orchestrator roles. |
| `internal/agent/prompt_test.go` | Scope 1 | Role target-root and omission tests; read targeted regions because the file is large. |
| `internal/agent/strategy_orchestrator.go` | Scope 1 | Orchestrator project-root prompt path; no Semble pre-indexing should be added. |
| `internal/agent/strategy_reviewer.go` | Scope 1 | Reviewer claim/recovery ordering is read-only unless a narrow prompt-target seam is needed. |
| `internal/agent/worktree_check.go` | Dependency task | Worktree safety/recovery behavior is owned by `architecture-main-1-architecture-2-code-planning-1`; this scope should not modify reviewer recovery internals. |
| `internal/semble/*` | Dependency task | Consumed prompt-context and target-safety API from `architecture-main-1-architecture-0-code-planning-0`. |

## Validation Plan

- Confirm the decomposition maps the assigned MAS Semble Prompt Guidance scope to exactly one output entry.
- Confirm `Output desc`, `Output scope`, `Output done_when`, dependency, and validation fields above are copied verbatim into the structured output JSON.
- Confirm the structured output entry has `arch_ref` set to `specs/arch-plan/20260601-integrate-semble/20260601-144312-architecture-main-1-architecture-3.md`.
- Confirm no `bootstrap-precommit` output entry is emitted because `.pre-commit-config.yaml` exists at worktree `HEAD`.
- Run the canonical validation command for this architecture task: `go test ./internal/prompts ./internal/agent`.
- Run `liza set-task-output architecture-main-1-architecture-3 --output specs/arch-plan/20260601-integrate-semble/20260601-144312-architecture-main-1-architecture-3-output.json --agent-id architect-2 --json`.
- Run pre-commit on the touched architecture artifacts, commit only those artifacts, verify `git status --short` is clean, and submit `HEAD` for review.

## Self-Review Notes

- The plan is implementation planning only; no runtime code is changed in this architecture task.
- The decomposition remains one code-planning scope because the assigned work is one prompt-guidance domain spanning prompt metadata, rendering, and role target selection.
- Interface ownership remains singular: `internal/semble` owns readiness, shell-quoted command metadata, and target safety; the worktree-safety scope owns `.sembleignore` preparation; this scope owns MAS prompt rendering and target selection.
- The task depends on upstream code-planning tasks so prompt code-planning can consume stable Semble and worktree-safety contracts rather than inventing duplicate APIs.
