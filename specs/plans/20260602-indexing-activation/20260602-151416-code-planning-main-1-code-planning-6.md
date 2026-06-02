# Code Planning: End-to-End Indexing Activation Coverage

Task: `code-planning-main-1-code-planning-6`
Agent: `code-planner-3`
Spec: `specs/goals/20260602-indexing-activation.md`
Timestamp: `20260602-151416`

## Based On

- Goal spec: `specs/goals/20260602-indexing-activation.md`, read in full.
- Parent plan: `specs/plans/20260602-indexing-activation/20260602-123514-code-planning-main-1.md`, especially Task 7, interface contracts, shared-file ownership, and dependency graph.
- Dependency plans:
  - `code-planning-main-1-code-planning-0`: setup default optional-index guidance.
  - `code-planning-main-1-code-planning-4`: pairing init orchestration.
  - `code-planning-main-1-code-planning-5`: MAS prompt gating and prompt metadata.
- Assigned task JSON for `code-planning-main-1-code-planning-6`.
- Stacklit repository summary for `internal/integration`, `internal/commands`, `internal/prompts`, `internal/agent`, `internal/ops`, `internal/scipsearch`, `internal/stacklit`, and `internal/semble`.
- Integration test structure reads: `internal/integration/e2e_workflow_test.go` helper and prompt sections, plus targeted test-name search across `internal/integration` and `internal/commands`.
- Software architecture review skill used as a planning lens for boundary ownership, dependency direction, test fixture seams, and shared-file risk.

## Planning Boundary

This plan owns integration-level tests only. Downstream coders may add narrowly scoped command test fixtures only when needed to drive existing public setup/init flows through the integration layer. This plan does not change production behavior, global guidance text, pairing hook implementation, SCIP root selection, Semble safety rules, SessionStart hook logic, MAS prompt rendering, runtime index generation, or documentation.

Sibling boundary references used in the compliance matrix:

| Sibling | Responsibility |
|---|---|
| `code-planning-main-1-code-planning-0` | Setup default optional-index routing guidance. |
| `code-planning-main-1-code-planning-1` | Pairing hook infrastructure and Stacklit lifecycle refresh. |
| `code-planning-main-1-code-planning-2` | Pairing SCIP root planning and ambiguity diagnostics. |
| `code-planning-main-1-code-planning-3` | Pairing Semble project-root safety activation. |
| `code-planning-main-1-code-planning-4` | Pairing init orchestration for project-local index activation. |
| `code-planning-main-1-code-planning-5` | MAS prompt gating and context boundaries. |
| `code-planning-main-1-code-planning-7` | User-facing documentation and embedded support-doc sync. |

## Architecture Notes

The integration layer should verify observable flows, not duplicate package-level assertions owned by implementation tasks. Each CP7 task exercises behavior through public command, hook, SessionStart, or prompt construction surfaces and should assert the externally meaningful result: generated files, hook paths, git status cleanliness, diagnostics, prompt content, or omitted prompt sections.

CP7-1 intentionally owns shared integration helpers because every later CP7 task needs isolated project roots, env-gate setup, and prompt/session assertions. Later tasks depend on CP7-1 and should read those helpers instead of writing the same fixture code. This serializes a small test-fixture seam and avoids broad shared-file contention across parallel integration-test additions.

## Planned Coding Tasks

### CP7-1 - Add setup and disabled-tool integration coverage

desc: Add setup and disabled-tool integration coverage.

done_when: Integration tests prove a fresh `liza setup` installs generic optional-index guidance while disabled Stacklit, SCIP, and Semble env gates leave pairing init without optional indexing hooks, leave SessionStart without optional command blocks, and leave MAS prompts without optional tool sections even when stale artifacts or SCIP config are present.

