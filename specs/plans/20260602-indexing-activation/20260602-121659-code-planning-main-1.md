# Code Planning: Indexing Activation

Task: `code-planning-main-1`
Agent: `code-planner-2`
Spec: `specs/goals/20260602-indexing-activation.md`

## Based On

- `specs/goals/20260602-indexing-activation.md` read in full.
- `support-docs/CONFIGURATION.md` lines 114-319.
- `support-docs/CUSTOMIZING_AGENT_TOOLS.md` lines 1-215.
- `internal/embedded/hooks/session-context.sh` lines 1-269.
- `internal/prompts/templates/base_prompt.tmpl` lines 100-214.
- `internal/commands/setup.go` lines 1-120 and 220-270.
- `internal/commands/init.go` lines 1-90 and 560-640.
- `internal/scipsearch/scipsearch.go` lines 1-760.
- ADR-0068 and ADR-0074.
- Stacklit module summary for `internal/commands`, `internal/ops`, `internal/prompts`, `internal/scipsearch`, `internal/stacklit`, and `internal/semble`.

## Architectural Approach

Keep setup, pairing init, MAS runtime context, and documentation as separate ownership boundaries.

`liza setup` owns only generic, global `AGENT_TOOLS.md` defaults. It must not learn project paths or readiness state.

Pairing init owns project-local activation artifacts. Put hook planning and generated script contracts behind a pairing-specific package so `internal/commands/init.go` remains orchestration, not hook implementation. The pairing hook package owns `.git/hooks/liza-index.sh` generation and lifecycle hook chaining. Language-specific SCIP root selection remains owned by `internal/scipsearch`, but exposed through a pairing-specific planning API so stricter pairing monorepo ambiguity behavior does not silently rewrite MAS runtime behavior.

MAS runtime activation remains the current env-gated, prompt-metadata-driven path: Stacklit and Semble are process-local gates, SCIP is process-local plus durable `config.scip_search`, and prompts include only metadata that actually exists for the current target root.

SessionStart and MAS prompt routing must stay separate. SessionStart may advertise repo-root pairing indexes; MAS prompts may advertise task/reviewer/orchestrator target metadata. Both surfaces consume the generic routing contract: indexes are navigation aids, direct reads are evidence.

## Interface Contracts

### Global Tool Guidance Contract

Owner: Task 1.

The installed default `AGENT_TOOLS.md` must contain generic conditional routing for Stacklit, SCIP, and Semble. It may mention capability classes and explicit paths supplied by Liza. It must not mention project-specific paths, generated paths for a particular repository, or claims that optional tools are installed.

### Pairing Hook Contract

Owner: Task 2.

`internal/pairingindex` owns the pairing Git hook installation contract:

- discover the effective hooks directory using Git, including non-default `core.hooksPath`;
- preserve, chain, or reject collisions for `post-commit`, `post-checkout`, `post-merge`, and `post-rewrite`;
- generate one repo-local `liza-index.sh` entrypoint;
- run automatic lifecycle refresh without Stacklit AI summary;
- allow manual `liza-index.sh ai` to include Stacklit AI-summary behavior;
- keep generated repo-root artifacts out of accidental diffs unless intentionally tracked.

Task 5 consumes this interface from pairing init. No other task writes generated hook script semantics.

### Pairing SCIP Planning Contract

Owner: Task 3.

`internal/scipsearch` owns language validation, supported language vocabulary, indexer command construction, and pairing root ambiguity diagnostics for SCIP. Pairing planning must return concrete Go, TypeScript, and Python command plans for one confidently selected root per enabled language, or a typed ambiguity diagnostic listing candidate roots and language. Repeated `--scip-search <language>` values restrict enabled languages but are not root/cwd selection.

MAS runtime APIs in `internal/scipsearch` keep their existing deterministic/fallback root inference unless Task 3 explicitly changes them and records the divergence decision in tests and task notes.

### Pairing Semble Safety Contract

