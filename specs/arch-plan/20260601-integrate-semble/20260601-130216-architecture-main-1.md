# Architecture Plan: Semble Optional Semantic Search

Status: review

## Goal

Integrate Semble as a strict opt-in semantic discovery tool by adding one reusable Semble capability layer, wiring it into init, worktree safety, MAS prompt rendering, and operator docs without replacing Stacklit, SCIP, `rg`, `ast-grep`, or source reads.

## Context

The goal spec adds Semble to Liza's repository-navigation stack. Existing optional tools already establish the main shape:

- `internal/scipsearch/` owns strict opt-in runtime indexing with generated task-worktree indexes under `.liza/scip/`, bounded diagnostics, prompt-safe index metadata, and a private worktree exclude pattern.
- `internal/stacklit/` owns strict opt-in `stacklit.json` refresh, worktree-clean generated artifacts, and prompt-safe index metadata.
- `internal/prompts` renders shared bootstrap guidance from `BasePromptConfig`; `internal/agent/prompt.go` gathers task or project-root tool metadata and passes it to prompt rendering.
- `internal/ops/wt_create.go` is the lifecycle point that prepares task worktrees before agents receive prompts.
- `cmd/liza/cmd_init.go` and `internal/commands/init.go` are the CLI-facing workspace init path; `internal/ops/init_project.go` is a non-interactive project init path that must preserve the same architectural contract where applicable.
- Pairing SessionStart Semble guidance already exists in `internal/embedded/hooks/session-context.sh`; this plan does not reimplement it.

Semble differs from Stacklit and SCIP because normal search indexes into an OS cache keyed by the target root and model environment, while task safety depends on a physical `.sembleignore` file in the walked tree. The design therefore separates three concerns: Semble availability/offline readiness, worktree ignore preparation, and prompt rendering.

### References

- Goal spec: `specs/goals/20260601-integrate-semble.md`
- Task: `architecture-main-1`
- Codebase:
  - `internal/scipsearch/doc.go`
  - `internal/scipsearch/scipsearch.go`
  - `internal/scipsearch/scipsearch_test.go`
  - `internal/stacklit/doc.go`
  - `internal/stacklit/stacklit.go`
  - `internal/stacklit/stacklit_test.go`
  - `internal/prompts/builder.go`
  - `internal/prompts/templates/base_prompt.tmpl`
  - `internal/prompts/builder_test.go`
  - `internal/agent/prompt.go`
  - `internal/agent/strategy_orchestrator.go`
  - `internal/agent/strategy_doer.go`
  - `internal/agent/strategy_reviewer.go`
  - `internal/agent/worktree_check.go`
  - `internal/ops/wt_create.go`
  - `internal/ops/scip_indexing.go`
  - `internal/ops/stacklit_indexing.go`
  - `internal/commands/init.go`
  - `internal/ops/init_project.go`
  - `cmd/liza/cmd_init.go`
  - `internal/embedded/hooks/session-context.sh`
  - `internal/embedded/session_context_hook_test.go`
  - `support-docs/CONFIGURATION.md`
  - `support-docs/CUSTOMIZING_AGENT_TOOLS.md`
  - `support-docs/USAGE_MULTI_AGENTS.md`
  - `README.md`
- ADRs:
  - `specs/architecture/ADR/0068-optional-repository-indexing-with-scip-and-stacklit.md`
  - `specs/architecture/ADR/0074-sessionstart-context-hooks.md`
- Guardrails and invariants:
  - `GUARDRAILS.md`
  - `INVARIANTS.md#cross-reference-protection-matrix`
  - `lessons/agents/worktree-file-path-consistency.md`
  - `lessons/agents/worktree-path-construction.md`
- Skills:
  - `architecture-planning`
  - `software-architecture-review`
  - `systemic-thinking`

### Constraints

- Semble is optional, external, and strict opt-in through `LIZA_ENABLE_SEMBLE`.
- No durable `state.yaml` Semble config field is added in the first milestone.
- Env truth semantics match existing optional-tool gates: trimmed, case-insensitive `1` and `true` enable; unset, empty, `0`, and `false` disable.
- MAS must not silently download models during unattended work. Init may prewarm intentionally; MAS prompt readiness must validate offline with `HF_HUB_OFFLINE=1`.
- Semble prompt guidance is injected only when Semble is enabled, installed, offline-ready, and target-root safety checks pass.
- Semble results are candidates only; prompt guidance must require direct source reads before edits or claims.
- MAS task and reviewer agents search their assigned worktree root, not the parent project root.
- Orchestrator project-root Semble guidance is allowed only after root safety excludes `.worktrees/`, `.liza/`, generated indexes, and credential-file patterns.
- Liza-generated task-worktree `.sembleignore` must be visible to Semble and hidden from `git status` unless the operator explicitly tracks it.
- Semble and SCIP private-exclude handling must not compete over `core.excludesFile`.
- Generated failure diagnostics must be bounded and must not dump file contents or secrets.
- G1.1 applies: no runtime behavior may assume Liza's own Go project, Makefile, or internal paths in target projects.
- The protection matrix intersects race conditions, secret exposure, unreviewed code, scope creep, and silent failures; this plan routes changes through existing atomic state/ops flows, private worktree excludes, bounded diagnostics, and review gates.
- `.pre-commit-config.yaml` exists at integration `HEAD`; no `bootstrap-precommit` output entry is emitted.

### Assumptions

- None.

### Open Questions

- None for the first milestone.

---

## Components

### Semble Capability Package (`internal/semble/`)

**Responsibility:** Own Semble activation, command planning, prewarm/offline validation, validation caching, default ignore patterns, target-root safety checks, and prompt-safe Semble metadata.