scope: Owns a new integration test file such as `internal/integration/indexing_activation_disabled_test.go` and minimal reusable helpers in `internal/integration/indexing_activation_helpers_test.go` for isolated setup, pairing init, SessionStart, and prompt fixtures. May add narrowly scoped command test fixtures only if existing public command surfaces cannot be driven from integration tests. Does not modify production setup, init, SessionStart, prompt, or index generation behavior.

spec_ref: specs/goals/20260602-indexing-activation.md

validation:

- `go test ./internal/integration ./internal/commands`

task_depends_on:

- `code-planning-main-1-code-planning-0`
- `code-planning-main-1-code-planning-4`
- `code-planning-main-1-code-planning-5`

decomposition:

```yaml
owned_files:
  - internal/integration/indexing_activation_disabled_test.go
  - internal/integration/indexing_activation_helpers_test.go
owned_modules:
  - internal/integration
interfaces_owned:
  - CP7 integration test helpers
interfaces_consumed:
  - Global Tool Guidance Contract
  - Init Orchestration Contract
  - Prompt Context Contract
depends_on: []
task_depends_on:
  - code-planning-main-1-code-planning-0
  - code-planning-main-1-code-planning-4
  - code-planning-main-1-code-planning-5
coverage_notes: "Covers fresh setup guidance, disabled optional-tool fallback, disabled pairing init, disabled SessionStart optional blocks, and disabled MAS prompt sections with stale artifacts/config."
```

### CP7-2 - Add Stacklit pairing activation integration coverage

desc: Add Stacklit pairing activation integration coverage.

done_when: Integration tests prove Stacklit-only pairing init installs effective lifecycle hook plumbing without editing `AGENT_TOOLS.md`, automatic lifecycle refresh creates or updates repo-root `stacklit.json` without running AI-summary behavior, and the generated `liza-index.sh ai` path is exercised when the script exposes manual AI-summary refresh.

scope: Owns a new integration test file such as `internal/integration/indexing_activation_stacklit_test.go`. Reads CP7-1 integration helpers. May add command fixture seams only for fake Stacklit command capture needed to prove no automatic AI-summary invocation. Does not modify pairing hook implementation, Stacklit command planning, global guidance, SessionStart hook logic, or production init behavior.

spec_ref: specs/goals/20260602-indexing-activation.md

validation:

- `go test ./internal/integration ./internal/commands`

depends_on:

- CP7-1

task_depends_on:

- `code-planning-main-1-code-planning-0`
- `code-planning-main-1-code-planning-4`
- `code-planning-main-1-code-planning-5`

decomposition:

```yaml
owned_files:
  - internal/integration/indexing_activation_stacklit_test.go
owned_modules:
  - internal/integration
interfaces_owned: []
interfaces_consumed:
  - CP7 integration test helpers
  - Pairing Hook Contract
  - Init Orchestration Contract
depends_on:
  - 0
task_depends_on:
  - code-planning-main-1-code-planning-0
  - code-planning-main-1-code-planning-4
  - code-planning-main-1-code-planning-5
coverage_notes: "Covers Stacklit-only pairing init, lifecycle refresh, no AGENT_TOOLS mutation, automatic no-AI refresh, and manual AI refresh path."
```

### CP7-3 - Add hook-path and generated-artifact cleanliness integration coverage

desc: Add hook-path and generated-artifact cleanliness integration coverage.

done_when: Integration tests prove pairing init with generated index artifacts keeps those artifacts out of `git status --short` unless they are intentionally tracked, and repositories with non-default `core.hooksPath` either receive Liza indexing hooks in the effective hook path or fail with a clear diagnostic instead of installing inert `.git/hooks` files.

scope: Owns a new integration test file such as `internal/integration/indexing_activation_hooks_test.go`. Reads CP7-1 integration helpers. May add command fixture seams only for asserting public init diagnostics and git status output. Does not modify hook installation production code, generated script content, worktree exclusion logic, or global guidance.

spec_ref: specs/goals/20260602-indexing-activation.md

validation:

- `go test ./internal/integration ./internal/commands`

depends_on:

- CP7-1

