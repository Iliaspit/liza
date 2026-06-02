# Integrate Semble as Optional Semantic Repository Search

Status: draft

## Problem Statement

Liza already has strong repository-navigation primitives for multi-agent work:

- `Stacklit` gives agents compact module, dependency, hot-file, and ownership
  orientation.
- `scip-search` gives agents precise indexed symbol, reference, and
  implementation navigation for supported languages.
- `rg`, `ast-grep`, `mdtoc`, and direct reads remain the reliable source-of-truth
  tools for text search, syntax-shaped search, Markdown navigation, and final
  verification.

The remaining gap is natural-language semantic chunk discovery. Agents often
begin with conceptual questions such as "where are agent CLI defaults
resolved?", "what enforces worktree cleanup?", or "how is review-boundary state
repaired?" Before a symbol name or module boundary is known, agents still burn
tokens through broad `rg` searches and repeated file reads.

`semble` is a Python code-search tool built for agents. It indexes local code
into chunks, combines static `potion-code-16M` embeddings with BM25 lexical
retrieval, and exposes a CLI suitable for Liza's explicit tool-routing model.
Initial exploration showed that Semble is valuable for the missing
semantic-discovery tier, but it must be integrated carefully:

- Its default model bootstrap may contact Hugging Face unless the model is
  already cached, explicitly configured, or intentionally downloaded during a
  controlled operator action.
- It is worktree-compatible only when pointed at the correct worktree root.
- It is not evidence by itself; source reads still verify behavior before edits.
- It overlaps with, but does not replace, Stacklit, SCIP, `rg`, or `ast-grep`.
- The current integration should stay CLI-first; MCP use belongs behind a
  separate MAS MCP policy rather than this goal.

Liza needs an optional, strict opt-in Semble integration that preserves MAS
worktree isolation, offline predictability, prompt honesty, and source-of-truth
verification discipline.

## Target Users

- MAS agents that need to locate relevant code before knowing exact symbols.
- MAS coders working in task worktrees on unfamiliar code paths.
- MAS reviewers looking for related implementations or policy-enforcement code
  during review.
- Orchestrator and planning roles that need conceptual repository discovery
  without loading large file sets.
- Pairing users who want a documented, disciplined way to use Semble alongside
  Liza's existing navigation tools.
- Operators who want semantic search but do not want unattended agents to
  silently trigger network downloads or index unrelated worktrees.

## Goal

Make Semble a documented, strict opt-in Liza repository-navigation tool for
semantic chunk discovery.

When Semble is enabled and offline-readiness validation succeeds, Liza should
surface concise prompt guidance telling agents when and how to use `semble
search` and `semble find-related` against the current target root. Semble
guidance must make the routing boundary explicit:

- Use Semble for natural-language or conceptual discovery when exact symbols
  are unknown.
- Use Semble with `--content docs` for documentation/spec questions and
  `--content config` for configuration questions.
- Use Stacklit for module/dependency orientation when a Stacklit index is
  available.
- Use `scip-search` for exact symbols, references, and implementations when a
  SCIP index is available.
- Use Morph MCP semantic/codebase search only as a fallback when Semble is
  disabled, unavailable, or not offline-ready and the current tool/MCP policy
  exposes Morph to the agent.
- Use `rg` for exhaustive literal search and quick exact confirmation; do not
  use `rg` for broad-scope or common-word conceptual queries.
- Use `ast-grep` for syntax-shaped search.
- Use direct file reads before editing or claiming evidence.

Semble should remain an external optional tool. Liza should not vendor Semble,
install Semble automatically, download Hugging Face models implicitly during
MAS work, route this first milestone through Semble MCP, or treat semantic
results as proof.

## Source Material