**Boundaries:**
- Exposes: `EnvEnableSemble`, `RuntimeEnabled`, init prewarm/offline validation entry points, runtime readiness checks, prompt context metadata, default `.sembleignore` content, bounded diagnostics, and target-root safety helpers.
- Depends on: `os`, `os/exec`, `path/filepath`, temp directories, Semble CLI behavior from the goal spec, and worktree/project roots passed by callers.

**Key decisions:**
- Create `internal/semble` instead of adding Semble behavior to `internal/stacklit` or `internal/scipsearch`. Semble has different cache, offline, and `.sembleignore` semantics, but can mirror their naming and result shapes.
- Keep activation environment-only. No `models.Config` Semble field is added.
- Represent lifecycle targets with `TargetKindProjectRoot`, `TargetKindTaskWorktree`, and `TargetKindReviewerWorktree` only if reviewer-specific diagnostics need to distinguish it. Otherwise reviewer worktrees use task-worktree safety.
- Provide a fixture-based command planner for prewarm and offline validation. The fixture is a temp directory with one tiny supported code file and the query `__liza_semble_prewarm__`.
- Treat prewarm exit code 0 as success even when no search hit appears; treat offline validation exit code 0 as offline-ready.
- Run MAS readiness and prompt examples with `HF_HUB_OFFLINE=1`; prewarm intentionally inherits normal operator network access.
- Key the process-local validation cache by resolved `semble` executable path, `SEMBLE_MODEL_NAME`, `HF_HOME`, `XDG_CACHE_HOME`, timeout, and fixture content identity. Cache entries live only in process memory.
- Keep cache invalidation conservative: any key change forces a new bounded offline fixture validation.
- Distinguish diagnostic classes at package boundary: executable missing, model unavailable offline, and Semble execution failure.
- Store the default ignore block in one package-level source of truth so worktree generation, root safety validation, docs, and tests do not drift.

### Init Integration (`cmd/liza/`, `internal/commands/`, `internal/ops/`)

**Responsibility:** Run controlled Semble prewarm and immediate offline validation during `liza init --spec` or equivalent workspace initialization when `LIZA_ENABLE_SEMBLE` is truthy and the CLI is present.

**Boundaries:**
- Exposes: operator-visible init diagnostics and warnings.
- Depends on: `internal/semble` validation APIs and the existing init flows.

**Key decisions:**
- Extend `commands.InitCommandWithConfig` as the primary CLI init path. It already validates SCIP init config before creating `.liza/`; Semble prewarm should sit in the same pre-scaffold validation window to avoid partial workspace state caused by Semble setup failures.
- Mirror the same behavior in `ops.InitProject` where non-interactive setup uses the ops path, so programmatic init observes the same strict opt-in semantics.
- Do not add a Semble CLI flag in this milestone. The goal spec explicitly names environment activation only.
- Missing `semble` when enabled is non-fatal for init: warn that Semble guidance will be omitted. This matches "Semble failures do not block agent spawn" while still making operator misconfiguration visible.
- A prewarm or offline validation failure is non-fatal for workspace initialization; it emits bounded diagnostics and later prompt injection omits Semble.
- Init does not create project-root `.sembleignore`. Root `.sembleignore` is operator-owned because project-root Semble search can index sensitive source-adjacent files unless the project explicitly opts into safe ignore coverage.

### Worktree Semble Safety (`internal/ops/`, shared worktree exclude helper)

**Responsibility:** Ensure Liza-managed task/reviewer worktrees have Semble-visible ignore rules before any MAS Semble prompt guidance can target them, while keeping generated `.sembleignore` out of task diffs.

**Boundaries:**
- Exposes: one idempotent worktree preparation function that creates/updates `<worktree>/.sembleignore` with the default ignore block and hides it through the linked worktree private exclude.
- Depends on: `internal/semble` default ignore patterns, `git rev-parse --git-dir`, worktree-specific git config, and the existing SCIP private-exclude precedent.

**Key decisions:**
- Introduce a small shared helper for linked-worktree private excludes rather than extending `internal/scipsearch` to know about Semble. The helper owns:
  - resolving a worktree's private gitdir,
  - appending named ignore entries idempotently,
  - enabling `extensions.worktreeConfig`,
  - setting `core.excludesFile` to the resolved private exclude,
  - serializing updates with one package-level lock.
- Refactor SCIP task-worktree exclude setup to use the shared helper so Semble and SCIP share one private exclude file and one lock.
- Semble worktree preparation creates a physical `.sembleignore` at the worktree root, appends any missing default patterns, and adds `.sembleignore` to the private exclude file.
- If `.sembleignore` is already tracked in a task worktree, do not hide it or rewrite operator-owned content blindly. The code-planner should preserve tracked operator content and ensure required patterns are present through an explicit project-file change only if the task scope allows it; otherwise mark blocked for the operator. For generated untracked `.sembleignore`, private exclude keeps `git status --porcelain` clean.
- Call Semble worktree preparation from `ops.CreateWorktree` on both fresh and existing worktree paths before Semble prompt metadata is collected.
- Reviewer worktree recovery in `internal/agent/worktree_check.go` must also prepare the recovered reviewer worktree before prompt building.
- Submit-for-review refresh points that already refresh SCIP/Stacklit should not reindex Semble; Semble indexes in its OS cache on demand. They may re-run `.sembleignore` preparation as an idempotent safety check if the worktree existed before the feature.

### MAS Prompt Semble Context (`internal/prompts/`, `internal/agent/`)

**Responsibility:** Inject concise Semble search guidance into MAS prompts only for the exact target root the agent is allowed to search and only after runtime readiness and target safety are satisfied.

**Boundaries:**
- Exposes: prompt-safe `SembleSearchContext` data, base prompt rendering, and unified query routing that positions Semble before Stacklit/SCIP only for conceptual discovery.
- Depends on: `internal/semble` runtime readiness, target safety, shell quoting, task/reviewer/orchestrator target root resolution, and existing prompt metadata assembly.

