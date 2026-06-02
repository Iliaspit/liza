# PRD: Indexing Activation for Setup and Init

Status: draft

## Goal

Make Liza's Stacklit, SCIP, and Semble activation reliable across setup, pairing
init, and MAS init while keeping generic tool routing resilient when optional
tools are disabled or unavailable.

## Context

Liza already supports optional repository-navigation tools in MAS prompts and
pairing SessionStart context. A temporary `upgrade.sh` workaround appends
concrete indexing guidance to `~/.liza/AGENT_TOOLS.md`, while project-local hook
setup remains manual. This should become first-class behavior split across
`liza setup` and `liza init`: setup owns global generic guidance, init owns
project-local activation artifacts.

## General information

Applies to: global setup guidance, pairing init, MAS init, and prompt/session
context routing for optional repository indexing tools.

### References

- User requirement: current pairing discussion - `liza setup` owns
  `AGENT_TOOLS.md`; `liza init` owns the rest.
- Source: `upgrade.sh` - temporary global append plus project init workflow.
- Source: `auto-indexing.md` - manual pairing procedure for Stacklit/SCIP hooks
  and ignore rules.
- Source: `support-docs/CONFIGURATION.md` - existing `LIZA_ENABLE_STACKLIT`,
  `LIZA_ENABLE_SCIP_SEARCH`, and `LIZA_ENABLE_SEMBLE` activation semantics.
- Source: `support-docs/CUSTOMIZING_AGENT_TOOLS.md` - current generic routing
  guidance.
- Source: `internal/embedded/hooks/session-context.sh` - pairing SessionStart
  context emission.
- Source: `internal/prompts/templates/base_prompt.tmpl` - MAS prompt routing
  sections.
- Source: `specs/architecture/ADR/0074-sessionstart-context-hooks.md` -
  SessionStart context hook rationale.

### Non-Functional Requirements

- NFR-000-1: Generic routing guidance must be safe when Stacklit, SCIP, or
  Semble are disabled, missing, or not advertised in the current session.
- NFR-000-2: Project-specific paths, generated index locations, and readiness
  state must not be written into global `AGENT_TOOLS.md`.
- NFR-000-3: Optional indexing failures must degrade by omitting unavailable
  prompt/context sections rather than asking agents to troubleshoot unrelated
  tooling during task work.

### Related External Components

- Component C-001 - Stacklit CLI.
- Component C-002 - `scip-search` plus language-specific SCIP indexers.
- Component C-003 - Semble CLI.

### Out of Scope

- Installing Stacklit, `scip-search`, language indexers, Semble, Python, Node,
  Go, or model dependencies.
- Replacing Stacklit, SCIP, Semble, `rg`, `ast-grep`, or direct reads.
- Changing MAS worktree indexing semantics already covered by existing SCIP,
  Stacklit, and Semble specs.
- Adding durable Semble config to `state.yaml`.

### Assumptions

- ASM-000-1: The requested path means
  `specs/goals/20260602-indexing-activation.md`, matching the repository's
  existing `specs/goals/` convention. Confidence: HIGH.
- ASM-000-2: `liza setup` may update embedded default `AGENT_TOOLS.md` content,
  but must preserve the existing user-customizable overwrite/prompt behavior.
  Confidence: HIGH.

### Open Questions

- None.

---

## Feature FT-001 - Global Generic Tool Guidance

### References

- User requirement: generic Stacklit/SCIP/Semble routing guidance should be
  resilient to disabled tools.
- Source: `support-docs/CUSTOMIZING_AGENT_TOOLS.md` - optional-tool routing and
  fallback guidance.
- Source: `internal/commands/setup.go` - `AGENT_TOOLS.md` customization
  protection during setup.

### Functional Requirements

- FR-001-1: `liza setup` must install generic Stacklit, SCIP, and Semble routing
  guidance into the default `AGENT_TOOLS.md` content.
- FR-001-2: The guidance must be conditional in wording: agents use Stacklit
  only when Liza supplies an explicit Stacklit index path, use `scip-search`
  only when Liza supplies an explicit SCIP index path, and use Semble only when
  Liza supplies an explicit target root or current session context says Semble
  is available.
- FR-001-3: The guidance must describe fallbacks for disabled or unavailable
  tools: direct reads, `rg` for exact literals and path discovery, `ast-grep`
  for syntax-shaped search, and approved semantic fallback tools only when
  policy exposes them.
