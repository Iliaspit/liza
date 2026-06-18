---
date: 2026-06-15
perspective: Authored in Liza's own repo, but redone as a deliberately fair, fact-grounded comparison. Each contested theme was argued by an adversarial pro-Liza advocate and an adversarial pro-Gastown advocate, then reconciled by a neutral arbitrator instructed to reward genuine advantages, drop overstatements from both sides, and converge. Where a theme genuinely favors one system, it says so; where it is a trade-off, it says that. Gastown facts are from a local clone at v1.2.1 (HEAD aead4d7e, 2026-06-15); Liza facts are from the local repository.
method: Adversarial debate + arbitration over 7 contested themes (21 agents). Facts verified against both repositories' source, not docs alone.
---

# Liza vs Gastown Comparison

## Source Snapshot

- **Liza**: local repository HEAD on `main`; release tag `v0.8.0`. Go, ~61k non-test LOC + ~144k lines of tests (~2.4:1 test-to-code). Apache 2.0. Single primary author (plus agent-generated commits via Liza's own MAS mode). Created January 2026. Stack- and provider-agnostic by design.
- **Gastown**: `gastownhall/gastown`, local clone at `v1.2.1` (2026-06-06; HEAD `aead4d7e`). Go, ~447k LOC across ~1,181 files (~46% tests). MIT License, © Steve Yegge. Created December 2025. Essentially single-author as well — Steve Yegge dominates the history (with heavy agent-generated commit identities), plus one substantial external contributor. Multi-provider by design.

This is the closest architectural comparator in the folder. Gastown and Liza are both Go CLIs that orchestrate multi-agent coding workflows through structured state, isolated worktrees, and role-based agent hierarchies. They were created within a month of each other, solve overlapping problems, and converged on many of the same structural choices. **Both are essentially solo-author projects, and both expose their workflow as runtime configuration rather than hard-coded code** — so the interesting question is not "which is configurable" (both are) but where their design philosophies genuinely diverge on trust, state, scale, and the boundary between code-enforced and agent-delegated decisions.

A note on a correction from earlier versions of this document: Gastown's **ZFC** principle is **"Zero Framework Cognition"** ("Agent decides; Go transports"), *not* "Zero Decisions in Code." The distinction matters — ZFC governs where *reasoning* lives (the framework layer does no judging; agents interpret), and Gastown's mechanical merge gates are still deterministic Go. The earlier framing of Gastown as a system that pushes *all* decisions out of code overstated the principle and is corrected throughout.

---

## 1. Identity & Positioning

**Liza** — "Hardened Multi-Agent Coding System." A standalone Go CLI that supervises long-lived role agents running concurrently on isolated git worktrees, coordinated through a file-backed YAML blackboard. Liza wraps agents in control machinery and treats LLMs as unreliable components to be mechanically constrained where it matters. The behavioral contract — 55 catalogued failure modes (14 mapped to the MAST taxonomy) with tiered countermeasures — and the binding adversarial review gate are its defining features. Provider-agnostic: Claude Code, Codex, Gemini, Kimi, Mistral.

**Gastown** — A multi-agent workspace manager built around the metaphor of a "Town" containing "Rigs" (projects). A Go CLI that orchestrates AI coding agents through a Dolt SQL database (Git-semantics SQL), tmux sessions, and a mail system. Gastown's design principle **ZFC (Zero Framework Cognition)** confines the Go layer to transport and detection — "Agent decides; Go transports" — and routes *judgment* (including supervisory judgment) to LLM agents. Built for higher concurrency (~20-30 agents), cross-rig orchestration, and a federation roadmap. Provider-agnostic across ten CLIs via tiered integration.

The positioning gap is real but narrower than a slogan war suggests. Liza spends its determinism on an enforcement *floor* (state-machine transitions, forbidden operations, a binding merge gate) and spends LLM judgment on code review. Gastown spends its determinism on mechanical merge gates and on transport, and spends LLM judgment on fleet supervision. Both use code where code is reliable and models where judgment is irreducible — they disagree about *which* problems fall in which bucket.

---

## 2. Core Philosophy

**Liza** starts from the premise that LLM agents are unreliable by default, and that the cheapest place to contain that unreliability is a compiled enforcement floor. The behavioral contract was developed incrementally as countermeasures to observed misbehavior — agents altering tests to pass, fabricating completions, silently drifting from scope. The contract is an *additive* quality layer; the load-bearing safety comes from Go: the supervisor validates every state transition, and nothing merges without an approved, commit-SHA-verified reviewer verdict. Remove the contract and the mechanical floor still holds.

**Gastown** organizes around three stated core principles (per its glossary):

- **MEOW (Molecular Expression of Work)**: decompose goals into atomic, agent-executable units (beads/molecules).
- **GUPP (Gas Town Universal Propulsion Principle)**: "If there is work on your hook, you must run it." Liveness over caution.
- **NDI (Nondeterministic Idempotence)**: accept that individual agent runs fail; design for useful outcomes through orchestration. Persistent beads and oversight agents guarantee *eventual completion of work*, not absence of defects.

**ZFC (Zero Framework Cognition)** is a design principle layered under these: the Go layer transports content and detects observable facts (session alive? heartbeat stale?), and agents do the interpreting. Notably, Gastown reaches for mechanical thresholds *first* in degraded mode (when tmux is unavailable, supervision falls back to "session dead → restart"), with LLM judgment as the enhanced default — the reverse of "models everywhere, code nowhere."

The honest epistemic difference: Liza treats agent unreliability as something to *prevent at the gate* with code; Gastown treats it as something to *detect and recover from* through orchestration. Liza's guarantee is "an unapproved or different-than-reviewed change does not become durable shared state." Gastown's guarantee is "work eventually completes despite failing runs." These are different guarantees, and neither subsumes the other — NDI's eventual-completion does not cover a latent defect that merged green.

---

## 3. State Substrate — Where the Plan Lives

This is a genuine "for whom" divergence, decided by operating point.

**Liza** — State lives in `.liza/state.yaml`, a file-backed YAML blackboard serialized through file locks. The write path is crash-safe despite being "just a file": flock + temp-file + fsync + atomic rename + atomic read-modify-write. Pipeline configuration lives in `pipeline.yaml`. Runtime dependencies are pure-Go (flock, yaml.v3, cobra, bubbletea) — no SQL driver, no daemon, no listening port, no external binary. State is one human-readable file you can `cat`/`grep`/`diff`/hand-edit.

**Gastown** — State lives in a **Dolt SQL database** (SQL with Git-like versioning: branch/merge/diff). A `dolt sql-server` process runs per town on port 3307; every write is wrapped `BEGIN`/`DOLT_COMMIT`/`COMMIT`; each rig gets its own database (canonically named, e.g. `mobile_apps`). Work is tracked as "Beads" — and **Beads/`bd` is a separate dependency** (`github.com/steveyegge/beads`), not part of Gastown itself. All agents currently write the `main` branch directly; the branch/merge capability is real but underexploited today, so the live advantage is durable per-row transactional history and SQL queryability rather than branching.

| | Liza | Gastown |
|:--|:--|:--|
| State format | YAML file | SQL database (Dolt) |
| Concurrency control | Single global file lock | Row-level SQL transactions |
| Operational footprint | Zero (no daemon, no port, no external binary) | dolt sql-server (port 3307) + external `bd` binary |
| Versioning/history | None built-in (`.liza` is committable but not auto-committed per transition) | Durable per-row history (every write is a `DOLT_COMMIT`) |
| Query model | Load YAML into a Go struct, walk in-process | SQL queries, cross-rig aggregation |
| Human inspection | Open the file | MySQL client or CLI wrappers |
| Scaling shape | Whole-file rewrite under one lock — fine at a handful of agents | Independent row commits — scales to 20-30 writers |

**Verdict — trade-off, persona-decided.** For Liza's design center (one human, a handful of agents, one project), the YAML blackboard's zero-ops and inspectability are a real, verified advantage across *every* sprint, not a beginner's convenience. For Gastown's target (~20-30 concurrent agents plus a learning loop that needs queryable durable history), Dolt is the correct and arguably necessary choice — a single global lock + whole-file rewrite is structurally wrong at that concurrency. Each substrate would be a poor fit for the other's scale. One caveat the file approach pays: Liza had to ship a bespoke regex repair pass for LLM-authored YAML indentation — a fragility class a typed SQL schema does not have.

---

## 4. Agent Architecture & Roles

**Liza** — 13 roles across 4 pipeline phases (Specification, Architecture, Coding, Integration), but they are one conceptual machine — a doer+reviewer adversarial pair — instanced across phases, declared in `pipeline.yaml` plus a small `roles.go`. Adding a role is a config edit. Agents are long-lived concurrent processes on isolated worktrees, leased and heartbeat-monitored. A handful per sprint, each durable. Supervision and recovery are centralized in the Go binary (lease expiry, circuit breaker, `liza recover-agent`).

**Gastown** — A deeper hierarchy of heterogeneous, long-lived daemons, each owning a distinct failure domain:

- **Town-level**: Mayor (global coordinator), Deacon (daemon supervisor / health / patrol), **Boot** (ephemeral watchdog that triages whether the Deacon itself is stuck), Dogs (maintenance plugins under the Deacon — e.g. `stuck-agent-dog`, `quota_dog`, compactor).
- **Per-rig**: Witness (polecat health, zombie classification, recovery), Refinery (Bors-style batch-then-bisect merge queue; polecats never push to main).
- **Workers**: Polecats (ephemeral sessions with persistent identity via an agent bead + CV chain), Crew (human workspaces — full git clones).

Watchdog chain: Daemon → Boot → Deacon → Witness/Refinery.

| | Liza | Gastown |
|:--|:--|:--|
| Worker model | Long-lived, leased, heartbeat-monitored | Persistent identity, ephemeral sessions |
| Supervision judgment | Go binary (deterministic) | LLM agents (Boot/Deacon/Witness/Dogs), mechanism as fallback |
| Scale target | Handful per sprint | 20-30 concurrent agents |
| Session management | Terminal sessions, manual or TUI | tmux-managed, automatic lifecycle |
| Worker isolation | Git worktrees (structural) | Git worktrees (structural) |

**Verdict — trade-off, with a scoped Liza win on simplicity and a scoped Gastown win on scale-proportionality.** Much of Gastown's role count tracks genuine failure-domain count at its scale: merge serialization across many parallel writers (Refinery), per-rig health classification (Witness), and a three-way messaging split are distinct problems a single process would have to re-grow internally. That part is *earned*, not accidental. But one slice is fairly called self-inflicted: the **supervision recursion** — Boot exists to watch the watchdog because the LLM Deacon can itself wedge, and `stuck-agent-dog` is itself an LLM reading tmux panes. A deterministic supervisor does not need a watchdog for its watchdog. Liza is genuinely simpler in *processes* and *authority model* (one binary, one lock-guarded state object, one place transitions are validated), but that simplicity is partly bought by targeting less: a handful of agents, no cross-rig federation.

(Precision note: in Gastown, "zombie" is a *derived classification* the Witness assigns, not an agent state; the agent-state enum is Running/Idle/Done/Stuck/Escalated/Spawning/Working/Nuked.)

---

## 5. Task Decomposition — From Goal to Work Units

**Important framing correction.** Earlier versions treated Gastown's `mol-idea-to-plan` formula as if it *were* "Gastown's decomposition approach." It is not — it is **one of 48 shipping formulas**, an example of what the formula system *can* express, not a built-in or default pipeline. A user can define an entirely different decomposition as a formula. The comparison below is therefore between Liza's configured default pipeline and one representative Gastown formula, not between two hard-coded designs.

**Liza** — Adversarially reviewed decomposition at every level. The Orchestrator reads a goal and creates planning tasks; each doer (Epic Planner, US Writer, Architect, Code Planner) writes `output[]` subtask definitions that the paired reviewer validates *before* the supervisor mechanically fans out downstream tasks. A bad decomposition is rejected before it propagates. For complex goals, the Orchestrator chains multiple planning tasks with `depends_on`. Because every level is adversarially reviewed with a binding verdict, the plan is challenged before any code is written.

**Gastown (the `mol-idea-to-plan` formula, as an example)** — A heavy parallel-swarm decomposition: intake → draft PRD → six parallel PRD reviewers → human clarification gate → six parallel plan designers → three PRD-alignment rounds → three plan self-review rounds → create beads → three bead-verification passes. Potentially 20+ polecat invocations for one decomposition.

| | Liza (default pipeline) | Gastown (`mol-idea-to-plan` example) |
|:--|:--|:--|
| Decomposition author | Doer/reviewer pairs per phase | Crew worker + parallel polecat swarms |
| Review of decomposition | Adversarial (reviewer validates before fan-out) | Self-review / alignment rounds (polecats reviewing polecats) |
| Review binding? | Yes (verdict blocks fan-out) | No (rounds are iterative/corrective, not gating) |
| Human gate | Between sprints | After PRD review (clarification questions) |
| Customizable? | Yes (pipeline config) | Yes (it's just a formula — rewrite it freely) |

**Verdict — trade-off.** Liza's decomposition has binding verdicts at each level (a flawed epic plan is rejected before user stories are written). Gastown's example formula throws more agents at the problem with corrective (not gating) review — a decomposition that survives six reviewers and three alignment rounds is likely good, but nothing mechanically blocks a flawed one from producing beads. The cost shapes differ: Gastown spends heavily in one decomposition burst; Liza spreads adversarial passes across the sprint lifecycle. Crucially, *both are user-editable* — Gastown via formula authoring, Liza via pipeline config.

---

## 6. Trust & Behavioral Control

**Liza** — A behavioral contract addresses 55 catalogued LLM failure modes (14 mapped to the MAST taxonomy; the rest Liza's own categories) — sycophancy, phantom fixes, scope creep, test corruption, hallucinated completions. A tiered rule system: Tier 0 invariants are never violated (no unapproved state change, no fabrication, no test corruption, no unvalidated success), enforced by the contract's halt/reset semantics. The Go supervisor mechanically enforces the merge/state floor — task state machine, approval-gated merges, commit-SHA verification. Trust is *suppression of known failure modes through both code-backed gates and behavioral-contract obligations*, with the contract as an additive layer that makes agents more thoughtful.

**Gastown** — Behavioral governance is a hybrid of prompt-level contracts and infrastructure verification: role contracts in markdown templates (e.g. polecats must not push directly to main), per-formula "Failure Modes" sections, explicit anti-hallucination instruction in templates, and GUPP-violation detection by the Deacon. Enforcement of the *hard* rule (no direct push to main) is structural — the Refinery owns merges. Judgment-level quality, however, is established by contract and verified by infrastructure rather than gated by a binding LLM verdict.

The trust boundary sits in different places:

- **Liza**: split enforcement — the Go supervisor validates transitions, blocks forbidden merge/state operations, and nothing merges without an independent reviewer's binding verdict; the Tier-0 behavioral rules are enforced by contract halt/reset semantics. The agent cannot bypass the merge gate because the gate is code.
- **Gastown**: structural where it's mechanical (push restriction, test/lint/build gates that block), but its LLM/adversarial *quality* review is **`judgment_enabled=false` by default and explicitly "measurement-only (Phase 1) — reviews are recorded but do NOT gate merges."** The blocking gates are mechanical, plus an optional human GitHub PR-approval gate (`require_review`, default off, PR-mode only).

**Verdict — a real Liza advantage on the narrow question, a deliberate Gastown choice.** For the class of defects that pass a green test suite but a reviewer would catch (misread spec, uncovered edge cases, plausible-but-wrong logic, security holes with no failing test), Liza is strictly safer at the moment of merge today. That gap is real, not hypothetical. But "measurement-only" is an honest, falsifiable staging discipline (record reviews, measure false-positive/negative rates, then promote to blocking) rather than mere absence — and shipping an *uncalibrated* blocking LLM reviewer has its own failure mode (wrongly blocking good work). The fair statement: Liza ships the stronger default guarantee; Gastown deliberately defers the LLM gate and currently relies on mechanical gates + detect-and-recover.

---

## 7. Judgment Allocation

LLM judgment is expensive, unreliable, and valuable; the architectural question is *where to spend it*. The two systems answer differently, and the honest result is a **split, not a one-sided "inversion."**

There are two kinds of decisions:

- **Infrastructure/liveness**: Is this merge mechanically sound (compiles, tests pass, no conflict)? — *deterministic*; code solves it best. Is this agent stuck, or composing a large artifact? — *genuinely fuzzy*; a fixed timeout can only guess.
- **Code-quality judgment**: Is this code correct, well-designed, secure, spec-aligned beyond what tests check? — the problem LLMs are uniquely suited for.

**Liza** gates merges on a binding LLM reviewer verdict, and handles liveness (stuck detection) with deterministic lease expiry + circuit breaker — no model tokens spent on liveness, keeping the judgment budget for review. Its binding gate is hardened: `MergeWorktree` refuses any task not in an approved state, re-validated under lock with a commit-SHA match; the verdict path enforces mandatory rejection reasons, impact-can-only-escalate, anti-stale SHA checks, a configurable quorum, and a ">95% approval-rate" calibration alarm.

**Gastown** gates merges on *mechanical* truth (build/typecheck/lint/test — an LLM cannot certify these more reliably than the compiler), and spends LLM judgment on the genuinely fuzzy supervision problem (Boot/`stuck-agent-dog` reading tmux panes per-situation, with mechanical thresholds as the degraded-mode fallback). It declines a binding LLM merge gate by choice.

**Verdict — split with one clear edge each.** The earlier "Gastown spends expensive judgment on deterministic problems and withholds it where it matters" framing is half wrong:

- Gastown's **merge** allocation is *correct* (mechanical questions get mechanical gates), and its **supervision** allocation is *defensible* (LLM judgment on irreducibly fuzzy fleet-liveness, mechanism as fallback). This is not misallocation.
- But Gastown has **no binding correctness judgment before a change becomes durable shared state** — a real current gap, conceded by its own "Phase 1" roadmap. NDI's eventual-completion guarantee covers *liveness of work*, not *absence of latent defects on a shared main*; recovering an unreviewed bug after merge is strictly costlier than preventing it.

So Liza wins "prevent-at-the-gate" and already owns the calibration tooling a blocking LLM gate needs; Gastown wins "judgment where fuzziness actually lives" and avoids shipping an unvalidated blocking reviewer. Both allocations are coherent. (One economic caveat against a naive "Gastown should just move that spend to the merge gate": a binding per-activity LLM review at 20-30 concurrent agents is a structural throughput tax Liza never has to pay at its scale — the reallocation is not free.)

---

## 8. Review & Verification

**Liza** — Adversarial doer/reviewer pairs on every task. A separate reviewer issues a *binding verdict* via PR-like interaction; approval means merge eligibility. Commit-SHA verification binds the verdict to the exact reviewed commit (the judged artifact is byte-for-byte the merged artifact). Default quorum is 2 with a *preferred* (not required) provider-diversity on the spec/architecture/integration pairs. (Caveat: the code-review step specifically defaults to quorum 1 with no diversity — provider separation is not mechanically required by the coding-pair policy, though operators can make doer/reviewer model separation easy through `LIZA_DEFAULT_DOER_CLI` and `LIZA_DEFAULT_REVIEWER_CLI`.)

**Gastown** — The Refinery manages a Bors-style merge queue:

1. Polecat completes → `gt done` → pushes branch → submits a Merge Request.
2. Refinery rebases onto target → runs mechanical gates (setup/typecheck/lint/build/test) in two phases (pre-merge and post-squash).
3. Pass → merge and push. Fail → diagnose, send `FIX_NEEDED` to the polecat.
4. Optional batch-then-bisect (when batching is enabled): assemble a batch, test, and on red recursively binary-search to isolate the culprit while innocent MRs still merge.
5. Optional `quality-review` (LLM, depth quick/standard/deep) — measurement-only, does not block.

| | Liza | Gastown |
|:--|:--|:--|
| Review model | Binding adversarial doer/reviewer | Mechanical gates + optional non-blocking LLM review |
| Review blocks merge? | Yes (binding verdict) | Tests/lint/build block; LLM review does not; optional human PR approval can |
| Provider diversity in review | Preferred (non-coding pairs); off for code-review step | No |
| Empirical culprit attribution | No bisecting queue (runs integration tests) | Yes (batch-then-bisect localizes the breaking commit) |
| Commit verification | SHA-bound verdict | Rebase-then-test (gate runs against integrated HEAD) |

**Verdict — genuine trade-off.** On the literal question — *does merged code carry a correctness guarantee beyond "tests pass"?* — Liza is genuinely stronger: it is the only system that puts a binding, independent, SHA-bound semantic verdict on the critical path today, catching exactly the bug class tests miss. But that verdict is an LLM opinion (uncalibrated, only *partially* decorrelated across providers since models share training corpora — a hedge, not true independence), and it is the most token-expensive verification topology available. Gastown trades that for a reproducible mechanical floor *plus* empirical bisection-based culprit attribution that Liza lacks entirely, at far lower latency and cost — betting calibrated-optional beats uncalibrated-mandatory. Which is "stronger" depends on whether one bad merge is expensive (favors Liza) or throughput at high concurrency dominates (favors Gastown).

---

## 9. Coordination & Communication

**Liza** — Coordination through the YAML blackboard: a broadcast, shared-state model. Every agent sees every task's full state, history, and PR-like review comments without explicit routing. Coordination is *implicit* through state transitions — the blackboard is simultaneously the task board, the communication channel, and the audit trail. Situational awareness is automatic: any agent can inspect any task's full history without being an explicit recipient.

**Gastown** — A point-to-point architecture with more primitives:

- **Mail**: persistent inter-agent messages stored as beads, threaded via dependencies.
- **Nudge**: lightweight, zero-Dolt-cost tmux notifications.
- **Hook**: a pinned bead serving as an agent's primary work queue (work is "slung" onto hooks).
- **Seance**: agents discover and query/resume previous sessions via `.events.jsonl` (`--talk` forks a predecessor session).
- **Escalate**: structured escalation with four severities (low/medium/high/critical, default medium), routed Deacon → Mayor → Overseer.
- Newer: beads-native groups/queues/channels and convoy/cross-rig dependency notifications.

**Verdict — different trade-offs, not a clear winner.** Gastown has more communication *primitives* enabling point-to-point patterns (agent-to-agent requests, clarifications, handoffs, escalation). Liza's blackboard is a more powerful *coordination substrate*: shared state gives global visibility without routing decisions, and review interaction is structured communication *through* state, not alongside it. Gastown requires explicit routing ("who do I message?"); Liza makes coordination implicit ("what changed on the board?"). Liza lacks Gastown's Seance (retrospective session query) and escalation routing; Gastown lacks Liza's broadcast situational awareness.

---

## 10. Persistence & Recovery

**Liza** — Built for failure: leases expire and tasks become reclaimable; crashed agents are recoverable (`liza recover-agent`, `liza recover-task`); a circuit breaker detects systemic failure; failure is a *recorded state transition* (BLOCKED, REJECTED, SUPERSEDED). Work persists across crashes on the blackboard. Handoff is structured but *forward-only* (notes on task completion), not retrospective.

**Gastown** — Also built for failure, with different mechanisms: Dolt atomic commits survive crashes (the daemon auto-restarts the server with backoff); the Witness classifies stalled/zombie polecats and triggers recovery; the Deacon runs patrol cycles and health checks; Seance recovers a predecessor's context; `gt handoff` cycles sessions at context limits (triggered by PreCompact hooks); polecat identity persists across ephemeral sessions.

**Verdict — both serious about recovery; Gastown's is richer, Liza's is simpler.** Both separate them from lighter multi-agent tools. Gastown's Seance (querying a predecessor's reasoning) and its monitoring agents are capabilities Liza lacks. Liza's recovery is centralized and deterministic (lease expiry + supervisor-managed transitions) — fewer moving parts, no watchdog-of-watchdog, but no retrospective session archaeology.

---

## 11. Workflow Definition & Configurability

**Both systems are runtime-configurable. Neither workflow is hard-coded.** This corrects the most important framing error in earlier versions. The real question is which configuration *model* is more powerful, and what it costs.

**Liza** — Two tiers. The full MAS pipeline is a declarative YAML pipeline (roles, phases, transitions) configurable three ways: a global `~/.liza/pipeline.yaml` (from `liza setup`), a per-project frozen `.liza/pipeline.yaml` (`liza init --config`, hand-editable after), and `--entry-point` to start mid-pipeline. For lighter work, the **adversarial-pairing skill** coordinates doer/reviewer sessions through a Markdown blackboard. Essentially one topology — a doer+reviewer assembly line — opinionated around adversarial review.

**Gastown** — A Formula system, far more expressive as an *authoring language*:

- **Four execution topologies**: `workflow` (sequential steps with a `needs` DAG), `convoy` (parallel legs + synthesis), `aspect` (parallel analysis passes), `expansion` (parameterized template).
- **Typed variables** (`[vars]` with required/default, `--set`), **inheritance** (`extends`) and **composition** (`compose`/`expand`), and **per-rig overlays** that override single steps without forking.
- **Three-tier resolution** (rig `.beads/formulas/` → town `~/gt/.beads/formulas/` → embedded) that survives upgrades — `gt install` never overwrites user formulas.
- **48 shipping formulas** spanning idea-to-plan, reviews, refinery/witness/deacon patrols, release engineering, TDD, and demos. The only hard-coded default is `mol-polecat-work` for bare-bead dispatch — itself a normal, overridable formula (and it ships *self-review only*).

Lifecycle: Formula (source TOML) → `cook` → Protomolecule (frozen) → `pour` → Molecule (persistent instance) **or** `wisp` → Wisp (ephemeral, `dolt_ignore`'d, no commits).

| | Liza | Gastown |
|:--|:--|:--|
| Workflow definition | YAML pipeline + Markdown-blackboard skill | TOML formulas (4 types) |
| Topologies | One (doer+reviewer chain) | Four (workflow/convoy/aspect/expansion) |
| Composition / inheritance / overlays | No (fork-and-edit the frozen pipeline) | Yes (`extends`, `compose`, per-step overlays) |
| Built-in review guarantee | Yes — binding review is the default floor | No — a formula need not include review; review never gates merge by default |

**Verdict — two distinct axes, no overall winner.**
- **Configurability/expressiveness: Gastown wins clearly.** More topologies, real composition and overlays, typed vars, 48 formulas vs one pipeline + one skill.
- **Built-in guarantee: Liza wins clearly.** Independent adversarial review is a Go-enforced merge precondition and the *default* — you must work to *remove* it. Gastown's flexible grammar can express "no review," and its out-of-the-box dispatch ships self-review only.

A grammar that can express "no review" is not more powerful *for the goal of trustworthy autonomous merges*; a single opinionated topology is not more powerful *for per-task flexibility*. Which matters more is goal-relative.

---

## 12. Human Role

**Liza** — The human owns intent and acts as observer/circuit-breaker. Within a sprint, agents are autonomous; between sprints, the human reviews artifacts and steers via CLI. In Pairing mode, the human is an active collaborator with approval gates and structured postures (Coach, Challenger, Spike, User Duck).

**Gastown** — The human is a "Crew member" with a long-lived workspace (a full git clone, not a worktree). The Mayor is the primary interface ("tell the Mayor what you want"). The human defines tasks, monitors via dashboards, and intervenes; some formulas include explicit human gates (e.g. `mol-idea-to-plan`'s clarification step). `require_review` lets a rig opt into a human PR-approval merge gate.

**Verdict — both keep the human at the boundary; different shapes.** Liza's human steers between sprints with a richer synchronous Pairing mode. Gastown's is more federated — the Mayor mediates and the human works alongside agents in their own Crew workspace, which Liza has no equivalent of.

---

## 13. Provider Support

**Liza** — Multi-provider: Claude Code, Codex, Gemini, Kimi, Mistral. The behavioral contract improves agent quality but is additive; mechanical enforcement constrains any model regardless. Provider diversity is a deliberate (preferred) feature in the review quorum. Integration is effectively binary: a model handles the contract or it doesn't.

**Gastown** — Multi-provider with a graduated integration ladder across ten CLIs (claude, gemini, codex, cursor, auggie, amp, opencode, copilot, pi, omp):

- **Tier 0**: any terminal CLI via tmux.
- **Tier 1**: JSON preset for lifecycle/resume/process detection.
- **Tier 2**: hooks (context injection, tool guards, mail delivery).
- **Tier 3**: deep (non-interactive mode, session forking, wrappers).

Copilot is now a full hooks-capable preset; Codex hooks are experimental opt-in.

**Verdict — Gastown is more inclusive here.** Any terminal CLI participates at Tier 0, with deeper capability unlocked progressively. Liza optimizes for behavioral compliance over breadth of integration.

---

## 14. Scale & Cost

**Liza** — Cost is dominated by the behavioral contract (every agent pays for it in context) plus the multi-sprint lifecycle and the binding-review topology. Context tiers (Full → Working Set → Kernel) manage degradation. A handful of agents per sprint, sustained over time. **No shipped cost tracking and no usage-limit/account-rotation concept** — a real gap at fleet scale.

**Gastown** — Designed for higher concurrency (~20-30 agents), with first-class cost/quota machinery: a Scheduler governs dispatch (`scheduler.max_polecats`); `gt costs` tracks per-session cost (from a local log, digested daily into a bead) with stop-hook auto-recording; `quota_dog` performs mechanical account rotation on rate limits, with usage-limit-aware backoff (`PauseBackoff`) that doesn't burn the crash-loop budget.

**Verdict — Gastown wins this dimension materially.** Cost attribution, quota rotation, and scheduler-governed dispatch are shipped, tested infrastructure with no Liza equivalent. Liza can run fewer concurrent agents and lacks operational cost/quota governance.

---

## 15. Maturity & Adoption

**Liza** — `v0.8.0`, single primary author, created January 2026, Apache 2.0. Self-implementing since v0.4.0. ~61k non-test LOC + ~144k test lines (~2.4:1, i.e. ~70% of the codebase is tests — verification depth, not API breadth). Small but real adoption; design coherence from one author, with the corresponding single-maintainer continuity risk.

**Gastown** — `v1.2.1` (2026-06-06), created December 2025, MIT (© Steve Yegge). ~447k LOC (~46% tests). **Also essentially single-author**: Steve Yegge dominates the history (with many agent-generated commit identities), plus one substantial external contributor. The larger surface — Wasteland federation, tiered provider integration, OTel, Dolt-backed state, the Formula/Molecule system — reflects broader ambition and a longer feature reach, not a bigger team. (This corrects an earlier speculation that Gastown implied a larger engineering investment than a solo project; it does not.)

**Verdict — two solo-author, architecturally serious systems at similar early maturity (v0.x/v1.x).** Neither has the community scale of CrewAI or BMAD. Gastown is materially ahead on *operational* maturity (ops tooling, scale machinery); Liza is ahead on *correctness* maturity (binding gate, failure-mode rigor, test density). Each side's strength is the other's gap. The design divergences are more interesting than the adoption counts.

---

## 16. Auditability & Continuous Improvement

**Liza** — Full audit trail as a design feature: the blackboard records every transition, assignment, verdict, rejection reason, and rescoping event. Two analysis skills operate on the trail: `/liza-logs` (anomaly patterns, token usage, behavioral signals at sprint boundaries) and `/context-engineering` (context budget, duplicated/missing context, prompt drift, handoff quality). Sprint checkpoints feed findings into the next sprint's configuration. The feedback loop is *built-in and actionable*.

**Gastown** — A different, broader stack:

- **Capability Ledger**: a permanent record of completions/handoffs/events — what agents *did*, used to route work to proven agents; formula-compliance (skipped steps) is detectable.
- **OpenTelemetry**: structured logs/metrics to any OTLP backend (VictoriaMetrics/Logs default), with cross-agent correlation.
- **`.events.jsonl`**: raw audit log, queryable by `gt seance`.
- **`gt doctor`**: a large diagnostic system — **128 check files** spanning workspace, infrastructure, Dolt health, clone divergence, routing, lifecycle — with `--fix` auto-repair.
- **A/B model testing**: a `gt-model-eval/` harness (promptfoo) for comparing models on similar tasks.

| | Liza | Gastown |
|:--|:--|:--|
| Audit trail | Blackboard (YAML, durable, human-readable) | Capability Ledger + `.events.jsonl` (Dolt + JSONL) |
| Analysis tooling | `/liza-logs` + `/context-engineering` (built-in skills) | External OTLP backends + A/B harness |
| Feedback loop | Sprint checkpoints → config → next sprint | Capability Ledger → work routing; Seance → context recovery |
| Diagnostics | Circuit breaker, `liza validate`/`analyze` (narrow) | `gt doctor` (128 checks + `--fix`) |
| Telemetry | Agent logs (session-level) | OTel (metrics + logs to external backends) |

**Verdict — both serious; different shapes, Gastown broader on ops observability.** Liza's analysis is *built-in* (skills producing actionable findings at sprint boundaries) and self-contained. Gastown's telemetry is *exported* (standard OTel, external backends) and its `gt doctor` is a genuine operational capability Liza lacks at that breadth. Liza's `validate`/`analyze` cover a narrower slice.

---

## 17. Notable Gastown Features

Some of these are genuine innovations; some are coordination machinery that Gastown's scale and ZFC choices require and Liza's blackboard + mechanical supervisor avoid. Both characterizations can be fair simultaneously.

### Genuine capabilities worth learning from

1. **Formula/Molecule system** — composable, parameterized, overlay-able workflow templates across four topologies. The most expressive workflow-authoring model in this survey.
2. **Seance** — retrospective session query/resume; an agent can interrogate a predecessor's reasoning directly.
3. **`gt doctor`** — 128 diagnostic checks with auto-fix.
4. **First-class cost + quota governance** — per-session cost tracking and mechanical account rotation.
5. **OTel telemetry** — standard observability to external backends.
6. **Tiered provider integration** — any terminal CLI participates at Tier 0.
7. **A/B model testing** — infrastructure for comparing models on similar tasks.
8. **Batch-then-bisect merge queue** — empirical, automatic culprit localization.
9. **Wasteland federation** — a federated work network linking towns via DoltHub with multi-dimensional reputation stamps and Spider-Protocol fraud detection. **Status: Phase 1 "wild-west," no trust enforcement yet — experimental.** Nothing comparable elsewhere in this survey, but not yet a shipped guarantee.

### Machinery that scale + ZFC require

10. **Point-to-point messaging (mail/nudge/escalate)** — needed because supervision is distributed across communicating LLM agents; Liza's shared blackboard makes coordination implicit.
11. **Boot + `stuck-agent-dog`** — a watchdog-of-watchdog tier and an LLM stuck-detector, required because the LLM supervisor can itself wedge; Liza's deterministic supervisor needs neither.
12. **Dolt SQL state** — load-bearing for 20-30 writers and cross-rig/federation; overhead a solo sprint won't amortize.
13. **Crew workspaces** — a managed human workspace as a first-class entity; Liza's Pairing mode serves the collaboration need without managing the human's workspace.

---

## 18. Notable Liza Features Gastown Lacks

1. **Binding adversarial review with commit-SHA verification** — an independent reviewer's verdict gates merge by default; Gastown's LLM review is measurement-only.
2. **Failure-mode catalog (55, with 14 MAST-mapped)** — documented LLM failure modes with specific mechanical countermeasures; Gastown has per-role failure-mode notes but no systematic catalog.
3. **Tiered invariant system (Tier 0-3)** — an explicit hierarchy of which rules never bend; Gastown has no equivalent tier system.
4. **Code-enforced state machine** — the Go supervisor validates every transition; Gastown delegates transition judgment to agents (ZFC).
5. **Provider-diversity review (preferred)** — deliberate cross-provider routing on the non-coding pairs to dampen correlated blind spots; Gastown's review is single-provider.
6. **Pairing mode with collaboration postures** — Coach, Challenger, Spike, User Duck.
7. **Behavioral contract as an additive quality layer** — improves reasoning without being load-bearing for safety; remove it and mechanical enforcement still holds.
8. **Integration phase** — a dedicated post-coding phase examining cross-task interactions after individual tasks merge; Gastown's Refinery verifies individual merges but has no equivalent cross-task integration review.
9. **Explicit context-degradation tiers** — Full → Working Set → Kernel, with defined re-read protocols.

---

## 19. Where They Overlap (The Convergence)

Two solo-author Go CLIs, created within a month, reaching for the same primitives:

- **Git worktree isolation** as the concurrency boundary.
- **Structured state persistence** (YAML blackboard / Dolt) surviving crashes.
- **Role-based agent hierarchies** with distinct boundaries.
- **Multi-provider support** — the agent CLI as a replaceable, loosely-coupled component.
- **Crash recovery** as a first-class concern.
- **Mechanical quality gates before merge** — both require tests/lint/build to pass.
- **CLI-mediated agent interaction** — agents act through the system CLI.
- **Human at the boundary** — approve-then-observe, not per-action queues.
- **Runtime-configurable workflows** — pipeline config (Liza) / formulas (Gastown), not hard-coded.
- **Go as implementation language.**
- **Spec-driven decomposition** of high-level goals into trackable work units.

The independent convergence suggests these are load-bearing decisions for multi-agent coding systems, not arbitrary choices.

---

## 20. Where They Diverge Most

The fundamental divergence is **where each spends its determinism and its judgment** — and, downstream of that, **what each guarantees.**

**Liza** spends determinism on an *enforcement floor* (state machine, forbidden operations, a binding merge gate) and spends LLM judgment on *code review with binding authority*. It handles liveness deterministically. Its guarantee: an unapproved or different-than-reviewed change does not become durable shared state.

**Gastown** spends determinism on *mechanical merge gates and transport* and spends LLM judgment on *fleet supervision* (the genuinely fuzzy liveness problem), declining a binding LLM merge gate by choice. Its guarantee: work eventually completes despite failing runs.

These are not symmetric "rigid vs flexible." They are two coherent, differently-targeted designs:

- Liza's design center is **correctness-bounded, human-supervisable-scale delivery** (a handful of durable agents per sprint), where a binding gate is affordable and one bad merge is expensive.
- Gastown's design center is **high-concurrency, federated, throughput-oriented orchestration** (20-30 agents, cross-rig), where a binding per-activity review is a structural tax and detect-and-recover is the operating model.

Liza's real risk is over-constraint and lower concurrency ceiling. Gastown's real risk is that test-passing-but-wrong code merges with no binding judgment gate, and that LLM supervision sets a reliability ceiling at the model's ceiling. Both risks are genuine; neither system is incoherent.

---

## 21. Framework Failure Modes

**Liza** struggles when:
- Work needs flexible, ad-hoc orchestration that fits neither the MAS pipeline nor adversarial-pairing.
- High agent counts are needed — a single global state lock and the handful-per-sprint model cap concurrency.
- Operational cost/quota governance at fleet scale matters — Liza ships none.
- (Note: the behavioral contract is additive, not load-bearing for safety. A model that ignores it is still constrained by the Go supervisor.)

**Gastown** struggles when:
- Judgment-level code quality matters beyond what tests catch — the LLM quality gate is measurement-only by default.
- LLM supervisory agents (Boot/Deacon/Witness) misjudge — the reliability ceiling is the model's ceiling, and the supervision recursion adds failure surface.
- Provider diversity in review is wanted — review is single-provider.
- Zero-ops simplicity is wanted — the Dolt server + external `bd` dependency add operational surface (which `gt doctor` exists partly to manage).
- A formal, code-enforced state machine is wanted — ZFC makes most transition decisions agent-mediated conventions rather than code-enforced rules.

---

## 22. What Each Could Steal

### What Liza could steal from Gastown

1. **Session query interface** — Liza has the data (blackboard history + agent logs) but no `gt seance`-style mechanism to directly interrogate a predecessor's reasoning. A `liza session-query` would close this without new data infrastructure.
2. **First-class cost + quota governance** — per-session cost tracking and rate-limit-aware dispatch are operational infrastructure Liza should ship.
3. **`gt doctor`-style diagnostics with auto-fix** — Liza's `validate`/`analyze` cover a narrower slice.
4. **Batch-then-bisect culprit attribution** — empirical localization of which merged commit broke the integrated tree.
5. **A more composable workflow grammar** — formula-style inheritance/overlays would extend Liza's two-tier model toward per-task reuse without forking pipelines.
6. **Wasteland-style federation concept** — portable, multi-dimensional reputation across instances (acknowledging it is still Phase 1 in Gastown).

### What Gastown could steal from Liza

1. **Binding adversarial review** — promote the already-built `quality-review` from measurement-only to a blocking gate once calibration data exists; this closes the "tests pass" vs "actually correct" gap.
2. **Failure-mode catalog with mechanical countermeasures** — a systematic catalog mapping each LLM failure mode to a countermeasure.
3. **Selective code-enforced state machine** — for high-stakes transitions (merge authority, review verdicts), mechanical enforcement provides guarantees prompt conventions cannot. ZFC is a fine default; selective enforcement at high-stakes boundaries would strengthen it.
4. **Provider-diversity review** — route review to a different model family to break correlated blind spots.
5. **Tiered invariant hierarchy** — an explicit hierarchy of what halts the town unconditionally vs what merely warns.
6. **Integration phase** — a dedicated post-merge review of cross-task interactions.

---

## 23. Layering & Integration — Can They Compose?

Gastown and Liza are peer systems at the same architectural layer — both external orchestrators wrapping agent CLIs. Direct composition is awkward: you wouldn't run Liza supervising a Gastown-managed town or vice versa.

The realistic path is **idea exchange, not system layering**:

- Liza adopting Gastown's communication/state/observability patterns (session query, cost tracking, OTel) without adopting Dolt or ZFC wholesale.
- Gastown adopting Liza's enforcement patterns (binding review, selective state-machine enforcement, tier system) at high-stakes boundaries without abandoning ZFC everywhere.
- A shared standard for agent session events (`.events.jsonl` or equivalent) both systems could produce and consume, enabling cross-system session discovery.

---

## 24. Bottom Line

Gastown is the comparator that most resembles Liza architecturally — same language, near-same month, same primitives (worktrees, roles, state persistence, CLI mediation, crash recovery, multi-provider, configurable workflows). Both are solo-author projects that converged on these load-bearing decisions independently.

The divergence is **two coherent designs with different targets**, not a winner and a loser:

- **Liza's durable advantages** cluster around *correctness and simplicity at human-supervisable scale*: a binding, commit-SHA-verified adversarial review gate as the default (the one place it puts judgment, and it puts it where tests can't reach); a documented failure-mode catalog with mechanical countermeasures; a code-enforced state machine; provider-diversity review; a tiered invariant hierarchy; and a genuinely simpler operational footprint (one binary, one file, no daemon).
- **Gastown's durable advantages** cluster around *scale, flexibility, and operations*: a Dolt substrate and control plane built for 20-30 concurrent agents; the most expressive workflow-authoring model here (formulas, four topologies, composition, overlays); empirical batch-then-bisect culprit attribution; first-class cost/quota governance; OTel observability and a 128-check `gt doctor`; tiered provider integration; session archaeology (Seance); and a federation concept (Wasteland, still experimental).

The sharpest, fairest lesson is about **judgment allocation, stated precisely**. Both systems use code where code is reliable and models where judgment is irreducible — they disagree about which problems fall where. Gastown's allocation is *not* the "inversion" earlier framing claimed: gating merges on mechanical truth is correct, and spending LLM judgment on fuzzy fleet-liveness is defensible. But Gastown today ships **no binding correctness judgment before a change becomes durable shared state**, and that is a real gap its own roadmap concedes. Liza's allocation puts binding judgment exactly at that boundary — at the cost of throughput Gastown's scale target can't afford to pay per-activity.

So the honest recommendation is workload-shaped: **for correctness-critical work at human-supervisable scale where one bad merge is expensive, Liza's enforced floor is the stronger foundation. For high-concurrency, federated, throughput-oriented orchestration where detect-and-recover is acceptable and operations matter, Gastown is the more capable platform.** Each is, in its own design center, the better architecture.
