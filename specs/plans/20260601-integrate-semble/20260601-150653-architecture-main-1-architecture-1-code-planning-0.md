# Code Plan: Semble Init Prewarm Integration

Status: draft

## Sources Read

- Assigned task `architecture-main-1-architecture-1-code-planning-0`
- Goal spec: `specs/goals/20260601-integrate-semble.md`
- Architecture ref: `specs/arch-plan/20260601-integrate-semble/20260601-135031-architecture-main-1-architecture-1.md`
- Upstream code-planning output: `architecture-main-1-architecture-0-code-planning-0`
- Active upstream coding tasks: `architecture-main-1-architecture-0-code-planning-0-coding-0`, `architecture-main-1-architecture-0-code-planning-0-coding-1`, `architecture-main-1-architecture-0-code-planning-0-coding-2`
- Current source shape: `cmd/liza/cmd_init.go`, `internal/commands/init.go`, `internal/ops/init_project.go`, `internal/models/config.go`
- ADR context: ADR-0021, ADR-0031, ADR-0068, ADR-0074
- Guardrails and invariants: `GUARDRAILS.md`, `INVARIANTS.md`

## Planning Notes

The assigned architecture slice is one feature but spans three independent init surfaces. The implementation is therefore split by integration boundary:

- Task 1 owns operator-facing workspace init in `internal/commands`.
- Task 2 owns programmatic/non-interactive workspace init in `internal/ops`.
- Task 3 owns Cobra/CLI surface regression coverage in `cmd/liza`.

Task 1 and Task 2 can run in parallel after the upstream Semble validation executor task is available because they do not share writable files. Task 3 depends on Task 1 because full `liza init --spec` CLI behavior reaches Semble through `commands.InitCommandWithConfig`.

Doc Impact: none for this code-planning scope. Operator documentation is explicitly out of scope here and owned by sibling task `architecture-main-1-architecture-4-code-planning-0-coding-0`.

Test Impact: each coding task writes or updates tests in its package. Parent validation remains `go test ./internal/commands ./internal/ops ./cmd/liza`.

## Task 1: Operator-Facing Init Semble Prewarm

desc: Semble operator-facing init prewarm: wire `internal/semble` into `internal/commands.InitCommandWithConfig` so full workspace init skips Semble entirely when disabled, performs enabled Semble prewarm plus immediate offline validation after existing hard init preconditions and SCIP init resolution but before `.liza/` scaffold creation, emits bounded stderr diagnostics for missing executable and offline-unready or execution-failure readiness results, keeps Semble readiness non-fatal, preserves existing SCIP init behavior, and does not persist Semble state.

done_when: `go test ./internal/commands` proves `InitCommandWithConfig` performs zero Semble executable lookups, prewarm calls, offline validation calls, and diagnostics when `LIZA_ENABLE_SEMBLE` is unset, empty, `0`, or `false`; with `LIZA_ENABLE_SEMBLE` truthy and a fake `semble` executable present it invokes the upstream `internal/semble` init prewarm/offline validation entry point before `.liza/state.yaml` is written; missing executable, offline-unready model, and generic Semble execution failures emit bounded `semble:` diagnostics on stderr without failing init; generated state YAML and `models.Config` contain no Semble field or readiness marker; existing SCIP init tests for explicit languages, autodetect diagnostics, disabled no-op behavior, and `config.scip_search` persistence still pass unchanged.

scope: In scope: `internal/commands/init.go` and `internal/commands/init_test.go` changes for importing and calling the upstream `internal/semble` init readiness API, placement after branch/spec/pre-commit/global-config validation and SCIP init resolution but before `.liza/` creation, stderr warning presentation, fake Semble readiness hooks in tests, and regression coverage for no durable Semble state plus preserved SCIP init behavior. Out of scope: `cmd/liza` flag registration and dispatch tests, `internal/ops` non-interactive init, `internal/semble` command execution implementation, MAS prompt rendering, worktree `.sembleignore` generation, operator docs, Pairing SessionStart shell hooks, automatic Semble/model installation, user-project build commands, and `.liza/agent-outputs/`.

spec_ref: specs/arch-plan/20260601-integrate-semble/20260601-135031-architecture-main-1-architecture-1.md#interactivecli-workspace-init-internalcommandsinitgo

validation: go test ./internal/commands

depends_on: []

task_depends_on: ["architecture-main-1-architecture-0-code-planning-0-coding-1"]

## Task 2: Programmatic Init Semble Prewarm

desc: Semble programmatic init prewarm: wire `internal/semble` into `internal/ops.InitProject` so non-interactive workspace initialization has the same enabled prewarm and immediate offline validation semantics as CLI init, exposes bounded diagnostics only through a transient caller-provided sink, keeps ops free of terminal I/O, leaves Semble readiness failures non-fatal, and does not persist Semble state.

