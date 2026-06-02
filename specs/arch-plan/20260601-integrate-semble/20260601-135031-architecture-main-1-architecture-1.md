# Architecture Plan: Semble Init Prewarm Integration

Status: draft

## Goal

Wire the Semble capability package into Liza workspace initialization so enabled init paths intentionally prewarm Semble, immediately validate offline readiness, and degrade without blocking workspace creation.

## Context

The parent Semble architecture split the first milestone into domain scopes. This task covers only Scope 2, the init-time lifecycle integration. The upstream Scope 1 architecture defines `internal/semble` as the owner of activation parsing, executable detection, fixture-based prewarm, offline validation, process-local cache, and bounded diagnostics. This plan defines where init callers consume that package and how diagnostics remain visible without creating durable Semble state.

### References

- Goal spec: `specs/goals/20260601-integrate-semble.md`
- Parent master plan: `specs/arch-plan/20260601-integrate-semble/20260601-130216-architecture-main-1.md`
- Upstream package plan: `specs/arch-plan/20260601-integrate-semble/20260601-133505-architecture-main-1-architecture-0.md`
- Parent task outputs: `architecture-main-1`, `architecture-main-1-architecture-0`
- Active upstream code-planning dependency: `architecture-main-1-architecture-0-code-planning-0`
- Codebase explored: `cmd/liza/cmd_init.go`, `cmd/liza/cmd_init_test.go`, `internal/commands/init.go`, `internal/commands/init_test.go`, `internal/ops/init_project.go`, `internal/ops/init_project_test.go`, `internal/scipsearch/scipsearch.go`, `internal/models/config.go`
- ADRs: `specs/architecture/ADR/0068-optional-repository-indexing-with-scip-and-stacklit.md`, `specs/architecture/ADR/0074-sessionstart-context-hooks.md`, `specs/architecture/ADR/0031-configurable-post-worktree-command.md`, `specs/architecture/ADR/0021-ops-service-layer-for-mutations.md`
- Guardrails and invariants: `GUARDRAILS.md`, `INVARIANTS.md`

### Constraints

- Semble activation remains environment-only through `LIZA_ENABLE_SEMBLE`.
- No Semble field is added to `models.Config` or serialized `state.yaml`.
- No Semble CLI flags are added to `cmd/liza init`; `hasExplicitInitFlags` remains unchanged for Semble.
- Init must skip Semble entirely when disabled: no executable lookup, no prewarm, no offline validation, no diagnostics.
- Enabled Semble failures are non-fatal for workspace initialization.
- Diagnostics must be bounded, operator-visible, and must not dump raw unbounded command output or file contents.
- Existing SCIP init behavior and tests must remain intact.
- Runtime behavior must remain stack-agnostic; no user-project build commands, Makefile assumptions, or Semble installation steps are introduced.
- This scope does not render MAS prompts, create `.sembleignore`, update operator docs, alter Pairing SessionStart shell hooks, or install Semble/models.
- No bootstrap-precommit output is emitted because `.pre-commit-config.yaml` exists on the task worktree/integration baseline.

### Assumptions

- None. The init integration depends on the upstream `internal/semble` code-planning task for concrete API names, but the interface responsibilities are already defined by the approved upstream architecture.

### Open Questions

- None.

---

## Components

### CLI Init Dispatch (`cmd/liza/cmd_init.go`)

**Responsibility:** Keep the public `liza init` surface unchanged while forwarding existing workspace-init parameters to `internal/commands`.

**Boundaries:**
- Exposes: existing Cobra `init` command behavior and tests.
- Depends on: `internal/commands.InitCommandWithConfig`.

**Key decisions:**
- Do not add `--semble`, `--enable-semble`, or any other Semble flag. The only activation gate is `LIZA_ENABLE_SEMBLE`.
- Do not include Semble in `hasExplicitInitFlags`; environment activation must not change pairing-vs-workspace dispatch.
- Add dispatch-level tests only where they prove no flag/state surface was introduced or that full init still routes to the command layer with Semble enabled.

### Interactive/CLI Workspace Init (`internal/commands/init.go`)

**Responsibility:** Run Semble init prewarm and offline validation in the operator-facing init path after ordinary init preconditions are known and before workspace scaffold creation.

**Boundaries:**
- Exposes: bounded diagnostics on the existing init stderr channel.
- Depends on: `internal/semble` for activation, executable detection, prewarm/offline validation, and diagnostic classification; existing `internal/scipsearch` init resolution remains independent.

