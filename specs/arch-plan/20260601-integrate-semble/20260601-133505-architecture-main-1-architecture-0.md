# Architecture Plan: Semble Capability Package

Status: draft

## Goal

Define a new `internal/semble` package that owns Semble activation, readiness validation, command metadata, target safety, and default ignore-pattern contracts without wiring Semble into Liza lifecycle callers yet.

## Context

The goal spec makes Semble an optional semantic-discovery tool for Liza. This architecture task is the package-level foundation only: it creates the testable Go API that later init, worktree, prompt, and documentation slices can call. Existing optional-tool packages (`internal/stacklit` and `internal/scipsearch`) establish the local pattern: keep external tools optional, expose fixed command plans before execution, bound diagnostics, avoid prompt guidance unless readiness is proven, and keep runtime-generated files out of task diffs.

### References

- Goal spec: `specs/goals/20260601-integrate-semble.md`
- Parent task: `architecture-main-1`
- Codebase: `internal/stacklit/doc.go`, `internal/stacklit/stacklit.go`, `internal/scipsearch/doc.go`, `internal/scipsearch/scipsearch.go`, `internal/prompts/builder.go`, `internal/prompts/templates/base_prompt.tmpl`, `internal/prompts/role_context.go`, `internal/ops/stacklit_indexing.go`, `internal/ops/scip_indexing.go`, `internal/gitenv/gitenv.go`, `internal/git/worktree.go`, `internal/embedded/hooks/session-context.sh`
- ADR index: `specs/architecture/ADR/README.md`
- Project guardrails: `GUARDRAILS.md`

### Constraints

- Semble remains strict opt-in through `LIZA_ENABLE_SEMBLE`; there is no durable `state.yaml` config in this milestone.
- The package must not install Semble, Python, models, or global tooling.
- The package must not wire itself into `liza init`, worktree creation, prompt rendering, SessionStart, or documentation updates in this scope.
- Semble output is candidate discovery only; package APIs must make prompt metadata safe to render but not imply source-of-truth evidence.
- MAS execution must be offline after controlled prewarm; offline validation and rendered command examples must use `HF_HUB_OFFLINE=1`.
- Runtime diagnostics must be bounded and classified without dumping file contents or secrets.
- Target-root handling must preserve MAS worktree isolation and must not invite task agents to search a parent project root.
- The default `.sembleignore` pattern list must have one source of truth for package code and tests.
- `.pre-commit-config.yaml` exists in the worktree, so no bootstrap-precommit output is emitted.

### Assumptions

- None.

### Open Questions

- None.

## Components

### Semble Capability Package (`internal/semble/`)

**Responsibility:** Own all Semble-specific runtime contracts that can be tested without lifecycle wiring.

**Boundaries:**
- Exposes: activation parsing, executable detection, prewarm and offline validation command plans, validation execution, validation cache behavior, default ignore patterns, target safety checks, bounded diagnostics, and prompt-safe context metadata.
- Depends on: Go standard library process, filesystem, path, environment, timeout, and quoting primitives. It should not depend on `internal/ops`, `internal/prompts`, `internal/commands`, or durable state models.

**Key decisions:**
- Keep `internal/semble` independent from lifecycle packages: this lets later slices wire Semble into init, worktree setup, and prompt rendering without creating cycles or broadening this package's scope.
- Mirror optional-tool package style from Stacklit and SCIP: callers receive explicit command plans and bounded results rather than ad hoc subprocess execution.
- Use one tiny temporary fixture for both prewarm and offline validation: the same fixture identity becomes part of cache invalidation and keeps validation independent from repository content.
- Put default `.sembleignore` patterns in this package as an exported read-only contract: future worktree/root safety code and tests consume the same ordered list instead of duplicating credential/runtime exclusions.

### Activation Gate

**Responsibility:** Parse `LIZA_ENABLE_SEMBLE` exactly as the goal spec defines.

**Boundaries:**
- Exposes: strict truthy/false parsing for a supplied string and process-local runtime enabled checks.
- Depends on: environment lookup only when checking the current process.

**Key decisions:**
- Only trimmed, case-insensitive `1` and `true` are enabled. Unset, empty, `0`, `false`, and all other values are disabled.
- Disabled mode returns no-op readiness and prompt metadata results. It does not detect executables, run commands, mention Semble commands, or surface diagnostics.

### Executable and Command Planning

**Responsibility:** Detect the Semble CLI and describe exact prewarm and offline validation invocations.

**Boundaries:**
- Exposes: a resolved executable path, a prewarm command plan, an offline validation command plan, and prompt command examples with shell-quoted absolute target roots.
- Depends on: executable lookup and absolute path resolution.

**Key decisions:**
- Command plans carry executable name/path, argv, working directory, environment additions, target root, timeout, and fixture identity as structured data.
- Prewarm uses inherited network behavior from the operator environment and runs `semble search "__liza_semble_prewarm__" <temp-dir> --top-k 1 --content code`.
- Offline validation uses the same fixture and command shape with `HF_HUB_OFFLINE=1`.
- A command exit code of 0 is success even if Semble reports no hits, matching the spec's readiness contract.

