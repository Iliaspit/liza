# Liza v0.8.0

Smoother autonomous runs, lower context pressure, easier operation.

Agents plan better through architecture sub-pipelines and master
planning gates, drift less through hardened prompts and await-verdict
discipline, and recover from blocks through explicit repair paths
instead of stalling silently. Context pressure is lower through refined
RTK command guidance, SessionStart hooks, and optional search tools
(Stacklit, SCIP, Semble) that reduce broad exploratory reads. Setup is
simpler with pre-commit bootstrap and ecosystem tool pre-config,
recovery is explicit through lifecycle repair commands, and adversarial
pairing adds lightweight terminal-based multi-agent workflows without the
full autonomous runtime.

294 commits since v0.7.0 (2026-04-13).

---

## Highlights

**Optional repository indexing and semantic discovery** -- Stacklit and
SCIP provide low-token module orientation and precise symbol tracing, and
Semble adds local semantic search for conceptual discovery before agents
know the right file, symbol, or keyword. All three are opt-in, advertised
only through explicit paths or roots, and remain candidate-routing tools
until verified by source reads.

**SessionStart context for Pairing** -- Provider startup hooks now emit
the initialization files, missing-file filtering, and repo-root index
paths needed before the agent's first substantive action. Pairing sessions
can use repo-root Stacklit, SCIP, and Semble guidance when enabled, while
multi-agent worktrees keep prompt-supplied task-local metadata.

**Agent prompt hardening** -- Agent prompts now constrain validation,
worktree commands, artifact references, polling loops, and context
scoping more precisely. This reduces drift before it reaches lifecycle
repair: agents are pushed toward executable evidence, bounded task graph
context, safer worktree behavior, and clearer stop conditions.

**Repairable lifecycle drift** -- Review boundary metadata, stale direct
dependencies, blocked-task rebases, ghost ownership, and invalid artifact
references now have narrower detection or repair paths. Liza preserves
review and integration invariants while giving operators explicit commands
such as `update-review-commit`, `retarget-dependency`, and claimable
`unblock-task` recovery.

**Planning coherence gates** -- Architecture now has its own sub-pipeline,
entry points distinguish broad goals, functional specs, and technical
specs, and the master planning task pattern adds a reviewed decomposition
gate before high-risk planning fan-out.

**Adversarial pairing blackboard** -- Pairing sessions can now coordinate
separate doer and reviewer terminals through a lightweight Markdown
blackboard. This gives users a simple terminal-based multi-agent workflow
that remains compatible with subscription-based billing across provider
CLIs, without requiring the full autonomous Liza multi-agent runtime.

**Provider and worktree readiness** -- Liza now handles more of the setup
needed before agents can work reliably: role-specific default CLIs, Codex
project hooks and sandbox permissions, Claude ecosystem-tool permissions,
monorepo-aware setup suggestions, pre-commit/worktree hook composition,
post-submit commit prevention, and opt-in ignored env-file provisioning.

---

## Breaking Changes

| Change | Impact |
|--------|--------|
| Drop native Windows support | Liza no longer builds or tests on Windows; macOS and Linux remain supported |

---

## Features

| Feature | Description |
|---------|-------------|
| Optional SCIP indexing | Generates explicit `.scip` indexes for supported worktrees and prompts agents to use `scip-search --index <path>` only for successful indexes |
| Optional Stacklit indexing | Refreshes `stacklit.json` for compact module/dependency orientation and injects explicit Stacklit paths into prompts and Pairing context |
| Semble semantic search | Adds opt-in local semantic discovery with offline readiness checks, `.sembleignore` safety, and source-read verification discipline |
| SessionStart context hooks | Emits initialization guidance, filtered project files, and Pairing repo-root index metadata before first substantive agent action |
| Setup/init split for index activation | Keeps global `AGENT_TOOLS.md` generic while `liza init` owns project-local hooks, index paths, Semble safety, and activation checks |
| Worktree env-file provisioning | Copies configured ignored project-root env files into task worktrees without logging or interpreting their contents |
| Architecture sub-pipeline | Separates architecture consolidation from coding and adds clearer `general-objective`, `functional-spec`, and `technical-spec` entry paths |
| Master planning task pattern | Adds quorum-reviewed decomposition-root tasks before uncertain or fan-out-heavy planning work |
| Declared validation commands | Lets task and output contracts carry ordered validation commands rendered to doers and reviewers |
| Adversarial pairing blackboard | Adds a lightweight Markdown blackboard for coordinated doer/reviewer terminals outside full multi-agent mode |
| Active task cancellation | Allows invariant-checked cancellation of executing, submitted, and reviewing tasks before approval |
| Automatic checkpoint summaries | Generates `.liza/checkpoint-summary.md` after successful merges as best-effort steering context |
| Pre-commit bootstrap and worktree hooks | Auto-detects project pre-commit configuration, deduplicates hooks by task kind, and chains project hooks through per-worktree guard hooks to prevent post-submit commits |
| Task-slug readable IDs | Adds configurable pipeline task-slug segments so generated child task IDs use readable transition names |
| Review commit repair command | Adds `update-review-commit` for repairing stale review boundary metadata after post-rebase recovery |
| Role-specific default CLI | Lets operators configure different provider CLIs per agent role through `LIZA_DEFAULT_DOER_CLI` and `LIZA_DEFAULT_REVIEWER_CLI` |
| Monorepo layout detection | Detects `package.json` in subdirectories during init for monorepo project structures |
| Zombie supervisor detection | Identifies dead supervisor processes that would otherwise block agent pool recovery |
| Context engineering audit skill | Adds a skill for analyzing agent prompts and outputs for context quality, bloat, and handoff fit |
| Pre-configured ecosystem tools | Pre-configures Python, Rust, and Node.js ecosystem tool permissions in the Claude settings template written during `liza init` |
| EpicRef and PlanRef separation | Distinguishes epic-level references from coding plan references in the pipeline task model |
| Codex project hook setup | Configures Codex-specific project hooks and sandbox permissions during init |
| Active task summary view | Adds compact summary output for `liza get` showing active tasks per agent |

