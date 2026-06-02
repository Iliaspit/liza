# Architecture Plan: Semble Operator Documentation

Status: review

## Goal

Document Semble as an optional external semantic discovery tool for operators without changing runtime implementation behavior in this task.

## Context

This task covers the documentation-only adoption slice from the Semble goal spec. The implementation slices for activation, prewarm, MAS prompt injection, and worktree safety are separate concerns; this plan scopes operator-facing documentation so downstream work has a stable setup, routing, and non-goal contract.

### References

- Goal spec: `specs/goals/20260601-integrate-semble.md`
- Parent tasks: `architecture-main-1`
- Codebase: `README.md`, `support-docs/CONFIGURATION.md`, `support-docs/CUSTOMIZING_AGENT_TOOLS.md`, `support-docs/USAGE_MULTI_AGENTS.md`, `internal/embedded/consistency_test.go`, `Makefile`, `specs/architecture/ADR/README.md`

### Constraints

- This architecture submission must contain only the architecture artifact and structured output. README and support-doc edits are assigned to Scope 1, not committed by the architecture task.
- The downstream task is documentation-only; implementation code, Pairing SessionStart shell code, generated runtime state under `.liza/agent-outputs/`, and canonicalizing `docs/` stubs are out of scope.
- Support docs are embedded artifacts. Updating `support-docs/*.md` requires syncing matching copies under `internal/embedded/support-docs/` so `go test ./internal/embedded` remains valid.
- `.pre-commit-config.yaml` already exists in this worktree, so no bootstrap-precommit output is emitted.
- Liza remains stack-agnostic. The documentation must describe optional operator tooling without implying a required project language or build system.

### Assumptions

- **ASM-001**: A single documentation planning scope is sufficient because all requested files form one operator documentation surface and share the same validation contract. *Why*: splitting by file would create shared-message drift without independent user-visible outcomes. Confidence: HIGH.

### Open Questions

- None.

## Components

### README Recommended Tools (`README.md`)

**Responsibility:** Introduce Semble as an optional recommended tool alongside existing token-saving and repository-navigation tools.

**Boundaries:**
- Exposes: a concise, non-mandatory Semble row in the tool table.
- Depends on: the detailed setup and safety contract in `support-docs/CONFIGURATION.md`.

**Key decisions:**
- Keep the README change to the existing recommended tools table: this preserves README altitude and avoids duplicating operator setup details.

### Configuration Reference (`support-docs/CONFIGURATION.md`)

**Responsibility:** Define the operator contract for `LIZA_ENABLE_SEMBLE`, offline readiness, model prewarm, the default 30-second prewarm/offline-validation timeout, `.sembleignore`, and Semble non-goals.

**Boundaries:**
- Exposes: activation values, init-time prewarm behavior, possible Hugging Face/model cache contact, `HF_HUB_OFFLINE=1` MAS mode, the named implementation constant's default 30-second timeout for prewarm and offline validation, bounded diagnostics, target-root scoping, default ignore patterns, and non-goals.
- Depends on: the Semble goal spec for default ignore patterns and first-milestone scope.

**Key decisions:**
- Place Semble near Stacklit and SCIP in the configuration matrix because it is another optional repository-navigation tool.
- Document `.sembleignore` as directory-scoped and physical in worktrees because Git private excludes alone are not visible to Semble.

### Tool Routing Guide (`support-docs/CUSTOMIZING_AGENT_TOOLS.md`)

**Responsibility:** Position Semble relative to Stacklit, SCIP, Morph MCP, `rg`, `ast-grep`, and direct reads in worktree-safe tool guidance.

**Boundaries:**
- Exposes: question-type routing and source-of-truth verification guidance.
- Depends on: prompt-supplied target roots and operator-enabled offline readiness.

**Key decisions:**
- Add Semble as conceptual discovery, not as proof, so semantic results cannot substitute for direct reads.
- Keep Morph MCP as fallback only when Semble is unavailable or not offline-ready and policy exposes Morph.

### Multi-Agent Usage Guide (`support-docs/USAGE_MULTI_AGENTS.md`)

**Responsibility:** Point MAS operators from quick-start setup to Semble configuration, offline validation, and safety docs.

**Boundaries:**
- Exposes: short setup pointer near existing SCIP and Stacklit optional-tool callouts.
- Depends on: the full configuration reference for operational details.

**Key decisions:**
- Keep quick-start text short to avoid duplicating the configuration reference.

### Embedded Support Docs (`internal/embedded/support-docs/`)

**Responsibility:** Carry byte-identical embedded copies of changed support docs.