- Semble repository: <https://github.com/MinishLab/semble/>
- Semble DeepWiki: <https://deepwiki.com/MinishLab/semble>
- Semble observed local behavior:
  - `semble search "where are agent CLI defined?"` returned relevant Liza chunks
    from `cmd/liza/cmd_agent.go`, `internal/agent/supervisor.go`,
    `internal/commands/recover_agent.go`, `internal/agent/cli.go`, and
    `cmd/liza/cmd_init.go`.
  - First unauthenticated execution emitted a Hugging Face Hub warning.
  - `HF_HUB_OFFLINE=1 semble search "where are agent CLI defined?"` succeeded
    after the model was cached.
  - `HF_HUB_OFFLINE=1 semble search "where is task superseding specified?"
    internal --top-k 5 --content docs` succeeded and searched Markdown content,
    demonstrating that docs/config content routing is relevant.
- Semble source behavior verified against current upstream source during exploration:
  - Default model resolves through `SEMBLE_MODEL_NAME`, falling back to
    `minishlab/potion-code-16M`.
  - Model loading calls `StaticModel.from_pretrained(..., force_download=False)`.
  - Local cache keys normalize local paths via absolute `Path.resolve()`.
  - Local cache validation compares file modification times and indexed file
    manifests.
  - Indexes are written under the OS cache directory, not inside the repository
    for normal `search` use.
  - `.sembleignore` is loaded per directory during recursive file walking; it is
    not a global ignore file.
  - Semble's `--content` flag defaults to `code` when omitted, matching Liza's
    first-milestone default.
- Existing Liza references:
  - `README.md` — recommended external tool table and Stacklit/SCIP setup
    sections.
  - `support-docs/CONFIGURATION.md` — `LIZA_ENABLE_STACKLIT` and
    `LIZA_ENABLE_SCIP_SEARCH` activation-gate model.
  - `support-docs/CUSTOMIZING_AGENT_TOOLS.md` — worktree-safe tool guidance.
  - `internal/scipsearch/` — optional external index lifecycle contract.
  - `internal/scipsearch/scipsearch.go:862-948` — established task-worktree
    private-exclude pattern for keeping generated runtime artifacts out of task
    diffs.
  - `internal/stacklit/` — optional Stacklit runtime index lifecycle contract.
  - `internal/prompts/builder_test.go` — prompt-routing expectations when
    Stacklit and SCIP are both available.
  - `internal/embedded/hooks/session-context.sh` — Pairing SessionStart hook
    that emits repo-root Stacklit and SCIP context while leaving MAS agents on
    prompt-supplied worktree context.
  - `specs/architecture/ADR/0074-sessionstart-context-hooks.md` — rationale for
    provider SessionStart hooks and mode-specific startup guidance.
  - `internal/embedded/session_context_hook_test.go` — SessionStart hook tests
    for indexed Pairing repositories.

## MVP Scope

- [ ] Document Semble as an optional Liza repository-navigation tool for
      semantic chunk discovery.
- [ ] Add a strict opt-in activation gate, provisionally named
      `LIZA_ENABLE_SEMBLE`.
- [ ] Treat unset, empty, `0`, and `false` as disabled values; treat `1` and
      `true` as enabled values after trimming whitespace and comparing
      case-insensitively.
- [ ] When disabled, do not validate Semble, run Semble, inject Semble prompt
      guidance, or mention Semble commands in spawned MAS prompts.
- [ ] When enabled, validate that `semble` is available before injecting Semble
      prompt guidance.
- [ ] During `liza init --spec`, when `LIZA_ENABLE_SEMBLE` is truthy and the
      `semble` CLI is present, intentionally prewarm/download the configured
      Semble model if it is not already available by running a controlled
      prewarm search against a temporary directory containing one tiny supported
      code file.
- [ ] After init-time prewarm, validate offline readiness before MAS use by
      running Semble with `HF_HUB_OFFLINE=1` against the same controlled prewarm
      fixture.
- [ ] Before any prompt-injection lifecycle that would mention Semble, run the
      bounded offline fixture validation unless a fresh process-local validation
      result is available.
- [ ] Define a fresh process-local validation result as current-process-only and
      keyed by the resolved `semble` executable path, `SEMBLE_MODEL_NAME`,
      cache-relevant environment (`HF_HOME`, `XDG_CACHE_HOME`), and validation
      fixture content; invalidate it when any key input changes.