task_depends_on:

- `code-planning-main-1-code-planning-0`
- `code-planning-main-1-code-planning-4`
- `code-planning-main-1-code-planning-5`

decomposition:

```yaml
owned_files:
  - internal/integration/indexing_activation_hooks_test.go
owned_modules:
  - internal/integration
interfaces_owned: []
interfaces_consumed:
  - CP7 integration test helpers
  - Pairing Hook Contract
  - Init Orchestration Contract
depends_on:
  - 0
task_depends_on:
  - code-planning-main-1-code-planning-0
  - code-planning-main-1-code-planning-4
  - code-planning-main-1-code-planning-5
coverage_notes: "Covers generated artifact cleanliness and non-default core.hooksPath behavior through public init and git status observations."
```

### CP7-4 - Add SCIP pairing planning integration coverage

desc: Add SCIP pairing planning integration coverage.

done_when: Integration tests prove `LIZA_ENABLE_SCIP_SEARCH=1` pairing init in a single-root Go, TypeScript, or Python repository writes concrete repo-specific SCIP indexer commands with required root/cwd arguments, repeated `--scip-search <language>` filters exclude other detected languages while still requiring a confident root for the requested language, and ambiguous monorepo roots fail with candidate-root diagnostics instead of writing guessed hook commands.

scope: Owns a new integration test file such as `internal/integration/indexing_activation_scip_test.go`. Reads CP7-1 integration helpers. May add command fixture seams only for fake SCIP indexer command capture and diagnostic assertions. Does not modify SCIP root planning, pairing init wiring, generated hook implementation, MAS SCIP runtime planning, or prompt rendering.

spec_ref: specs/goals/20260602-indexing-activation.md

validation:

- `go test ./internal/integration ./internal/commands`

depends_on:

- CP7-1

task_depends_on:

- `code-planning-main-1-code-planning-0`
- `code-planning-main-1-code-planning-4`
- `code-planning-main-1-code-planning-5`

decomposition:

```yaml
owned_files:
  - internal/integration/indexing_activation_scip_test.go
owned_modules:
  - internal/integration
interfaces_owned: []
interfaces_consumed:
  - CP7 integration test helpers
  - Pairing SCIP Planning Contract
  - Init Orchestration Contract
depends_on:
  - 0
task_depends_on:
  - code-planning-main-1-code-planning-0
  - code-planning-main-1-code-planning-4
  - code-planning-main-1-code-planning-5
coverage_notes: "Covers SCIP single-root command generation, language filters, and monorepo ambiguity diagnostics through pairing init."
```

### CP7-5 - Add Semble safety and SessionStart integration coverage

desc: Add Semble safety and SessionStart integration coverage.

done_when: Integration tests prove Semble-enabled pairing init creates or verifies a safe physical project-root `.sembleignore` before Semble is advertised, unsafe or missing safety artifacts produce the implemented clear diagnostic when safety cannot be established, and pairing SessionStart emits concrete repo-root optional-tool guidance only when the corresponding Stacklit, SCIP, or Semble artifacts and readiness checks exist.

scope: Owns a new integration test file such as `internal/integration/indexing_activation_sessionstart_test.go`. Reads CP7-1 integration helpers. May add command fixture seams only for running the embedded SessionStart hook or equivalent public hook entrypoint in an isolated project. Does not modify Semble safety implementation, SessionStart hook logic, pairing init production behavior, MAS prompt rendering, or global guidance.

spec_ref: specs/goals/20260602-indexing-activation.md

validation:

- `go test ./internal/integration ./internal/commands`

depends_on:

- CP7-1

task_depends_on:

- `code-planning-main-1-code-planning-0`
- `code-planning-main-1-code-planning-4`
- `code-planning-main-1-code-planning-5`

decomposition:

```yaml
owned_files:
  - internal/integration/indexing_activation_sessionstart_test.go
owned_modules:
  - internal/integration
interfaces_owned: []
interfaces_consumed:
  - CP7 integration test helpers
  - Pairing Semble Safety Contract
  - Init Orchestration Contract
  - Prompt Context Contract
depends_on:
  - 0
task_depends_on:
  - code-planning-main-1-code-planning-0
  - code-planning-main-1-code-planning-4
  - code-planning-main-1-code-planning-5
coverage_notes: "Covers Semble physical safety artifacts, Semble diagnostics, and SessionStart concrete repo-root optional-tool advertisement boundaries."
```

### CP7-6 - Add MAS optional-index prompt integration coverage

desc: Add MAS optional-index prompt integration coverage.

done_when: Integration tests prove MAS task, reviewer, and orchestrator prompts include Stacklit, SCIP, and Semble sections only from env-gated metadata for the current target root, use task or reviewer worktree paths instead of pairing repo-root paths, omit disabled sections even when stale artifacts exist, and omit only the failed optional-tool section when another enabled tool remains ready.

scope: Owns a new integration test file such as `internal/integration/indexing_activation_mas_prompt_test.go`. Reads CP7-1 integration helpers and existing integration prompt builders. May add command fixture seams only for prompt-building fixtures that exercise public agent strategy behavior. Does not modify MAS prompt templates, prompt metadata builders, runtime index generation, pairing SessionStart hooks, pairing init activation, or global guidance.

spec_ref: specs/goals/20260602-indexing-activation.md

validation:

- `go test ./internal/integration ./internal/commands`

depends_on:

- CP7-1

task_depends_on:

- `code-planning-main-1-code-planning-0`
- `code-planning-main-1-code-planning-4`
- `code-planning-main-1-code-planning-5`

decomposition:

```yaml
owned_files:
  - internal/integration/indexing_activation_mas_prompt_test.go
owned_modules:
  - internal/integration
interfaces_owned: []
interfaces_consumed:
  - CP7 integration test helpers
  - Prompt Context Contract
depends_on:
  - 0
task_depends_on:
  - code-planning-main-1-code-planning-0
  - code-planning-main-1-code-planning-4
  - code-planning-main-1-code-planning-5
coverage_notes: "Covers MAS prompt env gates, target-specific paths, stale artifact omission, and failure isolation through integration-level prompt construction."
```

## Dependency Graph

```text
Existing code-planning-main-1-code-planning-0
Existing code-planning-main-1-code-planning-4
Existing code-planning-main-1-code-planning-5
  -> CP7-1
CP7-1
  -> CP7-2
  -> CP7-3
  -> CP7-4
  -> CP7-5
  -> CP7-6
```

CP7-1 is the only within-plan prerequisite because it owns shared helpers. CP7-2 through CP7-6 can run in parallel after CP7-1 because each writes a separate integration test file and consumes implemented behavior through public surfaces.

## Shared-File Audit

| File or area | Writing task | Other planned task access | Dependency |
|---|---|---|---|
| `internal/integration/indexing_activation_helpers_test.go` | CP7-1 | CP7-2, CP7-3, CP7-4, CP7-5, CP7-6 read helper APIs | CP7-2 through CP7-6 depend on CP7-1 |
| `internal/integration/indexing_activation_disabled_test.go` | CP7-1 | none | none |
| `internal/integration/indexing_activation_stacklit_test.go` | CP7-2 | none | none |
| `internal/integration/indexing_activation_hooks_test.go` | CP7-3 | none | none |
| `internal/integration/indexing_activation_scip_test.go` | CP7-4 | none | none |
| `internal/integration/indexing_activation_sessionstart_test.go` | CP7-5 | none | none |
| `internal/integration/indexing_activation_mas_prompt_test.go` | CP7-6 | none | none |

No downstream CP7 task writes production files. Any command fixture seam must be narrowly scoped and either owned by the task that needs it or added to CP7-1 helpers if multiple CP7 tasks need it; if a later coder discovers a shared fixture need, that coder must preserve this dependency direction rather than writing the same helper in parallel.