**Boundaries:**
- Exposes: embedded support docs distributed by setup/init flows.
- Depends on: `make sync-embedded` copying canonical `support-docs/*.md`.

**Key decisions:**
- Treat embedded copies as generated-from-master artifacts and validate with `go test ./internal/embedded`.

## Interfaces

### README Recommended Tools -> Configuration Reference

**Contract:** README links readers to `support-docs/CONFIGURATION.md` for optional-tool setup details.
**Direction:** README provides discovery; configuration owns Semble setup semantics.
**Invariants:** README must not make Semble required.

### Configuration Reference -> Tool Routing Guide

**Contract:** Configuration defines when Semble is enabled and safe; tool routing defines when agents should choose it.
**Direction:** Operators configure Semble first, then agents receive prompt guidance and follow routing.
**Invariants:** Semble must remain candidate discovery; direct reads remain evidence.

### Support Docs -> Embedded Support Docs

**Contract:** Embedded support docs are byte-identical copies of `support-docs/*.md`.
**Direction:** `make sync-embedded` copies canonical support docs into `internal/embedded/support-docs/`.
**Invariants:** `go test ./internal/embedded` must pass after documentation changes.

## Data Flow

```text
Goal spec requirements
  -> README optional-tool mention
  -> CONFIGURATION activation/setup/safety contract
  -> CUSTOMIZING_AGENT_TOOLS routing contract
  -> USAGE_MULTI_AGENTS quick-start pointer
  -> make sync-embedded
  -> embedded-doc consistency tests
```

Semble runtime output is not part of this documentation task. The documented operator flow is:

```text
Operator sets LIZA_ENABLE_SEMBLE=1
  -> liza init --spec may prewarm model and contact Hugging Face/model cache
  -> Liza validates with HF_HUB_OFFLINE=1
  -> MAS prompts include Semble only when offline-ready
  -> Agents use Semble for candidates and direct reads for proof
```

## Cross-Cutting Concerns

| Concern | Approach |
|---------|----------|
| Error handling | Document Semble failures as non-blocking for MAS spawn with bounded diagnostics and omitted prompt guidance. |
| Observability | Operator-visible diagnostics are documented as bounded and must distinguish unavailable CLI from offline model/cache failures; the prewarm/offline-validation timeout is documented as the named implementation constant's default of 30 seconds. |
| Configuration | `LIZA_ENABLE_SEMBLE` is process-local and strict opt-in; no durable `state.yaml` field is introduced by this task. |
| Testing | Scope 1 owns the declared documentation `rg` checks, embedded support-doc sync, and `go test ./internal/embedded`; the architecture submission validates only the plan/output contract and leaves docs unchanged. |
| Security | Document `.sembleignore` defaults for `.liza/`, `.worktrees/`, generated indexes, and credential patterns; warn that direct source reads remain required. |

## Decomposition

Each scope becomes a code-planning child task.

### Scope 1: Semble Operator Documentation

**Desc:** Semble Operator Documentation: update README.md, support-docs/CONFIGURATION.md, support-docs/CUSTOMIZING_AGENT_TOOLS.md, support-docs/USAGE_MULTI_AGENTS.md, and synced embedded support-doc copies to document Semble activation, offline behavior, .sembleignore safety, routing, and non-goals.
**Component(s):** README Recommended Tools, Configuration Reference, Tool Routing Guide, Multi-Agent Usage Guide, Embedded Support Docs.
**Boundary:** In scope: update `README.md`, `support-docs/CONFIGURATION.md`, `support-docs/CUSTOMIZING_AGENT_TOOLS.md`, `support-docs/USAGE_MULTI_AGENTS.md`, and synced embedded support-doc copies to document Semble activation, offline behavior, `.sembleignore` safety, routing, and non-goals. Out of scope: implementation code, Pairing SessionStart shell code, generated runtime state under `.liza/agent-outputs/`, and converting `docs/` stubs into canonical content.
**Done when:** Documentation checks prove `README.md` lists Semble as an optional semantic discovery tool without making it a requirement; `support-docs/CONFIGURATION.md` documents `LIZA_ENABLE_SEMBLE`, truthy/false values, init-time prewarm, possible Hugging Face/model cache contact, offline validation, `HF_HUB_OFFLINE=1`, the named implementation constant's default 30-second timeout for prewarm and offline validation, bounded diagnostics, Semble non-goals, `.sembleignore` directory scope, and default runtime/generated/credential exclusions; `support-docs/CUSTOMIZING_AGENT_TOOLS.md` documents Semble routing relative to Stacklit, SCIP/scip-search, Morph MCP, `rg`, `ast-grep`, and direct reads; `support-docs/USAGE_MULTI_AGENTS.md` points MAS operators to Semble setup; and `go test ./internal/embedded` passes after embedded support-doc sync.
**Depends on:** None.