- [ ] If offline-readiness validation fails, degrade gracefully: omit Semble
      prompt guidance and surface a bounded operator-visible diagnostic.
- [ ] Run Semble commands for MAS guidance with `HF_HUB_OFFLINE=1` so agents do
      not implicitly trigger model downloads during unattended work.
- [ ] Prefer Semble over Morph MCP semantic/codebase search when Semble is
      enabled and offline-ready.
- [ ] Treat Morph MCP semantic/codebase search as the semantic fallback when
      Semble is disabled, unavailable, or not offline-ready and the current
      tool/MCP policy exposes Morph to the agent.
- [ ] Scope task-agent Semble guidance to the task worktree root, never the
      parent project root.
- [ ] Scope orchestrator Semble guidance to the project root only when that root
      does not cause task worktrees to be indexed as ordinary source content, or
      when Liza can guarantee `.worktrees/` exclusion.
- [ ] Ensure Semble searches exclude Liza runtime artifacts and generated index
      artifacts: `.liza/`, `.worktrees/`, `stacklit.json`, and `*.scip`.
- [ ] Automatically create a worktree-local `.sembleignore` file for
      Liza-managed task worktrees.
- [ ] Ensure generated worktree `.sembleignore` handling leaves task diffs clean
      unless the operator explicitly approves a project-tracked ignore file.
- [ ] Use the existing SCIP task-worktree private-exclude pattern as the
      precedent for hiding Liza-generated `.sembleignore` files from Git status,
      while keeping the physical `.sembleignore` visible to Semble in the walked
      tree.
- [ ] Document that `.sembleignore` is directory-scoped, not global.
- [ ] Inject prompt guidance that positions Semble as candidate discovery, not
      source-of-truth evidence.
- [ ] Preserve Stacklit and SCIP prompt guidance when those indexes are
      available; Semble must be additive, not a replacement.
- [ ] Preserve `rg` and `ast-grep` as the expected tools for exact text search
      and syntax-shaped search.
- [ ] Explicitly forbid `rg` as the fallback for broad-scope or common-word
      conceptual queries in Semble-enabled routing guidance.
- [ ] Support content-specific Semble guidance for code, docs, and config
      search.
- [x] Update Pairing SessionStart context to include Semble repo-root guidance
      alongside Stacklit and SCIP when Semble is enabled and safe.
- [ ] Keep MAS Semble context in spawned prompts, not SessionStart, so MAS agents
      receive task/reviewer/orchestrator target roots rather than repo-root
      assumptions.
- [ ] Add operator documentation explaining Semble's first-run Hugging Face model
      bootstrap behavior and the supported offline-ready workflow.
- [ ] Keep Semble failures non-blocking for MAS agent spawn.
- [ ] Ensure any Semble-generated or Semble-related files remain out of task
      diffs unless the operator explicitly requests configuration changes.

## Required Agent Prompt Contract

When Semble is enabled, validated, and safe for the current target root, agents
should receive concise prompt content equivalent to:

```text
=== SEMBLE SEARCH ===
Semble is available for semantic repository search in this target root:
<shell-quoted-absolute-target-root>

Use Semble for natural-language or conceptual discovery when you do not yet know
the exact symbol or module:

semble search "where is review submission validated?" <shell-quoted-absolute-target-root>
semble search "agent CLI defaults" <shell-quoted-absolute-target-root> --top-k 10
semble search "where is task superseding specified?" <shell-quoted-absolute-target-root> --content docs
semble search "default CLI config" <shell-quoted-absolute-target-root> --content config
semble find-related <file_path> <line> <shell-quoted-absolute-target-root>

Semble returns candidate chunks, not proof. Before editing or claiming behavior,
read the relevant source files directly and verify against current file content.
Do not use rg for broad-scope or common-word conceptual queries. Use rg for
exhaustive literal matches, ast-grep for syntax-shaped searches, Stacklit for
module/dependency orientation when available, scip-search for exact
symbol/reference/implementation tracing when available, and Morph MCP semantic
search only when Semble is unavailable and the current tool/MCP policy exposes
Morph to this agent.
```