Owner: Task 4.

`internal/semble` owns the default physical `.sembleignore` payload and project-root safety check used before pairing SessionStart advertises Semble. Pairing init may create or verify `.sembleignore`; private Git excludes alone are not sufficient.

### Init Orchestration Contract

Owner: Task 5.

`internal/commands.InitPairingCommand` and CLI pairing init wiring own when project-local activation runs. They consume Task 2, Task 3, and Task 4 package APIs. They do not define hook script internals, SCIP root selection, or Semble ignore contents.

### Prompt Context Contract

Owner: Task 6.

`internal/prompts/templates/base_prompt.tmpl` and prompt metadata builders own MAS prompt context boundaries. Prompt sections render only from explicit generated metadata/readiness for the target root. Generic query routing text must avoid duplicated long-form instructions and must state that index and semantic-search output is navigation evidence only after direct source verification.

## Shared File Ownership

| File or generated artifact | Writing task | Read-only consumers | Dependency rule |
|---|---|---|---|
| `contracts/AGENT_TOOLS.md` | Task 1 | Task 8 | Task 8 depends on Task 1 |
| `internal/embedded/contracts/AGENT_TOOLS.md` | Task 1 | Task 8 | Task 8 depends on Task 1 |
| `internal/pairingindex/**` | Task 2 | Task 5, Task 7 | Task 5 depends on Task 2; Task 7 depends on Task 5 |
| `internal/scipsearch/**` | Task 3 | Task 5, Task 6, Task 7 | Task 5 depends on Task 3 |
| `internal/semble/**` | Task 4 | Task 5, Task 6, Task 7 | Task 5 depends on Task 4 |
| `internal/commands/init.go` and related pairing init command wiring | Task 5 | Task 7, Task 8 | Task 7 and Task 8 depend on Task 5 |
| `internal/prompts/templates/base_prompt.tmpl` and prompt metadata tests | Task 6 | Task 7, Task 8 | Task 7 and Task 8 depend on Task 6 |
| `support-docs/CONFIGURATION.md` and embedded support docs | Task 8 | none | Task 8 depends on implementation tasks |
| Pairing repo `.git/hooks/liza-index.sh` content contract | Task 2 | Task 3 supplies commands; Task 5 invokes install | Task 5 depends on Tasks 2 and 3 |

## Planned Tasks

### Task 1 - Install generic optional-index routing in setup defaults

desc: Install generic optional-index routing in setup defaults.

done_when: A fresh `liza setup` installs default `AGENT_TOOLS.md` content that conditionally routes Stacklit, SCIP, and Semble only when Liza supplies explicit paths or readiness, describes fallback search/read tools for disabled tools, preserves user customization protection, and contains no project-specific generated paths.

scope: Owns `contracts/AGENT_TOOLS.md`, `internal/embedded/contracts/AGENT_TOOLS.md`, and setup/embedded tests that verify installed default content and customization protection. Does not modify pairing init, MAS prompt templates, SessionStart hooks, or project-local activation artifacts.

spec_ref: specs/goals/20260602-indexing-activation.md

validation:

- `go test ./internal/embedded ./internal/commands`

decomposition:

```yaml
owned_files:
  - contracts/AGENT_TOOLS.md
  - internal/embedded/contracts/AGENT_TOOLS.md
  - internal/embedded/embedded_test.go
  - internal/commands/setup_test.go
owned_modules:
  - contracts
  - internal/embedded
  - internal/commands setup tests
read_only_depends_on: []
read_only_task_depends_on: []
interfaces_owned:
  - Global Tool Guidance Contract
interfaces_consumed: []
coverage_notes: "Covers FT-001 and general fallback guidance. Unit tests should assert default installed text, fallback wording, no repo-specific paths, and existing setup customization protection."
```

### Task 2 - Add pairing index hook infrastructure with Stacklit lifecycle refresh

desc: Add pairing index hook infrastructure with Stacklit lifecycle refresh.