**Validation:**

- `rg -q 'Semble.*optional|optional.*Semble' README.md`
- `rg -q 'semantic discovery|semantic repository search|repository-navigation' README.md`
- `rg -q 'LIZA_ENABLE_SEMBLE' support-docs/CONFIGURATION.md`
- `rg -q 'truthy' support-docs/CONFIGURATION.md`
- `rg -q 'false|disabled' support-docs/CONFIGURATION.md`
- `rg -q 'prewarm|prewarms|prewarming' support-docs/CONFIGURATION.md`
- `rg -q 'Hugging Face' support-docs/CONFIGURATION.md`
- `rg -q 'model cache|cache' support-docs/CONFIGURATION.md`
- `rg -q 'offline validation|offline-readiness|offline readiness' support-docs/CONFIGURATION.md`
- `rg -q 'HF_HUB_OFFLINE=1' support-docs/CONFIGURATION.md`
- `rg -q '30 seconds|30-second' support-docs/CONFIGURATION.md`
- `rg -q 'bounded diagnostic|bounded diagnostics' support-docs/CONFIGURATION.md`
- `rg -q 'automatic install|automatically install|does not install' support-docs/CONFIGURATION.md`
- `rg -q 'semble init' support-docs/CONFIGURATION.md`
- `rg -q 'Semble MCP' support-docs/CONFIGURATION.md`
- `rg -q 'remote Git URL|remote.*URL' support-docs/CONFIGURATION.md`
- `rg -q '\.sembleignore' support-docs/CONFIGURATION.md`
- `rg -q 'directory-scoped|directory scope' support-docs/CONFIGURATION.md`
- `rg -q '\.liza/' support-docs/CONFIGURATION.md`
- `rg -q '\.worktrees/' support-docs/CONFIGURATION.md`
- `rg -q 'stacklit\.json' support-docs/CONFIGURATION.md`
- `rg -q '\*\.scip' support-docs/CONFIGURATION.md`
- `rg -q '\.env' support-docs/CONFIGURATION.md`
- `rg -q '\*\.pem|credential' support-docs/CONFIGURATION.md`
- `rg -q 'Semble' support-docs/CUSTOMIZING_AGENT_TOOLS.md`
- `rg -q 'Stacklit' support-docs/CUSTOMIZING_AGENT_TOOLS.md`
- `rg -q 'SCIP|scip-search' support-docs/CUSTOMIZING_AGENT_TOOLS.md`
- `rg -q 'Morph MCP' support-docs/CUSTOMIZING_AGENT_TOOLS.md`
- `rg -q 'rg' support-docs/CUSTOMIZING_AGENT_TOOLS.md`
- `rg -q 'ast-grep' support-docs/CUSTOMIZING_AGENT_TOOLS.md`
- `rg -q 'direct read|direct source read|exact read' support-docs/CUSTOMIZING_AGENT_TOOLS.md`
- `rg -q 'Semble' support-docs/USAGE_MULTI_AGENTS.md`
- `rg -q 'LIZA_ENABLE_SEMBLE' support-docs/USAGE_MULTI_AGENTS.md`
- `rg -q 'CONFIGURATION|setup|offline' support-docs/USAGE_MULTI_AGENTS.md`
- `go test ./internal/embedded`

### Spec Coverage

| Spec Requirement | Scope |
|------------------|-------|
| Document Semble as optional repository-navigation semantic discovery | Scope 1 |
| Document `LIZA_ENABLE_SEMBLE` truthy and false activation values | Scope 1 |
| Document init-time model prewarm and possible Hugging Face/model cache contact | Scope 1 |
| Document offline validation and `HF_HUB_OFFLINE=1` MAS operation | Scope 1 |
| Document the named implementation constant's default 30-second timeout for prewarm and offline validation | Scope 1 |
| Document bounded diagnostics and non-blocking MAS behavior | Scope 1 |
| Document task/root worktree scoping and `.sembleignore` directory scope | Scope 1 |
| Document default `.sembleignore` runtime, generated-index, and credential exclusions | Scope 1 |
| Document routing relative to Stacklit, SCIP, Morph MCP, `rg`, `ast-grep`, and direct reads | Scope 1 |
| Document non-goals: automatic installation, vendoring, `semble init`, Semble MCP, and remote Git URL indexing | Scope 1 |
| Keep implementation code, Pairing SessionStart shell code, runtime logs, and docs-stub conversion out of scope | Scope 1 |