For MAS prompts, Liza should include only the exact target root the agent should
search. Agents must not infer parent repository paths, sibling worktree paths,
or cache locations.

`<shell-quoted-absolute-target-root>` is role-specific: task agents receive the
task worktree root, reviewer agents receive the reviewer worktree root for the
submitted review candidate, and orchestrator agents receive the project root
only when project-root Semble search is safe.

When Stacklit and SCIP are also available, the unified query-routing prompt
should place Semble before Stacklit/SCIP only for conceptual discovery:

```text
For broad conceptual questions, start with Semble to find candidate chunks.
Use Semble --content docs for documentation/spec questions and --content config
for configuration questions.
Use Stacklit to understand module ownership and impact.
Use scip-search for exact symbols, references, and implementations.
Use Morph MCP semantic search only as a fallback when Semble is unavailable and
the current tool/MCP policy exposes Morph to this agent.
Verify with direct source reads before editing.
```

When Semble validation fails, omit the Semble section entirely. Do not inject a
prompt section that asks agents to troubleshoot Semble during unrelated work.

Content-mode guidance should stay to one line in prompts:

```text
Use --content with one of: code, docs, config, all; code is the default.
```

## Configuration Shape

The first milestone should follow Stacklit's optional-tool pattern: environment
activation only, with no durable `state.yaml` config field. This avoids a
premature nested `config.semble` shape. A durable config can be added later if
Liza needs project-specific Semble preferences.

Required semantics:

- Environment activation is strict opt-in. `LIZA_ENABLE_SEMBLE` must be truthy
  for MAS runtime behavior.
- No durable config field is required for the first milestone.
- Semble activation is global for MAS in the first milestone. Role-specific
  activation is out of scope unless operational use shows prompt bloat or role
  mismatch.
- Default indexed content should be code only, which aligns with Semble's
  built-in default when `--content` is omitted.
- Documentation and config content should be supported in the first integration
  milestone through explicit `--content docs` and `--content config` routing.
- `--content all` may be documented for human/operator use, but MAS prompt
  guidance should prefer targeted `code`, `docs`, or `config` content modes to
  avoid noisy corpora.
- Agents must not index `.liza/`, `.worktrees/`, generated indexes, or runtime
  logs as ordinary semantic corpus.
- Configurable content lists must preserve runtime-artifact exclusions.
- A future config may allow content preferences, an explicit model path, or a
  model name mapping to Semble's `SEMBLE_MODEL_NAME` behavior.

## Behavioral Decisions

- Semble is optional and external. Liza does not install it automatically.
- Semble is not a hard dependency for pairing or MAS.
- Pairing sessions may use the user's normal Semble installation without Liza
  runtime mediation.
- MAS sessions must not rely on Semble unless Liza has validated availability
  and offline readiness.
- `liza init --spec` is the controlled point where Liza may intentionally
  download or prewarm the Semble model, but only when `LIZA_ENABLE_SEMBLE` is
  truthy and the `semble` CLI is present.
- Init-time prewarm should use a temporary fixture directory containing one tiny
  supported code file rather than relying on Semble behavior for empty
  directories.
- After init-time prewarm, `HF_HUB_OFFLINE=1` should be set for Semble
  validation and MAS-facing Semble command guidance.
- A Semble offline validation failure means "omit Semble guidance," not "block
  the agent."
- A Semble search result is a candidate pointer. It does not satisfy Liza's
  source-validation requirements.
- Task agents search their task worktree root.
- Reviewer agents search the reviewer worktree root representing the submitted
  review candidate.
- Orchestrator agents search the project root only when runtime-artifact
  exclusions prevent sibling worktrees and Liza state from entering the search
  corpus.
- Liza must never ask an agent to search the parent project root from inside a
  task worktree as a substitute for the task worktree root.