- FR-001-4: The guidance must not contain project-specific absolute paths,
  generated file locations for a particular repo, or claims that optional tools
  are installed.

### Non-Functional Requirements

- NFR-001-1: The installed default guidance should be concise enough that setup
  does not materially increase global contract context beyond the routing
  behavior it adds.

### Acceptance Criteria

- AC-001-1: Given a fresh `liza setup`, when `AGENT_TOOLS.md` is installed from
  embedded defaults, then generic Stacklit/SCIP/Semble routing guidance is
  present.
- AC-001-2: Given all optional indexing tools are disabled, when an agent reads
  `AGENT_TOOLS.md`, then the guidance still routes to valid fallback tools and
  does not require invoking disabled tools.
- AC-001-3: Given a user-customized `~/.liza/AGENT_TOOLS.md`, when `liza setup`
  runs, then existing customization protection still applies.

### Depends on:

Runtime coupling:

- Component C-001 - Stacklit CLI.
- Component C-002 - `scip-search` plus language-specific SCIP indexers.
- Component C-003 - Semble CLI.

### Out of Scope

- Appending repo-specific instructions to `~/.liza/AGENT_TOOLS.md`.
- Removing user customization protection from `liza setup`.

---

## Feature FT-002 - Pairing Project Index Activation

### References

- User requirement: `liza init` owns project-local activation artifacts.
- Source: `auto-indexing.md` - manual project hook and ignore procedure.
- Source: `internal/commands/init.go` - pairing init flow.
- Source: `internal/embedded/hooks/session-context.sh` - detection of pairing
  repo-root indexes and Semble readiness.
- Source: `support-docs/CONFIGURATION.md` - activation gate semantics.

### Functional Requirements

- FR-002-1: `liza init --claude`, `liza init --codex`, and combined pairing init
  flows must install project-local provider hooks as they do today.
- FR-002-2: When `LIZA_ENABLE_STACKLIT` is truthy, pairing init must install or
  verify project-local Git hook plumbing that refreshes repo-root `stacklit.json`
  at safe lifecycle points.
- FR-002-2a: Automatic Git lifecycle refresh must not run `stacklit ai-summary`.
  The generated `liza-index.sh` may run AI-summary only when invoked manually with
  an explicit `ai` argument, matching the temporary manual procedure.
- FR-002-3: When `LIZA_ENABLE_SCIP_SEARCH` is truthy, pairing init must
  autodetect a repo-specific SCIP indexing plan and install or verify
  project-local Git hook plumbing that refreshes repo-root SCIP indexes for
  enabled languages.
- FR-002-3a: Pairing SCIP autodetection must plan concrete indexer commands,
  including language-specific roots such as Go `--module-root`, TypeScript
  `--cwd` plus project path, and Python `--cwd`.
- FR-002-3b: Repeated `--scip-search <language>` flags may restrict which
  languages pairing init considers, but they must not be treated as sufficient
  root/cwd selection for monorepos.
- FR-002-3c: If autodetection finds multiple plausible roots for an enabled
  language and cannot choose one confidently, pairing init must fail with a
  monorepo ambiguity diagnostic listing candidate roots and the unresolved
  language instead of generating a guessed hook command.
- FR-002-3d: If autodetection finds exactly one confident root for each enabled
  language, pairing init must generate `.git/hooks/liza-index.sh` with concrete
  repo-specific SCIP indexer commands.
- FR-002-4: Pairing init must ensure generated pairing indexes are either
  ignored, privately excluded, or otherwise kept out of accidental task diffs
  unless already intentionally tracked.
- FR-002-5: When `LIZA_ENABLE_SEMBLE` is truthy, pairing init must ensure the
  project root has a safe physical `.sembleignore` before SessionStart
  advertises Semble.
- FR-002-6: Pairing init must not modify `~/.liza/AGENT_TOOLS.md`; global
  generic guidance remains a `liza setup` responsibility.
- FR-002-7: Pairing init must preserve existing project Git hooks. It must not
  silently overwrite existing `post-commit`, `post-checkout`, `post-merge`, or
  `post-rewrite` hooks.
- FR-002-8: Pairing init must detect non-default `core.hooksPath`. If Liza cannot
  safely install or chain its indexing hook under the effective hooks path, init
  must report a clear diagnostic instead of installing inert `.git/hooks/*`
  files.