**Key decisions:**
- Add Semble metadata to `prompts.BasePromptConfig`, not to individual role templates. Semble is cross-role navigation guidance like SCIP and Stacklit.
- Render no Semble section when metadata is absent. Prompt absence is the degradation mechanism.
- For task doers and architecture/code-planning roles, target root is `data.Worktree`.
- For reviewers, target root is the reviewer worktree representing the review candidate after `ensureReviewerWorktree` succeeds.
- For orchestrators, target root is `config.ProjectRoot` only when project-root safety validation confirms a root `.sembleignore` with all required patterns. Otherwise orchestrator prompts omit Semble.
- Prompt examples use explicit shell-quoted absolute target roots and `HF_HUB_OFFLINE=1`.
- Prompt wording states Semble is candidate discovery, direct reads are source of truth, `--content` supports `code`, `docs`, `config`, `all`, `rg` is for exact/literal searches, and Morph MCP semantic search is fallback only when Semble is unavailable and current tool policy exposes Morph.
- Query routing keeps Stacklit and SCIP distinct: Semble for broad conceptual discovery; Stacklit for module/dependency orientation; SCIP for exact symbol/reference/implementation tracing; `ast-grep` for syntax-shaped search.

### Operator Documentation (`README.md`, `support-docs/`)

**Responsibility:** Document Semble's optional status, first-run model behavior, offline-ready workflow, `.sembleignore` safety, and routing relationship to existing tools.

**Boundaries:**
- Exposes: public README positioning, configuration reference, worktree-safe tool guidance, and MAS usage pointer.
- Depends on: goal spec and the contracts established by the Semble capability, init, worktree safety, and prompt components.

**Key decisions:**
- `README.md` should list Semble as an optional recommended semantic discovery tool near existing navigation tools, not as a requirement.
- `support-docs/CONFIGURATION.md` is the canonical place for `LIZA_ENABLE_SEMBLE`, truth table, prewarm/offline validation, timeout, diagnostics, and non-goals.
- `support-docs/CUSTOMIZING_AGENT_TOOLS.md` should explain Semble's routing position relative to Stacklit, SCIP, Morph MCP, `rg`, `ast-grep`, and direct reads.
- `support-docs/USAGE_MULTI_AGENTS.md` should include a concise setup pointer and link to configuration details.
- Documentation must state that enabling Semble at init may contact Hugging Face unless `SEMBLE_MODEL_NAME` points to a local model or the model is already cached.
- Documentation must state that `semble init`, Semble MCP, remote Git URL indexing, vendoring, and automatic installation are out of scope.
- Documentation must warn operators that Semble indexes matching file chunks and that projects with sensitive source-adjacent files should add project-specific ignore rules before enabling project-root Semble guidance.

---

## Interfaces

### Init Flow -> Semble Capability

**Contract:** Init calls Semble prewarm/offline validation only when `LIZA_ENABLE_SEMBLE` is truthy. The Semble package returns success, warnings, and bounded diagnostics without mutating Liza state.

**Direction:** `cmd/liza` and `internal/commands`/`internal/ops` call `internal/semble`; Semble does not call init code.

**Invariants:** Disabled Semble performs no CLI lookup, no prewarm, no offline validation, and no diagnostics. Enabled missing/unready Semble is visible but non-fatal.

### Worktree Lifecycle -> Semble Worktree Safety

**Contract:** Worktree creation/recovery asks the worktree safety component to ensure `.sembleignore` exists and required generated-safety patterns are present before prompt rendering can advertise Semble for that worktree.

**Direction:** `internal/ops/wt_create.go` and reviewer worktree recovery call the safety function; prompt builders only consume resulting readiness metadata.

**Invariants:** Generated `.sembleignore` remains visible to Semble, hidden from task diffs through the shared private exclude, and idempotent across repeated lifecycle calls.

### Semble Worktree Safety -> Shared Private Exclude Helper

**Contract:** Callers supply the target worktree root and private exclude entries; the helper serializes updates and ensures linked worktrees consult the same private exclude file through worktree-specific `core.excludesFile`.

**Direction:** Semble and SCIP call the helper; the helper has no knowledge of Semble or SCIP command semantics.

**Invariants:** Only one `core.excludesFile` target is configured for the linked worktree, entries are appended idempotently, and concurrent lifecycle hooks cannot race on the private exclude file.

### Agent Prompt Assembly -> Semble Capability

**Contract:** Prompt assembly asks for prompt-safe Semble context for one explicit target root. The Semble package returns context only when enabled, offline-ready, and target-safe; otherwise it returns empty context plus optional bounded diagnostics for logs.

**Direction:** `internal/agent/prompt.go` and orchestrator prompt assembly call Semble; prompt templates render the supplied metadata.

**Invariants:** MAS prompt text never tells an agent to search a sibling or parent worktree by inference. Empty Semble context omits the section entirely.

### Prompt Template -> Agent

**Contract:** The prompt provides exact `HF_HUB_OFFLINE=1 semble ... <target-root>` commands, content-mode guidance, and routing boundaries.

**Direction:** Liza renders guidance; agents execute only if they need conceptual discovery.

**Invariants:** Prompt text describes Semble as candidate discovery and requires direct reads for evidence.

### Documentation -> Operator

**Contract:** Operator docs explain enablement, model prewarm, offline validation, ignore rules, routing, and non-goals without implying automatic installation or required use.

**Direction:** Operators configure their environment and projects; Liza runtime consumes only environment variables and files already present.

**Invariants:** Docs remain stack-agnostic and do not hardcode Liza's own build/test commands as runtime defaults.

---

## Data Flow

Init-time Semble readiness:

```text
Operator sets LIZA_ENABLE_SEMBLE=1
  -> liza init --spec
  -> init flow calls internal/semble prewarm fixture
  -> Semble may download/cache model under operator-controlled environment
  -> init flow calls internal/semble offline fixture with HF_HUB_OFFLINE=1
  -> success warms current process validation cache and diagnostics
  -> workspace state is created without durable Semble config
```

Task worktree prompt flow:

```text
Task claim creates or reuses .worktrees/<task-id>
  -> ops.CreateWorktree prepares .sembleignore and private exclude
  -> optional SCIP/Stacklit refreshes run as today
  -> agent prompt assembly resolves exact worktree root
  -> internal/semble validates env, executable, offline readiness cache, and target safety
  -> prompts.BasePromptConfig receives Semble context
  -> base_prompt.tmpl renders Semble section or omits it
  -> agent uses Semble for conceptual discovery and direct reads for proof
```

Reviewer prompt flow:

```text
Reviewer claim verifies review_commit and ensures reviewer worktree
  -> reviewer worktree recovery/preparation ensures .sembleignore safety
  -> prompt assembly targets the reviewer worktree root
  -> Semble readiness and target safety gate prompt context
```

Orchestrator prompt flow:

```text
Orchestrator PreExecution refreshes project-root optional indexes
  -> Semble does not index proactively
  -> prompt assembly validates root .sembleignore safety and offline readiness
  -> project-root Semble guidance is injected only when root safety is proven
```

---

## Cross-Cutting Concerns

| Concern | Approach |
|---------|----------|
| Error handling | Semble failures return bounded diagnostics and omit prompt guidance; init and agent spawn stay non-blocking except for ordinary worktree safety failures that would dirty task state or expose unsafe roots. |
| Observability | Init prints/warns bounded diagnostics; supervisor lifecycle can log bounded readiness/safety failures while prompts omit unusable Semble sections. |
| Configuration | `LIZA_ENABLE_SEMBLE` is process-local activation only. `SEMBLE_MODEL_NAME`, `HF_HOME`, and `XDG_CACHE_HOME` are preserved as external Semble/model cache inputs and included in validation cache keys. |
| Security | Default ignore rules exclude Liza runtime files, generated indexes, and credential-file patterns. Diagnostics are bounded and do not include file contents. Prompt examples avoid remote Git URLs and use explicit local target roots. |
| Worktree isolation | Task and reviewer agents receive only their task/reviewer worktree root. Generated task `.sembleignore` is Semble-visible and Git-hidden via shared private excludes. |
| Concurrency | Shared private exclude helper serializes `info/exclude` and `core.excludesFile` updates for SCIP and Semble. Semble OS cache keys remain absolute-path based, so separate worktrees use separate local cache entries. |
| Prompt budget | Prompt guidance is concise and absent when not usable. Semble sits in the unified query-routing section only when enabled and ready. |
| Testing | Each domain has focused package tests: env parsing and fixture validation in `internal/semble`; init wiring through command/ops tests; clean worktree ignore behavior through ops/scipsearch tests; prompt rendering through prompts/agent tests; docs through grep and embedded consistency tests. |
| Stack agnosticism | Runtime command plans do not assume a Go/Makefile target project. Go package tests validate Liza's implementation only, not user-project behavior. |

---

## Structural Decisions

1. **Separate Semble package instead of extending Stacklit/SCIP packages.** Semble shares optional-tool behavior, but its offline model readiness and tree-visible ignore requirements are distinct enough to merit a package boundary.
2. **Environment-only first milestone.** A durable `config.semble` shape would be premature; the goal spec explicitly defers future content/model preferences.
3. **Prompt context is output of readiness, not cause of readiness.** Agent prompts never tell agents to troubleshoot Semble. Liza decides whether Semble is usable before rendering.
4. **Worktree safety precedes prompt context.** `.sembleignore` must be physically present before a worktree root can be advertised for Semble search.
5. **Private exclude helper is shared infrastructure.** The existing SCIP helper already owns a generic linked-worktree concern. Extracting it prevents competing `core.excludesFile` values and makes concurrency behavior explicit.
6. **Project-root search is stricter than task-root search.** Task roots can receive generated `.sembleignore`; project roots require operator-owned ignore safety because project-root Semble can traverse `.worktrees/` and sensitive source-adjacent files.
7. **Pairing SessionStart stays separate.** The existing shell hook remains the Pairing surface. MAS Semble prompt guidance belongs in spawned prompts so role-specific target roots are exact.

---

## Systemic Decomposition Review

No systemic issues identified.

The draft decomposition was reviewed against the long-term pressures in the goal spec: optional-tool sprawl, model-download side effects, worktree contamination, prompt bloat, and over-trust in semantic search. The component boundaries isolate those pressures: Semble readiness is owned once, worktree safety gates target roots before prompt rendering, and documentation owns operator expectations. The only load-bearing coupling is the readiness-to-prompt interface, and it is explicitly owned by `internal/semble` plus `internal/prompts` rather than duplicated across roles.

Second-pass review after the Scope 5 validation revision found no new systemic issues. The revision changes only executable evidence for the documentation scope; it does not alter component boundaries, interface ownership, or dependency ordering.

---

## Decomposition

Each scope becomes a code-planning child task. No bootstrap-precommit entry is emitted because `.pre-commit-config.yaml` exists on integration `HEAD`.

### Scope 1: Semble Capability Package

**Component(s):** Semble Capability Package

**Boundary:** In scope: create `internal/semble/` with strict `LIZA_ENABLE_SEMBLE` parsing, Semble executable detection, prewarm and offline validation command planning/execution using the temporary one-file fixture, process-local validation cache keyed by executable path, `SEMBLE_MODEL_NAME`, `HF_HOME`, `XDG_CACHE_HOME`, timeout, and fixture identity, bounded diagnostic classification, prompt-safe context metadata, shell-quoted command examples, root/worktree target-safety checks, and the single source of truth for default `.sembleignore` patterns. Out of scope: wiring the package into init, worktree creation, prompt rendering, or documentation beyond package docs/tests; adding durable state config; installing Semble, Python, or models; Semble MCP; remote Git URL indexing.