- Liza should prefer local paths over remote Git URLs for Semble in MAS.
- Remote Git URL Semble indexing is out of the first milestone because remote
  cache validation does not verify branch movement the same way local path
  validation verifies current files.
- `semble init` writes agent integration files and must not be run
  automatically by Liza agents.
- `semble savings` is informational and not part of first-milestone MAS prompt
  guidance.
- Semble MCP integration is out of the first milestone. It may be reconsidered
  after MAS MCP restriction/allowlist behavior is resolved.
- `rg` is not an acceptable fallback for broad-scope or common-word conceptual
  queries when Semble is unavailable. Use Morph MCP semantic/codebase search
  only when the current tool/MCP policy exposes Morph to the agent; otherwise
  ask for narrower terms or use Stacklit/module maps before exact searches.

## SessionStart Requirements

Pairing sessions should learn about Semble through the same provider
SessionStart surface that already exposes repo-root Stacklit and SCIP context.

Implementation status: Pairing SessionStart support was implemented in
`8716af67 feat(semble): surface pairing session context`. The committed slice
adds `session-context.sh` Semble guidance, root `.sembleignore` safety gating,
offline fixture validation, MAS suppression, hook tests, and Claude permission
for `Bash(semble:*)`. This does not complete MAS prompt injection,
`liza init --spec` model prewarm, task-worktree `.sembleignore` generation, or
operator documentation.

Required outcomes:

- `session-context.sh` includes Semble Pairing guidance when
  `LIZA_ENABLE_SEMBLE` is truthy, the `semble` CLI is present, and offline
  readiness is established.
- Pairing SessionStart must not depend on an init-time validation marker;
  Pairing should work for repositories where `liza init --spec` has not been
  run.
- Pairing SessionStart may perform a bounded live offline readiness check using
  the same temporary one-file fixture as init validation, with
  `HF_HUB_OFFLINE=1` and without indexing the project root.
- Pairing SessionStart emits Semble guidance only when project-root search is
  safe. The shell hook must validate that root `.sembleignore` contains the
  required runtime, generated-index, and credential-file exclusion patterns
  before emitting root Semble guidance; file existence alone is not sufficient.
- SessionStart Semble guidance is repo-root scoped and shell-quotes the project
  root in every example command.
- SessionStart Semble guidance includes the one-line content-mode instruction:
  `Use --content with one of: code, docs, config, all; code is the default.`
- SessionStart must not trigger model downloads or expensive repository
  indexing. MAS model prewarm and offline validation happen during `liza init
  --spec` when Semble is enabled; Pairing-only SessionStart checks must stay
  offline and fixture-scoped.
- SessionStart Semble guidance must remain concise and should not include MCP
  setup, Semble troubleshooting, or `semble init` instructions.
- MAS agents must not receive repo-root Semble guidance through SessionStart.
  They receive Semble target-root guidance through normal spawned prompts, using
  the task worktree root, reviewer worktree root, or safe orchestrator project
  root as appropriate.
- The same project-root safety rule applies before injecting orchestrator
  project-root Semble guidance.

## Worktree Safety Requirements

Semble can be worktree-compatible, but only if scoped correctly.

- Local Semble cache keys are absolute-path based; distinct worktrees receive
  distinct local cache entries.
- Normal Semble search writes indexes to the OS cache directory, not the target
  worktree.
- Local cache invalidation compares file mtimes and file manifests.
- MCP mode can watch local paths and rebuild after file changes, but the first
  milestone does not require MCP mode.
- `.sembleignore` is loaded from each directory during traversal; placing
  `.worktrees/` in a task worktree's `.sembleignore` does not hide the task
  worktree itself.
- Indexing the parent project root while `.worktrees/` is not ignored can cause
  all task worktrees to be indexed as ordinary source files.
- Execution directory and explicit path arguments both matter. Liza should run
  or present Semble commands from the intended target context and pass the
  intended target root explicitly, so Semble discovers the relevant
  `.sembleignore` and does not fall back to the wrong current directory.