done_when: Pairing index infrastructure can install or verify effective Git hook plumbing for lifecycle refresh, preserve or explicitly reject existing hook collisions, respect non-default `core.hooksPath`, generate a repo-local `liza-index.sh` contract that refreshes `stacklit.json` automatically without AI-summary, supports manual `ai` invocation for AI-summary, and keeps generated Stacklit artifacts out of accidental task diffs unless intentionally tracked.

scope: Owns a pairing-specific hook infrastructure package such as `internal/pairingindex`, its tests, and generated hook/script content contract. May add test fixtures for hook directories and Git status behavior. Does not wire pairing init CLI behavior, does not implement SCIP root detection, does not update global `AGENT_TOOLS.md`, and does not change MAS worktree index generation.

spec_ref: specs/goals/20260602-indexing-activation.md

validation:

- `go test ./internal/pairingindex`

depends_on:

- Task 1

decomposition:

```yaml
owned_files:
  - internal/pairingindex/**
owned_modules:
  - internal/pairingindex
read_only_depends_on:
  - 0
read_only_task_depends_on: []
interfaces_owned:
  - Pairing Hook Contract
interfaces_consumed:
  - Global Tool Guidance Contract
coverage_notes: "Covers Stacklit hook generation, lifecycle hook preservation, core.hooksPath handling, automatic no-AI refresh, manual AI refresh, and generated artifact cleanliness."
```

### Task 3 - Add pairing SCIP root planning with ambiguity diagnostics

desc: Add pairing SCIP root planning with ambiguity diagnostics.

done_when: `internal/scipsearch` exposes pairing-specific SCIP planning that respects repeated `--scip-search <language>` filters, selects exactly one confident root per enabled Go, TypeScript, or Python language, emits concrete indexer commands with required root/cwd arguments, fails with a diagnostic listing candidate roots and unresolved language when root selection is ambiguous, and states whether MAS runtime inference remains intentionally different.

scope: Owns `internal/scipsearch` pairing planning APIs and tests for Go, TypeScript, Python, explicit language filtering, confident single-root planning, and monorepo ambiguity. Does not install Git hooks, does not wire pairing init CLI behavior, and does not change MAS runtime planning unless the task explicitly documents and tests that choice.

spec_ref: specs/goals/20260602-indexing-activation.md

validation:

- `go test ./internal/scipsearch`

depends_on:

- Task 1

decomposition:

```yaml
owned_files:
  - internal/scipsearch/scipsearch.go
  - internal/scipsearch/scipsearch_test.go
owned_modules:
  - internal/scipsearch
read_only_depends_on:
  - 0
read_only_task_depends_on: []
interfaces_owned:
  - Pairing SCIP Planning Contract
interfaces_consumed:
  - Global Tool Guidance Contract
coverage_notes: "Covers SCIP single-root command construction, explicit language filters, ambiguity diagnostics, and the FT-003 divergence note for MAS runtime inference."
```

### Task 4 - Add pairing Semble project-root safety activation

desc: Add pairing Semble project-root safety activation.

done_when: Pairing Semble activation can create or verify a safe physical project-root `.sembleignore` using the default sensitive/runtime ignore patterns before SessionStart can advertise Semble, reports a clear diagnostic when safety cannot be established, and leaves MAS Semble worktree safety semantics intact.

scope: Owns `internal/semble` project-root ignore helpers and tests for default payload, idempotent verification, unsafe/missing ignore handling, and diagnostics. Does not wire pairing init CLI behavior, does not edit SessionStart hook output, and does not add durable Semble config to state.

spec_ref: specs/goals/20260602-indexing-activation.md

validation:

- `go test ./internal/semble`

depends_on:

- Task 1

decomposition:

```yaml
owned_files:
  - internal/semble/semble.go
  - internal/semble/semble_test.go
owned_modules:
  - internal/semble
read_only_depends_on:
  - 0
read_only_task_depends_on: []
interfaces_owned:
  - Pairing Semble Safety Contract
interfaces_consumed:
  - Global Tool Guidance Contract
coverage_notes: "Covers physical .sembleignore creation/verification, credential/runtime ignore patterns, and non-interference with MAS Semble prompt safety."
```