- FR-002-9: Pairing init must document the pairing-init meaning of
  `LIZA_ENABLE_STACKLIT`, `LIZA_ENABLE_SCIP_SEARCH`, and `LIZA_ENABLE_SEMBLE`
  separately from MAS runtime activation in `support-docs/CONFIGURATION.md` and
  synced embedded support docs.

### Non-Functional Requirements

- NFR-002-1: Pairing index activation must preserve the existing rule that
  SessionStart only supplies concrete paths/readiness that exist for the current
  repository.
- NFR-002-2: Pairing index activation must not assume Liza's own build system or
  language stack applies to target projects.

### Acceptance Criteria

- AC-002-1: Given pairing init with all three env vars disabled, when init
  completes, then no optional indexing hook behavior is activated.
- AC-002-2: Given pairing init with Stacklit enabled, when a supported Git
  lifecycle event occurs, then repo-root Stacklit context can be refreshed
  without editing `AGENT_TOOLS.md`.
- AC-002-3: Given pairing init with Semble enabled but no safe `.sembleignore`,
  when init runs, then it creates or reports the missing safety artifact before
  Semble is advertised.
- AC-002-4: Given pairing init with generated indexes, when `git status` runs
  after init and initial refresh, then generated artifacts do not appear as
  accidental untracked files unless the project intentionally tracks them.
- AC-002-5: Given a single-root Go, TypeScript, or Python repository and
  `LIZA_ENABLE_SCIP_SEARCH=1`, when pairing init runs, then the generated
  `.git/hooks/liza-index.sh` contains concrete SCIP indexer commands for the
  confidently detected language roots.
- AC-002-6: Given a monorepo with multiple plausible roots for an enabled SCIP
  language, when pairing init runs, then init fails with a clear ambiguity error
  instead of writing a guessed SCIP hook.
- AC-002-7: Given `--scip-search go` in pairing mode, when TypeScript and Python
  roots also exist, then SCIP autodetection considers only Go roots while still
  requiring confident Go root selection.
- AC-002-8: Given an existing project Git hook at a lifecycle event Liza needs,
  when pairing init runs, then the existing hook is preserved, chained, or init
  reports an explicit collision diagnostic; it is not overwritten silently.
- AC-002-9: Given a repository with non-default `core.hooksPath`, when pairing
  init runs, then Liza installs into the effective hook path or reports that it
  cannot safely do so.
- AC-002-10: Given pairing Stacklit activation, when automatic Git lifecycle
  refresh runs, then it refreshes `stacklit.json` without running
  `stacklit ai-summary`.
- AC-002-11: Given the generated `liza-index.sh` is invoked manually with the
  `ai` argument, then Stacklit refresh includes AI-summary behavior.
- AC-002-12: Given this goal is implemented, when docs are checked, then
  `support-docs/CONFIGURATION.md` and embedded support docs explain pairing-init
  env-gate behavior separately from MAS runtime activation.

### Depends on:

Implementation ordering:

- Feature FT-001 - Global Generic Tool Guidance.

Runtime coupling:

- Component C-001 - Stacklit CLI.
- Component C-002 - `scip-search` plus language-specific SCIP indexers.
- Component C-003 - Semble CLI.

### Out of Scope

- Replacing MAS worktree-local generated indexes with repo-root pairing indexes.
- Installing or downloading optional external tools.

---

## Feature FT-003 - MAS Init Activation Continuity

### References

- Source: `specs/goals/20260517-use-scip-search.md` - MAS SCIP activation and
  durable language allowlist.
- Source: `specs/goals/20260601-integrate-semble.md` - MAS Semble activation
  and offline readiness.
- Source: `support-docs/CONFIGURATION.md` - Stacklit, SCIP, and Semble
  configuration behavior.
- Source: `internal/scipsearch/scipsearch.go` - current MAS SCIP runtime command
  planning and language/root inference behavior.
- Source: `internal/prompts/templates/base_prompt.tmpl` - MAS prompt routing.

### Functional Requirements

- FR-003-1: `liza init --spec` must preserve existing MAS activation semantics:
  Stacklit and Semble remain process-local env-gated tools, while SCIP uses the
  env gate plus durable `config.scip_search`.
- FR-003-2: MAS prompts must include Stacklit, SCIP, and Semble sections only
  when the corresponding generated context or readiness metadata is present.