**Key decisions:**
- Place Semble prewarm after branch/spec/pre-commit/global-config validation and after existing SCIP init resolution, but before creating `.liza/`. This avoids leaving partial workspace state because of a Semble helper bug while still keeping Semble failures non-fatal.
- Keep SCIP and Semble independent. SCIP config can still persist configured languages, while Semble never persists init readiness or config.
- Prefix init diagnostics consistently, for example `semble: <bounded message>` or `Warning: semble: <bounded message>`, without printing raw unbounded stdout/stderr.
- Missing executable when enabled produces a bounded diagnostic and skips prewarm/offline validation rather than failing init.
- When the executable is present, invoke the package-level prewarm/offline validation contract. That contract may intentionally contact Hugging Face during prewarm through the operator environment, then validates with `HF_HUB_OFFLINE=1`.

### Programmatic Workspace Init (`internal/ops/init_project.go`)

**Responsibility:** Provide the same Semble prewarm/offline validation semantics for non-interactive workspace initialization without coupling ops to terminal I/O.

**Boundaries:**
- Exposes: optional transient diagnostic delivery through an `InitProjectParams` field or callback; if absent, Semble diagnostics may be suppressed but init behavior remains identical.
- Depends on: `internal/semble` and the existing ops init precondition/scaffold flow.

**Key decisions:**
- Mirror the command path placement: after branch/spec/pre-commit/global-config validation and before `.liza/` directory creation.
- Preserve the ops layer's no-terminal-I/O boundary by not writing directly to `os.Stdout` or `os.Stderr` from `internal/ops`.
- Any diagnostic sink added to `InitProjectParams` is transient and test-facing/operator-facing only; it is not serialized into state.
- Keep the initial `models.Config` literal free of Semble fields.

### Semble Capability Package (`internal/semble`)

**Responsibility:** Own all Semble-specific behavior consumed by init.

**Boundaries:**
- Exposes: strict environment gate, init prewarm/offline validation entry point, result status, and bounded diagnostic strings/classes.
- Depends on: external `semble` CLI and temporary fixture execution.

**Key decisions:**
- Init callers should consume one package-level init readiness/prewarm function rather than duplicating command planning in `internal/commands` or `internal/ops`.
- The package boundary decides whether Semble is disabled, missing, prewarmed, offline-ready, model-unavailable, or failed with another execution error.
- The package handles `HF_HUB_OFFLINE=1` for offline validation; init callers should not assemble Semble command argv or environment manually.

### Init Test Harnesses

**Responsibility:** Prove the integration through command, ops, and Cobra dispatch tests without requiring a real Semble installation or network.

**Boundaries:**
- Exposes: fake Semble runner/executable hooks through `internal/semble` test helpers.
- Depends on: existing init test repositories and helper patterns.

**Key decisions:**
- Tests should use fake runners and temporary repositories, matching the existing SCIP `SetCommandRunnerForTest` pattern.
- Tests must assert disabled mode performs zero Semble calls.
- Tests must assert state YAML omits Semble configuration after enabled init.
- Tests should preserve existing SCIP assertions, especially repeated `--scip-search`, disabled SCIP no-op, and autodetect diagnostics.

---

## Interfaces

### Init Callers -> Semble Capability

**Contract:** Init callers provide current process environment, project root context only for diagnostic context if required, and a diagnostic sink. The Semble package returns a no-op disabled result, a non-fatal unavailable result, or prewarm/offline validation diagnostics.

**Direction:** `internal/commands` and `internal/ops` call `internal/semble`. `internal/semble` never imports init packages or models.

**Invariants:** Disabled Semble does not look up or run the CLI. Enabled missing/unready Semble does not block init. No result is persisted in `models.Config` or state YAML.

### `cmd/liza` -> `internal/commands`

**Contract:** Cobra continues forwarding existing init parameters only.

**Direction:** `cmd/liza` reads flags and invokes `commands.InitCommandWithConfig`.

**Invariants:** No Semble flag is registered. Pairing-only dispatch remains unaffected by Semble environment variables.

### `internal/ops` -> Operator/Caller Diagnostics

**Contract:** Programmatic init can expose bounded Semble diagnostics through a transient writer/callback supplied by the caller or tests.

**Direction:** `ops.InitProject` emits through the supplied sink only; CLI-oriented presentation remains outside `internal/ops`.

**Invariants:** Ops does not write terminal output directly and does not store diagnostics in state.

---

## Data Flow