### Task 5 - Wire pairing init to project-local index activation

desc: Wire pairing init to project-local index activation.

done_when: `liza init --claude`, `liza init --codex`, and combined pairing init flows preserve existing provider hook installation while env-gated project-local activation invokes Stacklit hook setup, SCIP hook command planning, and Semble `.sembleignore` safety without modifying `~/.liza/AGENT_TOOLS.md`; disabled env gates activate no optional indexing behavior; failures report clear diagnostics instead of installing inert hooks or guessed SCIP commands.

scope: Owns `internal/commands/init.go`, CLI command wiring for pairing `--scip-search <language>` where applicable, and command-level tests for disabled gates, Stacklit enabled, SCIP enabled/filtering, Semble enabled, hook collisions, and non-default hooks paths. Consumes Task 2, Task 3, and Task 4 package APIs. Does not redefine hook script internals, SCIP root selection rules, Semble ignore payload, global setup guidance, MAS prompt metadata, or user-facing documentation.

spec_ref: specs/goals/20260602-indexing-activation.md

validation:

- `go test ./cmd/liza ./internal/commands`

depends_on:

- Task 1
- Task 2
- Task 3
- Task 4

decomposition:

```yaml
owned_files:
  - internal/commands/init.go
  - internal/commands/init_test.go
  - cmd/liza/main.go
  - cmd/liza/main_test.go
owned_modules:
  - internal/commands
  - cmd/liza
read_only_depends_on:
  - 0
  - 1
  - 2
  - 3
read_only_task_depends_on: []
interfaces_owned:
  - Init Orchestration Contract
interfaces_consumed:
  - Global Tool Guidance Contract
  - Pairing Hook Contract
  - Pairing SCIP Planning Contract
  - Pairing Semble Safety Contract
coverage_notes: "Covers pairing init orchestration across provider combinations, disabled gates, enabled gates, collision diagnostics, hooksPath diagnostics, and no AGENT_TOOLS mutation."
```

### Task 6 - Preserve MAS prompt gating and context boundaries

desc: Preserve MAS prompt gating and context boundaries.

done_when: MAS prompt construction includes Stacklit, SCIP, and Semble routing sections only when the corresponding generated metadata or readiness is present for the current task, reviewer, or orchestrator target; disabled env gates omit sections even when stale artifacts exist; available-tool failures omit only the failed section; rendered guidance distinguishes discovery, orientation, symbol tracing, exact search, syntax search, and direct evidence reads without repo-root pairing assumptions.

scope: Owns MAS prompt metadata/rendering code and tests in `internal/prompts`, plus directly related prompt metadata builders in `internal/agent` or `internal/ops` if existing ownership requires it. Does not modify pairing SessionStart hooks, pairing init activation, global `AGENT_TOOLS.md`, or generated index locations.

spec_ref: specs/goals/20260602-indexing-activation.md

validation:

- `go test ./internal/prompts ./internal/agent ./internal/ops`

decomposition:

```yaml
owned_files:
  - internal/prompts/templates/base_prompt.tmpl
  - internal/prompts/builder.go
  - internal/prompts/builder_test.go
  - internal/prompts/role_context.go
  - internal/agent/prompt.go
  - internal/agent/prompt_test.go
  - internal/agent/strategy_orchestrator.go
  - internal/agent/strategy_orchestrator_scip_test.go
  - internal/ops/scip_indexing.go
  - internal/ops/stacklit_indexing.go
  - internal/ops/semble_indexing.go
  - internal/ops/semble_indexing_test.go
owned_modules:
  - internal/prompts
  - internal/agent prompt metadata paths
  - internal/ops prompt metadata paths
read_only_depends_on: []
read_only_task_depends_on: []
interfaces_owned:
  - Prompt Context Contract
interfaces_consumed: []
coverage_notes: "Covers MAS env gates, metadata-present rendering, stale artifact omission, failure isolation, role/target-specific paths, and concise query routing boundaries."
```