**Output desc:** Semble Capability Package: create `internal/semble/` with strict `LIZA_ENABLE_SEMBLE` parsing, Semble executable detection, prewarm and offline validation command planning/execution using the temporary one-file fixture, process-local validation cache keyed by executable path, `SEMBLE_MODEL_NAME`, `HF_HOME`, `XDG_CACHE_HOME`, timeout, and fixture identity, bounded diagnostic classification, prompt-safe context metadata, shell-quoted command examples, root/worktree target-safety checks, and the single source of truth for default `.sembleignore` patterns. Out of scope: wiring the package into init, worktree creation, prompt rendering, or documentation beyond package docs/tests; adding durable state config; installing Semble, Python, or models; Semble MCP; remote Git URL indexing.

**Output scope:** In scope: create `internal/semble/` with strict `LIZA_ENABLE_SEMBLE` parsing, Semble executable detection, prewarm and offline validation command planning/execution using the temporary one-file fixture, process-local validation cache keyed by executable path, `SEMBLE_MODEL_NAME`, `HF_HOME`, `XDG_CACHE_HOME`, timeout, and fixture identity, bounded diagnostic classification, prompt-safe context metadata, shell-quoted command examples, root/worktree target-safety checks, and the single source of truth for default `.sembleignore` patterns. Out of scope: wiring the package into init, worktree creation, prompt rendering, or documentation beyond package docs/tests; adding durable state config; installing Semble, Python, or models; Semble MCP; remote Git URL indexing.

**Output done_when:** `go test ./internal/semble` proves `LIZA_ENABLE_SEMBLE` truthy/false parsing, disabled-mode no-op behavior, exact prewarm and offline validation command plans, `HF_HUB_OFFLINE=1` offline execution, process-local cache hit/miss behavior for executable/model/cache/fixture key changes, bounded diagnostics for missing executable/model-offline/execution failure classes, default `.sembleignore` pattern coverage including runtime/generated/credential patterns, target-root safety validation, and prompt-safe command rendering with shell-quoted absolute target roots.

**Depends on:** None

**Decomposition metadata:**
- Owned files: `internal/semble/doc.go`, `internal/semble/semble.go`, `internal/semble/semble_test.go`
- Owned modules: `internal/semble`
- Read-only depends on: none
- Interfaces owned: `Semble readiness and prompt-context API`, `Semble default ignore pattern contract`, `Semble target safety contract`
- Interfaces consumed: none
- Coverage notes: Covers optional activation, offline/model readiness, command planning, diagnostics, cache keys, ignore pattern source of truth, and prompt metadata primitives.

### Scope 2: Semble Init Prewarm Integration

**Component(s):** Init Integration

**Boundary:** In scope: wire `internal/semble` into `cmd/liza` and workspace initialization paths so `liza init --spec` and equivalent non-interactive init perform controlled Semble prewarm and immediate offline validation only when `LIZA_ENABLE_SEMBLE` is truthy and `semble` is present; emit bounded operator-visible diagnostics without blocking init for missing/unready Semble; preserve existing SCIP init behavior and avoid adding durable Semble state or CLI flags. Out of scope: MAS prompt rendering, worktree `.sembleignore` generation, operator docs, Pairing SessionStart shell hook behavior, automatic Semble/model installation, and any user-project build command assumptions.

**Output desc:** Semble Init Prewarm Integration: wire `internal/semble` into `cmd/liza` and workspace initialization paths so `liza init --spec` and equivalent non-interactive init perform controlled Semble prewarm and immediate offline validation only when `LIZA_ENABLE_SEMBLE` is truthy and `semble` is present; emit bounded operator-visible diagnostics without blocking init for missing/unready Semble; preserve existing SCIP init behavior and avoid adding durable Semble state or CLI flags. Out of scope: MAS prompt rendering, worktree `.sembleignore` generation, operator docs, Pairing SessionStart shell hook behavior, automatic Semble/model installation, and any user-project build command assumptions.

**Output scope:** In scope: wire `internal/semble` into `cmd/liza` and workspace initialization paths so `liza init --spec` and equivalent non-interactive init perform controlled Semble prewarm and immediate offline validation only when `LIZA_ENABLE_SEMBLE` is truthy and `semble` is present; emit bounded operator-visible diagnostics without blocking init for missing/unready Semble; preserve existing SCIP init behavior and avoid adding durable Semble state or CLI flags. Out of scope: MAS prompt rendering, worktree `.sembleignore` generation, operator docs, Pairing SessionStart shell hook behavior, automatic Semble/model installation, and any user-project build command assumptions.

**Output done_when:** `go test ./internal/commands ./internal/ops ./cmd/liza` proves init skips Semble entirely when disabled, invokes Semble prewarm/offline validation when `LIZA_ENABLE_SEMBLE` is truthy and the CLI is present, preserves bounded diagnostics for missing executable and offline-unready model cases, does not add a Semble field to `models.Config` or state YAML, does not add Semble CLI flags, preserves existing SCIP init tests, and leaves workspace initialization non-blocking for Semble readiness failures.

**Depends on:** Scope 1

**Decomposition metadata:**
- Owned files: `cmd/liza/cmd_init.go`, `cmd/liza/cmd_init_test.go`, `internal/commands/init.go`, `internal/commands/init_test.go`, `internal/ops/init_project.go`, `internal/ops/init_project_test.go`
- Owned modules: `cmd/liza`, `internal/commands`, `internal/ops` init path
- Read-only depends on: Scope 1
- Interfaces owned: `Init-time Semble prewarm integration`
- Interfaces consumed: `Semble readiness and prompt-context API`
- Coverage notes: Covers init-time model bootstrap/offline validation and no durable Semble config.

