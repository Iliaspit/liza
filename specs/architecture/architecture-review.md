# Architecture Review — Liza

**Date:** 2026-04-13
**Mode:** Adversarial (after pass 17, entry: new packages)
**Reviewer:** software-architecture-review skill

---

## Update Policy

1. `Phase 3: Recommendations` tracks open issues only.
2. When an issue is fixed, remove it from Recommendations.

---

## Table of Contents

- [Update Policy](#update-policy)
- [Phase 1: Discovery](#phase-1-discovery)
- [1.1 Overview](#11-overview)
- [1.2 Component Walkthrough](#12-component-walkthrough)
- [1.3 Dependency Map](#13-dependency-map)
- [1.4 Coverage Checkpoint](#14-coverage-checkpoint)
- [Phase 2: Analysis](#phase-2-analysis)
- [2.1 Analysis Framework](#21-analysis-framework)
- [2.2 Strengths](#22-strengths)
- [2.3 Smells](#23-smells)
- [2.4 Patterns](#24-patterns)
- [2.5 Test Coverage](#25-test-coverage)
- [Phase 3: Recommendations](#phase-3-recommendations)
- [Summary](#summary)
- [Appendix: File Reference](#appendix-file-reference)

## Phase 1: Discovery

### 1.1 Overview

Liza is a peer-supervised multi-agent coding system built in Go. Agents (Planner, Coder, Code Reviewer) coordinate through a shared YAML blackboard file, with each agent operating in its own terminal via a supervisor loop.

```
Human Input    →    Planner    →    Coder(s)    →    Code Reviewer    →    Merge
     ↓                ↓               ↓                  ↓                 ↓
  vision.md       state.yaml     git worktrees     review verdicts    integration branch
```

**Single binary:** `liza` CLI with 30+ cobra commands. *(health check: MCP server removed in `90c132d5` — see ADR-0057; `liza-mcp` binary deleted)*

**Source size:** ~33,400 LOC production Go, ~87,400 LOC test Go (2.6:1 test-to-code ratio). *(health check: was ~23,100/~55,600 (2.4:1); +44% production, +57% test — TUI, pipeline expansion, ops growth, new agent strategies)*

### 1.2 Component Walkthrough

#### models (`internal/models/`) — ~1,410 LOC *(health check: was ~1,150)*

**Purpose:** Core domain model. Task lifecycle state machine, agent state, sprint tracking.

**Observations:**
- `State` struct is the central data type — serialized to/from `state.yaml`. Split into `state.go` (70), `task.go` (558), `agent.go` (52), `sprint.go` (141), `config.go` (138), `history.go` (211), `diagnostics.go` (230), `lease.go` (8) *(health check: LOC updated; task.go +127, history.go +48, diagnostics.go +28)*
- `Task` struct has 30+ fields covering full lifecycle
- `TaskType` → role workflow registry (`taskWorkflows` map)
- `IsClaimable()` encodes claiming rules with dependency checking
- 12 task statuses with `IsValid()`, `IsTerminal()` methods
- `models` now imports `internal/roles` (both `state.go` and `diagnostics.go`). `roles` is itself a leaf, so this is a shallow dependency, but `models` is no longer the zero-dependency foundation described in earlier passes *(pass 13)*
- `diagnostics_test.go` exists. File grew to 202 LOC *(pass 13)*

#### db (`internal/db/`) — ~530 LOC *(health check: was ~500)*

**Purpose:** Thread-safe YAML state access with file locking via `internal/filelock`.

**Pattern:** Repository pattern — `Blackboard` wraps file I/O with atomic read-modify-write.

**Observations:**
- `Read()`, `ReadCached()` (mtime-based), `Write()`, `Modify()` (atomic closure)
- Atomic write via temp file + fsync + rename — correct durability pattern
- Stale lock detection with PID checking
- `LockError` with 5 classified types (Timeout, Permission, DiskFull, Filesystem, Stale)
- `Watcher` uses fsnotify on directory (not file) to catch atomic renames
- `Metrics` for lock acquisition timing

#### agent (`internal/agent/`) — ~3,870 LOC *(health check: was ~2,580)*

**Purpose:** Supervisor loop, heartbeat, work detection, logging.

**Observations:**
- 18 files: `supervisor.go` (831 LOC), `claiming.go` (448), `registration.go` (358), `prompt.go` (338), `systemctl.go` (311), `workdetection.go` (254), `waitforwork.go` (236), `strategy_reviewer.go` (163), `quota.go` (160), `worktree_check.go` (138), `strategy_orchestrator.go` (132), `strategy_doer.go` (122), `masker.go` (117), `heartbeat.go` (103), `strategy.go` (64), `cli.go` (39), `output.go` (32), `logging.go` (28) *(health check: 7 new files — strategy_*.go (4), quota.go, masker.go, cli.go; supervisor.go 831 (was 535), claiming.go 448 (was 241))*
- `RunSupervisor()` (186 LOC, nesting depth 5): checkAbort → waitWhilePaused → handleApprovedMerges → waitForWork → claimTask → buildPrompt → executeAgent → handleExitCode *(pass 7, Complexity lens: nesting depth noted)*
- `CLIExecutor` interface enables mock testing (supports claude, codex, gemini, vibe, kimi). `DefaultCLIExecutor` concrete implementation hardcodes per-CLI flag formats in a switch statement with `mistral → vibe` rename mapping *(pass 17, Coupling lens)*
- `waitForWorkEventDriven()` (116 LOC) with fsnotify + polling fallback
- `verifyPlannerStateChanges()` (137 LOC) — 6 switch cases with repetitive before/after counting structure *(pass 2, Complexity lens)*
- `heartbeat.go`: independent Blackboard instance, 60s tick, extends lease
- `workdetection.go` (~170 LOC): 6 planner wake trigger types, now declarative via `plannerWakeTriggerSpecs` (trigger → description → state predicate) replacing imperative branching. `DetectOrchestratorWakeTriggers()` is a pure state-query function consumed by both `agent/supervisor.go` and `commands/status.go` (the latter creating a cross-layer dependency) *(pass 14, Boundaries lens)*
- `logging.go`: package-level singleton `slog.Logger`, hardcoded to stdout
- **Core execution paths untested**: `Execute()`, `ExecuteInteractive()`, `handleApprovedMerges()`, `logTaskSubmissionIfCompleted()` at 0% statement coverage; `resumeHandoffTask()` at 11.4%. These are the actual agent loop entry points — tested indirectly via `TestSupervisorBasicLoop` with mock executor but not at statement level *(pass 4, Coverage lens)*
- **`handleApprovedMerges` nesting**: 47 LOC (was 55) with max nesting depth ~4-5 (for-range → if-conditions → if-err → errors.As). Improved with cleaner early returns. Still has nested `IntegrationFailedError` field processing. `resumeHandoffTask` extracted to `ops.ResumeHandoff` *(pass 7, resolved: `ac4ce6f5`)*
- **Role string literals**: `internal/roles` package introduced with `RuntimeCoder`, `RuntimeCodeReviewer`, `RuntimePlanner` constants and `ToWorkflow()`/`ToRuntime()` mapping. All agent/, cmd/, and ops/ files now import role constants *(pass 6, resolved: `a60c72e`)*
- **Duplicated identity validation**: `registration.go:validateIdentity()` reimplements `identity.ValidateFormat()` + `identity.ValidateRole()` — same algorithm (split on last hyphen, validate numeric suffix, check role prefix) without importing the `identity` package *(pass 6, Coupling lens)*
- **Hardcoded `"terminal-1"` and raw `1800`**: `supervisor.go:386` passes `"terminal-1"` literal and `1800` instead of `models.DefaultLeaseDurationSeconds`; `supervisor.go:501` also uses raw `1800` *(pass 6, Coupling lens; pass 13: line numbers updated)*
- **waitForXWork refactored**: 2 generic functions (`waitForWorkEventDriven`, `waitForWorkPolling`) accepting a `workCheckFunc` callback. File is now 213 LOC (was 412). Adding a new role no longer requires new wait functions *(pass 13, resolved)*
- **RoleStrategy pattern** *(Adversarial pass, new packages)*: `strategy.go` defines 7-method `RoleStrategy` interface; factory `NewRoleStrategy()` creates doer/reviewer/orchestrator from pipeline resolver role types. Strategies orchestrate wait→claim→build→execute→post cycles without owning business logic (delegates to ops/claiming/prompt). Reviewer is only strategy with mutable state (`mergeRetries` counter for merge retry loop). `ApplyYAMLTimeouts()` uses type-switch over all three strategy types instead of interface method — code smell
- **quota.go** *(Adversarial pass, new packages)*: Self-contained quota exhaustion detection subsystem. Hardcoded patterns for known providers (claude, codex, gemini, etc.). Signal files in `.liza/` for cross-agent communication. Well-scoped utility, no business logic
- **masker.go** *(Adversarial pass, new packages)*: Allowlist-based secret masking from `os.Environ()`. Filters short values (<8 chars), sorts longest-first for overlapping substrings. Pure stdlib, zero internal deps
- **cli.go** *(Adversarial pass, new packages)*: CLI backend resolution chain (flag → env `LIZA_DEFAULT_CLI` → `DefaultCLI` const). 39 LOC, well-scoped

#### statevalidate (`internal/statevalidate/`) — ~900 LOC *(health check: was ~660)*

**Purpose:** Shared state validation pipeline, extracted from `commands/validate.go` to allow both CLI and ops to run identical validation without import cycles.

**Observations:**
- `ValidateStateFile()` runs 9 validators: required fields, task states, task invariants, dependencies, agent invariants, handoff, discovered, anomalies, sprint
- Accepts `io.Writer` for non-fatal warnings (nil defaults to `io.Discard`)
- Exported shims `ValidateAgentInvariants()` and `ValidateAnomalies()` expose individual validators for existing `commands/` test callsites
- Used by: `commands/validate.go` (CLI `liza validate`), `ops/add_task.go` (post-write validation)
- Split into `validate.go` (127), `validate_task.go` (411), `validate_agent.go` (42), `validate_deps.go` (84), `validate_entity.go` (128), `validate_sprint.go` (88), `validate_roles.go` (23) *(health check: LOC updated; new validate_roles.go)*

#### ops (`internal/ops/`) — ~9,920 LOC production, ~24,820 LOC test *(health check: was ~5,900/~12,070)*

**Purpose:** Pure business logic layer for all task workflow and system operations. Returns structured results with no terminal I/O side effects.

**Pattern:** Service layer — extracted from `commands` to break the agent→commands upward dependency and eliminate MCP protocol corruption risk.

**Observations:**
- 30+ operations covering all mutation commands, including new Phase 2 operations:
  - Task workflow: `ClaimTask`, `ClaimReviewerTask`, `SubmitForReview`, `SubmitVerdict`, `Handoff`, `ResumeHandoff`, `MarkBlocked`, `ReleaseClaim`, `SupersedeTask`, `AddTask`, `CheckDeleteTask`, `DeleteTask`, `Proceed`, `AwaitVerdict`, `AwaitResubmission`, `Replan`, `RecoverTask`, `SetTaskOutput`
  - Agent lifecycle: `DeleteAgent`, `IsAgentProcessRunning`
  - System mode: `Start`, `Stop`, `Pause`, `Resume`
  - Worktree: `CreateWorktree`, `DeleteWorktree`, `MergeWorktree`
  - Sprint: `UpdateSprintMetrics`, `Checkpoint`, `Analyze`, `AdvanceSprint`, `SprintCheckpoint`
  - Init: `InitProject`
  - Maintenance: `ClearStaleReviewClaims`
- Each function returns a typed result struct (e.g., `*VerdictResult`, `*HandoffResult`, `*ModeChangeResult`)
- Zero `fmt.Print*` or `os.Stdin` calls — verified by grep
- Two consumers: `agent/` (orchestration), `commands/` (CLI presentation) *(health check: was three — `mcp/` removed in `90c132d5`)*
- Depends on: `db`, `models`, `git`, `log`, `paths`, `analysis`, `statevalidate`, `pipeline`, `roles`, `identity`, `errors` — same layer as `commands` minus presentation concerns
- `wt_merge.go` (389 LOC): `MergeWorktree` — linear phased flow (validate → merge → integration tests → update state → cleanup). Now logs WARNING when integration test script is missing and persists `tests_ran` in merge history. Tri-state stat handling for test script presence *(pass 7, Complexity lens; `bce626d`, `52ceac5`)*
- `helpers.go` provides `readTaskState()` for Read-path task lookup, but no equivalent exists for the Modify-callback path *(pass 5, Duplication lens)*
- **Structural repetition within ops** *(pass 5, Duplication lens)*: Most ops functions share an identical skeleton — input validation → `paths.New(projectRoot)` + `db.For(lp.StatePath())` → `bb.Modify(func(state) { FindTask + nil check + status check + mutate + history append })` → wrap error → return result. Quantified: `if taskID == ""` guard in 10/21 files, `FindTask + NotFoundError` inside Modify in 10 files, `task.History = append(...)` in 12 files. See Duplication smell below.
- **Inconsistent parameter conventions** *(pass 6, Coupling lens)*: Some ops functions take `projectRoot` and internally construct `paths.New()` + `db.For()` (ClaimTask, MergeWorktree, DeleteTask, SubmitReview, etc.), while others take `statePath`/`logPath` directly (AddTask). Callers must know which convention each function uses. See Coupling smell below.
- **6 new ops files** *(Adversarial pass, new packages)*: `await_verdict.go` (445), `advance_sprint.go` (444), `await_resubmission.go` (403), `init_project.go` (279), `replan.go` (274), `recover_task.go` (246). All follow established patterns (projectRoot param, loadResolver, bb.Modify, FindTask guard). `await_verdict` and `await_resubmission` implement event-loop + polling fallback with ~40% structural overlap (see Duplication smell below). `recover_task` has well-documented multi-phase recovery (read → git cleanup → state cleanup). `advance_sprint` uses plan/apply separation with archive-before-mutate inside Modify() lock
- **Pipeline config loaded per-operation from disk** *(pass 13, Complexity lens; health check: count updated)*: `loadResolver(projectRoot)` called from 54 sites in ops files — each operation independently reads and parses `pipeline.yaml` via `pipeline.LoadFrozen()`. A multi-step workflow (claim → build prompt → execute → submit → review → merge) reads the same file 6+ times. The overhead is negligible (small YAML file), but the pattern prevents session-level caching if performance becomes a concern. *(health check: grew from 16+ to 54 sites — magnitude tripled with ops expansion)*

#### commands (`internal/commands/`) — ~5,300 LOC *(health check: was ~4,200)*

**Purpose:** CLI presentation wrappers over `ops/` business logic, plus read-only query commands.

**Pattern:** Thin wrapper per command: call `ops.*`, format and print result. Read-only commands (inspect, status, validate) retain their own logic since they already return structured data.

**Observations:**
- 30+ command implementations — mutation commands are thin wrappers (~20-75 LOC each), read-only commands retain logic
- `watch.go` (846 LOC): health checks with alert deduplication, comprehensive monitoring *(health check: was 516)*
- `validate.go` (28 LOC): thin wrapper delegating to `internal/statevalidate` package *(pass 2; pass 7: LOC updated; `6fe5bcc`: validation logic extracted to shared package)*
- `format.go` (164 LOC): centralized JSON/YAML/table formatting
- Templates in `commands/templates/`: status_dashboard, agent_value, metrics_value
- **Self-constructing infrastructure** — each command function creates fresh `paths.New()`, `db.New()`, `git.New()` instances internally; no dependency injection *(pass 3, Boundaries lens)*
- **Watch thresholds hardcoded** — 10 constants (`DefaultCheckInterval`, `LeaseGracePeriod`, `StallThreshold`, etc.) with no path to `models.Config`. Operationally tunable parameters hardcoded in source *(pass 6, Coupling lens)*
- **Imports `internal/agent`** — `status.go:282` calls `agent.DetectOrchestratorWakeTriggers()` for wake trigger display. This breaks the intended peer relationship where `commands` and `agent` are both consumers of `ops`. See boundary smell *(pass 14, Boundaries lens)*

#### cmd (`cmd/`) — ~3,090 LOC *(health check: was ~1,530)*

**Purpose:** Binary entry point (single binary).

**Observations:**
- Split into `main.go` + 6 `cmd_*.go` files: `cmd_task.go` (910), `cmd_system.go` (683), `cmd_review.go` (436), `cmd_init.go` (363), `cmd_agent.go` (241), `cmd_worktree.go` (123). Total ~3,089 LOC across 7+ files. Business logic delegates to `commands` package *(health check: cmd_task.go doubled, cmd_system.go +40%)*
- `cmd/liza-mcp/` removed *(health check: MCP server deleted in `90c132d5`)*

#### mcp — REMOVED *(health check: entire `internal/mcp/` and `cmd/liza-mcp/` deleted in `90c132d5`; see ADR-0057)*

Agents now interact via CLI `--json` mode instead of MCP JSON-RPC. All MCP findings from prior passes are stale.

#### git (`internal/git/`) — ~740 LOC *(health check: was ~590)*

**Purpose:** Git worktree and branch operations.

**Observations:**
- `CreateWorktree()`, `RemoveWorktree()`, `MergeBranch()` (ff then no-ff), `RebaseOnto()`
- Centralized `runGit()` / `runGitCombined()` helpers
- `CalculateDrift()` for worktree-to-main divergence measurement

#### prompts (`internal/prompts/`) — ~600 LOC + 9 templates *(health check: was ~633 + 14 templates)*

**Purpose:** Role-specific prompt generation using Go `text/template`.

**Observations:**
- Template-driven: all text in `.tmpl` files, clean logic/text separation
- 9 templates: base prompt, 8 wake triggers *(health check: was 14 — restructured; templates in `templates/` dir)*
- `executeTemplate()` panics on error rather than returning it
- `PlannerContextConfig` replaced by `CodePlannerContextConfig`, `EpicPlannerContextConfig`, etc. with actual fields for Phase 2 roles *(pass 13, resolved)*
- Template execution pattern (embed.FS + funcMap + template.Must + executeTemplate) is duplicated nearly identically in `commands/templates.go` *(pass 3, Boundaries lens)*
- **Imports `internal/ops`** for 5+ utility functions: `LoadDetectionContext`, `IsPlanningPair`, `IsPlanningCompleteEligible`, `IsTransitionCycleBlocked`, `CountReadyManyToOneCohorts`, `ManyToOneTransitionInfo`. Dependency deepened since pass 14 (was 3 functions) — architecturally inverted (see boundary smell) *(pass 14, Boundaries lens; health check: worsened)*

#### embedded (`internal/embedded/`) — ~640 LOC *(health check: was ~500)*

**Purpose:** `go:embed` for contracts and skills, Claude settings management.

**Observations:**
- Synced from source via `make sync-embedded` before build
- `WriteClaudeSettings()` merges with existing settings; accepts `io.Reader` parameter
- Frontmatter management for CLAUDE.md files
- `PlanGlobalFiles()` — previously at 0% coverage due to stdin coupling *(pass 4, Coverage lens)*; stdin coupling now resolved via `io.Reader` injection
- `consistency_test.go` (126 LOC): byte-exact comparison of repo masters vs embedded copies. Wired into `make check-embedded` → `make lint`
- *(health check: `WriteMCPSettings()`, `mergeMCPSettings()` removed with MCP server)*

#### paths (`internal/paths/`) — ~346 LOC *(health check: was ~276)*

**Purpose:** Path resolution with worktree awareness.

**Observations:**
- `GetProjectRoot()` via `git rev-parse --show-toplevel`
- `ValidateTaskID()` with path traversal protection
- `TaskBranchPrefix = "task/"` constant — single source of truth for branch naming *(added: `59a8e3e`)*
- All standard `.liza/` paths centralized

#### pipeline (`internal/pipeline/`) — ~1,170 LOC *(health check: was 641)*

**Purpose:** Declarative pipeline configuration — types, parsing, validation, and state resolution for multi-stage agent workflows defined in YAML.

**Pattern:** Configuration + Resolver — `PipelineConfig` defines the static structure (agent roles, role-pairs with 6-state lifecycles, sub-pipelines, transitions, entry points); `Resolver` wraps a validated config for runtime queries (status lookup, transition maps, sprint terminal states).

**Observations:**
- `config.go` (488 LOC): YAML parsing with strict mode (`KnownFields(true)`), comprehensive validation *(health check: was 317 — +54%)*
- `resolver.go` (642 LOC): Query interface over validated config — `TransitionMap()` generates full state machine, `AvailableTransitions()` filters by status + executed set, `SprintTerminalStates()` with lazy-cached `TransitionSourcePairs()` *(health check: was 324 — doubled)*
- `migrate.go` (37 LOC): Pipeline migration support *(health check: new file)*
- `LoadFrozen()` loads from `.liza/pipeline.yaml`
- Imports only `models` — clean leaf-adjacent position in dependency graph
- 7 consumers: `ops/`, `agent/` (registration), `commands/` (init), `statevalidate/`, `prompts/`
- Well-tested: 3,613 LOC tests (3.1:1 ratio) *(health check: was 1,569 — test coverage investment increased)*

#### tui (`internal/tui/`) — ~2,265 LOC *(Adversarial pass, new packages)*

**Purpose:** Interactive terminal dashboard for agent monitoring via `liza system watch`.

**Pattern:** Bubbletea Model-Update-View — message-driven update loop with stateless rendering.

**Observations:**
- 6 files: `view.go` (688), `update.go` (643), `commands.go` (342), `styles.go` (251), `model.go` (241), `keymap.go` (100)
- Model struct has 47 fields (typical for TUI state)
- 11 keybindings: spawn/terminate agents, pause/resume/stop system, add tasks, checkpoint, help
- Responsive layout: 4 column tiers (≥80/120/160 cols)
- Form-based task creation via Huh library
- Alert management with 10s auto-dismiss, 200-entry activity feed cap
- Imports: `agent`, `db`, `log`, `models`, `paths`, `commands`, `ops`, `pipeline`, `process`, `render`
- Only consumer: `cmd/liza/cmd_system.go` — good isolation, no reverse dependencies
- **Boundary compliance**: All state mutations delegate to ops/commands; no business logic in UI layer
- **Large functions**: `Update()` 161 LOC (~31 control flow paths), `renderTaskPanel()` 199 LOC
- **Well-tested**: 172 tests across 6 test files (view, update, commands, styles, keymap, model)
- External deps: Bubbletea, Bubbles, Huh, Lipgloss

#### New small packages *(Adversarial pass, new packages)*

- `interactive/` (~238 LOC, ~109 test): Init wizard for pairing/full mode setup. Imports only `commands` (for symlink mapping). Consumer: `cmd/liza/cmd_init.go`. Pure UI layer, clean boundary. Minor: hardcoded `"specs/vision.md"` path
- `render/` (~175 LOC, ~234 test): Output formatting (JSON, YAML, tables, durations, templates). Zero internal imports — pure formatter. Consumers: `commands/inspect_*.go`, `tui/view.go`. Excellent boundary
- `jsonout/` (~149 LOC, ~275 test): JSON response wrapping and error classification for `--json` mode. Imports `errors` and `ops` (for error type assertions in `ClassifyError()`). Consumer: `cmd/liza/main.go`. Acceptable boundary crossing — intentional error type mapping
- `process/` (~67 LOC, no tests): Detached subprocess spawning with platform-specific attributes (`procattr_unix.go`, `procattr_windows.go`). Zero internal imports. Consumer: `tui/commands.go`. No test file *(Adversarial pass: test gap)*
- `gitenv/` (~28 LOC, no tests): Forces `LC_ALL=C` for git commands to normalize locale-dependent output. Zero internal imports. Consumers: `git/`, `statevalidate/`. No test file *(Adversarial pass: test gap)*

#### Other leaf packages

- `log/` (~210 LOC): YAML append log with flock (via shared `filelock` package). Now uses append-only writes (no O(n) rewrite) and bounded tail-window `GetLastTimestamp()` for sub-linear reads *(perf: `fe8de6b`)*
- `filelock/` (~490 LOC): Shared file-locking with flock, diagnostic owner metadata, error classification, metrics
- `analysis/` (~224 LOC): Circuit breaker pattern detection (6 patterns)
- `identity/` (~123 LOC): Agent ID resolution and validation *(health check: was ~108)*
- `errors/` (~70 LOC): Exit codes and `NotFoundError` type (with `Entity`, `ID`, `Field` fields) *(health check: was ~45)*
- `roles/` (~90 LOC): Role constants with runtime↔workflow mapping *(health check: was ~60 — grew with Phase 2 roles)*
- `testhelpers/` (~784 LOC): Fixtures, git setup, assertions, utilities *(health check: was ~700)*

### 1.3 Dependency Map

```
models/ (stable, near-leaf → roles)  paths/ (stable, leaf)
   ↑                                   ↑
   ├── db/                             ├── db/
   ├── agent/                          ├── agent/
   ├── commands/                       ├── commands/
   ├── ops/                            ├── ops/
   ├── prompts/                        ├── git/
   ├── analysis/                       └── embedded/
   └── testhelpers/

errors/ (stable, leaf)              log/ (stable, leaf)
   ↑                                   ↑
   ├── db/                             ├── commands/
   ├── ops/                            └── ops/
   ├── agent/
   ├── commands/
filelock/ (stable, leaf)
   ↑
   ├── db/
   └── log/

git/ (volatile)                     prompts/ (stable)
   ↑                                   ↑
   ├── commands/                       └── agent/
   └── ops/

pipeline/ (stable, near-leaf — imports only models)
   ↑
   ├── ops/ (release_claim, pipeline_ops, proceed)
   ├── agent/ (registration)
   ├── commands/ (init)
   ├── statevalidate/
   └── prompts/

db/ (stable core)
   ↑
   ├── agent/
   ├── commands/
   ├── ops/
   └── testhelpers/

ops/ (service layer — pure logic, no I/O)
   ↑
   ├── agent/ (orchestration — uses structured results)
   └── commands/ (CLI — adds presentation)

commands/ (volatile, high-level — also imports agent/)
   ↑
   └── cmd/liza (CLI entry point)

prompts/ (stable — imports ops for queries, see boundary smell)
   ↑
   └── agent/

tui/ (volatile, high-level)    jsonout/ (stable, leaf)
   ↑                              ↑
   └── cmd/liza                    └── agent/
```

**No import cycles.** Dependency graph is a clean DAG. Leaf packages: `paths`, `errors`, `filelock`, `identity`, `jsonout`, `gitenv`, `process`, `render`. Near-leaf: `models` (imports only `roles`), `roles` (leaf). *(health check: `mcp/protocol` removed; new leaf packages added)*

**Single consumer of `commands`**: CLI (`cmd/liza`). *(health check: was two consumers — MCP server removed in `90c132d5`)*

### 1.4 Coverage Checkpoint

**What exists that shouldn't?**
- `commands/format.go` has bubble-sort for map keys (functional but O(n^2); `sort.Strings` exists)
- `dashboardSection` type with `"table"` format case is a no-op (line 155: just appends empty string)

**What's implicit that should be explicit?**
- The "Blackboard must remain stateless beyond cache" constraint (documented in architectural-issues.md)
- The contract between `commands` and its consumers — now single-transport (CLI) since MCP removal *(health check: was three transports)*

**What's missing from the walkthrough?**
- `db/metrics.go` (113 LOC): lock timing metrics — read and noted
- `commands/status.go` (469 LOC): status dashboard rendering — read via templates

**What requires cross-file comparison?**
- `derefString()` in prompts/builder.go duplicates `deref` template function
- Template execution pattern in `commands/templates.go` vs `prompts/templates.go` — nearly identical: embed.FS + funcMap with `deref` + template.Must + executeTemplate that panics *(pass 3, Boundaries lens)*
- **Worktree path helper not reused** *(pass 16, Duplication lens)*: `git.GetWorktreeRelPath(taskID)` at `worktree.go:304` is unused by 4 other call sites that inline the same `filepath.Join(paths.WorktreesDirName, taskID)` computation
- **Ops Modify-callback boilerplate** *(pass 5, Duplication lens)*: `FindTask(taskID) + nil→NotFoundError` inside `bb.Modify` callbacks appears in 10 production files. The existing `readTaskState()` helper only works outside callbacks. The guard is identical in every file — see Smell below.
- **Ops input validation** *(pass 5, Duplication lens)*: `if taskID == "" { return nil, fmt.Errorf("task ID is required") }` in 10/19 ops files; `if agentID == ""` in 7/19. Each function validates its own required parameters independently.
- **Command test harness** *(pass 5, Duplication lens)*: 82 table-driven test loops across 34 command test files, 23 `initialState := &models.State{...}` constructions. The loop body (~15 lines: tmpDir → SetupLizaDir → create state → setup → WriteInitialState → call command → check error → validate state) is structurally identical.
- **Test setup sequence** *(pass 5, Duplication lens)*: 625 occurrences of `testhelpers.{SetupLizaDir|CreateValidState|WriteInitialState}` across 55 test files. The 3-4 line setup is per-test-function, not per-file.
- **Ops parameter convention split** *(pass 6, Coupling lens)*: `ClaimTask`, `MergeWorktree`, `DeleteTask`, `SubmitReview`, `Start`, `Stop`, `Pause`, `Resume`, `CreateWorktree`, `DeleteWorktree` take `projectRoot`; `AddTask` takes `statePath`/`logPath` directly — callers must track which convention each function uses
- **Identity validation duplicated** *(pass 6, Coupling lens)*: `agent/registration.go:validateIdentity()` reimplements `identity.ValidateFormat()` + `identity.ValidateRole()` without importing the package

**Coverage lens statement-level data** *(pass 4)*:

| File/Area | Statement Coverage | Notes |
|-----------|-------------------|-------|
| **Total** | **75.3%** | From `go tool cover -func` |
| `supervisor.Execute/ExecuteInteractive` | 0% | Core agent loop entry points |
| `supervisor.handleApprovedMerges` | 0% | Merge orchestration |
| `supervisor.resumeHandoffTask` | 11.4% | Complex handoff logic |
| `supervisor.RunSupervisor` | 54.4% | Main loop |
| `embedded.WritePipelineConfig` | 0% | I/O-coupled |
| `validate.validateAnomalies` | 13.3% | Only first branch tested |
| `validate.validateHandoff` | 33.3% | |
| `inspect_field.getSprintMetricsField` | 29.4% | |

**Files without any test file** *(pass 4, Coverage lens; health check: MCP entries removed)*:
- `internal/prompts/templates.go` (42 LOC) — template execution
- `internal/gitenv/gitenv.go` (28 LOC) — git environment *(health check: new package, no tests)*
- `internal/process/` (67 LOC) — process spawning *(health check: new package, no tests)*

**Complexity lens metrics** *(pass 2; pass 7: updated with ops files, nesting depth, function LOC; health check: LOC updated)*:

| File | LOC | Longest Function (LOC) | Max Nesting Depth | Branch Density (ifs/LOC) | Notes |
|------|-----|----------------------|-------------------|------------------------|-------|
| **ops/proceed.go** | **1,200** | — | **4** | — | **Largest file — god file candidate** *(health check: was 533 — +125%)* |
| cmd_task.go | 910 | — | 2 | — | CLI task command wiring *(health check: was 275 — tripled)* |
| commands/init.go | 854 | — | — | — | **New god file candidate** *(health check: new entry)* |
| watch.go | 846 | — | 3 | 1:10.4 (62 ifs) | Health checks *(health check: was 645)* |
| supervisor.go | 831 | RunSupervisor (186) | 5 | 1:8.0 (80 ifs) | Main event loop *(health check: was 637)* |
| cmd_system.go | 683 | — | 2 | — | CLI system command wiring *(health check: was 489)* |
| **ops/claim_task.go** | **563** | **ClaimTask (~345)** | **6** | **1:7.3 (90 ifs)** | **Highest density** *(health check: was 655 — shrank, improved)* |
| **ops/wt_merge.go** | **520** | **MergeWorktree (189)** | **4** | — | **Linear but many error-handling paths** *(health check: was 458)* |
| claiming.go | 448 | — | 5 | — | *(health check: was 240 — grew with merge handling)* |
| ops/await_verdict.go | 445 | — | — | — | *(health check: new file)* |
| ops/advance_sprint.go | 444 | — | — | — | *(health check: new file)* |
| inspect_field.go | 276 | — | 3 | — | **9 switch statements** — manual reflection *(health check: was 327 — shrank)* |

**Boundaries lens import analysis** *(pass 3)*:

| Package | Internal Imports | External Imports | Consumers |
|---------|-----------------|------------------|-----------|
| `models` | 1 (roles) | 0 | 6 packages *(pass 13: was 0 internal imports — now imports `roles`)* |
| `paths` | 0 | 0 | 6 packages |
| `errors` | 0 | 0 | 6 packages (ops, db, agent, commands, testhelpers, prompts) *(health check: mcp removed)* |
| `identity` | 0 | 0 | 1 binary |
| `filelock` | 0 | flock | 2 packages |
| `log` | filelock | yaml.v3 | 1 package |
| `db` | models, filelock | fsnotify, yaml.v3 | 3 packages |
| `git` | paths | 0 | 1 package |
| `embedded` | paths | 0 | 1 (commands) *(health check: liza-mcp removed)* |
| `analysis` | models | yaml.v3 | 1 package |
| `pipeline` | models | yaml.v3 | 7 (ops, agent, commands, statevalidate, prompts) *(health check: updated)* |
| `prompts` | models, **ops**, pipeline | 0 | 1 package *(pass 14: ops import was undocumented — see boundary smell below; health check: ops dependency deepened)* |
| `statevalidate` | db, models, pipeline | 0 | 2 (ops, commands) |
| `ops` | **11 packages** (analysis, db, errors, git, identity, log, models, paths, pipeline, roles, statevalidate) | 0 | 3 (agent, commands, prompts) *(health check: was 4 — mcp removed)* |
| `commands` | **12 packages** (agent, analysis, db, embedded, errors, log, models, ops, paths, pipeline, statevalidate) | yaml.v3 | 1 (liza) *(health check: was 2 — mcp removed)* |
| `agent` | **9+ packages** (db, errors, git, jsonout, models, ops, paths, pipeline, prompts, roles) | 0 | 2 (commands, liza) *(health check: added jsonout)* |

---

## Phase 2: Analysis

### 2.1 Analysis Framework

| # | Question | Assessment |
|---|----------|------------|
| 1 | **Problem being solved?** | Multi-agent coordination for coding tasks with peer review and human oversight |
| 2 | **Change vectors?** | New task types (stable: models, volatile: commands/supervisor), new agent providers (volatile: supervisor.CLIExecutor), new CLI commands (volatile: cmd/commands) *(health check: MCP tools vector removed)* |
| 3 | **Constraints?** | Solo developer, Go CLI, filesystem-based state, no external services |
| 4 | **Cost of being wrong?** | State corruption is costly (manual recovery). Code changes are reversible (git). Agent misconfiguration wastes compute. |
| 5 | **Error handling?** | Errors propagate via Go conventions. Lock errors classified. State mutation errors surface to supervisor. Partial failures can leave state stuck but not corrupt (flock protection). |
| 6 | **Expected lifespan?** | Active development, evolving. v1 with accepted limitations documented. |
| 7 | **Concurrency model?** | Multiple supervisor processes, single shared file with flock. No in-process concurrency beyond heartbeat goroutine. |
| 8 | **Data ownership?** | `state.yaml` owned by Blackboard. Git state owned by worktree operations. Prompts are read-only derived. |
| 9 | **Boundaries?** | Domain layer (`models`) clean. Persistence layer (`db`) clean. CLI entry (`cmd`) clean. Service layer (`ops`) clean — pure business logic, no I/O. Commands are thin presentation wrappers; agent imports `ops`. New `tui/` layer for terminal UI. *(health check: MCP transport removed; TUI added)* |
| 10 | **Runtime constraints?** | Filesystem I/O bound. Lock contention under concurrent agents. Git operations can be slow on large repos. |

### 2.2 Strengths

#### Clean Dependency Architecture

Dependencies flow inward toward stability. No import cycles. Leaf packages (`models`, `paths`, `errors`) have zero internal dependencies. The `commands` package is correctly positioned as a high-level orchestrator. This enables confident refactoring — changing a leaf package has bounded impact.

#### Excellent Test Infrastructure

2:1 test-to-code ratio with consistent patterns: table-driven tests, filesystem isolation via `t.TempDir()`, real git repos for integration, lightweight hand-written mocks. The `testhelpers` package (733 LOC) provides reusable primitives (`SetupLizaDir`, `CreateValidState`, `WriteInitialState`, `BuildTaskByStatus`, assertion helpers) used across 55 test files (625 call sites). Integration tests in `internal/integration/` verify complete workflows. All `internal/` packages have tests. *(pass 5: quantified testhelpers usage)*

#### Atomic State Persistence

The `Blackboard.Modify()` pattern (read-lock-mutate-write-unlock) combined with temp file + fsync + rename provides correct durability guarantees. The `ReadCached()` mtime-based invalidation avoids unnecessary file reads. This is the right level of complexity for a file-based coordination mechanism.

#### Comprehensive Health Monitoring

`watch.go` implements 11 distinct health checks (expired leases, blocked tasks, orphaned rejections, review loops, integration failures, hypothesis exhaustion, reassignment, approaching limits, stall detection, stale drafts, immediate discoveries) with alert deduplication and throttling. This provides operational visibility appropriate for a system that runs unattended.

#### Template-Driven Prompt Generation

All prompt text lives in `.tmpl` files, cleanly separated from Go logic. 14 templates cover all role contexts and wake trigger types. Adding new prompt content requires no Go code changes. The `prompts` package depends only on `models` — minimal coupling.

#### Well-Classified Error System

The `LockError` taxonomy (5 types with `classifyLockError()` mapping syscall errors) provides actionable error information at system boundaries. Error wrapping with `%w` is used consistently.

#### Clean Strategy Pattern for Role-Specific Behavior *(Adversarial pass, new packages)*

The `RoleStrategy` interface (7 methods) cleanly separates role-specific orchestration from the supervisor loop. Three implementations (doer, reviewer, orchestrator) are created via factory based on pipeline resolver role types. Strategies delegate all business logic to ops/claiming/prompt — they orchestrate, not implement. Adding a new role type requires only a new strategy implementation, no supervisor changes. Test coverage includes dedicated strategy tests + indirect testing through supervisor integration.

#### Well-Structured TUI with Clean Boundaries *(Adversarial pass, new packages)*

The `tui` package (2,265 LOC, 172 tests) follows the Bubbletea Model-Update-View pattern precisely. Stateless rendering (View derived entirely from Model), non-blocking async I/O (all blocking ops in tea.Cmd closures), and proper delegation to ops/commands for all state mutations. No business logic leakage. 47-field Model struct is large but typical for interactive TUI state. Good isolation — only consumed by `cmd/liza/cmd_system.go`.

### 2.3 Smells

#### Smell: Hardcoded configuration — magic number 1800 *(mostly resolved — residual sites, pass 6)*

**Signal:** `leaseDuration = 1800` appeared as a fallback default in 3 locations, plus 6 more magic numbers in wait config (now `RoleStrategy.WaitConfig`).

**Fix:** Defined `DefaultLeaseDurationSeconds` and `Default{Coder,Planner,Reviewer}{PollInterval,MaxWait}` constants in `internal/models/state.go` alongside `Config`. `heartbeat.DefaultLeaseDuration` derives from `models.DefaultLeaseDurationSeconds`.

**Residual** *(pass 6, Coupling lens)*: `supervisor.go:127` (`registerAgent(..., 1800)`) and `supervisor.go:221` (`claimReviewerTask(..., 1800, ...)`) still use raw `1800` instead of `models.DefaultLeaseDurationSeconds`. These were missed during the original extraction.

#### Smell: Untested critical execution paths *(pass 4, Coverage lens — partially resolved)*

**Signal:** The system's most critical runtime paths have 0% statement coverage:
- `supervisor.Execute()` and `ExecuteInteractive()` — the actual agent execution entry points that build `exec.Cmd`, set stdin/stdout, run the CLI, and handle exit codes
- `supervisor.handleApprovedMerges()` — orchestrates post-approval merge workflow
- `supervisor.logTaskSubmissionIfCompleted()` — completion logging

**Impact:** The tested code (helpers, validators, work detection) is exercised thoroughly, but the code that wires it all together at runtime has no direct tests. This creates a "tested parts, untested whole" pattern. If `Execute` mishandles an exit code, the supervisor loop misbehaves. The remaining untested paths are I/O-coupled functions requiring injection seams.

The root cause is I/O coupling: functions at 0% are precisely those with hardwired `os.Stdin`, `os.Stdout`, or `os/exec.Command`. The `CLIExecutor` interface in supervisor was the right move — but it was the only such seam created.

**Direction:** For `supervisor.Execute`/`ExecuteInteractive`: already abstracted behind `CLIExecutor` interface, which is mocked in `TestSupervisorBasicLoop`, but the `DefaultCLIExecutor` concrete implementation is untested. *(health check: MCP server paths removed — no longer relevant)*

#### Smell: Ops callback boilerplate — FindTask + guard + history *(pass 5, Duplication lens)*

**Signal:** Inside `bb.Modify` callbacks, 10 of 19 ops files repeat identical 3-line task lookup:
```go
task := state.FindTask(taskID)
if task == nil {
    return &errors.NotFoundError{Entity: "task", ID: taskID}
}
```
The `readTaskState()` helper (helpers.go) only works outside callbacks (it calls `bb.Read()`). Inside callbacks, no helper exists.

Additionally, `task.History = append(task.History, models.TaskHistoryEntry{Time: now, Event: "...", Agent: &agentID})` appears in 12 locations with minor variations (some add `Reason`, `Note`, `PreviousAssignee`).

**Impact:** Low-medium. Each occurrence is small (3-5 lines), and the variations in history entries make full extraction non-trivial. The repetition is coincidental similarity rather than copy-paste — each file was independently authored with the same pattern. Risk: if the guard pattern changes (e.g., adding logging on not-found), 10 files need updating.

**Direction:** A `modifyTask(bb, taskID, func(state, task) error) error` helper could encapsulate the Modify+FindTask+guard pattern. History append could get a `task.RecordEvent(time, event, agentID, opts...)` method. However, the current repetition is idiomatic Go — evaluate whether the abstraction adds clarity or obscures intent.

#### Smell: `validateTaskInvariants` monolithic if-chain *(pass 7, Complexity lens)*

**Signal:** 142 LOC, ~15 sequential `if task.Status == X && field == nil` checks with no grouping. Each status's invariants are scattered across the function rather than grouped by status.

**Impact:** Low. The function is simple despite its length — each check is independent and self-documenting. However, the lack of grouping means a developer adding a new status must scan the entire function to ensure all required invariants are covered.

**Direction:** Group checks by status (all IMPLEMENTING invariants together, all REVIEWING together, etc.) — or use a `switch task.Status` with per-status validation functions. Not urgent; the function is correct and readable despite its length.

#### Smell: High nesting depth in `claiming.go` *(pass 7, Complexity lens)*

**Signal:** `handleApprovedMerges` (55 LOC) reaches nesting depth 6: `for range → if status → if approved → if merge_commit → if err → errors.As`. `resumeHandoffTask` (63 LOC) reaches depth 5 inside its `bb.Modify` closure. Both functions are short enough that the nesting doesn't create horizontal scroll, but the depth-to-LOC ratio signals tightly packed control flow.

**Impact:** Low. Both functions are well-commented and the deep nesting follows natural error-handling patterns (check condition → attempt operation → classify error). The `handleApprovedMerges` pattern is particularly common in Go error handling.

**Direction:** `handleApprovedMerges` could extract the inner merge-attempt body into a `tryMergeTask(projectRoot, task, agentID) error` helper, reducing the for-loop body to: filter + call + log. Low priority.

#### Smell: Ops input validation boilerplate *(pass 5, Duplication lens)*

**Signal:** `if taskID == "" { return nil, fmt.Errorf("task ID is required") }` appears in 10 ops files. `if agentID == ""` appears in 7 files. Each function independently validates required string parameters with identical code.

**Impact:** Low. The validation is trivial (1-3 lines per parameter) and self-documenting. The "cost" is mostly visual noise rather than maintenance risk.

**Direction:** A validation helper (`requireNonEmpty("task ID", taskID)`) or struct-based input with a `Validate()` method could reduce noise, but this is borderline — idiomatic Go favors explicit validation at function entry. Not worth abstracting unless the pattern grows beyond simple emptiness checks.

#### Smell: Command test harness repetition *(pass 5, Duplication lens)*

**Signal:** 34 command test files share a structurally identical test loop body (~15 lines): create tmpDir → `SetupLizaDir` → construct `initialState` with common config → apply `setupState` → `WriteInitialState` → call command → check error → validate state. The `initialState` construction (Config fields: IntegrationBranch, LeaseDuration, etc.) is repeated 23 times with near-identical values.

**Impact:** Low. This is standard Go table-driven test boilerplate. The `testhelpers` package already extracts the reusable primitives. Further abstraction would need to handle the variety in command signatures (different parameter sets per command) — a generic harness would trade boilerplate for indirection.

**Direction:** A `testhelpers.RunCommandTest(t, CommandTestCase{...})` helper could encapsulate the common loop body, accepting the command-under-test as a function parameter. Alternatively, a `testhelpers.DefaultState()` function returning a pre-configured `*models.State` (with standard Config values) would eliminate the 23 repeated `initialState` constructions. The latter is lower-risk and higher-value.

#### Smell: Duplicated identity validation *(pass 6, Coupling lens; health check: corrected — NOT dead code)*

**Signal:** `agent/registration.go:validateIdentity()` reimplements the same algorithm as `identity.ValidateFormat()` + `identity.ValidateRole()` without importing the `identity` package. Both: split on last hyphen, validate numeric suffix with `strconv.Atoi`, check role prefix match. Called from `supervisor.go:558` and has tests in `registration_test.go`. *(health check: prior verification pass incorrectly labeled this as dead code — it is actively used)*

**Impact:** Low-medium. If identity format rules change (e.g., allowing non-numeric suffixes), two implementations need updating independently. The `identity` package is the canonical source but the `agent` package doesn't know it exists.

**Direction:** Replace `validateIdentity()` call with `identity.ValidateFormat()` + `identity.ValidateRole()`. The `identity` package already returns structured errors.

#### Smell: Inconsistent ops parameter conventions *(pass 6, Coupling lens)*

**Signal:** Most ops functions take `projectRoot string` and internally construct `paths.New(projectRoot)` + `db.For(lp.StatePath())` (e.g., `ClaimTask`, `MergeWorktree`, `DeleteTask`, `Start`, `Stop`). But `AddTask` takes `statePath, logPath string` directly — the caller must construct these paths and pass them in.

**Impact:** Low-medium. Callers must know which convention each function uses. The inconsistency creates a maintenance tax: if path derivation logic changes, functions using `projectRoot` auto-adapt (via `paths.New()`), but `AddTask` callers must update manually. New ops functions must decide which convention to follow with no documented guidance.

**Direction:** Standardize on `projectRoot` parameter convention (majority pattern). Migrate `AddTask` to take `projectRoot` and derive paths internally, consistent with all other ops functions.

#### Smell: Watch thresholds not configurable *(pass 6, Coupling lens)*

**Signal:** `commands/watch.go` defines 10 hardcoded constants:
- `DefaultCheckInterval = 10s`, `LeaseGracePeriod = 120s`, `StallThreshold = 30m`
- `StaleDraftThreshold = 30m`, `CheckpointStaleThreshold = 30m`, `CheckpointStuckThreshold = 2h`
- `CheckpointAbandonedThreshold = 8h`, `PauseStaleThreshold = 30m`, `PauseForgottenThreshold = 2h`
- `OrphanedGracePeriod = 30s`

These are operational tuning parameters with no path to `models.Config`.

**Impact:** Low. The values are reasonable defaults and rarely need changing. However, the `models.Config` struct already exists as the configuration mechanism for other runtime parameters (`LeaseDuration`, `IntegrationBranch`, etc.). Watch thresholds are the only operational parameters that bypass this pattern entirely.

**Direction:** Add watch-related fields to `models.Config` (or a nested `WatchConfig` struct) with the current values as defaults. Not urgent — these are stable values.

#### Smell: Hardcoded `"terminal-1"` in supervisor *(pass 6, Coupling lens)*

**Signal:** `supervisor.go:127` passes `"terminal-1"` as a literal string to `registerAgent()`. The terminal field is recorded in agent state but always set to this fixed value regardless of the agent's actual terminal.

**Impact:** Low. The terminal field is informational (used in status display). However, if multiple agents run in different terminals, they all report the same terminal ID, reducing operational visibility.

**Direction:** Derive from `config.Terminal` or the agent's TTY, or remove the field if it's not providing value.

#### Smell: No interface-based seams beyond CLIExecutor *(pass 3, Boundaries lens)*

**Signal:** The entire production codebase has exactly **one interface**: `CLIExecutor` in `agent/supervisor.go`. All other cross-package dependencies use concrete types: `*db.Blackboard`, `*git.Git`, `*log.Logger`, `paths.LizaPaths`. There is one test-only interface (`testingT` in `testhelpers/assertions.go`).

**Impact:** This is a deliberate simplicity choice appropriate for v1 scope. However, it means testing any package that uses `db.Blackboard` requires real file I/O (creating temp directories, writing YAML files). The `testhelpers` package exists specifically to manage this overhead. If the system grows, introducing seams at the `db` and `git` boundaries would enable faster, more isolated tests.

**Direction:** No action for v1 — the current approach works. If test suite time becomes a concern, introduce interfaces at package boundaries (particularly `db` and `git`) to enable in-memory test doubles.

#### Smell: Sprint governance Vision link is broken *(Adversarial pass, entry: specs/)*

**Signal:** `specs/protocols/sprint-governance.md` links to `../vision.md`, while canonical Vision in this repo is `specs/build/0 - Vision.md` (as indexed in `specs/README.md`).

**Impact:** Low. Navigation drift weakens spec coherence and slows onboarding/review.

**Direction:** Update the related-doc link to the canonical Vision path.

#### Smell: Temporal test coupling and non-parallelizable suite *(Adversarial pass, entry: tests/ — partially resolved: `1914732`, `1ff88d2`)*

**Signal:** Shared process-level globals exist (`db.instances` singleton map and package-level `rootCmd`), which encourages serial execution in packages that use them.

**Partial fix:** `resetRootCmdForTest(t)` helper isolates `rootCmd` flag state and `db.For()` singletons between tests. `t.Parallel()` introduced in 15 call sites across 4 test files (stateless tests in `roles`, `errors`, `filelock/metrics`, `agent/prompt`). `time.Sleep()` calls reduced from 21 to 5 by replacing brittle sleep-based waits with event-driven synchronization (polling with condition checks) in watcher, heartbeat, and supervisor tests. `internal/testguard/` package (116 LOC) added with ratchet tests enforcing `t.Parallel()` floor (≥10 calls) and `time.Sleep()` ceiling (≤11 calls), preventing regression.

**Remaining:** Tests sharing process-global `rootCmd` (all `cmd/liza` tests) cannot use `t.Parallel()` due to `os.Chdir` and cobra flag state. Further parallelization requires either a `--project-root` flag on rootCmd or test-level process isolation.

**Impact:** Low (downgraded from Medium). The ratchet tests and global-reset helpers address the structural concern; remaining serial tests are constrained by process-global state, not by missing infrastructure.

#### Smell: `LIZA_LOG_LEVEL` is documented but not implemented *(Adversarial pass, entry: config/)*

**Signal:** Docs define `LIZA_LOG_LEVEL`, but no runtime code path reads it; agent logger is initialized at fixed `slog.LevelInfo`.

**Impact:** Low-medium. Documented observability control is a no-op, causing confusion during debugging/operations.

**Direction:** Either implement environment-driven level selection (with validation and defaults) or remove this variable from docs to keep contract-to-runtime alignment.

#### Smell: Cleanup failures are suppressed in rebase/worktree recovery paths *(Adversarial pass, entry: error handling — partially resolved: `729da05`)*

**Signal:** Best-effort cleanup drops failures in multiple mutation flows.

**Partial fix:** `729da05` surfaced cleanup errors in `claim_task.go` — worktree/branch cleanup failures during claim failure recovery and worktree recreation recovery are now logged with context instead of silently dropped.

**Remaining:** `internal/ops/submit_review.go:88` ignores `AbortRebase` error. `internal/git/worktree.go` ignores cleanup errors in 3 locations. `internal/ops/wt_delete.go:61` ignores branch-deletion error.

**Direction:** Keep best-effort semantics where appropriate, but surface cleanup outcomes as warning/error in result structs and command output. Escalate to error when cleanup failure can leave the system in a materially inconsistent state.


#### Smell: File existence checks often collapse non-existence with other stat failures *(Adversarial pass, entry: error handling)*

**Signal:** Several paths only branch on `err == nil` or `os.IsNotExist(err)` (for example `internal/commands/setup.go:33`, `internal/commands/init.go:32`, `internal/commands/init.go:38`, `internal/git/worktree.go:162`) without explicit handling for permission/I/O errors.

**Impact:** Low-medium. Permission or transient filesystem errors can be misreported as simple presence/absence outcomes, producing misleading diagnostics and control flow.

**Direction:** Use explicit triage on `os.Stat` calls: exists, not-exist, and other-error (returned with context).

#### Smell: Pipeline config loaded per-operation *(pass 13, Complexity lens)*

**Signal:** `loadResolver(projectRoot)` called from 54 sites in ops files *(health check: was 16+ — tripled with ops expansion)*. Each invocation reads `pipeline.yaml` from disk via `pipeline.LoadFrozen()`. A multi-step workflow (claim → submit → review → merge) reads and parses the same unchanging file 6+ times.

**Impact:** Low. The file is small and parsing is fast. The pattern is correct (each op is independent and stateless). However, it prevents session-level caching and means pipeline config changes require no coordination — a feature, not a bug.

**Direction:** Accept as-is. If profiling shows I/O overhead, consider passing a `*pipeline.Resolver` as a parameter to ops functions. The current pattern's simplicity (each op is self-contained) outweighs the redundant reads.

#### Smell: `prompts → ops` dependency inversion *(pass 14, Boundaries lens; health check: worsened)*

**Signal:** `prompts/builder.go` and `prompts/wake.go` import `internal/ops` for 5+ utility functions:
- `ops.LoadDetectionContext(projectRoot)` — loads pipeline config from disk, returns detection context
- `ops.IsPlanningPair(rolePair, planningPairs)` — simple predicate on planning pairs
- `ops.IsPlanningCompleteEligible(task, planningPairs, state)` — planning completion check
- `ops.IsTransitionCycleBlocked(task)` — transition cycle detection
- `ops.CountReadyManyToOneCohorts(state, transitions)` — cohort counting
- `ops.ManyToOneTransitionInfo` — type dependency

*(health check: was 3 functions — expanded to 5+ across 2 files; dependency deepened significantly)*

The `ops` package is the business-logic-with-side-effects layer. `prompts` is a template-driven generation layer that should be downstream of ops, not dependent on it. The current import direction means prompt building cannot be tested without pulling in the entire ops dependency chain (db, git, models, etc.).

**Impact:** Low-medium. The functions used are read-only queries, not mutations — the coupling is safe at runtime. However, the import direction is architecturally wrong: `agent → prompts → ops → db/git/...` creates a deep transitive chain where the prompt layer inherits all ops dependencies. The dependency has deepened since pass 14 — moving in the wrong direction.

**Direction:** Move pipeline query functions (`IsPlanningPair`, `IsPlanningCompleteEligible`, `IsTransitionCycleBlocked`, `CountReadyManyToOneCohorts`) to `pipeline` or a dedicated query package. Pass `PipelineDetectionContext` as a parameter to prompt-building functions instead of having prompts load it from disk via `LoadDetectionContext`.

#### Smell: `commands → agent` boundary crossing *(pass 14, Boundaries lens)*

**Signal:** `commands/status.go:282` calls `agent.DetectOrchestratorWakeTriggers(state, pipelineTerminals, planningPairs)` to include orchestrator wake trigger information in the status dashboard. This creates a dependency from the CLI presentation layer to the agent supervisor layer.

The intended architecture is `cmd → commands → ops` and `cmd → agent → ops`, with `commands` and `agent` as peer consumers of `ops`. The `commands → agent` edge breaks this peer relationship.

**Impact:** Low. `DetectOrchestratorWakeTriggers` is a pure state-query function (no side effects) that happens to live in `agent/workdetection.go`. The function itself is well-placed for the agent's use, but the status command's cross-layer call reinforces the "No Query Layer" issue documented in `architectural-issues.md`.

**Direction:** Move `DetectOrchestratorWakeTriggers` and its supporting functions to `ops` (or a future query package). This aligns with the existing trajectory issue. The function operates on `*models.State` and returns a pure result — it has no agent-specific dependencies.

#### Smell: Query logic scattered across architectural layers *(pass 14, Boundaries lens)*

**Signal:** Read-only query functions are distributed across layers that were designed for different purposes:
- `agent/workdetection.go` — `DetectOrchestratorWakeTriggers()` (state query consumed by `commands/status.go`)
- `ops/pipeline_ops.go` — `LoadDetectionContext()`, `IsPlanningPair()` (pipeline queries consumed by `prompts/builder.go` and `agent/workdetection.go`)
- `commands/inspect*.go`, `commands/status.go` — query+format logic

This creates a cross-cutting query dependency chain: `commands → agent → ops`. Each arrow is technically correct in isolation, but the chain means the CLI status display transitively depends on the agent supervisor layer for query data. *(health check: MCP layer removed, but the commands → agent → ops chain remains)*

**Impact:** Low. All functions involved are stateless and side-effect-free, so the coupling is safe at runtime. The cost is architectural: adding a new query requires choosing which layer to put it in, with no clear guidance. The documented "No Query Layer" trajectory issue in `architectural-issues.md` captures this direction.

**Direction:** This is the code-level manifestation of the "No Query Layer" issue already tracked in `specs/architecture/architectural-issues.md`. When a query layer is introduced, it should absorb: (1) inspect/status logic from `commands`, (2) `DetectOrchestratorWakeTriggers` from `agent`, (3) pipeline query functions from `ops`. Low priority — the current scatter works, but it will compound as the query surface grows.

#### Smell: Pipeline-aware status check triplication *(pass 16, Duplication lens)*

**Signal:** `models/state.go:215-239` — three functions (`IsApprovedForMerge`, `IsSubmittedStatus`, `IsExecutingStatus`) with identical 6-line structure:
```go
func IsXStatus(task *Task, pr PipelineResolver) bool {
    if task.RolePair != "" && pr != nil {
        status, err := pr.XStatus(task.RolePair)
        return err == nil && task.Status == status
    }
    return task.Status == TaskStatusX || task.Status == TaskStatusY
}
```
Only the resolver method name differs.

**Impact:** Low. Legacy fallback branches removed in `581d377` — all three functions now use pipeline resolver only. The structural triplication remains (three functions with identical shape differing by resolver method), but the legacy fallback half of each function is gone.

**Direction:** A parameterized `checkPipelineStatus(task, pr, resolverFn)` helper would collapse three functions into one. Low priority — the pattern is unlikely to grow beyond ~5 variants.

#### Smell: Worktree path construction duplication *(pass 16, Duplication lens)*

**Signal:** `filepath.Join(paths.WorktreesDirName, taskID)` + branch name derivation appears in 5 locations:
- `ops/claim_task.go:55-56`
- `ops/wt_create.go:34-35`
- `git/worktree.go:133-135`
- `git/worktree.go:203-205`
- `git/worktree.go:305` (as `GetWorktreeRelPath()` helper — **exists but unused by the other 4 sites**)

**Impact:** Low. The computation is trivial (one `filepath.Join`). However, `GetWorktreeRelPath()` was deliberately created to centralize this, suggesting the other sites predate or missed it. The branch name construction (`paths.TaskBranchPrefix + taskID`) is similarly repeated but has no helper.

**Direction:** Use `git.GetWorktreeRelPath()` at the 4 remaining sites. Consider adding `GetWorktreeBranchName(taskID)` to `paths/` for the branch construction.

#### Smell: Dual logging systems — structured Logger vs ad-hoc log.Printf *(pass 16, Duplication lens)*

**Signal:** `internal/log/` provides a formal YAML append Logger (used for persistent task event logs via `ops/` → `log.Logger.Append()`). Alongside it, `ops/` contains 21+ `log.Printf()` calls (standard library) for runtime warnings and diagnostics — `claim_task.go` (7), `wt_merge.go` (7), `proceed.go` (3), `pipeline_ops.go` (2), `release_claim.go` (2), `submit_review.go` (1).

The two systems serve different purposes: formal Logger for persistent event audit trail, `log.Printf` for transient operational warnings. But the split is implicit — there's no documented contract for which to use when.

**Impact:** Low. Both systems work. However, the `log.Printf` messages go to stderr (via Go's default logger) while the formal Logger writes to `.liza/logs/`. An operator watching logs has two separate streams with inconsistent formatting. Agents running in supervisor mode see the `log.Printf` output mixed with their own output.

**Direction:** Document the dual-purpose split (event log vs diagnostic log). Consider routing `log.Printf` warnings through a thin wrapper that adds structured context (task ID, operation name) for consistent formatting. Not urgent — the current approach works.

#### Smell: statevalidate internal micro-duplication *(pass 16, Duplication lens)*

**Signal:** `statevalidate/validate.go` (658 LOC) contains three repeated micro-patterns:
1. **statusClassifier methods** — 6 methods (`IsExecuting`, `IsInitial`, `IsSubmitted`, `IsReviewing`, `IsApproved`, `IsRejected`) with identical structure (hardcoded status check + `containsStatus` fallback), differing only by status name and field
2. **Set uniqueness checks** — `map[string]bool` + loop + duplicate detection pattern repeated for `FailedBy` agents and `SprintHistory` IDs
3. **Required field validation** — `if field == "" { return fmt.Errorf("missing required field '%s'", name) }` repeated ~5 times

**Impact:** Low. Each instance is 3-5 lines. The repetition is internal to a single file, not cross-file. Total: ~40 lines of structural repetition.

**Direction:** The `statusClassifier` methods could use a generic `IsStatus(s TaskStatus, hardcoded []TaskStatus, pipeline []TaskStatus) bool` but that trades clarity for terseness. The set uniqueness check could use a `checkUnique(items []string, context string) error` helper. Neither is urgent — the file is internally consistent.

#### Smell: `ops/proceed.go` god file — 5 responsibilities in 1,200 LOC *(Adversarial pass, new packages)*

**Signal:** `proceed.go` grew from 533 to 1,200 LOC (+125%) to support 3 transition cardinalities (per-subtask, one-to-one, many-to-one). Now the largest file in the codebase with 26 functions and 107 `if` statements (0.089 if/LOC — highest density). Contains 5 distinct responsibilities:
1. Manual transition execution (`Proceed`)
2. Supervisor batch execution with cycle detection (`ExecuteAvailableTransitions`, 210 LOC — largest function)
3. Crash recovery for all 3 cardinalities (`recoverCrashedTransition`, 108 LOC)
4. Graph algorithms — topological sort + Tarjan's SCC (`topoSortPending` 92 LOC, `findSCCs` 61 LOC)
5. Child task construction + dependency propagation (`buildChildTask`, `buildOneToOneChild`, `buildManyToOneChild`, `computeInheritedDeps`)

**Impact:** Medium. The file is well-tested (31 tests) and error handling is solid (16 wrapped errors, idempotency guards). But cognitive load is high — a developer modifying crash recovery must navigate past graph algorithms, and vice versa. The responsibilities are clearly separable.

**Direction:** Extract into 4-5 cohesive files:
- `proceed_graph.go` (~170 LOC): `topoSortPending`, `findSCCs`, `hasSelfLoop`, `hasCycleBlockedEvent`
- `proceed_recovery.go` (~150 LOC): `recoverCrashedTransition`, `isTransitionIncomplete`
- `proceed_builders.go` (~130 LOC): `buildChildTask`, `buildOneToOneChild`, `buildManyToOneChild`, `validateOutputEntry`, `patchInheritedDeps`
- `proceed_deps.go` (~85 LOC): `computeInheritedDeps`, `extraToStringSlice`
- `proceed.go` (~400 LOC): core orchestration (`Proceed`, `proceedInner`, `proceedManyToOneInner`, `ExecuteAvailableTransitions`)

#### Smell: `await_verdict.go` / `await_resubmission.go` structural duplication *(Adversarial pass, new packages)*

**Signal:** Two event-loop files (445 + 403 LOC) share ~180 LOC of structurally identical code (~40% overlap):
- Main function skeleton: argument validation, path/blackboard setup, watcher creation with polling fallback, deadline/ticker setup — ~120 LOC identical
- Polling fallback function: 37-39 LOC, 98% identical (only result type and status-check function differ)
- Event loop structure: all 5 select cases (`ctx.Done`, `abortTicker`, `watcher.Events`, `watcher.Errors`, `deadlineTimer`) identical in structure

Functional differences: verdict handles rejection reclaim + budget escalation (78 LOC result handler); resubmission handles lease renewal (15 LOC result handler, delegates to ops).

Both hardcode `1 * time.Second` (abort ticker) and `5 * time.Second` (polling ticker).

**Impact:** Low-medium. If the event-loop pattern changes (e.g., new select case, different watcher setup), both files need identical updates. The pattern will likely be reused if new await_* operations are added.

**Direction:** Extract `awaitWithWatcher(ctx, bb, taskID, agentID, timeout, acquireFn, checkStatusFn, handleResultFn)` helper (~150 LOC). Would reduce both files by ~120 LOC each. Evaluate whether the abstraction aids or obscures — the current duplication is readable.

#### Smell: `ApplyYAMLTimeouts` type-switch in agent strategies *(Adversarial pass, new packages)*

**Signal:** `strategy_doer.go` exports `ApplyYAMLTimeouts(strategy RoleStrategy, ...)` which uses a `switch s := strategy.(type)` over all three concrete strategy types to inject YAML timeout overrides. Adding a new strategy type requires updating this switch.

**Impact:** Low. The strategy set is small (3 types) and stable. But the pattern violates the Strategy interface contract — a method on the interface would be cleaner.

**Direction:** Add `ApplyTimeoutOverrides(executionTimeout, pollSec, maxWaitSec)` to the `RoleStrategy` interface. Each strategy applies to its own fields. Eliminates the type-switch.

#### Smell: CLI Executor concrete implementation tightly coupled to CLI tools *(pass 17, Coupling lens)*

**Signal:** `DefaultCLIExecutor.Execute()` in `agent/supervisor.go` has a `switch actualCLI` statement hardcoding 5 CLI tools ("claude", "codex", "gemini", "vibe", "kimi") with per-tool flag formats:
- `claude`: `-p`, `--verbose`, `--output-format stream-json`
- `codex`: `exec [prompt] --full-auto`, `--json` *(Adversarial pass: MCP reference removed — was `-c mcp_servers.liza.command=...`)*
- `gemini`: `-p`, `--output-format stream-json`
- `vibe`: `-p`, `--output streaming`
- `kimi`: `-p`, `--verbose`, `--output-format stream-json`

The `mistral → vibe` rename mapping is still hardcoded. `ExecuteInteractive()` duplicates the same switch. Adding a new CLI tool requires modifying both functions.

**Impact:** Low-medium. The `CLIExecutor` *interface* is clean — tests use `MockCLIExecutor` successfully. But the *concrete implementation* is a switch statement that couples the agent layer to specific external tool CLI APIs. If any tool changes its flag format, the supervisor needs updating. The pattern isn't extensible without code changes.

**Direction:** Consider a data-driven CLI profile (map/config of CLI name → flag templates) or a per-CLI adapter. Alternatively, accept this as intentional — the set of supported CLIs is small and stable, and the switch provides clear, readable per-CLI logic. Low priority unless the CLI set grows.

#### Smell: MCP access control, extractStringSlice, extractOutputEntries — REMOVED *(health check: MCP server deleted in `90c132d5`)*

#### Smell: Timestamp inconsistency — `time.Now()` vs `time.Now().UTC()` in `wt_merge.go` *(Adversarial pass, data flow)*

**Signal:** `ops/wt_merge.go` uses `time.Now()` (local timezone) at 3 sites (lines 118, 451, 473) for `TaskHistoryEntry.Time` and `Timestamp`, while every other production ops file uses `time.Now().UTC()`. *(health check: was 2 sites — worsened to 3)* The pattern across 20+ ops files is consistently `.UTC()`:
- `claim_task.go:258`, `submit_review.go:182,273`, `submit_verdict.go:119`, `proceed.go:76,257`, `release_claim.go:177`, `mark_blocked.go:42`, `delete_agent.go:38,95`, `recover_task.go:148`, etc.

**Impact:** Low. On most servers, UTC and local time are the same. However, in development environments or non-UTC deployments, history entries from `wt_merge.go` will have different timezone offsets than entries from all other ops files, causing inconsistent timestamp sorting and display.

**Direction:** Change `time.Now()` to `time.Now().UTC()` at lines 118, 451, and 473 in `wt_merge.go`. Three-line fix.

#### Smell: Unbounded integration test execution in `MergeWorktree` *(Adversarial pass, data flow)*

**Signal:** `ops/wt_merge.go` runs `scripts/integration-test.sh` via `exec.Command` with no timeout or context cancellation. If the test script hangs (network dependency, infinite loop, blocking I/O), the merge operation blocks indefinitely, which blocks the CAS retry loop, which prevents all further merges.

The supervisor's execution timeout (default 30m, `agent/supervisor.go:403`) does not protect this path — `MergeWorktree` is called by agents during review, not by the supervisor loop.

**Impact:** Medium. A hanging integration test is an unbounded operation that can stall the entire merge pipeline. Other merge candidates queue behind the stuck CAS lock. The only recovery is manual process termination.

**Direction:** Use `exec.CommandContext` with a configurable timeout (e.g., `Config.IntegrationTestTimeout`, default 10m). The timeout should be generous enough for legitimate test suites but bounded enough to prevent indefinite hangs. Consider also adding a timeout to the CAS retry loop itself.

#### Smell: Role mapping bidirectionality unverified *(pass 17, Coupling lens)*

**Signal:** `roles/roles.go` maintains two maps that must stay synchronized:
- `runtimeToWorkflow` (line 39): maps 9 runtime roles → workflow roles
- `workflowToRuntime` (line 52): maps 9 workflow roles → runtime roles

Both are hand-maintained `var` declarations. Adding a new role requires adding entries to both maps plus updating `AllRuntime()`, `AllWorkflow()`, and potentially `DoerRoles()` / `ReviewerRoles()`. No compile-time or startup verification catches a missing entry in either direction.

**Impact:** Low. The maps are small (9 entries each), and the `roles_test.go` file exists with tests. However, adding Phase 2 roles (which already happened — us-writer, us-reviewer, epic-planner, etc.) requires touching 4-6 locations per role. A forgotten entry in `workflowToRuntime` would cause `ToRuntime()` to return an error for valid workflow roles, surfacing as a runtime failure during task claiming or review.

**Direction:** Add a test that verifies bidirectionality: for each entry in `runtimeToWorkflow`, confirm the inverse exists in `workflowToRuntime` and vice versa. This is a 10-line test that prevents a class of bugs. Alternatively, generate one map from the other.

#### Smell: Scattered timeout constants across packages *(pass 17, Coupling lens)*

**Signal:** Beyond the documented watch thresholds (pass 6) and raw `1800` residuals (pass 6), timeout/interval constants are scattered across 5 packages with no centralized registry:

| Package | Constant | Value | Location |
|---------|----------|-------|----------|
| `filelock` | `DefaultLockTimeout` | 10s | `filelock.go:14` |
| `filelock` | `LockCheckInterval` | 100ms | `filelock.go:15` |
| `db` | (inline debounce) | 50ms | `watcher.go:59` |
| `agent` | (abort check) | 5s | `waitforwork.go:133` |
| `agent` | (pause check) | 5s | `systemctl.go:41` |
| `agent` | (execution timeout default) | 30m | `supervisor.go:403` |
| `agent` | (exit42 initial backoff) | 2s | `supervisor.go:194` |

The `models.Config` struct already centralizes heartbeat, poll, lease, and iteration parameters. These timeout/interval constants bypass that pattern entirely — some are package-level `const`, some are inline literals.

**Impact:** Low. Each value is reasonable and unlikely to need per-environment tuning. The concern is discoverability: an operator troubleshooting a timeout-related issue has no single place to see all timing parameters. Values like the 5-second abort/pause checks are duplicated (same magic number in two packages) without sharing a constant.

**Direction:** Document the timing constants as a reference table (this smell serves that purpose). Moving them to `models.Config` is low priority — most are infrastructure-level constants that don't benefit from runtime configurability. The inline 50ms debounce and 5s checks could be extracted to named constants for clarity.

#### Smell: `statevalidate` composition gap — entry-point validators untested *(pass 15, Coverage lens)*

**Signal:** `statevalidate` has the lowest coverage of any functional package at **55.1%** (27 functions, 9 at 0%). The 0% functions are precisely the entry-point validators that compose the inner checks: `ValidateStateFile`, `ValidateAgentInvariants`, `ValidateAnomalies`, `validateRequiredFields`, `validateAgentInvariants`, `validateHandoff`, `validateDiscovered`, `validateAnomalies`, `checkSpecFileExists`. Inner validators (`validateTaskStates` 92.9%, `validateDependencies` 91.3%, `checkCircular` 81.8%) are well-covered.

**Impact:** Medium. This package validates data integrity — exactly the kind of code where "tested parts, untested whole" matters. The composition logic determines which validators run, in what order, and how errors aggregate. A bug in `ValidateStateFile` (the top-level entry point called by `liza validate`) could skip validators entirely without test detection.

**Direction:** Table-driven tests calling `ValidateStateFile` with various malformed states would cover the composition layer cheaply. The inner validators' existing coverage means only the wiring needs testing.

#### Smell: `models/state.go` governance helpers at 0% *(pass 15, Coverage lens)*

**Signal:** Several task/system governance query methods have zero coverage: `IsApprovedForMerge`, `IsSubmittedStatus`, `IsExecutingStatus` (status classification), `ReleaseAgent` (agent cleanup), `ValidateTransition` (SystemMode transition validation), `NormalizeHeartbeatInterval`, `IsPipelineSprintTerminal`. Also `ValidTaskTypeNames` and `isBlockedByDepsPipeline`/`isInProgressPipeline` in diagnostics.go.

**Impact:** Low-medium. These functions are simple (most are 5-15 line helpers) but they're called from the untested runtime orchestration layer — the same "tested parts, untested whole" pattern. `ValidateTransition` is notable: it governs the system mode state machine (RUNNING → PAUSED → STOPPED) with a transition table, and has no test at all.

**Direction:** `ValidateTransition` deserves a table-driven test — it's a small, pure function governing system-level state transitions. The status-query helpers could be batch-tested. `NormalizeHeartbeatInterval` is a pure function with clear boundary conditions (≤0, >300, valid) — trivial to test.

#### Smell: `SetTaskOutput` does not validate `spec_ref` *(Adversarial pass, data flow: role-pair-to-role-pair)*

**Signal:** `set_task_output.go` validates `desc`, `done_when`, `scope` as non-empty but does NOT validate `spec_ref`:
```go
// set_task_output.go — validates 3 of 4 fields:
if entry.Desc == "" { return fmt.Errorf("output[%d].desc is required", i) }
if entry.DoneWhen == "" { return fmt.Errorf("output[%d].done_when is required", i) }
if entry.Scope == "" { return fmt.Errorf("output[%d].scope is required", i) }
// spec_ref NOT checked
```
By contrast, `proceed.go:validateOutputEntry` (line 405-418) requires all four fields including `spec_ref`. An agent can successfully set output entries with empty `spec_ref` via `liza_set_task_output`, then `liza proceed` fails at transition time.

This is the ops-layer validation allowing empty `spec_ref` through.

**Impact:** Low-medium. The failure surfaces at proceed time with a clear error message, so it's not silent. But the feedback is delayed — the agent's work is already merged and the sprint is checkpointed before the human discovers the validation failure. Fixing at that point requires manual state editing or re-running the planning pair.

**Direction:** Add `spec_ref` validation to `SetTaskOutput`, consistent with `validateOutputEntry`. Alternatively, make `spec_ref` optional in `validateOutputEntry` since not all decompositions may reference a spec file.

#### Smell: Sprint metrics lossy compression at sprint boundary *(Adversarial pass, data flow: sprint-to-sprint)*

**Signal:** `applySprintAdvance` (advance_sprint.go:100-107) reduces the full 12-field `SprintMetrics` struct to a single `TasksDone` count in `SprintSummary`:
```go
s.SprintHistory = append(s.SprintHistory, models.SprintSummary{
    // ... only TasksDone from Metrics survives:
    TasksDone: plan.archivedSprint.Metrics.TasksDone,
})
s.Sprint = models.Sprint{
    // ... new sprint gets zeroed metrics:
    Metrics: models.SprintMetrics{},
}
```
Fields lost: `TasksTotal`, `TasksAbandoned`, `AvgReviewIterations`, `MaxReviewIterations`, `ReviewApprovalRate`, `TasksBlockedCount`, `AnomaliesLogged`, `AvgTaskDurationMinutes`, `ContextExhaustionCount`, `IntegrationFailures`, `SupersededCount`. These are only preserved in the archive YAML on disk.

`BuildOrchestratorContext` (builder.go:127) passes `state.SprintHistory` to orchestrator templates, but those summaries carry only `TasksDone`. The orchestrator cannot see cross-sprint performance trends (e.g., rising rejection rates, increasing iteration counts) without reading archive files — which no prompt template instructs it to do.

**Impact:** Low-medium. The full archive is preserved on disk, so no data is permanently lost. However, the orchestrator's planning decisions for sprint N+1 cannot be informed by sprint N's performance metrics because the active state summary discards them. Distinct from the existing "Retrospective Findings Don't Feed Forward" issue (which covers qualitative findings) and "Metrics Collection Without Query Interface" (which covers the absence of a query layer).

**Direction:** Extend `SprintSummary` with a small set of the most decision-relevant metrics (e.g., `AvgReviewIterations`, `ReviewApprovalRate`, `TasksBlockedCount`) or add a `MetricsSummary` sub-struct. Alternatively, have the orchestrator template reference archive files for the previous sprint. The archive path is deterministic: `.liza/archive/sprint-N.yaml`.

#### Smell: Per-subtask child task priority flattened to 1 *(Adversarial pass, data flow: role-pair-to-role-pair)*

**Signal:** `buildChildTask` (proceed.go:369) hardcodes `Priority: 1` for per-subtask children:
```go
func buildChildTask(...) models.Task {
    return models.Task{
        Priority: 1,  // hardcoded, parent priority ignored
        // ...
    }
}
```
By contrast, `buildOneToOneChild` (proceed.go:395) correctly copies `parent.Priority`. This inconsistency means elevated-priority parent tasks pass priority through one-to-one transitions but not through per-subtask transitions.

**Impact:** Low. Current priority usage is limited — the system doesn't have priority-based scheduling beyond ordering. However, it's a data flow inconsistency between the two transition modes that could cause surprising behavior if priority-aware scheduling is added.

**Direction:** Pass parent priority (or the output entry's priority, if added to `OutputEntry`) to `buildChildTask`. At minimum, copy from the parent task as `buildOneToOneChild` does.

#### Smell: `buildChildTask` hardcodes `Type: TaskTypeCoding` for all children *(Adversarial pass, data flow: role-pair-to-role-pair)*

**Signal:** Both `buildChildTask` (proceed.go:365) and `buildOneToOneChild` (proceed.go:390) set `Type: models.TaskTypeCoding` regardless of the target role-pair. When the epic-to-us transition creates US Writer tasks from epic planner output, those children get `Type: coding` even though their role-pair is `us-writing-pair`.

This is the concrete code-level manifestation of two existing issues in `architectural-issues.md`:
- "Task Type Registry Only Supports Coding Workflows" (medium, TENSION)
- "Task Type Registry is Partial Abstraction" (low, TRAJECTORY)

**Impact:** Low (currently). The task type is used by `IsClaimable()` for role eligibility lookup, but pipeline-aware claiming bypasses the task type registry entirely — it uses role-pair status matching. The hardcoded type becomes a latent bug only if non-pipeline code paths start using task type to distinguish workflow behavior.

**Direction:** Derive task type from the target role-pair (e.g., `us-writing-pair` → `specification` type, if added to the registry). Alternatively, accept `TaskTypeCoding` as the universal type until the registry is extended — but document this as an intentional simplification, not an oversight.

#### Smell: INVARIANTS.md §7 clean worktree check not enforced at submission *(Adversarial pass, targeted: INVARIANTS.md)*

**Signal:** INVARIANTS.md §7 states: "Clean sync: before READY_FOR_REVIEW, working tree must be clean (no staged, unstaged, or untracked files)." `submit_review.go` validates: commit SHA match (line 118-119), TDD enforcement (line 123-131), pre-execution checkpoint (line 89-91), rebase onto integration (line 147), and re-validates status/assignment under lock (line 189-199). But **no `git status` or working tree cleanliness check exists** in the submission path.

**Impact:** Medium. A coder can submit work with uncommitted changes in the worktree — the reviewer sees only the committed code (via `review_commit`), but the coder may have relied on uncommitted files during local testing. The invariant document promises this protection but the code doesn't deliver it.

**Direction:** Add a clean worktree check (no staged, unstaged, or untracked files) to `submit_review.go` before the rebase step. Alternatively, update INVARIANTS.md to remove the claim if this is intentionally deferred. The spec `worktree-management.md` should match whichever decision is made.

#### Smell: INVARIANTS.md §6 provider diversity not enforced at verdict time *(Adversarial pass, targeted: INVARIANTS.md)*

**Signal:** INVARIANTS.md §6 states: "Quorum enforcement: approval count tracked, provider diversity required (≥2 distinct providers for multi-reviewer quorum)" and attributes enforcement to `submit_verdict.go`. The code has all the building blocks:
- `task.HasProviderDiversity()` (task.go:278) — checks if approvals come from ≥2 distinct providers
- `resolver.ProviderDiversity()` (resolver.go:287) — returns the diversity setting per role-pair/impact
- `filterDoerProviderDiversity()` (claim_reviewer_task.go:347) — filters reviewer candidates at claim time

But `submit_verdict.go` **only checks `ApprovalCount() < effectiveQuorum`** (line 237) — it never calls `HasProviderDiversity()` or `ProviderDiversity()`. Provider diversity is actually enforced:
1. At reviewer claim-filtering time (doer-vs-reviewer provider, `claim_reviewer_task.go:107`)
2. At merge-readiness evaluation (soft "preferred" enforcement, `agent/claiming.go:151`)

Neither matches the invariant's description of hard enforcement during verdict submission.

**Impact:** Medium. If two reviewers from the same provider (e.g., both "claude") approve a task, the quorum count is met and the task transitions to APPROVED. The diversity check only applies to merge readiness (with "preferred" semantics that can be overridden), not to the approval itself. This is weaker than what the invariant document promises.

**Direction:** Either (a) add provider diversity enforcement to `submit_verdict.go` — reject second approval from same provider when diversity is required, or (b) update INVARIANTS.md to accurately describe the current enforcement model: "provider diversity preferred at merge-readiness, not enforced at verdict time."

#### Smell: INVARIANTS.md §12 system mode transition table inaccurate *(Adversarial pass, targeted: INVARIANTS.md)*

**Signal:** INVARIANTS.md §12 states: "System mode transitions enforced: RUNNING↔PAUSED, any→CIRCUIT_BREAKER_TRIPPED, TRIPPED→PAUSED." The actual transition landscape:
- RUNNING→PAUSED: ✓ via `ValidateTransition` (config.go:48)
- PAUSED→RUNNING: via `Resume()` (mode_change.go:88-92), bypasses `ValidateTransition` — `ValidateTransition` rejects this with "use liza resume"
- **TRIPPED→RUNNING**: via `Resume()` (mode_change.go:93-99) — invariant only lists TRIPPED→PAUSED
- any→TRIPPED: via direct `s.Config.Mode = SystemModeCircuitBreakerTripped` (analyze.go:89) — not in transition table at all
- RUNNING/PAUSED/TRIPPED→STOPPED: via `ValidateTransition` — not mentioned in invariant
- STOPPED→RUNNING: via `ValidateTransition` — not mentioned in invariant

The `ValidateTransition` map in config.go and the `Resume()` function in mode_change.go form a split implementation with no shared validation.

**Impact:** Low. The transitions work correctly — the invariant description is simply incomplete. Operators relying on the invariant table for mode transition understanding would miss valid paths.

**Direction:** Update INVARIANTS.md §12 with the complete transition graph including the Resume() path and the STOPPED mode.

#### Smell: INVARIANTS.md §7 different-coder rejected worktree recreation not enforced *(Adversarial pass, targeted: INVARIANTS.md)*

**Signal:** INVARIANTS.md §7 states: "Different coder reclaiming REJECTED task → delete and recreate fresh worktree." In `claim_task_strategy.go`, the `rejectedClaimStrategy.handleWorktree()` (line 120-126) delegates to `ensureRejectedWorktreeExists()` (claim_task.go:397-444), which preserves the existing worktree when both directory and branch exist — **regardless of whether the claimer is the same or a different coder**. Same-coder vs different-coder only affects the history event type (line 140-149 of claim_task_strategy.go). The worktree is recreated only when the directory or branch is missing (recovery from partial state, not policy enforcement).

**Impact:** Low. The intent is to prevent "context contamination from failed work" (per the invariant). In practice, rejected tasks retain the prior coder's uncommitted state, which a different coder inherits. However, since the review process already identified the issues, and the new coder gets the rejection reason in context, the practical impact is limited.

**Direction:** If the invariant is intentional, add a same-coder check to `ensureRejectedWorktreeExists()` and recreate from integration branch when a different coder claims. If the current behavior is preferred (preserve work for debugging), update INVARIANTS.md to reflect it.

#### Smell: INVARIANTS.md §3.2 transition map attribution and §3.1 status naming drift *(Adversarial pass, targeted: INVARIANTS.md)*

**Signal:** Two documentation drift issues in INVARIANTS.md §3:

1. **§3.2** attributes the transition map to "`models/task.go` transition map." In reality, `task.go` only has `TransitionWith()` (line 309-318) which validates against a **provided** map. The actual map is built by `BuildPipelineTransitions()` in `ops/pipeline_ops.go:67-103` from pipeline config via `resolver.TransitionMap()`. No hardcoded transition map exists in task.go since commit `581d377`.

2. **§3.1** uses legacy status names (DRAFT, IMPLEMENTING, READY_FOR_REVIEW, REVIEWING, APPROVED, REJECTED). The code uses pipeline-prefixed names: DRAFT_CODE, IMPLEMENTING_CODE, CODE_TO_REVIEW, REVIEWING_CODE, CODE_APPROVED, CODE_REJECTED. The coding-plan pair uses DRAFT_CODING_PLAN, CODE_PLANNING, etc. The legacy constants in `task.go:79-103` still exist but are mapped from pipeline-declared names via the resolver.

**Impact:** Low. The invariants are conceptually correct — the enforcement exists, just at different locations and with different naming than documented. A developer using INVARIANTS.md to navigate the code would look in the wrong file.

**Direction:** Update §3.2 enforcement to reference `ops/pipeline_ops.go:BuildPipelineTransitions` and `pipeline/resolver.go:TransitionMap`. Generalize §3.1 status names to use role-pair-qualified terminology or add a note that specific status names are pipeline-configured.

### 2.4 Patterns

| Pattern | Where Used | Purpose |
|---------|------------|---------|
| Repository (Blackboard) | `internal/db/` | Encapsulates file-based state persistence with locking |
| Strategy (CLIExecutor) | `internal/agent/` | Pluggable agent CLI backend (claude, codex, gemini, vibe) |
| Command | `internal/commands/` | Each CLI command is an independent function with uniform interface |
| Template Method | `internal/prompts/` | Role-specific prompts built from shared templates |
| Observer (Watcher) | `internal/db/watcher.go` | Event-driven state change notification via fsnotify |
| Strategy (claimRelease) | `internal/ops/release_claim.go` | Parameterized coder/reviewer claim release — eliminates duplication between two nearly-identical release flows *(pass 5, Duplication lens: notable counterexample)* |
| Registry | `internal/models/`, `internal/roles/` | Task type → role workflow mapping; unified role constants with runtime↔workflow mapping |
| State Machine | `internal/models/`, `internal/ops/` | Pipeline-driven only: `TransitionWith()` using `BuildPipelineTransitions()` *(pass 2: added; `581d377`: hardcoded map removed)* |
| Circuit Breaker | `internal/analysis/` | Pattern detection on anomalies triggers system pause |
| Heartbeat/Lease | `internal/agent/heartbeat.go` | Agent liveness detection via periodic lease extension |
| Embed | `internal/embedded/` | Contract/skill files embedded in binary via `go:embed` |
| Strategy (role) | `internal/agent/strategy*.go` | Role-specific claim/work strategies (doer, reviewer, orchestrator) *(health check: new pattern)* |

### 2.5 Test Coverage

**Overall:** ~33,400 source LOC, ~87,400 test LOC. 2.6:1 ratio. *(health check: was ~23,100/~55,600 (2.4:1); +44% production, +57% test; coverage % needs re-measurement after MCP removal and ops/tui growth)*

**Well-covered:**
| Package | Ratio | Notes |
|---------|-------|-------|
| prompts | 3.9:1 | Highest |
| identity | 3.0:1 | |
| db | 2.8:1 | Includes concurrency tests |
| commands | 3.7:1 | 35+ test files, ~19,520 test LOC *(health check: was 2.1:1)* |
| git | 1.7:1 | Real git repos in tests |
| embedded | 1.9:1 | |
| agent | 2.4:1 | MockCLIExecutor, supervisor tests |
| pipeline | 2.4:1 | Config parsing + resolver tests *(health check: new)* |
| ops | 2.5:1 | ~24,820 test LOC *(health check: was 2.0:1 — test investment grew faster than code)* |
| models | 1.8:1 | |

**Gaps:**
- `cmd/liza/main.go` (~3,089 LOC across 7 files): CLI wiring *(health check: was 1,462 — doubled; needs coverage re-measurement)*
- `internal/models/diagnostics.go` (230 LOC): `diagnostics_test.go` exists *(pass 4 resolved: pass 13)*
- `internal/statevalidate/` (903 LOC): 9 of 27 functions at 0% — lowest functional package *(pass 15, Coverage lens; health check: grew from 658)*
- `internal/roles/` (60%): 4 Phase 2 role-query functions at 0% (`DoerRoles`, `ReviewerRoles`, `IsDoerRole`, `IsReviewerRole`) *(pass 15, Coverage lens)*
- `internal/tui/` (~2,265 LOC, 172 tests): Well-tested via 6 test files. No integration/E2E tests for full Bubbletea program lifecycle *(Adversarial pass: assessed)*
- `internal/process/` (67 LOC): No test file *(Adversarial pass: test gap)*
- `internal/gitenv/` (28 LOC): No test file *(Adversarial pass: test gap)*

**Critical 0% coverage paths** *(pass 4, Coverage lens; health check update)*:
Uncovered code concentrates in three patterns:
1. **Runtime orchestration** — `supervisor.Execute/ExecuteInteractive/RunSupervisor`, `systemctl.executeAgent/waitWhilePaused/checkAbort`, `claiming.handleApprovedMerges/handleAvailableTransitions` — the agent lifecycle loop
2. **I/O-coupled functions** — `embedded.WritePipelineConfig/WriteGuardrails/PlanGlobalFiles`, `DefaultCLIExecutor` — hardwired to OS-level I/O *(health check: MCP stdio removed)*
3. **Validation composition** — `statevalidate.ValidateStateFile/ValidateAgentInvariants/ValidateAnomalies/validateRequiredFields` + 5 inner validators — entry points at 0% while composed validators are well-covered *(pass 15, Coverage lens)*

**468 functions at 0% total** *(pass 15, Coverage lens)*: Many are trivial (error type `.Error()` methods, path accessor one-liners), but the absolute count is useful as a trend metric. The significant zero-coverage functions cluster in the three patterns above.

**Partially covered functions of concern** *(pass 4; pass 15 update)*:
| Function | Coverage | Status |
|----------|----------|--------|
| `statevalidate.validateStatusFields` | 27.0% | Status-specific field validation — many branches untested *(pass 15)* |
| `statevalidate.validateTaskInvariants` | 45.5% | Core task invariant checks — partially covered *(pass 15)* |
| `agent.verifyOrchestratorStateChanges` | 34.7% | Orchestrator verification — mostly uncovered *(pass 15)* |
| `agent.waitForWorkPolling` | 52.4% | Polling fallback path *(pass 15)* |

**I/O coupling as testability barrier** *(pass 4, Coverage lens)*: Functions at 0% coverage strongly correlate with hardwired I/O — this is the Coverage lens perspective on the Boundaries smell (pass 3). The `CLIExecutor` interface demonstrates the solution pattern: abstracting one I/O boundary enabled comprehensive supervisor testing. *(health check: MCP StdioTransport no longer relevant — removed)*

**Integration tests:** `internal/integration/` (~4,120 LOC) covering concurrent operations, sprint/merge workflows, e2e command sequences, lease expiry. All files guarded by `testing.Short()` — skipped under `go test -short` *(health check: was 1,665 — +147% growth, significant investment)*.

**Test patterns:** Table-driven (dominant), filesystem isolation, hand-written mocks (no frameworks), real git operations. No property-based or fuzz testing. *(pass 3: `os.Stdin` monkey-patching pattern noted as testing boundary smell — 8+ test files)*

**Temporal coupling signal** *(Adversarial pass, entry: tests/ — partially resolved: `1914732`, `1ff88d2`)*: Now 5 `time.Sleep()` calls and 14 `t.Parallel()` uses across 118 test files *(pass 15: was 15/101)*. `resetRootCmdForTest(t)` isolates process-global state. `internal/testguard/` ratchet tests enforce `t.Parallel()` floor (≥10) and `time.Sleep()` ceiling (≤11), preventing regression. Remaining serial tests are constrained by process-global state (`rootCmd`, `os.Chdir`), not by missing infrastructure.

**Per-package statement coverage** *(pass 15, Coverage lens)*:
| Package | Avg Coverage | Zero-Coverage Funcs | Risk Level |
|---------|-------------|--------------------:|------------|
| analysis | 98.1% | 0 | Low |
| errors | 100% | 0 | Low |
| identity | 100% | 0 | Low |
| db | 91.5% | 0 | Low |
| filelock | 90.5% | 1 | Low |
| prompts | 89.0% | 1 | Low |
| pipeline | 86.4% | 0 | Low |
| commands | 86.1% | 4 | Low |
| ops | 85.9% | 4 | Low |
| log | 79.5% | 0 | Low |
| tui | — | — | Low *(Adversarial pass: 172 tests, 6 test files; no E2E)* |
| main (cmd) | 71.1% | 1 | Low |
| agent | 70.7% | 14 | **High** |
| models | 70.6% | 11 | Medium |
| git | 65.9% | 10 | Medium |
| testhelpers | 65.8% | 5 | Low |
| embedded | 64.0% | 5 | Medium |
| paths | 63.7% | 9 | Low |
| roles | 60.0% | 4 | Medium |
| statevalidate | 55.1% | 9 | **High** |

**`check-testhelpers` build guard** *(pass 4)*: Makefile target prevents `testhelpers` import in production code — good practice for maintaining test/production boundary.

---

## Phase 3: Recommendations

| Priority | Issue | Rationale | Action |
|----------|-------|-----------|--------|
| **Medium** | INVARIANTS.md §7 clean worktree check not enforced *(Adversarial pass, targeted: INVARIANTS.md)* | `submit_review.go` validates commit SHA, TDD, rebase, checkpoint — but never checks working tree cleanliness; invariant promises this protection | Add `git status` check to submission, or update INVARIANTS.md |
| **Medium** | INVARIANTS.md §6 provider diversity not enforced at verdict time *(Adversarial pass, targeted: INVARIANTS.md)* | `submit_verdict.go` only checks approval count, never calls `HasProviderDiversity()`; diversity enforced at claim-filtering (hard) and merge-readiness (soft "preferred") instead | Add diversity check to verdict, or update invariant to reflect actual enforcement model |
| **Medium** | `ops/proceed.go` god file *(Adversarial pass, new packages)* | 1,200 LOC, 26 functions, 107 `if` statements, 5 distinct responsibilities. Largest file in codebase. Graph algorithms, crash recovery, child builders, dependency propagation all bundled together | Extract into 4-5 cohesive files (proceed_graph.go, proceed_recovery.go, proceed_builders.go, proceed_deps.go) |
| **Low** | INVARIANTS.md §12 system mode transitions inaccurate *(Adversarial pass, targeted: INVARIANTS.md)* | Invariant says "TRIPPED→PAUSED" but Resume() goes TRIPPED→RUNNING; STOPPED mode transitions omitted entirely | Update §12 with complete transition graph |
| **Low** | INVARIANTS.md §7 different-coder worktree recreation not enforced *(Adversarial pass, targeted: INVARIANTS.md)* | `ensureRejectedWorktreeExists()` preserves worktree regardless of claimer identity; invariant promises fresh worktree for different coders | Enforce same-coder check or update INVARIANTS.md |
| **Low** | INVARIANTS.md §3 status naming and transition map attribution drift *(Adversarial pass, targeted: INVARIANTS.md)* | §3.2 cites `models/task.go transition map` but map lives in `ops/pipeline_ops.go:BuildPipelineTransitions`; §3.1 uses pre-pipeline status names | Update file references and generalize status names |
| **Low** | `await_verdict` / `await_resubmission` structural duplication *(Adversarial pass, new packages)* | ~180 LOC overlap (40%) — identical event-loop skeleton, watcher setup, polling fallback. Will compound if new await_* ops are added | Extract shared `awaitWithWatcher()` helper; evaluate clarity vs abstraction |
| **Low** | `ApplyYAMLTimeouts` type-switch *(Adversarial pass, new packages)* | Type-switch over 3 concrete strategy types instead of interface method; violates Strategy pattern contract | Add `ApplyTimeoutOverrides()` to `RoleStrategy` interface |
| **Low** | Timestamp `time.Now()` inconsistency in `wt_merge.go` *(Adversarial pass, data flow; health check: 3 sites)* | Lines 118, 451, 473 use `time.Now()` while all other ops files use `time.Now().UTC()` — worsened from 2 to 3 sites | Change to `time.Now().UTC()` — three-line fix |
| **Low** | Role mapping bidirectionality unverified *(pass 17, Coupling lens)* | `runtimeToWorkflow` / `workflowToRuntime` maps (9 entries each) manually synchronized; missing entry causes runtime failure | Add 10-line test verifying bidirectional consistency |
| **Low** | CLI Executor concrete coupling *(pass 17, Coupling lens)* | `DefaultCLIExecutor.Execute()` switch statement hardcodes 5 CLI tools + per-tool flags; `mistral→vibe` rename hardcoded; adding CLIs requires code modification in two functions | Accept if CLI set is stable; consider data-driven profiles if growing |
| **Low** | Scattered timeout constants *(pass 17, Coupling lens; health check: mcp/protocol removed)* | 7+ timeout/interval values across 4 packages (filelock, db, agent) bypass `models.Config` centralization pattern; some values duplicated (5s in two packages) | Document as reference table; extract inline values to named constants |
| **Low** | Pipeline-aware status check triplication *(pass 16, Duplication lens — partially resolved `581d377`)* | 3 functions with identical shape differing only in resolver method; legacy fallback branches removed but structural triplication remains | Parameterize into `checkPipelineStatus(task, pr, resolverFn)` helper |
| **Low** | Worktree path helper not reused *(pass 16, Duplication lens; verification: 8 sites now)* | 8 sites inline `filepath.Join(paths.WorktreesDirName, taskID)` (was 4) despite `git.GetWorktreeRelPath()` at `worktree.go:171` — worsened as new call sites were added without adopting the helper | Use `GetWorktreeRelPath()` at remaining sites; add `GetWorktreeBranchName()` to `paths/` |
| **Low** | Dual logging undocumented *(pass 16, Duplication lens; verification: 19 calls)* | 19 `log.Printf` in `ops/` alongside formal `internal/log/` Logger; two streams (stderr vs `.liza/logs/`) with no documented contract | Document the dual-purpose split; consider structured wrapper for `log.Printf` warnings |
| **Low** | `statevalidate` composition gap *(pass 15, Coverage lens)* | Data-integrity package at 55.1% — entry-point validators all at 0% while inner validators are well-covered | Table-driven tests calling `ValidateStateFile` with various malformed states |
| **Low** | `models/state.go` `ValidateTransition` untested *(pass 15, Coverage lens)* | System mode transition table (RUNNING/PAUSED/STOPPED) with no test coverage — pure function, easy to test | Table-driven test covering valid transitions, known rejections, and unknown source modes |
| **Low** | `roles` Phase 2 functions untested *(pass 15, Coverage lens)* | 4 role-query functions at 0% (`DoerRoles`, `ReviewerRoles`, `IsDoerRole`, `IsReviewerRole`) | Add simple assertions for these pure functions |
| **Low** | `embedded` installation functions partially untested *(pass 15, Coverage lens; partially resolved; health check: MCP functions removed)* | `WriteGlobalFiles` (5 tests) and `WriteClaudeSettings` (4 tests) now covered. Still untested: `PlanGlobalFiles`, `WritePipelineConfig`, `WriteGuardrails` | Smoke tests for remaining untested functions |
| **Low** | `prompts → ops` dependency inversion *(pass 14, Boundaries lens; health check: worsened to 5+ functions)* | Prompt layer depends on business-logic layer for 5+ utility functions; architecturally wrong direction, deepened since pass 14 | Move pipeline query functions to `pipeline` or query package; pass `PipelineDetectionContext` as parameter |
| **Low** | `commands → agent` boundary crossing *(pass 14, Boundaries lens)* | Status command calls `agent.DetectOrchestratorWakeTriggers()`, breaking peer relationship | Move wake detection to `ops` or future query package |
| **Low** | Import analysis table drift *(pass 14, Boundaries lens; health check: mcp row removed)* | Packages had undercounted imports; corrected in pass 14 | Corrected; mcp entries now stale |
| **Low** | Residual raw `1800` in supervisor.go *(pass 6; verification: 1 site remains)* | 1 call site (supervisor.go:393) bypasses `models.DefaultLeaseDurationSeconds` constant | Replace with named constant |
| **Low** | Duplicated identity validation *(pass 6; health check: NOT dead code — actively called from supervisor.go:558)* | `agent/registration.go:validateIdentity()` reimplements `identity` package logic; called at registration time with tests | Replace with `identity.ValidateFormat()` + `identity.ValidateRole()` imports |
| **Low** | Inconsistent ops parameter conventions *(pass 6)* | `AddTask` takes `statePath`/`logPath` while 15+ others take `projectRoot` | Standardize on `projectRoot` |
| **Low** | `LIZA_LOG_LEVEL` documentation drift *(Adversarial pass, entry: config/)* | Env var documented but no runtime reader; logger is fixed at INFO | Implement env-driven log level or remove from docs |
| **Low** | `os.Stat` existence checks under-handle non-`IsNotExist` errors *(Adversarial pass, entry: error handling — partially resolved: `52ceac5`)* | Some presence checks classify only exists/missing and miss permission/I/O distinctions. `wt_merge.go` integration-test stat now handles tri-state correctly | Standardize tri-state handling in remaining sites |
| **Low** | High nesting in `claiming.go` helpers *(pass 7, partially resolved: `ac4ce6f5`; verification: improved)* | `handleApprovedMerges` improved: now 47 LOC (was 55) with max depth ~4-5. Cleaner early returns. Still has nested `IntegrationFailedError` handling. `resumeHandoffTask` extracted to `ops.ResumeHandoff` | Accept current nesting or extract error-classification into helper |
| **Low** | No interface-based seams *(pass 3)* | Deliberate simplicity; acceptable for v1 | Monitor test suite time; introduce seams if needed |
| **Low** | Regenerate `coverage.out` *(pass 4)* | Report shows 0% for functions with thorough tests; may predate recent commits | Run `make test` to update |
| **Low** | Broken Vision link in sprint governance spec *(Adversarial pass, entry: specs/)* | `../vision.md` target is missing; canonical Vision is under `specs/build/` | Fix link to canonical Vision path |
| **Low** | Ops Modify-callback task guard *(pass 5, Duplication lens)* | 10 files repeat identical FindTask+NotFoundError inside Modify callbacks | Consider `modifyTask(bb, taskID, fn)` helper; evaluate clarity vs indirection |
| **Low** | Command test `initialState` construction *(pass 5, Duplication lens)* | 23 near-identical State constructions with same Config values | Add `testhelpers.DefaultState()` returning pre-configured State |
| **Low** | Watch thresholds not configurable *(pass 6)* | 10 operational constants bypass `models.Config` pattern | Add to Config with current values as defaults |
| **Low** | Hardcoded `"terminal-1"` *(pass 6)* | All agents report same terminal regardless of actual TTY | Derive from config or actual terminal |
| **None** | Pipeline config loaded per-operation *(pass 13; health check: 54 call sites)* | 54 call sites via `loadResolver` — correct, simple, negligible overhead. Tripled with ops expansion | Accept as-is; each op is self-contained |
| **None** | Ops input validation boilerplate *(pass 5)* | 10 files with `if taskID == ""` — idiomatic Go, low risk | Not worth abstracting |
| **None** | `task.History = append(...)` pattern *(pass 5)* | 12 occurrences with variations — coincidental similarity | Not worth abstracting |
| **None** | statevalidate internal micro-patterns *(pass 16, Duplication lens)* | ~40 lines of structural repetition (6 identical classifier methods, set uniqueness, field checks) within single 658-LOC file | Not worth abstracting; file is internally consistent |
| **None** | `formatKeyValue` bubble sort | Works, small data sets, not perf-sensitive | Not worth changing |
| **None** | Global logger singleton | Acceptable for CLI scope | Not worth changing for v1 |

---

## Summary

Liza's architecture is well-suited to its constraints: a file-based multi-agent coordination system for solo developers. The dependency graph is clean with no cycles. Test coverage is excellent (2.6:1 ratio, up from 2.4:1) with consistent patterns and strong helper infrastructure. The atomic state persistence via flock and fsync+rename is correctly implemented. Health monitoring is comprehensive. The task state machine is now explicit with a complete transition map.

**Pass 2 (Complexity lens)** identified monolithic command functions — `WtMergeCommand` and `ClaimTaskCommand` at 310-319 LOC each (since resolved via ops extraction). Task-lookup duplication (55+ inline loops) was also resolved via `State.FindTask()`.

**Pass 3 (Boundaries lens)** identified the `commands` package as the system's central boundary concern. Business logic was extracted to `internal/ops/` service layer; agent now imports `ops` instead of `commands`. All stdin reads now accept `io.Reader` parameter. Domain/persistence boundaries are well-drawn. *(health check: MCP adapter layer removed in `90c132d5`)*

**Pass 4 (Coverage lens)** adds quantitative depth: statement coverage overall concentrated uncovered code in two patterns — runtime orchestration code (supervisor Execute) and I/O-coupled functions. I/O coupling is the primary driver of untested critical paths — functions with hardwired `os.Stdin`/`os.Stdout`/`os/exec` account for the majority of the 0% coverage.

All six primary structural concerns identified across passes 1-4 have been resolved: supervisor decomposition, commands/ops extraction, monolithic functions, MCP locking, MCP dispatch testing, and agent→commands dependency. The ops layer now contains ~9,920 LOC serving 2 consumers (agent, commands). *(health check: MCP consumer removed; ops grew 68%)*

**Pass 5 (Duplication lens)** examined cross-file repetition patterns. The most significant duplication pattern is within the `ops/` package itself: 10 of 19 ops files repeat an identical 3-line FindTask+NotFoundError guard inside `bb.Modify` callbacks, and 12 files share structurally similar history-append code. The `readTaskState()` helper addresses this for the Read path but has no equivalent for the Modify path. This is idiomatic Go — each function is independently authored with the same pattern — and the impact is low (maintenance burden if the guard pattern changes). In test code, 23 command test files construct near-identical `initialState` objects; a `testhelpers.DefaultState()` helper would be a low-risk improvement. Overall, the codebase's earlier duplication issues (task-lookup loops 55×, file-locking, magic numbers) have been resolved. The remaining repetition is largely structural — Go's explicit style trading conciseness for clarity.

**Pass 6 (Coupling lens)** focused on configuration hardcoding, tight dependencies, and hidden state sharing. Major items resolved: `"task/"` branch prefix centralized (`paths.TaskBranchPrefix`), role naming unified (`internal/roles` package), `GracePeriod` divergence unified (`models.LeaseExpiryGracePeriod`). Remaining open items: identity validation duplication, ops parameter convention split, watch threshold configurability, raw 1800 residuals in supervisor.go, and hardcoded `"terminal-1"`.

**Pass 7 (Complexity lens)** revisits complexity with the benefit of 6 prior passes of context. `ClaimTask` complexity and `inspect_field.go` manual reflection have been resolved. `ops/wt_merge.go:MergeWorktree` at 520 LOC (file total) remains a complex function with phased flow. `ops/claim_task.go` shrank to 563 LOC (was 655) — improvement. *(health check: `ops/proceed.go` at 1,200 LOC is now the new complexity leader)*

**Adversarial pass (entry: docs/)** forced a doc-first path and surfaced contract-level drift missed by prior code-centric passes. All items resolved: state-machine spec drift, troubleshooting branch naming, and testing-doc short-mode drift.

**Adversarial pass (entry: specs/)** surfaced coherence gaps: (1) Pairing Session Initialization doc pointer drift resolved (`docs/USAGE.md` now exists as index file), and (2) sprint governance links Vision via `../vision.md` while canonical Vision lives in `specs/build/0 - Vision.md`. Watcher stall detection resolved (`61b16d5`).

**Adversarial pass (entry: tests/)** CLI contract coverage gap resolved (`9d95c1c` — `mutation_wiring_test.go`). Temporal coupling partially resolved: `time.Sleep` reduced from 21 to 5, `t.Parallel()` introduced (15 uses), ratchet tests prevent regression. Remaining serial tests constrained by process-global state.

**Adversarial pass (entry: config/)** exposed a config-contract gap cluster. Resolved: iteration limit enforcement, heartbeat interval wiring, config field projection. Remaining open config drift: `LIZA_LOG_LEVEL` remains unimplemented.

**Adversarial pass (entry: error handling)** surfaced a reliability-observability gap cluster. MCP parse-error and stale-lock cleanup errors resolved. Remaining: rebase/worktree cleanup flows in `submit_review.go`, `git/worktree.go`, and `wt_delete.go` still suppress secondary failures. Some `os.Stat` checks still under-handle non-`IsNotExist` filesystem errors.

**Adversarial pass (entry: data flow, first)** traced the task lifecycle. `DeleteTask` side-effect ordering resolved (`7dd05ce`). `submit-for-review` commit_sha semantics fixed (`d4c688e`) then regressed — needs re-verification.

**Adversarial pass (entry: data flow, second)** traced input through handlers → ops → blackboard, focusing on data integrity and authorization. *(health check: MCP handler findings (1, 3, 4, 6) are now stale — MCP removed in `90c132d5`).* Remaining findings: (2) **Unbounded integration test execution** (Medium) — `MergeWorktree` runs `integration-test.sh` with no timeout; a hanging test blocks the entire merge pipeline indefinitely. (5) Timestamp inconsistency — `wt_merge.go` uses `time.Now()` (now 3 sites) while all other ops files use `.UTC()`.

**Adversarial pass (entry: documented smells)** — all four items resolved: REJECTED reassignment atomicity, planner max-wait enforcement, watch/log O(n) growth, and MCP stdio frame-size guard. *(health check: MCP item now irrelevant — package removed)*

**Pass 14 (Boundaries lens)** revisits import direction and layer violations with a complete production-only import graph. Key findings: (1) `prompts → ops` dependency inversion — prompt builder imports utility functions from the business-logic layer, creating a deep transitive dependency chain *(health check: worsened to 5+ functions)*. (2) `commands → agent` boundary crossing — `status.go` calls `agent.DetectOrchestratorWakeTriggers()`, breaking the intended peer relationship. (3) Query logic scattered across 3 architectural layers creates a transitive chain `commands → agent → ops` for read-only queries — the code-level manifestation of the "No Query Layer" trajectory issue. *(health check: MCP layer removed from chain)*

**Pass 16 (Duplication lens)** revisits cross-file and intra-file duplication patterns. Four new findings: (1) `models/state.go:215-239` has three pipeline-aware status-check functions (`IsApprovedForMerge`, `IsSubmittedStatus`, `IsExecutingStatus`) with identical 6-line structure — parameterizable but low priority. (2) Worktree path construction (`filepath.Join(paths.WorktreesDirName, taskID)`) appears in 5 locations despite `git.GetWorktreeRelPath()` helper existing at `worktree.go:304` — the helper was created but never adopted by other call sites. (3) `ops/` contains 21+ `log.Printf()` calls alongside the formal `internal/log/` structured Logger, creating two undocumented logging streams (stderr vs `.liza/logs/`). (4) `statevalidate/validate.go` has ~40 lines of internal micro-duplication (6 identical `statusClassifier` methods, repeated set-uniqueness checks, field-presence patterns). All are Low or None priority. The pass 5 Duplication findings (ops callback boilerplate, test harness repetition) remain accurate and unchanged. Overall, the codebase's duplication posture is healthy — remaining repetition is idiomatic Go or below the abstraction threshold.

**Pass 17 (Coupling lens)** revisits coupling with emphasis on configuration hardcoding, tight dependencies, and hidden state sharing. The import dependency graph was independently verified — clean acyclic DAG with one known inversion (`prompts → ops`, documented pass 14). Four new findings: (1) `DefaultCLIExecutor.Execute()` switch statement hardcodes 5 CLI tools and their specific flag formats — the `CLIExecutor` interface is clean but the concrete implementation is tightly coupled to specific external tool APIs, with `mistral → vibe` rename also hardcoded in two functions. (2) Dual transition system coupling — `models.taskTransitions` (hardcoded) and `pipeline.Resolver.TransitionMap()` (config-driven) coexist with manual branching in 7+ ops files; meta-state transitions are independently maintained in both systems without cross-validation. (3) `roles` package `runtimeToWorkflow` / `workflowToRuntime` maps are manually synchronized without startup verification — a missing entry would cause runtime failures during task claiming. (4) Timeout/interval constants scattered across 5 packages (8+ values from 50ms to 30m) bypass the `models.Config` centralization pattern used for other runtime parameters. All existing findings verified as still accurate. `loadResolver` call count updated from 14 to 16+.

**Health check (after pass 12)** updated LOC figures across all components (~20,900 production / ~54,900 test, up 32% from prior review). Added `internal/pipeline/` package (641 LOC). Resolved High-priority recommendation (Pairing init doc pointer drift). All other open findings verified as still present.

**Pass 13 (Complexity lens)** revisits complexity with fresh LOC and branch density metrics. Key findings: branch density quantified, `models` reclassified as near-leaf (imports `roles`), `waitforwork.go` refactored to generic callbacks, pipeline config loaded per-operation (now 54 call sites). *(health check: condensed)*

**Pass 15 (Coverage lens)** revisits test coverage. Key findings: `statevalidate` lowest-coverage functional package at 55.1%, `models/state.go` governance helpers at 0%, `roles` at 60%, `embedded` installation functions partially untested. *(health check: `WriteMCPSettings` reference now stale — MCP removed; coverage % needs re-measurement)*

**Verification pass (2026-03-11)** confirmed current state of all findings. 7 recommendations resolved: `cmd/liza/main.go` split into 7 files, `waitForXWork` refactored to generic callbacks, `validateTaskInvariants` decomposed with helpers, MCP dispatch layer tests added, template duplication eliminated, `derefString` consolidated, temporal test coupling at acceptable level. *(health check: `validateIdentity` was incorrectly labeled "dead code" — it is actively called from `supervisor.go:558`; MCP-related resolutions now moot)* 4 changed: `handleApprovedMerges` nesting improved, `embedded` installation partially covered, worktree path helper sites grew, raw `1800` reduced to 1. ~40 issues confirmed still present.

**Health check (after pass 17, 2026-04-13)** — major structural change: MCP server removed (`90c132d5`, ADR-0057). `internal/mcp/` (~2,070 LOC) and `cmd/liza-mcp/` deleted. Agents now interact via CLI `--json` mode. Codebase grew 44% to ~33,400 production LOC / ~87,400 test LOC (2.6:1 ratio, improved from 2.4:1). Six new packages added: `tui/` (2,265 LOC — 3rd largest, TUI for agent monitoring), `interactive/`, `render/`, `jsonout/`, `process/`, `gitenv/`. Key file growth: `ops/proceed.go` doubled to 1,200 LOC (new largest file), `commands/init.go` at 854 LOC (new god file candidates). `ops/claim_task.go` shrank from 655→563 (improved). `prompts → ops` dependency worsened (3→5+ functions). `loadResolver` call sites tripled (16→54). `validateIdentity` corrected from "dead code" to actively used (called from `supervisor.go:558`). ~8 MCP-related findings removed as stale; all non-MCP findings verified still present. Integration tests grew 147% to ~4,120 LOC.

**Adversarial pass (entry: new packages, 2026-04-13)** explored 6 new packages and new ops/agent files via targeted exploration. Key findings: (1) **`ops/proceed.go` god file** (Medium) — 1,200 LOC with 5 distinct responsibilities (manual transitions, batch execution, crash recovery, cycle detection/toposort, child builders), 26 functions, 107 `if` statements. Well-tested (31 tests) but cognitive load is high; extractable into 4-5 cohesive files. (2) **`await_verdict` / `await_resubmission` structural duplication** (Low) — ~180 LOC overlap (40%) in event-loop skeleton, watcher setup, and polling fallback. (3) **`ApplyYAMLTimeouts` type-switch** (Low) — violates Strategy interface contract. Two new strengths documented: RoleStrategy pattern (clean role-specific orchestration) and TUI boundary compliance (2,265 LOC, 172 tests, no business logic leakage). All 6 new packages assessed: `tui/` is well-structured Bubbletea MVC with proper delegation to ops/commands; `render/` and `gitenv/` are exemplary leaf packages with zero internal deps; `jsonout/` has acceptable boundary crossing for error classification; `process/` and `gitenv/` lack test files. New ops files (await_verdict, advance_sprint, await_resubmission, replan, recover_task, init_project — ~2,091 LOC total) follow established patterns consistently. CLI Executor codex flag updated — no longer references `mcp_servers` (stale smell description corrected).

**Adversarial pass (targeted: INVARIANTS.md)** traced 10 specific invariant claims against enforcement code. Two Medium-severity gaps found: (1) **§7 clean worktree check** — `submit_review.go` does not check working tree cleanliness before submission despite the invariant promising this; the submission validates commit SHA, TDD, rebase, and checkpoint but not `git status`. (2) **§6 provider diversity** — `submit_verdict.go` checks approval count but never calls `HasProviderDiversity()`; diversity is enforced at reviewer claim-filtering (doer-vs-reviewer provider mismatch) and merge-readiness evaluation ("preferred" soft enforcement), not at verdict time as the invariant implies. Three Low-severity documentation gaps: (3) §12 system mode transitions incomplete — TRIPPED→RUNNING (via Resume) not listed, STOPPED transitions omitted, any→TRIPPED bypasses ValidateTransition entirely. (4) §7 different-coder rejected worktree recreation not enforced — `ensureRejectedWorktreeExists()` preserves worktree regardless of claimer identity. (5) §3 status naming uses pre-pipeline names and transition map attributed to `models/task.go` when it lives in `ops/pipeline_ops.go`. Additionally: §8 hypothesis exhaustion enforcement is spec-only (assess_blocked.go only records history, doesn't force rescope); §3.5 appendUniqueAgentID also called from submit_review.go, not just wt_merge.go. Overall: the invariant *concepts* are correct, but several enforcement claims are aspirational rather than implemented. The gap between documented invariants and code reality creates a false confidence surface.

---

## Appendix: File Reference

| Component | Location |
|-----------|----------|
| Domain model | `internal/models/` |
| State persistence | `internal/db/` |
| Agent supervisor | `internal/agent/` |
| Task operations | `internal/ops/` |
| CLI commands | `internal/commands/` |
| Terminal UI | `internal/tui/` |
| Git operations | `internal/git/` |
| Pipeline configuration | `internal/pipeline/` |
| Prompt generation | `internal/prompts/` |
| Embedded assets | `internal/embedded/` |
| State validation | `internal/statevalidate/` |
| Path utilities | `internal/paths/` |
| Logging | `internal/log/` |
| Pattern analysis | `internal/analysis/` |
| Identity resolution | `internal/identity/` |
| Role constants | `internal/roles/` |
| Error types | `internal/errors/` |
| JSON output | `internal/jsonout/` |
| Output rendering | `internal/render/` |
| Interactive init | `internal/interactive/` |
| Process spawning | `internal/process/` |
| Git environment | `internal/gitenv/` |
| Test helpers | `internal/testhelpers/` |
| CLI entry point | `cmd/liza/` |
| Integration tests | `internal/integration/` |