### Task 7 - Add end-to-end coverage for indexing activation

desc: Add end-to-end coverage for indexing activation.

done_when: Integration tests exercise the implemented setup, pairing init, SessionStart, and MAS prompt behavior for disabled tools, Stacklit-only activation, SCIP single-root planning, SCIP monorepo ambiguity, Semble safety artifact handling, non-default hook paths, and generated artifact cleanliness through the project's integration test layer.

scope: Owns integration-level tests under `internal/integration` and any narrowly scoped command test fixtures needed to run full flows. Does not change production behavior except mechanical test fixture seams required to exercise existing public commands.

spec_ref: specs/goals/20260602-indexing-activation.md

validation:

- `go test ./internal/integration ./internal/commands`

depends_on:

- Task 1
- Task 5
- Task 6

decomposition:

```yaml
owned_files:
  - internal/integration/**
owned_modules:
  - internal/integration
read_only_depends_on:
  - 0
  - 4
  - 5
read_only_task_depends_on: []
interfaces_owned: []
interfaces_consumed:
  - Global Tool Guidance Contract
  - Pairing Hook Contract
  - Pairing SCIP Planning Contract
  - Pairing Semble Safety Contract
  - Init Orchestration Contract
  - Prompt Context Contract
coverage_notes: "Cross-cutting e2e coverage for the new user-visible setup/init/prompt behavior; no production ownership."
```

### Task 8 - Document setup and init indexing activation

desc: Document setup and init indexing activation.

done_when: `support-docs/CONFIGURATION.md`, `support-docs/CUSTOMIZING_AGENT_TOOLS.md` if needed, and synced embedded support docs explain that `liza setup` owns global generic guidance while pairing `liza init` owns project-local Stacklit/SCIP/Semble activation artifacts, distinguish pairing-init env-gate behavior from MAS runtime activation, document disabled-tool fallback behavior, and keep generated/project-specific paths out of global guidance examples.

scope: Owns user-facing support docs and embedded support-doc copies plus consistency tests that enforce sync. Does not change runtime behavior, command wiring, hook scripts, prompt templates, or contract files except documentation copies.

spec_ref: specs/goals/20260602-indexing-activation.md

validation:

- `go test ./internal/embedded`

depends_on:

- Task 1
- Task 5
- Task 6

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
read_only_depends_on:
  - 0
  - 4
  - 5
read_only_task_depends_on: []
interfaces_owned:
  - User documentation for indexing activation
interfaces_consumed:
  - Global Tool Guidance Contract
  - Init Orchestration Contract
  - Prompt Context Contract
coverage_notes: "Cross-cutting documentation deliverable for user-visible setup/init behavior and embedded doc sync."
```

## Dependency Graph

```text
Task 1
  -> Task 2
  -> Task 3
  -> Task 4
Task 2 + Task 3 + Task 4
  -> Task 5
Task 6
Task 1 + Task 5 + Task 6
  -> Task 7
  -> Task 8