done_when: `go test ./internal/ops` proves `InitProject` skips Semble lookup, prewarm, offline validation, and diagnostics when `LIZA_ENABLE_SEMBLE` is disabled; with `LIZA_ENABLE_SEMBLE` truthy and a fake `semble` executable present it invokes the upstream `internal/semble` init prewarm/offline validation entry point after branch/spec/pre-commit/global-config validation and before `.liza/` directory creation; missing executable, offline-unready model, and generic Semble execution failures are delivered as bounded transient diagnostics through an optional `InitProjectParams` sink without failing init; `InitProject` never writes Semble diagnostics directly to stdout or stderr; generated state YAML and `models.Config` contain no Semble field or readiness marker.

scope: In scope: `internal/ops/init_project.go` and `internal/ops/init_project_test.go` changes for adding a transient diagnostic sink or callback to `InitProjectParams`, importing and calling the upstream `internal/semble` init readiness API, placement before `.liza/` creation, tests for disabled/enabled/missing/offline-unready/non-blocking behavior, and state serialization assertions. Out of scope: `internal/commands` CLI presentation, `cmd/liza` flag registration and dispatch tests, `internal/semble` command execution implementation, MAS prompt rendering, worktree `.sembleignore` generation, operator docs, Pairing SessionStart shell hooks, automatic Semble/model installation, user-project build commands, and `.liza/agent-outputs/`.

spec_ref: specs/arch-plan/20260601-integrate-semble/20260601-135031-architecture-main-1-architecture-1.md#programmatic-workspace-init-internalopsinit_projectgo

validation: go test ./internal/ops

depends_on: []

task_depends_on: ["architecture-main-1-architecture-0-code-planning-0-coding-1"]

## Task 3: CLI Init Surface Guard

desc: Semble CLI init surface guard: add `cmd/liza` regression coverage proving Semble remains environment-only for `liza init`, no Semble CLI flags are registered, `hasExplicitInitFlags` is unchanged by `LIZA_ENABLE_SEMBLE`, pairing dispatch remains unaffected by Semble environment variables, and full `liza init --spec` reaches the command-layer Semble integration without adding durable flags or config.

done_when: `go test ./cmd/liza` proves the init command has no `--semble`, `--enable-semble`, or Semble-specific flag; setting `LIZA_ENABLE_SEMBLE=true` does not make `hasExplicitInitFlags` return true and does not force pairing-only invocations into full workspace init; the full workspace init command path still forwards only the existing `commands.InitParams` fields; and an enabled `liza init --spec` test exercises the command-layer Semble integration through the Cobra path without introducing a Semble field in state YAML or a Semble flag in help output.

scope: In scope: `cmd/liza/cmd_init_test.go`, `cmd/liza/rootcmd_test_helpers_test.go` if helper reset coverage is needed, and narrowly scoped `cmd/liza/cmd_init.go` changes only if required to preserve the existing dispatch contract while adding tests. Out of scope: registering any Semble CLI flag, adding Semble to `hasExplicitInitFlags`, changing pairing SessionStart behavior, changing `internal/commands` Semble execution semantics beyond Task 1, changing `internal/ops`, MAS prompt rendering, worktree `.sembleignore` generation, operator docs, automatic Semble/model installation, user-project build commands, and `.liza/agent-outputs/`.

spec_ref: specs/arch-plan/20260601-integrate-semble/20260601-135031-architecture-main-1-architecture-1.md#cli-init-dispatch-cmdlizacmd_initgo

validation: go test ./cmd/liza

depends_on: ["0"]

task_depends_on: []

## Shared-File Audit

| File/Area | Planned writer(s) | Dependency decision |
|---|---|---|
| `internal/commands/init.go` | Task 1 | Single writer. |
| `internal/commands/init_test.go` | Task 1 | Single writer. |
| `internal/ops/init_project.go` | Task 2 | Single writer. |
| `internal/ops/init_project_test.go` | Task 2 | Single writer. |
| `cmd/liza/cmd_init.go` | Task 3 only if needed | Single writer. Task 3 depends on Task 1 for behavior, not file overlap. |
| `cmd/liza/cmd_init_test.go` | Task 3 | Single writer. |
| `cmd/liza/rootcmd_test_helpers_test.go` | Task 3 only if helper reset coverage is needed | Single writer. |
| `internal/semble/*` | Upstream coding tasks | Read-only dependency. Task 1 and Task 2 depend on `architecture-main-1-architecture-0-code-planning-0-coding-1`; no writes here. |
| `internal/models/config.go` | None | Read-only verification that no Semble field is added. |

No file appears in more than one planned task's writable scope, so no sibling dependency is required for shared-file conflict avoidance. Task 3 depends on Task 1 for executable behavior through Cobra.

## Dependency Rationale

- Task 1 and Task 2 depend on `architecture-main-1-architecture-0-code-planning-0-coding-1` because that upstream task owns Semble executable lookup, prewarm/offline validation execution, bounded diagnostic classification, and process-local readiness behavior consumed by init.
- Task 3 depends on Task 1 because Cobra full-init tests exercise Semble through `commands.InitCommandWithConfig`.
- Task 1 and Task 2 do not depend on upstream target-safety task `architecture-main-1-architecture-0-code-planning-0-coding-2` because init prewarm validates a controlled temporary fixture and does not render prompt metadata or validate target-root search safety.