- Semble does not read Git private exclude files directly; Git excludes alone
  cannot replace a tree-visible `.sembleignore`.

Required outcomes:

- Liza-managed task worktrees receive Semble-visible ignore rules before agents
  are prompted to use Semble.
- Task worktree Semble searches do not include sibling task worktrees.
- Task worktree Semble searches do not include `.liza/` runtime state, prompts,
  outputs, alerts, SCIP indexes, or Stacklit indexes.
- Project-root Semble searches do not include `.worktrees/`, `.liza/`,
  `stacklit.json`, `*.scip`, or credential-file patterns.
- Liza creates a physical `.sembleignore` in each Liza-managed task worktree
  before Semble prompt guidance is injected.
- Liza hides the generated task-worktree `.sembleignore` from `git status` using
  the same private worktree exclude pattern established for SCIP indexes:
  resolve the task worktree gitdir with `git rev-parse --git-dir`, write the
  exclude entry to that worktree's `info/exclude`, enable
  `extensions.worktreeConfig`, and set the task worktree's `core.excludesFile`
  to the shared private exclude file when safe.
- Semble and SCIP exclusion handling must share the same task-worktree private
  exclude file instead of competing over `core.excludesFile`.
- Semble and SCIP task-worktree exclude setup must be serialized or use a
  shared helper/lock so concurrent lifecycle hooks cannot corrupt
  `info/exclude` or race while setting `core.excludesFile`.
- Generated Semble-related ignore/config files leave task
  `git status --porcelain` clean unless they are explicit user-approved project
  config.
- Concurrent task worktrees can use Semble without cache-path collision.

Default generated task-worktree `.sembleignore` content:

```gitignore
.liza/
.worktrees/
stacklit.json
*.scip
.env
.env.*
*.env
credentials.*
secrets.*
*secret*.*
*.pem
*.key
*.p12
*.pfx
*.jks
*_rsa
*_dsa
*_ecdsa
*_ed25519
*.keystore
*.truststore
config/secrets/
**/secrets/
serviceAccountKey.json
*-credentials.json
```

`stacklit.json` is a root-level generated file and needs an explicit
`.sembleignore` entry unless already ignored by the project. SCIP files normally
live under `.liza/scip/`; excluding `.liza/` covers them, and `*.scip` is a
defensive fallback for projects with nonstandard index locations.

## Offline and Network Requirements

Semble's runtime promise is local after model availability is satisfied, but its
default model acquisition path may contact Hugging Face.

Required outcomes:

- MAS Semble integration must not silently download models during unattended
  task execution.
- `liza init --spec` may intentionally download or prewarm the Semble model when
  `LIZA_ENABLE_SEMBLE` is truthy and `semble` is present.
- Init prewarm command contract:
  - Create a temporary directory outside the project/worktree.
  - Create the temporary directory under `$TMPDIR` / `os.TempDir()` so abnormal
    process death leaves cleanup to normal OS temporary-file handling.
  - Write one tiny supported code file, for example `prewarm.py` containing
    `def liza_semble_prewarm(): pass`.
  - Run `semble search "__liza_semble_prewarm__" <temp-dir> --top-k 1 --content code`
    with normal network access inherited from the operator environment.
  - Treat exit code 0 as prewarm success even if the result payload reports no
    search hits.
  - Capture stdout/stderr with bounded diagnostics and delete the temporary
    directory after the command completes.
- Offline validation command contract:
  - Recreate the same temporary fixture directory.
  - Run `HF_HUB_OFFLINE=1 semble search "__liza_semble_prewarm__" <temp-dir> --top-k 1 --content code`.
  - Treat exit code 0 as offline-ready.
  - Treat command-not-found as Semble unavailable.
  - Treat model/cache-related failure as model unavailable offline.
  - Treat any other nonzero exit as Semble execution failure.
  - Capture stdout/stderr with bounded diagnostics and delete the temporary
    directory after the command completes.
- Both prewarm and offline validation should use a named implementation
  constant with a default timeout of 30 seconds, documented in operator-facing
  configuration docs.