```text
Operator sets LIZA_ENABLE_SEMBLE=true
  -> liza init --spec or ops.InitProject
  -> normal init preconditions validate branch, spec, pre-commit config, and global setup
  -> existing SCIP init resolution runs unchanged where applicable
  -> init calls internal/semble init prewarm/offline validation
  -> disabled: return no-op with no diagnostics or CLI calls
  -> missing CLI: bounded non-fatal diagnostic
  -> CLI present: controlled prewarm fixture may use network inherited from operator environment
  -> immediate offline fixture validation runs with HF_HUB_OFFLINE=1
  -> bounded diagnostics emitted for offline-unready or execution failure
  -> .liza scaffold and state are created without Semble config
```

---

## Cross-Cutting Concerns

| Concern | Approach |
|---------|----------|
| Error handling | Semble readiness failures are classified by `internal/semble` and converted to warnings/diagnostics by init callers. Invalid ordinary init preconditions still fail as today. |
| Observability | CLI init writes bounded Semble diagnostics to stderr. Ops init uses a transient diagnostic sink instead of direct terminal output. |
| Configuration | Semble uses only process environment (`LIZA_ENABLE_SEMBLE`, plus Semble/model cache variables owned by `internal/semble`). No durable config or flags. |
| Testing | Use fake Semble runner hooks to prove call order, disabled no-op, missing executable, offline-unready diagnostics, and state serialization. |
| Security | Diagnostics are bounded and package-classified. Init never prints raw unbounded Semble output, cache paths beyond what the package marks safe, or fixture contents. |
| Worktree isolation | Init prewarm validates only a temporary one-file fixture outside project/worktree roots. It does not index the user project or task worktrees. |
| Stack agnosticism | No user-project build, package-manager, or Makefile commands are added. Semble remains an optional external executable. |
| Regression risk | Existing SCIP init resolution stays independent; command tests should keep SCIP assertions while adding Semble tests around them. |

---

## Structural Decisions

1. **Consume Semble through `internal/semble`, not through duplicated init command code.** The package already owns command planning, offline env, cache keys, and diagnostics. Init only decides placement and presentation.
2. **Run Semble before scaffold creation but after hard preconditions.** This gives controlled model prewarm without allowing non-fatal Semble failures to leave half-created `.liza` state.
3. **Use transient diagnostics, not durable readiness.** The goal spec rejects first-milestone durable Semble config, and process-local validation cache belongs in `internal/semble`.
4. **Keep CLI surface closed.** Environment activation matches the spec and avoids flag interactions with pairing dispatch.
5. **Preserve ops presentation boundary.** `internal/ops` should not start writing terminal output; diagnostics flow through an optional caller-provided sink.

---

## Decomposition

Each scope becomes a code-planning child task. No bootstrap-precommit entry is emitted because `.pre-commit-config.yaml` exists on integration `HEAD`.

### Scope 1: Semble Init Prewarm Integration

**Component(s):** CLI Init Dispatch; Interactive/CLI Workspace Init; Programmatic Workspace Init; Semble Capability Package; Init Test Harnesses

**Boundary:** In scope: wire `internal/semble` into `cmd/liza` and workspace initialization paths so `liza init --spec` and equivalent non-interactive init perform controlled Semble prewarm and immediate offline validation only when `LIZA_ENABLE_SEMBLE` is truthy and `semble` is present; emit bounded operator-visible diagnostics without blocking init for missing/unready Semble; preserve existing SCIP init behavior and avoid adding durable Semble state or CLI flags. Out of scope: MAS prompt rendering, worktree `.sembleignore` generation, operator docs, Pairing SessionStart shell hook behavior, automatic Semble/model installation, and any user-project build command assumptions.

**Output desc:** Semble Init Prewarm Integration: wire `internal/semble` into `cmd/liza` and workspace initialization paths so `liza init --spec` and equivalent non-interactive init perform controlled Semble prewarm and immediate offline validation only when `LIZA_ENABLE_SEMBLE` is truthy and `semble` is present; emit bounded operator-visible diagnostics without blocking init for missing/unready Semble; preserve existing SCIP init behavior and avoid adding durable Semble state or CLI flags. Out of scope: MAS prompt rendering, worktree `.sembleignore` generation, operator docs, Pairing SessionStart shell hook behavior, automatic Semble/model installation, and any user-project build command assumptions.

**Output scope:** In scope: wire `internal/semble` into `cmd/liza` and workspace initialization paths so `liza init --spec` and equivalent non-interactive init perform controlled Semble prewarm and immediate offline validation only when `LIZA_ENABLE_SEMBLE` is truthy and `semble` is present; emit bounded operator-visible diagnostics without blocking init for missing/unready Semble; preserve existing SCIP init behavior and avoid adding durable Semble state or CLI flags. Out of scope: MAS prompt rendering, worktree `.sembleignore` generation, operator docs, Pairing SessionStart shell hook behavior, automatic Semble/model installation, and any user-project build command assumptions.