---

## Fixes

| Fix | Impact |
|-----|--------|
| Kernel `flock` as lock authority | Avoids unsafe stale-lock decisions based on misleading PID metadata |
| Agent execution progress watchdog | Detects assigned tasks that stop making observable progress and routes them through blocked recovery |
| Ghost agent claim prevention | Rejects corrupt agent identities at claim time and validates task/agent ownership drift |
| Blocked task alerts and re-wake | Makes blocked work visible through alerts, diagnostics, watch output, and unblock guidance |
| Candidate artifact ref guard | Rejects merge candidates that would remove, replace, or invalidate active artifact references before integration advances |
| Dependency edge canonicalization | Rewrites or rejects stale dependency edges at mutation and transition time instead of relying on recursive read-time interpretation |
| Retarget dependency repair | Adds a guarded operation for replacing a stale direct dependency without superseding otherwise-valid work |
| Claimable rebase unblock | Lets repaired blocked tasks rebase preserved worktrees and return to normal scheduler claimability |
| Repairable review boundaries | Keeps reviewers pinned to recorded `base_commit..review_commit` while allowing stale metadata repair through `update-review-commit` |
| Review verdict write failures | Surfaces submit-verdict write failures and fails closed when reviewer release cannot be resolved |
| Passive reviewer ownership release | Releases stale passive reviewer ownership so review flow can progress |
| Fresh resubmission baseline | Returns updated review baseline data after resubmission repair paths |
| Prompt graph compaction | Caps and deduplicates task graph prompt digests to reduce context growth |
| Planning role scope guard | Prevents planning roles from editing downstream implementation scope |
| Provider CLI identity export | Exports resolved agent IDs to provider CLIs for consistent agent-runtime behavior |
| Codex sandbox configuration | Aligns embedded Codex workspace permissions and headless sandbox overrides |
| Claude temporary writes | Allows Claude runtime writes to `/tmp` where required by provider behavior |
| RTK env syntax | Documents and supports `rtk env VAR=value ...` command routing |
| Await-verdict polling discipline | Stops agents from polling after terminal verdicts, reassignment, or backgrounded resubmission waits |
| Agent prompt hardening | Tightens validation evidence requirements, worktree command safety, artifact ref resolution, and context scoping across doer and reviewer prompts |
| Init edge-case hardening | Requires pre-commit config and committed specs before init, fails fast on unborn HEAD, recovers from empty placeholder files, and reports clear integration branch errors |
| Integration failure diagnostics | Records failure diagnostics, persists safe integration metadata, and recovers stale failed tasks |
| State validation hardening | Rejects duplicate task IDs, enforces active review ownership, and repairs invalid doer and reviewer ownership at validation time |
| Review submission correctness | Rebases submissions onto resolved integration commits, splits code review diff boundaries, and classifies operational submission failures |
| Planning handoff repair | Unblocks partial planning handoffs, blocks outbound transitions on replanned tasks, and routes orchestrator-only repairs |
| Blackboard YAML preservation | Preserves valid YAML block scalar indentation during state mutations |
| Bash 3 SessionStart compatibility | Keeps SessionStart context hook compatible with macOS default bash 3 |
| Agent lifecycle resilience | Resumes owned executing tasks, retries approved integration fixes, stops processes before state deletion, and surfaces provider degradation |

---

## Documentation

- ADR-0058 through ADR-0068 and ADR-0070 through ADR-0082 backfill the
  decisions behind task dependencies, planning handoff, watchdog recovery,
  locking, ownership, alerts, review repair, repository indexing, pairing
  blackboards, validation commands, Semble, and worktree env-file
  provisioning.
- `AGENT_TOOLS.md` centralizes optional Stacklit, SCIP, Semble, `rg`,
  `ast-grep`, and direct-read routing guidance.
- Support docs add guidance for sandbox process-state confusion, lifecycle
  churn reports, and optional indexing activation.
- Adversarial-pairing docs cover coordination invariants, pre-coding
  context hygiene, multi-reviewer usage, blackboard worktree ordering,
  and Codex sandbox workarounds.
- Code review skill adds re-review stop criteria.
- Multi-agent docs clarify setup commands, tool configuration, and git
  boundaries.
- README reorganizes onboarding flow and contributing scope boundaries.
- Architecture docs add BMAD and gstack competitive surveys.
- Pre-commit bootstrap and hook-composition specifications added under
  `specs/goals/`.
- Installable support docs restructured into separate files and expanded
  with pipeline-agnostic troubleshooting and replan/exhaustion workflows.
- Release and repository docs are updated for current GoReleaser behavior,
  public docs layout, and setup/configuration surfaces.

---

## Installation

**Quick install (macOS/Linux):**
```bash
curl -fsSL https://raw.githubusercontent.com/liza-mas/liza/main/install.sh | bash
```

**From source:**
```bash
make install
```

---

## What's Next

- Tighten release automation around generated release notes and tag-time
  verification.
- Continue reducing prompt/context load now that indexing and semantic
  discovery have first-class activation paths.
- Expand repair-oriented lifecycle commands where operators still need
  manual state edits.