- Liza documentation must explain that enabling Semble at init may contact
  Hugging Face to populate the Semble/model2vec/Hugging Face cache.
- Liza validation must distinguish "Semble executable missing" from "Semble
  model not available offline."
- Liza should surface bounded diagnostics for offline-readiness failures, for
  example: `semble: model unavailable offline; prewarm Semble or disable
  LIZA_ENABLE_SEMBLE`.
- Agents should not be prompted to resolve Hugging Face authentication or model
  cache issues during normal task work.
- If an operator configures a local model path through `SEMBLE_MODEL_NAME`, Liza
  should preserve that environment when validating and when showing Semble
  command guidance.

## Query Routing Requirements

Semble should be positioned by question type, not by enthusiasm for semantic
search.

| Question type | Preferred tool |
|---|---|
| "Where is this behavior implemented?" with no symbol known | Semble |
| "Where is this behavior specified?" | Semble with `--content docs` |
| "Where is this config/default defined?" | Semble with `--content config`, then direct read |
| "What module owns this domain?" | Stacklit |
| "Where is symbol `Foo` defined?" | `scip-search` when indexed, otherwise `rg` |
| "Who references this exact symbol?" | `scip-search` when indexed |
| "Find this literal string, config key, or exact error message" | `rg` |
| "Find calls shaped like this AST pattern" | `ast-grep` |
| "Broad semantic search when Semble is unavailable" | Morph MCP semantic/codebase search, if the current tool/MCP policy exposes Morph |
| "What does the code actually do?" | Direct source reads |

Semble guidance should explicitly discourage using semantic results as the final
answer when a source read is required.

`rg` should not be routed as the fallback for broad-scope or common-word
queries. If Semble is unavailable and Morph MCP semantic search is unavailable
or not exposed by the current tool policy, agents should narrow the query first
through Stacklit/module context or ask for better search terms instead of
spraying common words through `rg`.

## Documentation Requirements

- Update `README.md` recommended tools table if Semble is adopted.
- Update `support-docs/CONFIGURATION.md` with Semble activation semantics,
  offline-readiness behavior, and explicit non-goals.
- Update `support-docs/CUSTOMIZING_AGENT_TOOLS.md` with Semble's position in
  the worktree-safe tool stack.
- Document `.sembleignore` behavior and why root scoping matters for
  `.worktrees/`.
- Document that `semble init` is not part of Liza-managed MAS setup.
- Document that Semble complements Stacklit and SCIP rather than replacing them.
- Document that Morph MCP semantic/codebase search is the fallback for broad
  semantic search when Semble is unavailable and Morph is exposed by the current
  tool/MCP policy.
- Document that `rg` remains for exact/literal search, not broad conceptual
  discovery.
- Document that Pairing SessionStart may surface Semble repo-root guidance when
  enabled, matching the Stacklit/SCIP SessionStart model.

## Security and Safety Requirements

- Liza-generated Semble ignore rules must exclude Liza runtime artifacts,
  generated index artifacts, and credential-file patterns listed in the default
  `.sembleignore` block in this spec.
- Documentation should warn operators that Semble indexes chunks from files
  matching its content selection; projects with sensitive source-adjacent files
  should add project-specific ignores beyond Liza's default block before
  enabling it.
- Prompt guidance must not encourage remote Git URL searches from unattended
  agents.
- Semble failure diagnostics must be bounded and must not dump file contents or
  secrets.
- Any Liza-managed Semble invocation should use explicit path arguments, not
  implicit current-directory assumptions, when producing prompt guidance or
  validation output.

## Success Criteria

1. Operators can enable Semble with a strict opt-in gate. Pairing behavior
   changes only when `LIZA_ENABLE_SEMBLE` is truthy.
2. `liza init --spec` intentionally prewarms/downloads the Semble model when
   Semble is enabled and the CLI is present.
3. MAS prompts include Semble guidance only when Semble is installed and usable
   offline.