**Output done_when:** `go test ./internal/commands ./internal/ops ./cmd/liza` proves init skips Semble entirely when disabled, invokes Semble prewarm/offline validation when `LIZA_ENABLE_SEMBLE` is truthy and the CLI is present, preserves bounded diagnostics for missing executable and offline-unready model cases, does not add a Semble field to `models.Config` or state YAML, does not add Semble CLI flags, preserves existing SCIP init tests, and leaves workspace initialization non-blocking for Semble readiness failures.

**Depends on:** Existing task `architecture-main-1-architecture-0-code-planning-0`

**Decomposition metadata:**
- Owned files: `cmd/liza/cmd_init.go`, `cmd/liza/cmd_init_test.go`, `internal/commands/init.go`, `internal/commands/init_test.go`, `internal/ops/init_project.go`, `internal/ops/init_project_test.go`
- Owned modules: `cmd/liza`, `internal/commands`, `internal/ops` init path
- Read-only depends on: `internal/semble` readiness and prompt-context API from existing upstream task `architecture-main-1-architecture-0-code-planning-0`
- Interfaces owned: `Init-time Semble prewarm integration`
- Interfaces consumed: `Semble readiness and prompt-context API`
- Coverage notes: Covers init-time model bootstrap/offline validation and no durable Semble config.

### Spec Coverage

| Spec Requirement | Scope |
|------------------|-------|
| `liza init --spec` intentionally prewarms/downloads Semble model when enabled and CLI is present | Scope 1 |
| Offline validation runs immediately after init-time prewarm with `HF_HUB_OFFLINE=1` | Scope 1 |
| Disabled Semble performs no validation or execution | Scope 1 |
| Missing executable and offline-unready model produce bounded diagnostics | Scope 1 |
| Semble readiness failures do not block workspace initialization | Scope 1 |
| No durable `state.yaml` Semble config field | Scope 1 |
| No Semble CLI flags | Scope 1 |
| Existing SCIP init behavior remains intact | Scope 1 |
| No user-project build command assumptions | Scope 1 |
| MAS prompt rendering, worktree `.sembleignore`, docs, Pairing SessionStart, automatic installs, and Semble MCP remain out of scope | Scope 1 |

## Shared-File Audit

| File/Area | Owner | Ordering |
|-----------|-------|----------|
| `internal/semble/*` | Upstream `architecture-main-1-architecture-0-code-planning-0` | This scope consumes only its public readiness API and declares `task_depends_on`. |
| `cmd/liza/cmd_init.go` | Scope 1 | No sibling scope should add Semble flags. |
| `internal/commands/init.go` | Scope 1 | No sibling writes in the parent master plan. |
| `internal/ops/init_project.go` | Scope 1 | No sibling writes in the parent master plan. |
| Init tests under `cmd/liza`, `internal/commands`, `internal/ops` | Scope 1 | Tests should preserve existing SCIP coverage while adding Semble cases. |

## Validation Plan

- Confirm this document maps the assigned task `done_when` to exactly one code-planning scope.
- Confirm the structured output entry copies `Output desc`, `Output scope`, and `Output done_when` verbatim from this document.
- Confirm the output entry has `arch_ref` set to `specs/arch-plan/20260601-integrate-semble/20260601-135031-architecture-main-1-architecture-1.md`.
- Confirm the output entry uses `task_depends_on: ["architecture-main-1-architecture-0-code-planning-0"]` because init integration consumes the upstream `internal/semble` implementation.
- Confirm no `bootstrap-precommit` entry is emitted because `.pre-commit-config.yaml` exists.
- Run `liza set-task-output architecture-main-1-architecture-1 --output <worktree-absolute-path-to-output-json> --agent-id architect-3 --json`.
- Commit only the architecture artifacts, verify `git status --short` is clean, and submit `HEAD` for review.

## Self-Review Notes

- The plan changes only architecture artifacts; no runtime code is modified in this task.
- The decomposition has one domain scope because the assigned task is already a narrow init integration slice.
- The plan keeps Semble activation environment-only and transient, preserving `models.Config` and state YAML shape.
- Ops diagnostics are designed as transient caller-facing data, preserving the ops layer's no-terminal-I/O convention.
- The existing SCIP init path remains separate from Semble and is explicitly covered by downstream validation.