## Spec Compliance Matrix

| # | Requirement | Source | Task(s) | Status |
|---|-------------|--------|---------|--------|
| 1 | Semble activation remains strict environment-only through `LIZA_ENABLE_SEMBLE`; no durable `state.yaml` config field is added. | Goal spec `Configuration Shape` lines 270-281; architecture constraints lines 24-34 | Task 1, Task 2, Task 3 | Covered |
| 2 | Disabled Semble does not validate, execute, emit diagnostics, or affect spawned/init behavior. | Goal spec `MVP Scope` lines 144-145; architecture constraints lines 26-30 | Task 1, Task 2, Task 3 | Covered |
| 3 | `liza init --spec` intentionally prewarms Semble when enabled and the `semble` CLI is present. | Goal spec `MVP Scope` lines 148-152; `Offline and Network Requirements` lines 478-493; architecture data flow lines 150-161 | Task 1, Task 3 | Covered |
| 4 | Equivalent non-interactive workspace initialization performs the same enabled prewarm/offline validation semantics. | Assigned scope; architecture `Programmatic Workspace Init` lines 77-89 | Task 2 | Covered |
| 5 | Offline validation runs immediately after init-time prewarm with `HF_HUB_OFFLINE=1`. | Goal spec `MVP Scope` lines 153-155; `Offline and Network Requirements` lines 494-502; architecture components lines 74-75 | Task 1, Task 2 | Covered |
| 6 | Missing executable is classified as Semble unavailable and does not fail init. | Goal spec `Offline and Network Requirements` lines 498-509; architecture components lines 70-75 | Task 1, Task 2 | Covered |
| 7 | Offline-unready model/cache and generic Semble execution failures emit bounded operator-visible diagnostics. | Goal spec `MVP Scope` lines 163-164; `Offline and Network Requirements` lines 499-512; `Security and Safety Requirements` lines 576-577 | Task 1, Task 2 | Covered |
| 8 | Semble readiness failures are non-blocking for workspace initialization. | Goal spec `MVP Scope` line 205; `Success Criteria` line 607; architecture constraints line 30 | Task 1, Task 2 | Covered |
| 9 | Init-time Semble validation uses a controlled temporary fixture and does not index user project roots or task worktrees. | Goal spec `Offline and Network Requirements` lines 482-493; architecture cross-cutting concern line 175 | Task 1, Task 2 | Covered |
| 10 | Init callers consume `internal/semble` package behavior instead of assembling Semble argv, environment, cache, and diagnostics manually. | Architecture `Semble Capability Package` lines 91-103; `Structural Decisions` lines 181-187 | Task 1, Task 2 | Covered |
| 11 | `internal/ops` preserves the no-terminal-I/O boundary while surfacing transient diagnostics. | Architecture `internal/ops -> Operator/Caller Diagnostics` lines 138-145; ADR-0021 lines 19-34 | Task 2 | Covered |
| 12 | No Semble CLI flags are added and `hasExplicitInitFlags` remains unchanged for Semble. | Architecture `CLI Init Dispatch` lines 49-60; constraints lines 26-29 | Task 3 | Covered |
| 13 | Existing SCIP init behavior and tests remain intact. | Architecture constraints line 32; `Init Test Harnesses` lines 112-116 | Task 1 | Covered |
| 14 | Generated state YAML and `models.Config` do not include Semble readiness, model, or config fields. | Goal spec `Configuration Shape` lines 270-296; architecture constraints lines 26-28; assigned done_when | Task 1, Task 2, Task 3 | Covered |
| 15 | Runtime behavior stays stack-agnostic and adds no user-project build command assumptions, Semble installation, or model installation. | Guardrail G1.1; goal spec `Non-Goals` lines 614-624; architecture constraints lines 33-34 | Task 1, Task 2, Task 3 | Covered |
| 16 | Pairing SessionStart hooks, MAS prompt rendering, worktree `.sembleignore`, docs, Semble MCP, remote Git URL indexing, and automatic installs remain out of this scope. | Assigned scope; goal spec `Non-Goals` lines 612-628; architecture decomposition lines 195-215 | Task 1, Task 2, Task 3 | Covered |
| E2E | e2e test coverage for new behavior | Cross-cutting | Task 3 covers the CLI path through `cmd/liza` Cobra tests; Task 1 and Task 2 cover package-level init surfaces. | Covered |
| DOC | Documentation updates for changed behavior | Cross-cutting | N/A: operator documentation is explicitly out of scope for this assigned task and owned by sibling task `architecture-main-1-architecture-4-code-planning-0-coding-0`. | N/A |

## Parent Validation

After all three coding tasks merge, the parent acceptance command is:

```bash
go test ./internal/commands ./internal/ops ./cmd/liza
```

This command is executable from the repository root or the task worktree root and does not violate the shell-shape constraints.