### Fixture Manager

**Responsibility:** Create and clean a temporary one-file supported-code fixture for prewarm and validation.

**Boundaries:**
- Exposes: fixture identity and a temporary directory containing `prewarm.py` with `def liza_semble_prewarm(): pass`.
- Depends on: `os.TempDir()` semantics and cleanup after execution.

**Key decisions:**
- Fixture identity includes the file name, file content, search query, top-k value, and content mode so cache invalidation changes when the validation corpus changes.
- Fixture directories live outside project/worktree roots under OS temp space, so validation cannot accidentally index user repositories or task worktrees.

### Validation Executor and Cache

**Responsibility:** Run planned validation commands, classify failures, and cache fresh process-local readiness results.

**Boundaries:**
- Exposes: readiness result with success, diagnostic class, bounded message, and prompt-safe metadata when ready.
- Depends on: a runner interface for subprocess execution and a process-local cache guarded for concurrent callers.

**Key decisions:**
- Cache keys include resolved executable path, `SEMBLE_MODEL_NAME`, `HF_HOME`, `XDG_CACHE_HOME`, validation timeout, and fixture identity.
- Cache scope is current process only. There is no durable marker because the spec requires cache invalidation across process and environment changes.
- The timeout default is the package-owned Semble validation timeout constant, 30 seconds, so later operator documentation can reference one runtime contract.
- Failure classes are bounded to `missing_executable`, `model_unavailable_offline`, and `execution_failure`. Callers can show concise operator diagnostics without exposing raw command output.
- Command output is captured only for bounded diagnostics. The diagnostic builder trims and truncates combined output to the package maximum.

### Target Safety and Ignore Contract

**Responsibility:** Define Semble-safe target roots and the default generated `.sembleignore` contract.

**Boundaries:**
- Exposes: default ignore patterns, a worktree ignore-file payload, checks for required patterns in an existing root `.sembleignore`, and target-root safety validation for project-root and task-worktree contexts.
- Depends on: filesystem stat/read and absolute path comparison.

**Key decisions:**
- The default pattern list exactly covers runtime/generated artifacts and credential patterns from the goal spec: `.liza/`, `.worktrees/`, `stacklit.json`, `*.scip`, env files, credential files, key/certificate stores, and secrets directories.
- Root safety checks require a physical root `.sembleignore` containing every required pattern before project-root guidance is considered safe.
- Task worktree checks require the explicit task worktree root and reject substitution of the parent project root.
- The package plans the `.sembleignore` content and safety result; lifecycle code later owns writing the file and hiding it through a shared private Git exclude mechanism.

### Prompt Context Metadata

**Responsibility:** Produce prompt-safe Semble metadata without rendering full prompts.

**Boundaries:**
- Exposes: target root, shell-quoted target root, offline environment prefix, content-mode guidance, and example command strings.
- Depends on: target safety and successful offline readiness.

**Key decisions:**
- Prompt context is emitted only when Semble is enabled, the executable is present, offline validation is ready or freshly cached, and the target root is safe for the caller role.
- Examples include `HF_HUB_OFFLINE=1` and shell-quoted absolute target roots in every command.
- Metadata names Semble as discovery only and leaves actual prompt wording to later prompt-rendering work.

## Interfaces

### Lifecycle Callers -> Semble Capability Package

**Contract:** Callers provide an intended target root, target kind, environment snapshot, and optional runner/timeout. The package returns either no-op disabled state, readiness failure with a bounded diagnostic, or prompt-safe Semble metadata.

**Direction:** Init, worktree, prompt, and SessionStart integrations will call `internal/semble`; `internal/semble` does not call back into those layers.

**Invariants:** Disabled mode performs no executable lookup or subprocess execution. Enabled prompt metadata is never returned unless offline validation is successful and target safety passes.

### Semble Package -> External Semble CLI

**Contract:** The package executes only fixed `semble search` command plans for prewarm/offline validation and never runs `semble init`, `semble savings`, remote Git URL indexing, or MCP flows.

**Direction:** Package runner invokes Semble against the temporary fixture. Later agents invoke rendered examples against target roots outside this package.

**Invariants:** Offline validation and MAS-facing command metadata include `HF_HUB_OFFLINE=1`. Prewarm is the only non-offline command path in this package.

### Semble Package -> Filesystem

**Contract:** The package may create temporary fixture directories under OS temp space during validation and may read target `.sembleignore` files for safety checks.

**Direction:** The package writes only temporary validation fixture files and returns planned worktree `.sembleignore` content for lifecycle callers to write later.

**Invariants:** Validation fixture cleanup is attempted after command execution. The package does not write into repository roots or task worktrees in this scope.

### Semble Package -> Future Prompt Renderer

**Contract:** Prompt metadata contains already shell-quoted command examples and absolute target-root information, plus availability and content-mode fields that are safe to render.