```

Task 6 can run independently because it owns MAS prompt gating and does not write files owned by pairing init tasks. Task 7 and Task 8 intentionally depend on implemented behavior to avoid tests/docs being written against planned-but-unmerged interfaces.

## Systemic Decomposition Review

[LOAD-BEARING]

The generated `liza-index.sh` contract is a shared boundary disguised as a script. Stacklit refresh, SCIP commands, Git lifecycle events, existing hook preservation, and generated artifact cleanliness all converge there.

Implication: If more than one task owns the generated script contract, parallel implementations will create incompatible hook semantics that only fail after init or SessionStart.

[TENSION]

The goal requires stricter pairing SCIP monorepo ambiguity behavior while preserving or explicitly superseding MAS runtime inference. Those are different operational contexts: pairing init writes durable repo-local hooks, while MAS runtime refresh produces best-effort worktree snapshots.

Implication: Treating one inference policy as automatically shared will either make pairing init guess unsafe roots or make MAS runtime fail in workflows that currently degrade.

[CASCADE]

Setup defaults, pairing init artifacts, SessionStart context, MAS prompt metadata, support docs, and embedded sync all describe optional tool availability. Drift in any one surface causes agents to invoke tools that are unavailable, stale, or pointed at the wrong root.

Implication: The feature needs implementation, e2e, and docs tasks tied by dependency order; otherwise partial success will look correct locally but still misroute agents in another mode.

## Spec Compliance Matrix

| # | Requirement | Source | Task(s) | Status |
|---|-------------|--------|---------|--------|
| 1 | Generic routing guidance must be safe when Stacklit, SCIP, or Semble are disabled, missing, or not advertised. | General NFR-000-1, lines 45-46 | Task 1, Task 6, Task 8 | Covered |
| 2 | Project-specific paths, generated index locations, and readiness state must not be written into global `AGENT_TOOLS.md`. | General NFR-000-2, lines 47-48 | Task 1, Task 5, Task 8 | Covered |
| 3 | Optional indexing failures must omit unavailable prompt/context sections instead of making agents troubleshoot unrelated tooling. | General NFR-000-3, lines 49-51 | Task 5, Task 6, Task 7 | Covered |
| 4 | `liza setup` installs generic Stacklit, SCIP, and Semble routing guidance into default `AGENT_TOOLS.md`. | FR-001-1, lines 96-97 | Task 1 | Covered |
| 5 | Guidance is conditional on explicit Stacklit path, explicit SCIP path, or supplied Semble target/readiness. | FR-001-2, lines 98-102 | Task 1, Task 6 | Covered |
| 6 | Guidance describes fallbacks for disabled or unavailable tools. | FR-001-3, lines 103-106 | Task 1, Task 8 | Covered |
| 7 | Guidance contains no project-specific absolute paths, generated repo locations, or installed-tool claims. | FR-001-4, lines 107-109 | Task 1, Task 8 | Covered |
| 8 | Installed default guidance remains concise. | NFR-001-1, lines 113-115 | Task 1 | Covered |
| 9 | Fresh setup installs generic Stacklit/SCIP/Semble routing guidance. | AC-001-1, lines 119-121 | Task 1 | Covered |
| 10 | With optional tools disabled, guidance still routes to valid fallback tools. | AC-001-2, lines 122-124 | Task 1 | Covered |
| 11 | User-customized `AGENT_TOOLS.md` keeps setup customization protection. | AC-001-3, lines 125-126 | Task 1 | Covered |
| 12 | Pairing init flows install project-local provider hooks as today. | FR-002-1, lines 156-157 | Task 5, Task 7 | Covered |
| 13 | Stacklit truthy pairing init installs or verifies Git hook plumbing for repo-root `stacklit.json` refresh. | FR-002-2, lines 158-160 | Task 2, Task 5 | Covered |
| 14 | Automatic Git lifecycle refresh must not run `stacklit ai-summary`; manual `ai` may. | FR-002-2a, lines 161-163 | Task 2, Task 7 | Covered |
| 15 | SCIP truthy pairing init autodetects repo-specific SCIP plan and hook plumbing for enabled languages. | FR-002-3, lines 164-167 | Task 3, Task 5 | Covered |
| 16 | Pairing SCIP autodetection plans concrete Go, TypeScript, and Python indexer commands with root/cwd arguments. | FR-002-3a, lines 168-170 | Task 3 | Covered |
| 17 | Repeated `--scip-search` flags restrict languages but are not sufficient root/cwd selection. | FR-002-3b, lines 171-173 | Task 3, Task 5 | Covered |
| 18 | Ambiguous monorepo SCIP roots fail with candidate-root diagnostic instead of guessed hook command. | FR-002-3c, lines 174-177 | Task 3, Task 5, Task 7 | Covered |
| 19 | Confident single-root SCIP detection generates concrete repo-specific SCIP commands. | FR-002-3d, lines 178-180 | Task 3, Task 5 | Covered |
| 20 | Pairing init keeps generated indexes ignored, privately excluded, or out of accidental task diffs unless intentionally tracked. | FR-002-4, lines 181-183 | Task 2, Task 5, Task 7 | Covered |
| 21 | Semble truthy pairing init ensures safe physical `.sembleignore` before SessionStart advertises Semble. | FR-002-5, lines 184-186 | Task 4, Task 5, Task 7 | Covered |
| 22 | Pairing init must not modify `~/.liza/AGENT_TOOLS.md`. | FR-002-6, lines 187-188 | Task 5, Task 7 | Covered |
| 23 | Pairing init preserves existing project Git hooks and does not silently overwrite lifecycle hooks. | FR-002-7, lines 189-191 | Task 2, Task 5, Task 7 | Covered |
| 24 | Pairing init detects non-default `core.hooksPath` and installs safely or reports diagnostic. | FR-002-8, lines 192-195 | Task 2, Task 5, Task 7 | Covered |
| 25 | Docs distinguish pairing-init env gates from MAS runtime activation. | FR-002-9, lines 196-199 | Task 8 | Covered |
| 26 | Pairing index activation preserves SessionStart concrete path/readiness rule. | NFR-002-1, lines 203-205 | Task 5, Task 6, Task 7 | Covered |
| 27 | Pairing activation does not assume Liza's own build system or target project stack. | NFR-002-2, lines 206-207 | Task 2, Task 3, Task 5 | Covered |
| 28 | Disabled pairing env vars activate no optional indexing hook behavior. | AC-002-1, lines 211-212 | Task 5, Task 7 | Covered |
| 29 | Stacklit-enabled pairing lifecycle refresh works without editing `AGENT_TOOLS.md`. | AC-002-2, lines 213-215 | Task 2, Task 5, Task 7 | Covered |
| 30 | Semble-enabled pairing init creates or reports missing safety artifact before advertisement. | AC-002-3, lines 216-218 | Task 4, Task 5, Task 7 | Covered |
| 31 | Generated indexes do not appear as accidental untracked files after init and refresh. | AC-002-4, lines 219-221 | Task 2, Task 5, Task 7 | Covered |
| 32 | Single-root Go, TypeScript, or Python pairing SCIP init generates concrete commands. | AC-002-5, lines 222-225 | Task 3, Task 5, Task 7 | Covered |
| 33 | Monorepo ambiguous SCIP roots fail clearly instead of writing guessed hook. | AC-002-6, lines 226-228 | Task 3, Task 5, Task 7 | Covered |
| 34 | `--scip-search go` restricts detection to Go while still requiring confident Go root selection. | AC-002-7, lines 229-231 | Task 3, Task 5, Task 7 | Covered |
| 35 | Existing project Git hooks are preserved, chained, or explicitly diagnosed. | AC-002-8, lines 232-234 | Task 2, Task 5, Task 7 | Covered |
| 36 | Non-default `core.hooksPath` installs into effective path or reports unsafe diagnostic. | AC-002-9, lines 235-237 | Task 2, Task 5, Task 7 | Covered |
| 37 | Automatic Stacklit lifecycle refresh skips AI-summary. | AC-002-10, lines 238-240 | Task 2, Task 7 | Covered |
| 38 | Manual `liza-index.sh ai` includes Stacklit AI-summary behavior. | AC-002-11, lines 241-242 | Task 2, Task 7 | Covered |
| 39 | Docs explain pairing-init env gates separately from MAS runtime activation. | AC-002-12, lines 243-245 | Task 8 | Covered |
| 40 | MAS init preserves Stacklit/Semble process-local gates and SCIP env gate plus durable config. | FR-003-1, lines 282-284 | Task 6, Task 7 | Covered |
| 41 | MAS prompts include optional tool sections only when corresponding metadata is present. | FR-003-2, lines 285-286 | Task 6 | Covered |
| 42 | MAS prompt routing stays role/target specific. | FR-003-3, lines 287-289 | Task 6 | Covered |
| 43 | MAS init behavior does not depend on pairing Git hooks or repo-root pairing indexes. | FR-003-4, lines 290-291 | Task 5, Task 6, Task 7 | Covered |
| 44 | Preserve or explicitly supersede current MAS SCIP runtime planning semantics. | FR-003-5, lines 292-296 | Task 3, Task 6 | Covered |
| 45 | Plan documents divergence or aligns MAS if pairing SCIP ambiguity behavior is stricter. | FR-003-6, lines 297-300 | Task 3 | Covered |
| 46 | Disabled SCIP env gate omits MAS SCIP prompt even with config. | AC-003-1, lines 304-305 | Task 6, Task 7 | Covered |
| 47 | Disabled Stacklit env gate omits MAS Stacklit prompt unless documented fallback exists. | AC-003-2, lines 306-308 | Task 6, Task 7 | Covered |
| 48 | Disabled Semble env gate omits MAS Semble prompt. | AC-003-3, lines 309-310 | Task 6, Task 7 | Covered |
| 49 | Enabled tool readiness/index failure omits failed MAS section while other guidance remains usable. | AC-003-4, lines 311-313 | Task 6, Task 7 | Covered |
| 50 | Pairing SCIP autodetection task states MAS inference choice. | AC-003-5, lines 314-317 | Task 3 | Covered |
| 51 | Pairing SessionStart continues emitting concrete repo-root optional tool instructions only when artifacts/readiness exist. | FR-004-1, lines 345-347 | Task 5, Task 6, Task 7 | Covered |
| 52 | MAS prompts continue using structured task/reviewer/orchestrator metadata rather than pairing SessionStart assumptions. | FR-004-2, lines 348-350 | Task 6 | Covered |
| 53 | Prompt and SessionStart guidance avoid duplicated long-form routing text when shorter shared wording is sufficient. | FR-004-3, lines 351-352 | Task 1, Task 6, Task 8 | Covered |
| 54 | Generic routing text states indexes/search results are navigation aids and direct source reads remain evidence. | FR-004-4, lines 353-355 | Task 1, Task 6, Task 8 | Covered |
| 55 | Pairing with no indexes and Semble disabled emits initialization guidance without optional tool blocks. | AC-004-1, lines 359-361 | Task 5, Task 7 | Covered |
| 56 | MAS mode with generated worktree indexes uses worktree-specific paths. | AC-004-2, lines 362-364 | Task 6, Task 7 | Covered |
| 57 | Full routing guidance distinguishes conceptual discovery, module orientation, symbol navigation, exact search, syntax search, and direct reads. | AC-004-3, lines 365-368 | Task 1, Task 6, Task 8 | Covered |
| 58 | Unavailable optional tools are not instructed before fallback search/read tools. | AC-004-4, lines 369-371 | Task 1, Task 6, Task 8 | Covered |
| E2E | e2e test coverage for new behavior | Cross-cutting | Task 7 | Covered |
| DOC | Documentation updates for changed behavior | Cross-cutting | Task 8 | Covered |

## Pre-Submit Self-Check Notes

- Every planned task has one observable intent; tests are colocated with the behavior they validate except Task 7, which owns cross-component e2e coverage.
- Shared production files have exactly one writing task. Readers declare dependencies where downstream tasks need implemented interfaces.
- Task 1 and Task 6 intentionally share the generic guidance concept but not files: Task 1 owns global setup defaults, Task 6 owns MAS prompt rendering.
- Task 3 explicitly owns the MAS SCIP divergence statement required by AC-003-5.
- Task 8 is separate because setup/init/prompt behavior is user-visible.