### Scope 3: Worktree Semble Ignore Safety

**Component(s):** Worktree Semble Safety; shared worktree exclude helper

**Boundary:** In scope: add an idempotent shared linked-worktree private-exclude helper, refactor SCIP task-worktree private exclude setup to use it, generate/update task-worktree `.sembleignore` files with the default Semble ignore block, privately exclude generated `.sembleignore` from task diffs, serialize Semble/SCIP exclude updates, and invoke Semble worktree preparation from fresh/existing task worktree creation and reviewer worktree recovery before Semble prompt guidance can target those roots. Out of scope: Semble offline validation internals, init prewarm, prompt template wording, operator documentation, project-root `.sembleignore` creation, and changing tracked operator-owned `.sembleignore` files without explicit task scope.

**Output desc:** Worktree Semble Ignore Safety: add an idempotent shared linked-worktree private-exclude helper, refactor SCIP task-worktree private exclude setup to use it, generate/update task-worktree `.sembleignore` files with the default Semble ignore block, privately exclude generated `.sembleignore` from task diffs, serialize Semble/SCIP exclude updates, and invoke Semble worktree preparation from fresh/existing task worktree creation and reviewer worktree recovery before Semble prompt guidance can target those roots. Out of scope: Semble offline validation internals, init prewarm, prompt template wording, operator documentation, project-root `.sembleignore` creation, and changing tracked operator-owned `.sembleignore` files without explicit task scope.

**Output scope:** In scope: add an idempotent shared linked-worktree private-exclude helper, refactor SCIP task-worktree private exclude setup to use it, generate/update task-worktree `.sembleignore` files with the default Semble ignore block, privately exclude generated `.sembleignore` from task diffs, serialize Semble/SCIP exclude updates, and invoke Semble worktree preparation from fresh/existing task worktree creation and reviewer worktree recovery before Semble prompt guidance can target those roots. Out of scope: Semble offline validation internals, init prewarm, prompt template wording, operator documentation, project-root `.sembleignore` creation, and changing tracked operator-owned `.sembleignore` files without explicit task scope.

**Output done_when:** `go test ./internal/ops ./internal/scipsearch ./internal/agent` proves fresh and existing task worktree creation installs a Semble-visible `.sembleignore` with every default runtime/generated/credential ignore pattern, generated `.sembleignore` and `.liza/scip/` entries share the same private worktree exclude without competing `core.excludesFile` values, repeated and concurrent lifecycle calls are idempotent and keep `git status --porcelain` clean, SCIP task-worktree index tests still pass through the shared helper, reviewer worktree recovery prepares Semble ignore safety, and tracked operator-owned `.sembleignore` handling is explicit rather than silently overwritten.

**Depends on:** Scope 1

**Decomposition metadata:**
- Owned files: `internal/ops/wt_create.go`, `internal/ops/wt_create_test.go`, `internal/ops/semble_indexing.go`, `internal/scipsearch/scipsearch.go`, `internal/scipsearch/scipsearch_test.go`, `internal/agent/worktree_check.go`, `internal/agent/worktree_check_test.go`, `internal/worktreeexclude/doc.go`, `internal/worktreeexclude/worktreeexclude.go`, `internal/worktreeexclude/worktreeexclude_test.go`
- Owned modules: `internal/worktreeexclude`, `internal/ops` worktree lifecycle, `internal/scipsearch` task private-exclude integration, `internal/agent` reviewer worktree recovery
- Read-only depends on: Scope 1
- Interfaces owned: `Shared linked-worktree private exclude helper`, `Task-worktree Semble ignore preparation`
- Interfaces consumed: `Semble default ignore pattern contract`
- Coverage notes: Covers worktree contamination, generated-file cleanliness, shared private excludes, and reviewer worktree recovery.

### Scope 4: MAS Semble Prompt Guidance

**Component(s):** MAS Prompt Semble Context

**Boundary:** In scope: extend prompt metadata and rendering so MAS prompts include a concise `=== SEMBLE SEARCH ===` section only when `internal/semble` reports enabled, offline-ready, and target-safe context for the exact role target root; wire task, reviewer, and safe orchestrator project-root target selection; preserve Stacklit/SCIP sections; update unified query routing to position Semble for conceptual discovery, content modes, Morph fallback, direct-read verification, and `rg`/`ast-grep` boundaries. Out of scope: init prewarm implementation, worktree `.sembleignore` generation internals, Pairing SessionStart shell hook changes, operator documentation, and Semble MCP.

**Output desc:** MAS Semble Prompt Guidance: extend prompt metadata and rendering so MAS prompts include a concise `=== SEMBLE SEARCH ===` section only when `internal/semble` reports enabled, offline-ready, and target-safe context for the exact role target root; wire task, reviewer, and safe orchestrator project-root target selection; preserve Stacklit/SCIP sections; update unified query routing to position Semble for conceptual discovery, content modes, Morph fallback, direct-read verification, and `rg`/`ast-grep` boundaries. Out of scope: init prewarm implementation, worktree `.sembleignore` generation internals, Pairing SessionStart shell hook changes, operator documentation, and Semble MCP.

**Output scope:** In scope: extend prompt metadata and rendering so MAS prompts include a concise `=== SEMBLE SEARCH ===` section only when `internal/semble` reports enabled, offline-ready, and target-safe context for the exact role target root; wire task, reviewer, and safe orchestrator project-root target selection; preserve Stacklit/SCIP sections; update unified query routing to position Semble for conceptual discovery, content modes, Morph fallback, direct-read verification, and `rg`/`ast-grep` boundaries. Out of scope: init prewarm implementation, worktree `.sembleignore` generation internals, Pairing SessionStart shell hook changes, operator documentation, and Semble MCP.

