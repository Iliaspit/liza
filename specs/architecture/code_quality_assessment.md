# Code Quality Assessment and Refactoring Roadmap

* Date: 2026-07-24 (commit `ffe89080`)
* Previous: 2026-04-13 (commit `672a7d95`) — 424 commits since
* Repository: liza
* Author: Codex
* Mode: Reassessment

## Document Role

This assessment is a dated measurement and prioritization snapshot. [`architectural-issues.md`](architectural-issues.md#open-issues-summary) is the sole lifecycle authority for open and resolved architectural findings; duplicated roadmap items below link to their canonical registry anchors.

## Repository Metrics Dashboard

Metrics cover tracked files only. Untracked working-tree files were excluded.

| Metric | Previous | Current | Delta |
|--------|----------|---------|-------|
| Go production LOC | 32,788 (195 files) | 65,509 (284 files) | +32,721 (+100%), +89 files |
| Go test LOC | 86,153 (166 files) | 150,683 (246 files) | +64,530 (+75%), +80 files |
| Go test-to-production ratio | 2.63:1 | 2.30:1 | -0.33 |
| Go test functions | 1,649 | 2,998 | +1,349 (+82%) |
| Go benchmarks | — | 6 | — |
| Go fuzz functions | 0 | 0 | — |
| Python production LOC | Treated as vestigial | 5,114 (8 files) | Material production surface |
| Python test LOC | — | 2,196 (6 files) | 0.43:1 ratio; 65 test functions |
| TypeScript LOC | — | 132 (1 embedded tool) | — |
| Shell LOC | — | 1,822 (10 files) | — |
| Production Go files >500 LOC | 12 | 35 | +23 (+192%) |
| Production Go functions/methods >150 LOC | 16 | 21 | +5 (+31%) |
| Specifications | 192 | 219 Markdown files / 26,763 LOC, excluding this report | +27 files |
| Active ADRs | 60 | 96 | +36 |
| Documentation | 36 guides | 53 Markdown files / 15,105 LOC | Broader measurement |
| Root contract files | 9 | 8 / 1,898 LOC | -1 |
| Skills | 22 | 28 | +6 |
| Lessons | 6 | 7 | +1 |
| Direct Go dependencies | 9 | 13 | +4 |
| External Go modules | — | 66 | — |
| Python dependencies | — | 3 runtime + 5 development | — |
| Pre-commit hooks | 23 | 22 | -1 |

### Largest Production Go Files

| LOC | File |
|----:|------|
| 1,566 | `internal/scipsearch/scipsearch.go` |
| 1,530 | `internal/embedded/embedded.go` |
| 1,500 | `internal/ops/proceed.go` |
| 1,407 | `internal/commands/watch.go` |
| 1,268 | `internal/commands/init.go` |
| 1,259 | `internal/updater/updater.go` |
| 1,258 | `cmd/liza/cmd_task.go` |
| 1,142 | `cmd/liza/cmd_launch.go` |
| 1,129 | `internal/agent/supervisor.go` |
| 1,053 | `internal/pairingindex/hooks.go` |
| 956 | `internal/agent/prompt.go` |
| 893 | `internal/pipeline/resolver.go` |
| 871 | `internal/tui/view.go` |
| 853 | `internal/models/task.go` |
| 806 | `internal/ops/wt_merge.go` |
| 757 | `internal/ops/claim_task.go` |
| 754 | `internal/semble/semble.go` |
| 742 | `internal/tui/update.go` |
| 736 | `internal/commands/inspect_tasks.go` |
| 720 | `cmd/liza/cmd_system.go` |

Sixty-seven production Go files exceed 300 LOC. Thirty-five exceed the 500 LOC investigation threshold. Declarative Cobra registration files account for part of this distribution, but the largest behavioral files show genuine responsibility concentration. *(reassessment 2026-07-24)*

### Longest Production Go Functions

Twenty-one production functions or methods exceed 150 LOC. The largest are:

| LOC | Function | Location |
|----:|----------|----------|
| 442 | `RunSupervisor` | `internal/agent/supervisor.go:637` |
| 405 | `InitCommandWithConfig` | `internal/commands/init.go:763` |
| 316 | `MergeWorktree` | `internal/ops/wt_merge.go` |
| 315 | `SubmitForReview` | `internal/ops/submit_review.go` |
| 302 | `SubmitVerdict` | `internal/ops/submit_verdict.go` |
| 222 | `InitProject` | `internal/commands/init.go` |
| 218 | `ExecuteAvailableTransitions` | `internal/ops/proceed.go` |
| 209 | `completeClaimTaskAfterValidation` | `internal/ops/claim_task.go` |
| 204 | `renderTaskPanel` | `internal/tui/view.go` |
| 194 | `Replan` | `internal/ops/replan.go` |

### Dependencies

The dependency surface remains moderate for a multi-provider terminal application: 13 direct Go dependencies and 66 external modules in the complete module graph. Python adds three runtime and five development dependencies.

Two pinned Go dependencies merit compatibility review rather than an urgent security response: the repository uses `golang.org/x/mod` v0.22.0 while [pkg.go.dev lists v0.38.0](https://pkg.go.dev/golang.org/x/mod), and `go-runewidth` v0.0.16 while its [version history lists v0.0.24](https://pkg.go.dev/github.com/mattn/go-runewidth?tab=versions). `x/mod` is used only for semantic-version handling in the updater, which limits the review surface. *(reassessment 2026-07-24)*

### Code Hygiene

Positive signals:

- No production Go or Python `TODO`, `FIXME`, or `HACK` markers.
- No Go `nolint` or `nosec` suppressions.
- No production use of the empty `interface{}` spelling.
- Three Python `type: ignore` comments are scoped to optional or platform-specific imports.
- Four `os.Exit` calls are confined to command entry points.

Corrections to the previous assessment:

- Production Go contains two `panic()` calls, in `cmd/liza/completion_values.go` and `internal/providers/catalog.go`. Both appear to protect embedded configuration invariants, but the count is not zero.
- Roles, cardinalities, phase categories, and output formats are not consistently typed. Repeated raw values such as `"doer"`, `"reviewer"`, `"orchestrator"`, `"per-subtask"`, `"one-to-one"`, and `"many-to-one"` participate in control flow across packages despite partial ownership in `internal/roles` and `internal/pipeline`.
- Uses of `any` are concentrated around JSON, YAML, ACP, and other dynamic boundaries; they are not a blanket hygiene concern.

## Executive Summary

Liza remains a well-engineered, specification-driven multi-agent orchestrator with strong correctness infrastructure. Its Go runtime has extensive tests, race-enabled CI, meaningful integration coverage, explicit state validation, and disciplined error handling. Documentation depth—219 specifications and 96 active ADRs—is exceptional.

The codebase has nevertheless crossed from isolated large-file concerns into sustained structural-debt growth. Production Go LOC doubled across 424 commits, while files over 500 LOC grew from 12 to 35. The three P1 decomposition targets from the previous assessment all grew substantially without remediation. Python is now a material production surface, but CI does not install Python or run its tests, linter, or type checker.

### Key Strengths

- **High testing investment:** 150,683 Go test LOC, 2,998 test functions, race-enabled unit runs, and 14 E2E files covering real supervisor and operations flows.
- **Explicit correctness boundaries:** state mutation, pipeline resolution, persistence, and validation remain separated and heavily exercised.
- **Traceable engineering decisions:** 219 specifications and 96 active ADRs preserve design intent at unusual depth.
- **Dependency restraint:** 13 direct Go dependencies is reasonable for the feature surface; dynamic integrations remain isolated in dedicated packages.
- **Clean source hygiene:** no production TODO backlog or broad suppression culture.

### Areas for Improvement

- **P1 structural debt compounded:** `proceed.go`, `init.go`, and `supervisor.go` grew to 1,500, 1,268, and 1,129 LOC respectively.
- **New responsibility concentrations emerged:** `embedded.go` combines provider-specific and artifact-specific mutation; `watch.go` combines monitoring, diagnosis, repair, and reporting.
- **CI and local quality gates diverge:** CI invokes `make lint`, but that target does not run staticcheck, goimports, duplicate detection, Python linting, Python typing, or Python tests.
- **Python quality parity is incomplete:** four production Python modules exceed 500 LOC; two skill packages have no tests; the aggregate test ratio is 0.43:1.
- **Control-flow vocabulary is only partially owned:** raw role, cardinality, phase, and format literals create cross-package typo and refactoring risk.

## Overall Grade: B+ (Good)

The foundation remains solid, and several subsystems are strong enough to support continued development. The deduction from A- to B+ is specifically for compounding structural complexity and incomplete enforcement of the repository's now multi-language quality contract.

The previous assessment stated that another review without progress on the P1 decomposition targets would warrant this downgrade. All three targets grew:

| Target | Previous | Current | Change |
|--------|---------:|--------:|-------:|
| `internal/ops/proceed.go` | 1,200 | 1,500 | +25% |
| `internal/commands/init.go` | 854 | 1,268 | +48% |
| `internal/agent/supervisor.go` | 831 | 1,129 | +36% |

The grade does not fall further because testing depth, state invariants, documentation, and dependency discipline remain strong.

---

## Detailed Subsystem Analysis

### Domain State, Pipeline, Persistence, and Validation (`internal/models/`, `internal/pipeline/`, `internal/db/`, `internal/statevalidate/`) ★★★★☆

**Strengths:**

- Test ratios remain strong: models 2.05:1, pipeline 2.97:1, database 4.43:1, and state validation 1.86:1.
- Pipeline configuration and resolution provide a declarative state-machine boundary rather than distributing transition rules through CLI handlers.
- State validation remains a distinct integrity layer rather than incidental checks inside operations.
- Persistence has 2,729 test LOC for 616 production LOC.

**Concerns:**

- `pipeline/resolver.go` reached 893 LOC and `pipeline/config.go` 600 LOC. They remain reasonably cohesive, but their growth raises navigation cost.
- `models/task.go` reached 853 LOC. Its methods remain focused, so this is structural density rather than a god-object finding.
- Phase categories and related control-flow values sometimes bypass the typed vocabulary already present in the pipeline package.

### Lifecycle Operations and Worktree Integration (`internal/ops/`, `internal/git/`) ★★★☆☆

**Strengths:**

- `internal/ops` has 41,637 test LOC for 16,377 production LOC (2.54:1).
- Operations retain explicit precondition checks and focused public entry points.
- Git operations isolate worktree and merge mechanics behind a package boundary.

**Concerns:**

- `proceed.go` grew 1,200→1,500 LOC and combines transition execution, cohort handling, dependency propagation, graph algorithms, crash recovery, and child construction.
- Seven operations files exceed 500 LOC.
- Several state-changing functions are long: `MergeWorktree` 316 LOC, `SubmitForReview` 315, `SubmitVerdict` 302, and `ClaimTask` 187.
- The volume and growth show that file-per-operation organization alone is no longer containing orchestration complexity.

### Agent Supervision, Prompts, and Providers (`internal/agent/`, `internal/prompts/`, `internal/providers/`) ★★★☆☆

**Strengths:**

- Agent tests remain substantial: 21,425 test LOC for 7,890 production LOC (2.72:1).
- Prompt tests have a 4.46:1 ratio.
- Provider catalog and runtime adapters remain separated from core state mutation.

**Concerns:**

- `supervisor.go` grew 831→1,129 LOC; `RunSupervisor` grew 287→442 LOC.
- Restart tracking, process execution, lease behavior, and the main supervision loop remain co-located.
- `prompt.go` reached 956 LOC and now contains repeated adapter plumbing for SCIP, Stacklit, functional clusters, and Semble.
- Raw role literals are repeated across agent, command, operation, and pipeline packages despite an existing roles package.

### Commands and CLI Wiring (`internal/commands/`, `cmd/liza/`) ★★★☆☆

**Strengths:**

- `internal/commands` maintains a 3.31:1 test ratio.
- Most Cobra handlers remain thin delegation layers.
- Large `cmd/liza` files are partly declarative registration, which is less concerning than equivalent behavioral LOC.

**Concerns:**

- `watch.go` grew 846→1,407 LOC and combines observation, anomaly diagnosis, automated repair, lifecycle decisions, and output.
- `init.go` grew 854→1,268 LOC; `InitCommandWithConfig` is now 405 LOC.
- `cmd_task.go` and `cmd_launch.go` exceed 1,100 LOC, increasing command-discovery and registration maintenance cost even where code is declarative.
- Inspect commands repeat format switches for JSON, YAML, table, and scalar output. This is imperative ceremony suited to shared declarative dispatch.

### Terminal UI and Interactive Flows (`internal/tui/`, `internal/interactive/`) ★★★★☆

**Strengths:**

- The Model-Update-View architecture remains recognizable.
- TUI tests total 4,680 LOC for 2,663 production LOC (1.76:1).
- View rendering and update behavior are tested separately.

**Concerns:**

- `view.go` grew to 871 LOC and `update.go` to 742 LOC.
- `renderTaskPanel` is 204 LOC and `Update` is 172 LOC.
- Interactive setup has a lower 0.43:1 test ratio.

The TUI remains strong because its large files are cohesive and follow the framework's shape. Split by panel or message family when extending it, rather than introducing abstractions solely to reduce LOC.

### Tooling Integrations (`internal/scipsearch/`, `internal/semble/`, `internal/functionalclusters/`, `internal/pairingindex/`) ★★★☆☆

**Strengths:**

- External tools are isolated behind dedicated packages.
- SCIP and Semble have meaningful tests with ratios of 1.22:1 and 1.09:1.
- Pairing-index behavior is kept separate from core lifecycle operations.

**Concerns:**

- `scipsearch.go` is the largest production Go file at 1,566 LOC. It combines configuration, runtime refresh, worktree exclusions, command execution, and language-specific planning for Go, TypeScript, and Python.
- The root cause in SCIP is variant dispatch, so a per-language planner strategy is more appropriate than a purely mechanical file split.
- `pairingindex/hooks.go` reached 1,053 LOC and combines hook installation, exclude handling, and script generation.
- Prompt and installation layers repeat integration-specific wiring, increasing the cost of adding another index provider.

### Installation, Embedding, Branding, Updates, and Toolchain (`internal/embedded/`, `internal/updater/`, `internal/toolchain/`, `internal/brand/`) ★★★☆☆

**Strengths:**

- Installation artifacts are embedded and testable rather than fetched implicitly at runtime.
- Updater behavior is isolated in a dedicated package with a 1.56:1 test ratio.
- Toolchain discovery is separate from project state.

**Concerns:**

- `embedded.go` reached 1,530 LOC and mixes global corpus writing, Claude JSON merging, Codex TOML and permission handling, hook installation, stale MCP cleanup, and project artifact generation.
- These artifact families have different schemas and change drivers; their co-location is responsibility mixing, not merely a large cohesive module.
- `updater.go` is 1,259 LOC. Its release/source/rollback responsibilities are related, but future growth should be partitioned by update source and transaction phase.

### Python Skill Utilities (`skills/**/*.py`) ★★☆☆☆

**Strengths:**

- Existing adversarial-pairing and log-analysis tests are behavioral: they exercise subprocess contracts, state transitions, sparse events, and rich event parsing.
- Dependencies remain small and explicit in `pyproject.toml`.

**Concerns:**

- Production totals 5,114 LOC versus 2,196 test LOC (0.43:1).
- `skills/liza-logs/scripts/analyze-log.py` is 1,924 LOC.
- `context-corpus-index.py` is 968 LOC; `blackboard_op.py` and `blackboard_state.py` are 661 and 650 LOC.
- The context-engineering and white-box-red-testing Python packages have no tests.
- CI installs no Python environment and runs no pytest, ruff, or mypy checks.

The previous characterization of Python tooling as vestigial is stale. It is now a maintained production surface and should receive the same enforcement expectations as Go.

### Testing and Quality Infrastructure ★★★★☆

**Strengths:**

- Go has a 2.30:1 test-to-production ratio and 2,998 test functions.
- `make test` enables the race detector and coverage collection.
- Fourteen E2E files total 6,208 LOC.
- Sampled integration tests execute the real supervisor and operations flows with the LLM boundary mocked, rather than only asserting internal helper behavior.
- Test guards ratchet parallel usage and cap real sleeps.

**Concerns:**

- Codecov upload is non-blocking and no coverage threshold or trend gate exists.
- There are no Go fuzz tests despite concurrency, YAML parsing, graph operations, and state-transition inputs.
- Fifteen `t.Parallel()` calls across 246 test files is limited adoption; the guard enforces a floor, not broad parallel-test discipline.
- Ten real `time.Sleep` uses remain in tests, within the configured ceiling of eleven.
- Python tests are absent from CI.

### Pre-Commit and CI Pipeline ★★★☆☆

**Strengths:**

- CI runs on Linux and macOS and executes lint, race-enabled unit tests, E2E tests, and builds.
- The local pre-commit configuration includes 22 hooks covering Go, Python, file hygiene, duplicate detection, and commit conventions.

**Concerns:**

- CI's `make lint` runs embedded/test-helper checks, `go fmt`, and `go vet`; it does not execute the complete pre-commit suite.
- Staticcheck, goimports verification, jscpd, ruff, mypy, and pytest are therefore not merge gates.
- `goimports@latest` and `staticcheck@latest` make local tool resolution non-reproducible.
- Codecov uses `fail_ci_if_error: false` and has no threshold.

### Documentation and Specifications ★★★★☆

**Strengths:**

- The repository contains 219 specification files, 96 active ADRs, and 53 documentation files.
- ADRs provide strong historical rationale and implementation traceability.
- Operational lessons capture recurring tool and worktree failures.

**Concerns:**

- `specs/architecture/overview.md` shows only the orchestrator/coder/reviewer topology while the embedded pipeline defines 13 roles.
- The overview lists four providers, while the provider catalog contains nine.
- Corpus scale creates lifecycle risk: superseded architecture descriptions remain discoverable unless indexes and current-overview documents are actively maintained.

---

## Previous Finding Disposition

| Previous finding | Status | Current evidence |
|------------------|--------|------------------|
| [Decompose `proceed.go`](architectural-issues.md#decompose-proceedgo-1500-loc) | Worsened | 1,200→1,500 LOC |
| [Decompose `init.go`](architectural-issues.md#decompose-initgo-1268-loc) | Worsened | 854→1,268 LOC; main function 405 LOC |
| [Decompose `supervisor.go`](architectural-issues.md#decompose-supervisorgo-1129-loc) | Worsened | 831→1,129 LOC; main loop 442 LOC |
| [Watch structural debt](architectural-issues.md#decompose-watchgo-1407-loc) | Worsened / elevated to P1 | 846→1,407 LOC |
| TUI file size | Still relevant | view 871; update 742 |
| Missing process/gitenv tests | Resolved | Both packages now have tests |
| Interactive test gap | Still relevant | 0.43:1 ratio |
| Coverage threshold | Still relevant | Reporting only |
| Fuzz testing | Still relevant | Zero fuzz functions |
| Spec-code automation | Partially addressed | Extensive specs, but overview drift remains |
| Binary-size tracking | Still relevant | No automated budget |
| Remove vestigial Python tooling | Stale / withdrawn | Python is now 5,114 production LOC |

---

## Prioritized Refactoring Roadmap

### P1: High Impact / Low Risk

#### 1.1 Decompose `internal/ops/proceed.go`
- **Registry:** [Decompose proceed.go (1,500 LOC)](architectural-issues.md#decompose-proceedgo-1500-loc)
- **What:** Separate transition execution, cohort management, graph algorithms, crash recovery, child construction, and available-transition scheduling along existing responsibility boundaries.
- **Risk:** Low — structural relocation with existing test coverage.
- **Impact:** Reduces the largest lifecycle coordination hotspot and makes state-transition reviews tractable.
- **Depends on:** None.

#### 1.2 Decompose `internal/commands/init.go`
- **Registry:** [Decompose init.go (1,268 LOC)](architectural-issues.md#decompose-initgo-1268-loc)
- **What:** Extract project detection, configuration generation, artifact setup, and interactive phases from `InitCommandWithConfig`.
- **Risk:** Low — sequential phases already have identifiable data boundaries.
- **Impact:** Makes brownfield initialization changes local and independently testable.
- **Depends on:** None.

#### 1.3 Decompose `internal/agent/supervisor.go`
- **Registry:** [Decompose supervisor.go (1,129 LOC)](architectural-issues.md#decompose-supervisorgo-1129-loc)
- **What:** Move restart/spin tracking and default CLI execution out of the core supervision loop.
- **Risk:** Low — the tracker types are internally cohesive and heavily tested.
- **Impact:** Narrows the 442 LOC main loop's context and separates policy from process execution.
- **Depends on:** None.

#### 1.4 Decompose `internal/commands/watch.go`
- **Registry:** [Decompose watch.go (1,407 LOC)](architectural-issues.md#decompose-watchgo-1407-loc)
- **What:** Separate observation, diagnosis, automated repair, lifecycle decisions, and rendering.
- **Risk:** Low — preserve behavior and output contracts while relocating cohesive helpers.
- **Impact:** Prevents the monitoring command from becoming a second orchestration engine.
- **Depends on:** None.

#### 1.5 Split `internal/embedded/embedded.go` by Artifact Family
- **Registry:** [Split embedded.go by Artifact Family (1,530 LOC)](architectural-issues.md#split-embeddedgo-by-artifact-family-1530-loc)
- **What:** Create focused implementation files for global corpus, Claude settings, Codex settings/permissions, hooks, project artifacts, and stale-artifact cleanup.
- **Risk:** Low — package API can remain unchanged.
- **Impact:** Aligns code ownership with independent schemas and change drivers.
- **Depends on:** None.

#### 1.6 Enforce the Multi-Language Quality Contract in CI
- **What:** Install the locked Python development environment; run pytest, ruff, and mypy; enforce the intended Go/static and duplicate-detection gates through a shared CI target or pre-commit invocation. Pin Go lint-tool versions.
- **Risk:** Low — enforcement only; initial failures should be treated as discovered debt rather than bypassed.
- **Impact:** Makes merge protection match the repository's actual language surface and local policy.
- **Depends on:** None.

#### 1.7 Own Control-Flow Vocabulary
- **What:** Define role, cardinality, and phase constants/types at their owning package boundaries and replace raw cross-package comparisons.
- **Risk:** Low — behavior-preserving compile-time consolidation.
- **Impact:** Removes typo-prone string coupling and makes vocabulary changes searchable and compiler-assisted.
- **Depends on:** None.

### P2: Medium Impact / Medium Risk

#### 2.1 Introduce SCIP Language Planner Strategies
- **What:** Move Go, TypeScript, and Python root/config planning behind a small planner interface or declarative registry.
- **Risk:** Medium — changes dispatch structure around external tooling.
- **Impact:** Addresses the design cause of `scipsearch.go` complexity instead of only moving lines.
- **Depends on:** Stable integration contract tests.

#### 2.2 Restore Python Utility Test and Structure Parity
- **What:** Add behavioral tests for context-engineering and white-box-red-testing utilities; split the 1,924 LOC log analyzer and other >500 LOC scripts around parsing, analysis, and rendering boundaries.
- **Risk:** Medium — Python scripts expose CLI and serialized-output contracts.
- **Impact:** Makes a material production surface safer to evolve.
- **Depends on:** Recommendation 1.6.

#### 2.3 Add Coverage Trend Enforcement
- **What:** Establish an initial non-regression threshold or diff-coverage policy instead of selecting an aspirational absolute number.
- **Risk:** Medium — poorly calibrated thresholds can reward gaming.
- **Impact:** Converts coverage reporting into a useful regression signal.
- **Depends on:** Python tests running in CI.

#### 2.4 Refresh the Current Architecture Overview
- **What:** Update role topology and provider inventory; link historical diagrams explicitly to their validity period.
- **Risk:** Low.
- **Impact:** Restores the overview as a reliable onboarding source.
- **Depends on:** None.

#### 2.5 Centralize Inspect Output Dispatch
- **What:** Express JSON, YAML, table, and scalar renderers through shared declarative format dispatch.
- **Risk:** Medium — output compatibility must be preserved.
- **Impact:** Removes repeated switch ceremony across inspect commands.
- **Depends on:** Snapshot/contract tests for output formats.

#### 2.6 Review Stale Direct Dependencies
- **What:** Compatibility-test current `x/mod` and `go-runewidth` releases, then update independently.
- **Risk:** Medium — updater semantics and terminal-width behavior require targeted verification.
- **Impact:** Reduces long-lived version drift without broad dependency churn.
- **Depends on:** Existing updater and TUI tests.

#### 2.7 Partition TUI Growth by Panel and Message Family
- **What:** Split rendering and update handlers only where panels or message families are already cohesive.
- **Risk:** Medium — premature abstraction would make Elm-style flow harder to follow.
- **Impact:** Keeps extension cost bounded while preserving the framework's natural architecture.
- **Depends on:** None.

### P3: Strategic / Long-Term

#### 3.1 Add Targeted Fuzz Tests
- **What:** Fuzz state mutation inputs, pipeline YAML, graph transitions, and concurrency-relevant parsers.
- **Risk:** Medium — requires stable invariants and deterministic seeds.
- **Impact:** Exercises combinations that table-driven tests are unlikely to enumerate.

#### 3.2 Automate Spec and Overview Drift Signals
- **What:** Generate or validate role/provider inventories and other enumerable architecture facts from canonical configuration.
- **Risk:** Medium — generated documentation needs clear ownership.
- **Impact:** Reduces staleness in a large specification corpus.

#### 3.3 Track Release Binary Size
- **What:** Record release artifact size and alert on significant unexplained growth.
- **Risk:** Low.
- **Impact:** Makes embedded-corpus and dependency growth visible.

---

## Conclusion

Liza's quality problem is not weak engineering practice; it is insufficient structural follow-through under rapid feature growth. The runtime remains reliable and heavily tested, but recurring concentration points are growing faster than they are decomposed, and CI has not caught up with the active Python surface.

The highest-return next step is a structural-debt iteration that performs the existing decompositions while aligning CI with the quality checks the repository already declares. Completing those items would provide a credible path back to A- without requiring an architectural rewrite.