4. MAS prompts omit Semble guidance when Semble is disabled, unavailable, or not
   offline-ready.
5. Task agents are guided to search only their task worktree root.
6. Project-root searches do not index `.worktrees/`, `.liza/`, `stacklit.json`,
   `*.scip`, or credential-file patterns.
7. Worktree-local Semble ignore rules are present before agents are prompted to
   use Semble.
8. Semble guidance clearly describes semantic results as candidate discovery,
   not source-of-truth evidence.
9. Stacklit and SCIP remain first-class tools with distinct routing guidance.
10. Morph MCP semantic/codebase search is documented as the semantic fallback
    only when Morph is exposed by the current tool/MCP policy.
11. `rg` is not routed as the fallback for broad/common-word conceptual queries.
12. Pairing SessionStart emits Semble repo-root guidance when Semble is enabled
    and offline-ready, alongside Stacklit and SCIP context.
13. MAS agents receive Semble guidance through spawned prompts, not repo-root
    SessionStart guidance.
14. Semble failures do not block agent spawn.
15. Documentation explains init-time model bootstrap and the `HF_HUB_OFFLINE=1`
   operating mode.
16. Worktree-heavy MAS runs remain safe when multiple task worktrees exist.

## Non-Goals

- Vendoring Semble into the Liza repository.
- Implementing a semantic search engine inside Liza.
- Replacing Stacklit, SCIP, `rg`, `ast-grep`, or direct source reads.
- Automatically installing Semble, Python, `uv`, or Semble optional
  dependencies.
- Downloading Semble models outside the controlled `liza init --spec` lifecycle
  point for MAS.
- Automatically running `semble init`.
- Making Semble required for MAS.
- Using Semble remote Git URL indexing as a first-milestone MAS workflow.
- Using Semble MCP integration as a first-milestone MAS workflow.
- Guaranteeing semantic search result relevance.
- Treating Semble output as validation evidence without source reads.
- Designing a general MCP server policy; see the separate MCP restriction goal.

## Open Questions

No open questions remain for the first milestone. Earlier questions were
resolved as follows:

- Semble activation is global for MAS in the first milestone.
- Pairing mode receives Semble guidance through SessionStart when enabled and
  safe, matching Stacklit and SCIP.
- Offline preflight belongs inside `liza init --spec`; no new Liza command is
  required for the first milestone.
- Prompt content-mode guidance is a one-liner:
  `Use --content with one of: code, docs, config, all; code is the default.`

## Risks

- **Network drift:** Agents could trigger model downloads if validation and
  prompt guidance do not enforce offline mode.
- **Worktree contamination:** Searching the parent project root can index
  sibling task worktrees if `.worktrees/` is not excluded.
- **Tool overreach:** Agents may over-trust semantic results and skip source
  verification unless the prompt contract is explicit.
- **Prompt bloat:** Adding another navigation tool can confuse routing unless
  guidance is terse and question-type based.
- **Operational variance:** Semble depends on Python packaging, model cache, and
  operator-local Hugging Face/model2vec cache state that may vary across
  machines.

## Recommended Implementation Slices

1. **Documentation-only adoption**
   - Add Semble to operator-facing tool docs as optional.
   - Document offline prewarm, `.sembleignore`, and routing boundaries.

2. **Prompt guidance behind activation gate**
   - Add `LIZA_ENABLE_SEMBLE`.
   - Prewarm/download the model during `liza init --spec` when enabled.
   - Validate executable and offline readiness before MAS prompt injection.
   - Inject Semble prompt guidance only when validation succeeds.

3. **Worktree safety hardening**
   - Generate task-worktree `.sembleignore` with the default exclusion block.
   - Hide generated `.sembleignore` with the SCIP-style private worktree exclude
     mechanism while keeping it visible to Semble.
   - Add tests proving task prompts use task worktree roots and do not mention
     parent roots.

4. **Optional MCP integration**
   - Out of the first milestone.
   - Consider only after MAS MCP restriction/allowlist behavior is resolved.
   - Keep CLI guidance as the stable default.