**Output done_when:** `go test ./internal/prompts ./internal/agent` proves Semble prompt sections are omitted when disabled, unavailable, offline-unready, or target-unsafe; task and reviewer prompts use shell-quoted absolute worktree roots and never parent project roots; orchestrator prompts use the project root only when root `.sembleignore` safety passes; rendered commands include `HF_HUB_OFFLINE=1`, `semble search`, `semble find-related`, and the one-line content-mode guidance; query routing preserves Stacklit/SCIP guidance and positions Morph as fallback only when Semble is unavailable; prompt text states Semble returns candidates, not proof; reviewer prompt assembly consumes the worktree safety contract without modifying reviewer recovery; and Pairing SessionStart tests remain unaffected.

**Depends on:** Scope 1, Scope 3

**Decomposition metadata:**
- Owned files: `internal/prompts/builder.go`, `internal/prompts/builder_test.go`, `internal/prompts/templates/base_prompt.tmpl`, `internal/agent/prompt.go`, `internal/agent/prompt_test.go`, `internal/agent/strategy_orchestrator.go`, `internal/agent/strategy_orchestrator_scip_test.go`, `internal/agent/strategy_reviewer.go`
- Owned modules: `internal/prompts`, `internal/agent` prompt assembly
- Read-only depends on: Scope 1, Scope 3
- Interfaces owned: `Prompt SembleSearchContext rendering`, `MAS Semble target-root selection`
- Interfaces consumed: `Semble readiness and prompt-context API`, `Task-worktree Semble ignore preparation`
- Coverage notes: Covers MAS prompt injection, target-root isolation, and routing guidance.

### Scope 5: Semble Operator Documentation

**Component(s):** Operator Documentation

**Boundary:** In scope: update `README.md`, `support-docs/CONFIGURATION.md`, `support-docs/CUSTOMIZING_AGENT_TOOLS.md`, and `support-docs/USAGE_MULTI_AGENTS.md` to document Semble as an optional external semantic discovery tool; `LIZA_ENABLE_SEMBLE` truthy/false activation; init-time model prewarm and possible Hugging Face contact; `HF_HUB_OFFLINE=1` MAS operating mode; `.sembleignore` default safety patterns and directory-scoped behavior; task/root worktree scoping; routing relative to Stacklit, SCIP, Morph MCP, `rg`, `ast-grep`, and direct reads; non-goals including automatic installation, vendoring, `semble init`, Semble MCP, and remote Git URL indexing. Out of scope: implementation code, Pairing SessionStart shell code, generated runtime state under `.liza/agent-outputs/`, and converting docs stubs under `docs/` into canonical content.

**Output desc:** Semble Operator Documentation: update `README.md`, `support-docs/CONFIGURATION.md`, `support-docs/CUSTOMIZING_AGENT_TOOLS.md`, and `support-docs/USAGE_MULTI_AGENTS.md` to document Semble as an optional external semantic discovery tool; `LIZA_ENABLE_SEMBLE` truthy/false activation; init-time model prewarm and possible Hugging Face contact; `HF_HUB_OFFLINE=1` MAS operating mode; `.sembleignore` default safety patterns and directory-scoped behavior; task/root worktree scoping; routing relative to Stacklit, SCIP, Morph MCP, `rg`, `ast-grep`, and direct reads; non-goals including automatic installation, vendoring, `semble init`, Semble MCP, and remote Git URL indexing. Out of scope: implementation code, Pairing SessionStart shell code, generated runtime state under `.liza/agent-outputs/`, and converting docs stubs under `docs/` into canonical content.

**Output scope:** In scope: update `README.md`, `support-docs/CONFIGURATION.md`, `support-docs/CUSTOMIZING_AGENT_TOOLS.md`, and `support-docs/USAGE_MULTI_AGENTS.md` to document Semble as an optional external semantic discovery tool; `LIZA_ENABLE_SEMBLE` truthy/false activation; init-time model prewarm and possible Hugging Face contact; `HF_HUB_OFFLINE=1` MAS operating mode; `.sembleignore` default safety patterns and directory-scoped behavior; task/root worktree scoping; routing relative to Stacklit, SCIP, Morph MCP, `rg`, `ast-grep`, and direct reads; non-goals including automatic installation, vendoring, `semble init`, Semble MCP, and remote Git URL indexing. Out of scope: implementation code, Pairing SessionStart shell code, generated runtime state under `.liza/agent-outputs/`, and converting docs stubs under `docs/` into canonical content.

**Output done_when:** Documentation checks prove `README.md` lists Semble as an optional semantic discovery tool without making it a requirement; `support-docs/CONFIGURATION.md` documents `LIZA_ENABLE_SEMBLE`, truthy/false values, init-time prewarm, possible Hugging Face/model cache contact, offline validation, `HF_HUB_OFFLINE=1`, bounded diagnostics, and Semble non-goals; `support-docs/CUSTOMIZING_AGENT_TOOLS.md` documents Semble routing relative to Stacklit, SCIP, Morph MCP, `rg`, `ast-grep`, and direct reads; `support-docs/USAGE_MULTI_AGENTS.md` points MAS operators to Semble setup; `.sembleignore` directory scope and default runtime/generated/credential exclusions are documented; and `go test ./internal/embedded` or equivalent embedded-doc consistency checks pass if support-doc embedding applies.

**Output validation:** Scope 5 must use targeted checks rather than a broad multi-file alternation:
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

**Depends on:** None

**Decomposition metadata:**
- Owned files: `README.md`, `support-docs/CONFIGURATION.md`, `support-docs/CUSTOMIZING_AGENT_TOOLS.md`, `support-docs/USAGE_MULTI_AGENTS.md`
- Owned modules: operator documentation
- Read-only depends on: none
- Interfaces owned: `Operator Semble documentation contract`
- Interfaces consumed: `Semble default ignore pattern contract`, `Semble readiness and prompt-context API`, `MAS Semble target-root selection`
- Coverage notes: Covers operator setup, safety, routing, and non-goals.