## Validation Plan

Canonical validation for every child task:

- `go test ./internal/integration ./internal/commands`

The command is intentionally broader than each individual file because the task's contract is integration behavior plus any narrowly scoped command fixture seams. If a stored validation command shape ever violates shell constraints, the downstream coder must translate it to an equivalent single-purpose command from the worktree and record both original and translated commands in validation evidence.

## Spec Compliance Matrix

| # | Requirement | Source | Task(s) | Status |
|---|-------------|--------|---------|--------|
| 1 | Generic routing guidance must be safe when Stacklit, SCIP, or Semble are disabled, missing, or not advertised. | General NFR-000-1, lines 45-46 | CP7-1, CP7-6; siblings `code-planning-main-1-code-planning-0`, `code-planning-main-1-code-planning-5` | Covered |
| 2 | Project-specific paths, generated index locations, and readiness state must not be written into global `AGENT_TOOLS.md`. | General NFR-000-2, lines 47-48 | CP7-1, CP7-2; sibling `code-planning-main-1-code-planning-0` | Covered |
| 3 | Optional indexing failures must omit unavailable prompt/context sections instead of making agents troubleshoot unrelated tooling. | General NFR-000-3, lines 49-51 | CP7-1, CP7-6; sibling `code-planning-main-1-code-planning-5` | Covered |
| 4 | `liza setup` installs generic Stacklit, SCIP, and Semble routing guidance into default `AGENT_TOOLS.md`. | FR-001-1, lines 96-97 | CP7-1; sibling `code-planning-main-1-code-planning-0` | Covered |
| 5 | Guidance is conditional on explicit Stacklit path, explicit SCIP path, or supplied Semble target/readiness. | FR-001-2, lines 98-102 | CP7-1, CP7-6; siblings `code-planning-main-1-code-planning-0`, `code-planning-main-1-code-planning-5` | Covered |
| 6 | Guidance describes fallbacks for disabled or unavailable tools. | FR-001-3, lines 103-106 | CP7-1; sibling `code-planning-main-1-code-planning-0` | Covered |
| 7 | Guidance contains no project-specific absolute paths, generated repo locations, or installed-tool claims. | FR-001-4, lines 107-109 | CP7-1; sibling `code-planning-main-1-code-planning-0` | Covered |
| 8 | Installed default guidance remains concise. | NFR-001-1, lines 113-115 | Sibling `code-planning-main-1-code-planning-0` | Covered |
| 9 | Fresh setup installs generic Stacklit/SCIP/Semble routing guidance. | AC-001-1, lines 119-121 | CP7-1; sibling `code-planning-main-1-code-planning-0` | Covered |
| 10 | With optional tools disabled, guidance still routes to valid fallback tools. | AC-001-2, lines 122-124 | CP7-1; sibling `code-planning-main-1-code-planning-0` | Covered |
| 11 | User-customized `AGENT_TOOLS.md` keeps setup customization protection. | AC-001-3, lines 125-126 | Sibling `code-planning-main-1-code-planning-0` | Covered |
| 12 | Pairing init flows install project-local provider hooks as today. | FR-002-1, lines 156-157 | CP7-1; sibling `code-planning-main-1-code-planning-4` | Covered |
| 13 | Stacklit truthy pairing init installs or verifies Git hook plumbing for repo-root `stacklit.json` refresh. | FR-002-2, lines 158-160 | CP7-2; siblings `code-planning-main-1-code-planning-1`, `code-planning-main-1-code-planning-4` | Covered |
| 14 | Automatic Git lifecycle refresh must not run `stacklit ai-summary`; manual `ai` may. | FR-002-2a, lines 161-163 | CP7-2; sibling `code-planning-main-1-code-planning-1` | Covered |
| 15 | SCIP truthy pairing init autodetects repo-specific SCIP plan and hook plumbing for enabled languages. | FR-002-3, lines 164-167 | CP7-4; siblings `code-planning-main-1-code-planning-2`, `code-planning-main-1-code-planning-4` | Covered |
| 16 | Pairing SCIP autodetection plans concrete Go, TypeScript, and Python indexer commands with root/cwd arguments. | FR-002-3a, lines 168-170 | CP7-4; sibling `code-planning-main-1-code-planning-2` | Covered |
| 17 | Repeated `--scip-search` flags restrict languages but are not sufficient root/cwd selection. | FR-002-3b, lines 171-173 | CP7-4; siblings `code-planning-main-1-code-planning-2`, `code-planning-main-1-code-planning-4` | Covered |
| 18 | Ambiguous monorepo SCIP roots fail with candidate-root diagnostic instead of guessed hook command. | FR-002-3c, lines 174-177 | CP7-4; siblings `code-planning-main-1-code-planning-2`, `code-planning-main-1-code-planning-4` | Covered |
| 19 | Confident single-root SCIP detection generates concrete repo-specific SCIP commands. | FR-002-3d, lines 178-180 | CP7-4; siblings `code-planning-main-1-code-planning-2`, `code-planning-main-1-code-planning-4` | Covered |
| 20 | Pairing init keeps generated indexes ignored, privately excluded, or out of accidental task diffs unless intentionally tracked. | FR-002-4, lines 181-183 | CP7-3; siblings `code-planning-main-1-code-planning-1`, `code-planning-main-1-code-planning-4` | Covered |
| 21 | Semble truthy pairing init ensures safe physical `.sembleignore` before SessionStart advertises Semble. | FR-002-5, lines 184-186 | CP7-5; siblings `code-planning-main-1-code-planning-3`, `code-planning-main-1-code-planning-4` | Covered |
| 22 | Pairing init must not modify `~/.liza/AGENT_TOOLS.md`. | FR-002-6, lines 187-188 | CP7-1, CP7-2; sibling `code-planning-main-1-code-planning-4` | Covered |
| 23 | Pairing init preserves existing project Git hooks and does not silently overwrite lifecycle hooks. | FR-002-7, lines 189-191 | CP7-2, CP7-3; siblings `code-planning-main-1-code-planning-1`, `code-planning-main-1-code-planning-4` | Covered |
| 24 | Pairing init detects non-default `core.hooksPath` and installs safely or reports diagnostic. | FR-002-8, lines 192-195 | CP7-3; siblings `code-planning-main-1-code-planning-1`, `code-planning-main-1-code-planning-4` | Covered |
| 25 | Docs distinguish pairing-init env gates from MAS runtime activation. | FR-002-9, lines 196-199 | Sibling `code-planning-main-1-code-planning-7` | Covered |
| 26 | Pairing index activation preserves SessionStart concrete path/readiness rule. | NFR-002-1, lines 203-205 | CP7-1, CP7-5; siblings `code-planning-main-1-code-planning-4`, `code-planning-main-1-code-planning-5` | Covered |
| 27 | Pairing activation does not assume Liza's own build system or target project stack. | NFR-002-2, lines 206-207 | CP7-2, CP7-4; siblings `code-planning-main-1-code-planning-1`, `code-planning-main-1-code-planning-2`, `code-planning-main-1-code-planning-4` | Covered |
| 28 | Disabled pairing env vars activate no optional indexing hook behavior. | AC-002-1, lines 211-212 | CP7-1; sibling `code-planning-main-1-code-planning-4` | Covered |
| 29 | Stacklit-enabled pairing lifecycle refresh works without editing `AGENT_TOOLS.md`. | AC-002-2, lines 213-215 | CP7-2; siblings `code-planning-main-1-code-planning-1`, `code-planning-main-1-code-planning-4` | Covered |
| 30 | Semble-enabled pairing init creates or reports missing safety artifact before advertisement. | AC-002-3, lines 216-218 | CP7-5; siblings `code-planning-main-1-code-planning-3`, `code-planning-main-1-code-planning-4` | Covered |
| 31 | Generated indexes do not appear as accidental untracked files after init and refresh. | AC-002-4, lines 219-221 | CP7-3; siblings `code-planning-main-1-code-planning-1`, `code-planning-main-1-code-planning-4` | Covered |
| 32 | Single-root Go, TypeScript, or Python pairing SCIP init generates concrete commands. | AC-002-5, lines 222-225 | CP7-4; siblings `code-planning-main-1-code-planning-2`, `code-planning-main-1-code-planning-4` | Covered |
| 33 | Monorepo ambiguous SCIP roots fail clearly instead of writing guessed hook. | AC-002-6, lines 226-228 | CP7-4; siblings `code-planning-main-1-code-planning-2`, `code-planning-main-1-code-planning-4` | Covered |
| 34 | `--scip-search go` restricts detection to Go while still requiring confident Go root selection. | AC-002-7, lines 229-231 | CP7-4; siblings `code-planning-main-1-code-planning-2`, `code-planning-main-1-code-planning-4` | Covered |
| 35 | Existing project Git hooks are preserved, chained, or explicitly diagnosed. | AC-002-8, lines 232-234 | CP7-2, CP7-3; siblings `code-planning-main-1-code-planning-1`, `code-planning-main-1-code-planning-4` | Covered |
| 36 | Non-default `core.hooksPath` installs into effective path or reports unsafe diagnostic. | AC-002-9, lines 235-237 | CP7-3; siblings `code-planning-main-1-code-planning-1`, `code-planning-main-1-code-planning-4` | Covered |
| 37 | Automatic Stacklit lifecycle refresh skips AI-summary. | AC-002-10, lines 238-240 | CP7-2; sibling `code-planning-main-1-code-planning-1` | Covered |
| 38 | Manual `liza-index.sh ai` includes Stacklit AI-summary behavior. | AC-002-11, lines 241-242 | CP7-2; sibling `code-planning-main-1-code-planning-1` | Covered |
| 39 | Docs explain pairing-init env gates separately from MAS runtime activation. | AC-002-12, lines 243-245 | Sibling `code-planning-main-1-code-planning-7` | Covered |
| 40 | MAS init preserves Stacklit/Semble process-local gates and SCIP env gate plus durable config. | FR-003-1, lines 282-284 | CP7-6; sibling `code-planning-main-1-code-planning-5` | Covered |
| 41 | MAS prompts include optional tool sections only when corresponding metadata is present. | FR-003-2, lines 285-286 | CP7-6; sibling `code-planning-main-1-code-planning-5` | Covered |
| 42 | MAS prompt routing stays role/target specific. | FR-003-3, lines 287-289 | CP7-6; sibling `code-planning-main-1-code-planning-5` | Covered |
| 43 | MAS init behavior does not depend on pairing Git hooks or repo-root pairing indexes. | FR-003-4, lines 290-291 | CP7-6; siblings `code-planning-main-1-code-planning-4`, `code-planning-main-1-code-planning-5` | Covered |
| 44 | Preserve or explicitly supersede current MAS SCIP runtime planning semantics. | FR-003-5, lines 292-296 | CP7-6; siblings `code-planning-main-1-code-planning-2`, `code-planning-main-1-code-planning-5` | Covered |
| 45 | Plan documents divergence or aligns MAS if pairing SCIP ambiguity behavior is stricter. | FR-003-6, lines 297-300 | Sibling `code-planning-main-1-code-planning-2` | Covered |
| 46 | Disabled SCIP env gate omits MAS SCIP prompt even with config. | AC-003-1, lines 304-305 | CP7-1, CP7-6; sibling `code-planning-main-1-code-planning-5` | Covered |
| 47 | Disabled Stacklit env gate omits MAS Stacklit prompt unless documented fallback exists. | AC-003-2, lines 306-308 | CP7-1, CP7-6; sibling `code-planning-main-1-code-planning-5` | Covered |
| 48 | Disabled Semble env gate omits MAS Semble prompt. | AC-003-3, lines 309-310 | CP7-1, CP7-6; sibling `code-planning-main-1-code-planning-5` | Covered |
| 49 | Enabled tool readiness/index failure omits failed MAS section while other guidance remains usable. | AC-003-4, lines 311-313 | CP7-6; sibling `code-planning-main-1-code-planning-5` | Covered |
| 50 | Pairing SCIP autodetection task states MAS inference choice. | AC-003-5, lines 314-317 | Sibling `code-planning-main-1-code-planning-2` | Covered |
| 51 | Pairing SessionStart continues emitting concrete repo-root optional tool instructions only when artifacts/readiness exist. | FR-004-1, lines 345-347 | CP7-1, CP7-5; siblings `code-planning-main-1-code-planning-4`, `code-planning-main-1-code-planning-5` | Covered |
| 52 | MAS prompts continue using structured task/reviewer/orchestrator metadata rather than pairing SessionStart assumptions. | FR-004-2, lines 348-350 | CP7-6; sibling `code-planning-main-1-code-planning-5` | Covered |
| 53 | Prompt and SessionStart guidance avoid duplicated long-form routing text when shorter shared wording is sufficient. | FR-004-3, lines 351-352 | CP7-1, CP7-6; siblings `code-planning-main-1-code-planning-0`, `code-planning-main-1-code-planning-5` | Covered |
| 54 | Generic routing text states indexes/search results are navigation aids and direct source reads remain evidence. | FR-004-4, lines 353-355 | CP7-1, CP7-6; siblings `code-planning-main-1-code-planning-0`, `code-planning-main-1-code-planning-5` | Covered |
| 55 | Pairing with no indexes and Semble disabled emits initialization guidance without optional tool blocks. | AC-004-1, lines 359-361 | CP7-1, CP7-5; sibling `code-planning-main-1-code-planning-4` | Covered |
| 56 | MAS mode with generated worktree indexes uses worktree-specific paths. | AC-004-2, lines 362-364 | CP7-6; sibling `code-planning-main-1-code-planning-5` | Covered |
| 57 | Full routing guidance distinguishes conceptual discovery, module orientation, symbol navigation, exact search, syntax search, and direct reads. | AC-004-3, lines 365-368 | CP7-1, CP7-6; siblings `code-planning-main-1-code-planning-0`, `code-planning-main-1-code-planning-5` | Covered |
| 58 | Unavailable optional tools are not instructed before fallback search/read tools. | AC-004-4, lines 369-371 | CP7-1, CP7-6; siblings `code-planning-main-1-code-planning-0`, `code-planning-main-1-code-planning-5` | Covered |
| E2E | e2e test coverage for new behavior | Cross-cutting | CP7-1, CP7-2, CP7-3, CP7-4, CP7-5, CP7-6 | Covered |
| DOC | Documentation updates for changed behavior | Cross-cutting | N/A: sibling `code-planning-main-1-code-planning-7` owns user-facing documentation and embedded support-doc sync | N/A |

## Pre-Submit Self-Check Notes

- Atomicity: CP7-1 owns the shared helper seam plus disabled/setup assertions; CP7-2 through CP7-6 each own one observable integration scenario family.
- Falsifiability: every CP7 done_when names concrete observable outcomes: installed guidance, absent hooks/sections, generated hook/script behavior, git status output, hook-path diagnostics, generated SCIP commands, ambiguity diagnostics, `.sembleignore`, SessionStart output, or prompt content.
- Shared files: only CP7-1 writes shared helper code; every other task depends on CP7-1 and writes a separate test file.
- Dependency consistency: all CP7 outputs depend on the already merged planning boundaries for setup guidance, pairing init orchestration, and MAS prompt metadata.
- Scope protection: implementation files named in CP7 tasks are downstream child-task scope, not modified by this planning task.