- FR-003-3: MAS prompt routing must stay role/target specific: task agents use
  task worktree paths, reviewer agents use reviewer worktree paths, and
  orchestrators use project-root context only when that root is safe.
- FR-003-4: MAS init behavior must not depend on pairing Git hooks or repo-root
  pairing indexes.
- FR-003-5: The goal must preserve or explicitly supersede current MAS SCIP
  runtime planning semantics: runtime command planning is centralized in
  `internal/scipsearch`, intersects `state.Config.ScipSearch` with languages
  detected in the current target root, and derives concrete Go, TypeScript, and
  Python indexer arguments from that target root.
- FR-003-6: If pairing mode introduces stricter monorepo ambiguity failure than
  current MAS runtime planning, the implementation plan must either document that
  divergence as intentional or include a follow-up task to align MAS ambiguity
  handling.

### Acceptance Criteria

- AC-003-1: Given `LIZA_ENABLE_SCIP_SEARCH` is false and `config.scip_search`
  exists, when MAS prompts are built, then no SCIP prompt section is included.
- AC-003-2: Given `LIZA_ENABLE_STACKLIT` is false and `stacklit.json` exists,
  when MAS prompts are built, then no Stacklit prompt section is included unless
  an existing implementation explicitly documents a different fallback.
- AC-003-3: Given `LIZA_ENABLE_SEMBLE` is false, when MAS prompts are built, then
  no Semble prompt section is included.
- AC-003-4: Given an enabled tool fails readiness or index generation, when MAS
  agent spawn proceeds, then the unavailable tool section is omitted and other
  available routing guidance remains usable.
- AC-003-5: Given this goal is decomposed into implementation tasks, when a task
  changes pairing SCIP autodetection, then the task must state whether MAS keeps
  its current deterministic/fallback root inference or adopts the same ambiguity
  failure behavior.

### Depends on:

Runtime coupling:

- Component C-001 - Stacklit CLI.
- Component C-002 - `scip-search` plus language-specific SCIP indexers.
- Component C-003 - Semble CLI.

### Out of Scope

- Changing existing task worktree generation locations for MAS indexes.
- Adding durable Stacklit or Semble config.

---

## Feature FT-004 - Session and Prompt Context Boundaries

### References

- Source: `specs/architecture/ADR/0074-sessionstart-context-hooks.md` -
  separation between pairing SessionStart context and MAS prompt context.
- Source: `internal/embedded/hooks/session-context.sh` - pairing startup context.
- Source: `internal/prompts/templates/base_prompt.tmpl` - MAS prompt context.

### Functional Requirements

- FR-004-1: Pairing SessionStart context must continue to emit concrete repo-root
  Stacklit, SCIP, and Semble instructions only when the corresponding
  artifacts/readiness checks are present.
- FR-004-2: MAS prompts must continue to receive task/reviewer/orchestrator
  specific context from structured prompt metadata rather than relying on
  pairing SessionStart repo-root assumptions.
- FR-004-3: Prompt and SessionStart guidance must avoid duplicated long-form
  routing text where a shorter shared wording is sufficient.
- FR-004-4: Generic routing text must state that index and semantic-search
  results are navigation aids, while direct source reads remain the evidence for
  edits, reviews, and success claims.

### Acceptance Criteria

- AC-004-1: Given pairing mode with no detected indexes and Semble disabled,
  when SessionStart runs, then it emits initialization guidance without optional
  tool command blocks.
- AC-004-2: Given MAS mode with generated worktree indexes, when an agent prompt
  is built, then the prompt uses worktree-specific paths rather than repo-root
  pairing paths.
- AC-004-3: Given Stacklit, SCIP, and Semble are all available, when routing
  guidance is rendered, then it distinguishes conceptual discovery, module
  orientation, symbol navigation, exact search, syntax search, and direct
  evidence reads.
- AC-004-4: Given any optional tool is unavailable, when routing guidance is
  rendered, then it does not instruct the agent to invoke that unavailable tool
  before using fallback search/read tools.

### Depends on:

Implementation ordering:

- Feature FT-001 - Global Generic Tool Guidance.
- Feature FT-002 - Pairing Project Index Activation.
- Feature FT-003 - MAS Init Activation Continuity.

### Out of Scope

- Introducing a new provider-specific startup mechanism beyond existing
  SessionStart hooks and prompt metadata.