### Spec Coverage

| Spec Requirement | Scope |
|------------------|-------|
| Document Semble as optional repository-navigation tool | Scope 5 |
| Add strict opt-in `LIZA_ENABLE_SEMBLE` with truthy/false parsing | Scope 1 |
| Disabled Semble performs no validation, execution, prompt injection, or MAS command mention | Scope 1, Scope 4 |
| Validate `semble` availability before prompt guidance | Scope 1, Scope 4 |
| Init-time prewarm/download through controlled fixture | Scope 1, Scope 2 |
| Offline fixture validation with `HF_HUB_OFFLINE=1` | Scope 1, Scope 2, Scope 4 |
| Fresh process-local validation cache keyed by executable/model/cache/fixture inputs | Scope 1 |
| Graceful degradation with bounded diagnostics | Scope 1, Scope 2, Scope 4 |
| MAS guidance uses `HF_HUB_OFFLINE=1` | Scope 4 |
| Prefer Semble over Morph when enabled/offline-ready; Morph fallback otherwise | Scope 4, Scope 5 |
| Task and reviewer agents search exact worktree roots | Scope 3, Scope 4 |
| Safe orchestrator project-root guidance only when `.worktrees/` exclusion is guaranteed | Scope 1, Scope 4 |
| Exclude `.liza/`, `.worktrees/`, `stacklit.json`, `*.scip`, and credential patterns | Scope 1, Scope 3, Scope 5 |
| Automatically create worktree-local `.sembleignore` | Scope 3 |
| Keep generated `.sembleignore` out of task diffs with SCIP-style private exclude | Scope 3 |
| Share Semble and SCIP private exclude handling and serialize updates | Scope 3 |
| Document `.sembleignore` directory scope | Scope 5 |
| Prompt guidance says Semble is candidate discovery, not evidence | Scope 4, Scope 5 |
| Preserve Stacklit/SCIP and `rg`/`ast-grep` routing | Scope 4, Scope 5 |
| Keep MAS Semble context in spawned prompts, not SessionStart | Scope 4 |
| Operator docs explain model bootstrap and `HF_HUB_OFFLINE=1` | Scope 5 |
| Semble failures do not block agent spawn | Scope 1, Scope 2, Scope 4 |
| Semble-related generated files stay out of task diffs | Scope 3 |
| Pairing SessionStart support already implemented and not regressed | Scope 4 through regression awareness, no new child scope |

## Shared-File Audit

| File/Area | Owner | Readers / Ordering |
|-----------|-------|--------------------|
| `internal/semble/*` | Scope 1 | Scopes 2, 3, 4, and 5 consume contracts; Scopes 2-4 depend on Scope 1 |
| `cmd/liza/cmd_init.go` | Scope 2 | No sibling writes |
| `internal/commands/init.go` | Scope 2 | No sibling writes |
| `internal/ops/init_project.go` | Scope 2 | No sibling writes |
| `internal/ops/wt_create.go` | Scope 3 | Scope 4 reads resulting safety contract; Scope 4 depends on Scope 3 |
| `internal/scipsearch/scipsearch.go` | Scope 3 | Scope 1 owns Semble patterns only; Scope 3 owns private-exclude refactor |
| `internal/worktreeexclude/*` | Scope 3 | Scope 3 owns helper; SCIP/Semble consumers read through it |
| `internal/agent/worktree_check.go` | Scope 3 | Scope 4 reads reviewer worktree readiness behavior through Scope 3's contract; Scope 4 depends on Scope 3 |
| `internal/agent/prompt.go` | Scope 4 | No sibling writes |
| `internal/agent/strategy_orchestrator.go` | Scope 4 | No sibling writes |
| `internal/prompts/builder.go` | Scope 4 | No sibling writes |
| `internal/prompts/templates/base_prompt.tmpl` | Scope 4 | No sibling writes |
| `README.md` and `support-docs/*` | Scope 5 | Other scopes read documentation contract only; no depends_on needed |

## Validation Plan

- Confirm this document maps every MVP and success criterion from `specs/goals/20260601-integrate-semble.md` to at least one scope.
- Confirm `Output desc`, `Output scope`, `Output done_when`, and Scope 5 validation commands above are copied verbatim into the structured output JSON where applicable.
- Confirm each output entry has `arch_ref` set to `specs/arch-plan/20260601-integrate-semble/20260601-130216-architecture-main-1.md`.
- Confirm each output entry includes decomposition metadata and no duplicate owned-file or interface ownership.
- Confirm depends_on expresses real interface/order dependencies only: Scope 2 depends on Scope 1; Scope 3 depends on Scope 1; Scope 4 depends on Scopes 1 and 3; Scope 5 is independent.
- Confirm no `bootstrap-precommit` output entry is emitted because `.pre-commit-config.yaml` exists at integration `HEAD`.
- Run `liza set-task-output architecture-main-1 --output specs/arch-plan/20260601-integrate-semble/20260601-130216-architecture-main-1-output.json --agent-id architect-1 --json`.
- Run pre-commit on touched architecture artifacts, commit only those artifacts, verify `git status --short` is clean, and submit `HEAD` for review.

## Self-Review Notes

- The plan is implementation planning only; no runtime code is changed in this task.
- The decomposition is by domain boundary rather than implementation step: capability package, init integration, worktree safety, prompt guidance, and operator docs.
- Interface ownership is singular: Semble readiness and ignore patterns belong to Scope 1; linked-worktree private excludes belong to Scope 3; prompt rendering belongs to Scope 4; docs belong to Scope 5.
- Worktree safety and prompt injection are ordered to prevent prompt guidance from advertising unsafe roots.
- Project-root Semble guidance remains stricter than task-root guidance to avoid indexing sibling worktrees or runtime state.