**Direction:** Prompt builders consume metadata later; this scope does not modify templates.

**Invariants:** Metadata must not include raw command output, unbounded diagnostics, cache paths, secrets, or inferred parent/sibling worktree paths.

## Data Flow

```text
Environment + target root
  -> Activation gate
  -> Executable detection
  -> Target safety check
  -> Fixture manager
  -> Prewarm or offline command plan
  -> Validation executor
  -> Process-local validation cache
  -> Readiness result
  -> Prompt-safe Semble metadata
```

- Disabled environment values terminate the flow at the activation gate with no side effects.
- Missing executable terminates at detection with `missing_executable`.
- Unsafe target roots terminate before prompt metadata is produced.
- Offline validation failures terminate with `model_unavailable_offline` or `execution_failure`; callers omit Semble guidance.
- Successful validation stores a process-local cache entry keyed by executable, model/cache environment, timeout, and fixture identity.

## Cross-Cutting Concerns

| Concern | Approach |
|---------|----------|
| Error handling | Return typed diagnostic classes plus bounded strings; reserve errors for invalid caller input or filesystem/setup failures that prevent classification. |
| Observability | Surface concise operator diagnostics such as `semble: model unavailable offline`; do not log raw stdout/stderr or file contents. |
| Configuration | Use only `LIZA_ENABLE_SEMBLE`, `SEMBLE_MODEL_NAME`, `HF_HOME`, and `XDG_CACHE_HOME` from the current process; no durable config. |
| Testing | Use fake runners, temporary directories, and environment overrides to prove command plans, offline env, cache invalidation, diagnostics, target safety, and command rendering. |
| Security | Default ignores include runtime/generated artifacts and credential patterns; diagnostics are bounded and prompts never reveal cache paths or raw outputs. |
| Concurrency | Guard the process-local validation cache with a package mutex; later lifecycle Git exclude serialization remains outside this package. |
| Reversibility | Package creation is local to `internal/semble` and does not alter runtime behavior until later slices wire it in. |

## Decomposition

Each scope becomes a code-planning child task.

### Scope 1: Semble Capability Package

**Component(s):** `internal/semble/`

**Description:** Semble Capability Package: create `internal/semble/doc.go`, `internal/semble/semble.go`, and `internal/semble/semble_test.go` with activation parsing, executable detection, prewarm/offline validation command planning and execution, process-local validation cache, bounded diagnostic classification, prompt-safe context metadata, shell-quoted command examples, target-root safety checks, and the default `.sembleignore` pattern source of truth. Out of scope: wiring into init, worktree creation, prompt templates, SessionStart, operator documentation, durable state config, installing Semble/Python/models, Semble MCP, and remote Git URL indexing.

**Boundary:** In scope: create `internal/semble/doc.go`, `internal/semble/semble.go`, and `internal/semble/semble_test.go` with activation parsing, executable detection, prewarm/offline validation command planning and execution, process-local validation cache, bounded diagnostic classification, prompt-safe context metadata, shell-quoted command examples, target-root safety checks, and the default `.sembleignore` pattern source of truth. Out of scope: wiring into init, worktree creation, prompt templates, SessionStart, operator documentation, durable state config, installing Semble/Python/models, Semble MCP, and remote Git URL indexing.

**Done when:** `go test ./internal/semble` proves `LIZA_ENABLE_SEMBLE` truthy/false parsing, disabled-mode no-op behavior, exact prewarm and offline validation command plans, `HF_HUB_OFFLINE=1` offline execution, process-local cache hit/miss behavior for executable/model/cache/fixture key changes, bounded diagnostics for missing executable/model-offline/execution failure classes, default `.sembleignore` pattern coverage including runtime/generated/credential patterns, target-root safety validation, and prompt-safe command rendering with shell-quoted absolute target roots.

**Depends on:** None.

### Spec Coverage

| Spec Requirement | Scope |
|------------------|-------|
| Strict `LIZA_ENABLE_SEMBLE` activation and disabled no-op behavior | Scope 1 |
| Semble executable detection before prompt guidance | Scope 1 |
| Init-time prewarm command contract using a temporary one-file fixture | Scope 1 |
| Offline validation command contract with `HF_HUB_OFFLINE=1` | Scope 1 |
| Process-local validation cache keyed by executable, model/cache env, timeout, and fixture identity | Scope 1 |
| Graceful Semble degradation with bounded diagnostics | Scope 1 |
| MAS command guidance uses offline Semble invocations | Scope 1 |
| Prompt metadata treats Semble as discovery, not evidence | Scope 1 |
| Shell-quoted absolute target-root command examples | Scope 1 |
| Target-root safety checks for task worktrees and project roots | Scope 1 |
| Default generated `.sembleignore` source of truth, including runtime/generated/credential exclusions | Scope 1 |
| No lifecycle wiring, durable config, installs, MCP, or remote Git URL indexing | Scope 1 |
