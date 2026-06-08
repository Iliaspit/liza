---
date: 2026-06-08
perspective: Authored by Liza maintainers. Hosted in Liza's own repo — weigh framing accordingly. Gastown facts are from DeepWiki analysis of the public GitHub repository (gastownhall/gastown) and its documentation, checked 2026-06-08. Liza facts are from the local repository.
---

# Liza vs Gastown Comparison

## Source Snapshot

- **Liza**: local repository HEAD on `main`; current release tag `v0.8.0`. Go, ~35k LOC + ~92k lines of tests. Apache 2.0. Single primary author. Created January 2026. Stack- and provider-agnostic by design.
- **Gastown**: `gastownhall/gastown` on GitHub; version 0.9.0 (as of March 2026). Go. MIT License. Created January 2026. Multi-provider by design.

This is the closest architectural comparator in the folder. Gastown and Liza are both Go CLIs that orchestrate multi-agent coding workflows through structured state, isolated worktrees, and role-based agent hierarchies. They were created independently in the same month, solve overlapping problems, and converged on many of the same structural choices. The interesting question is not whether they overlap — they deeply do — but where their design philosophies diverge on trust, state management, and the boundary between code-enforced and agent-delegated decisions.

---

## 1. Identity & Positioning

**Liza** — "Hardened Multi-Agent Coding System." A standalone Go CLI that supervises long-lived role agents running concurrently on isolated git worktrees, coordinated through a file-backed YAML blackboard. Liza wraps agents in control machinery and treats LLMs as unreliable components to be mechanically constrained. The behavioral contract — 55+ failure modes with tiered countermeasures — is the defining feature. Provider-agnostic: Claude Code, Codex, Gemini, Kimi, Mistral.

**Gastown** — A multi-agent workspace manager built around the metaphor of a "Town" containing "Rigs" (projects). A Go CLI that orchestrates AI coding agents through a Dolt SQL database (Git-semantics SQL), tmux sessions, and a mail system. Gastown's design philosophy is **ZFC (Zero Decisions in Code)**: the Go binary provides infrastructure — worktrees, state, messaging, merge queues — but delegates all judgment to LLM agents. Provider-agnostic: Claude Code, Copilot, Codex, Gemini, and others via tiered integration.

The positioning gap matters. Liza distrusts the LLM and wraps it in mechanical constraints (code validates transitions, enforces gates, prevents forbidden operations). Gastown trusts the LLM with judgment and provides it with infrastructure: state tracking, communication channels, and raw outputs for the model to interpret. Liza asks "what stops the agent from doing something wrong?" and answers with Go code. Gastown asks "what does the agent need to do its job?" and answers with tooling. One constrains; the other enables.

---

## 2. Core Philosophy

**Liza** starts from the premise that LLM agents are unreliable by default. The behavioral contract was developed incrementally as countermeasures to actually-observed misbehavior — agents altering tests to pass, fabricating completions, silently drifting from scope. Trust is engineered mechanically, not assumed. The supervisor is code; invariants are enforced by Go, not by prompts the model is asked to honor.

**Gastown** starts from the premise that LLM agents need **infrastructure, not micromanagement**. Its three core principles capture this:

- **GUPP (Gas Town Universal Propulsion Principle)**: "If you have work on your hook, you run it." Agents must autonomously proceed without confirmation or waiting. Liveness over caution.
- **ZFC (Zero Decisions in Code)**: All judgment calls are delegated to agents. The Go binary returns raw output (e.g., raw `git` errors) for the model to interpret, rather than pre-digesting results.
- **NDI (Nondeterministic Idempotence)**: Accept that individual agent runs may fail; design for useful outcomes through orchestration of potentially unreliable processes.

The epistemic difference is fundamental. Liza treats agent unreliability as a problem to suppress mechanically — the supervisor validates every state transition, and nothing merges without an enforced external verdict. Gastown treats agent unreliability as a fact of life to route around — agents will fail, the system will detect it, another agent will retry or escalate, and eventually work completes. Liza prevents bad outcomes; Gastown recovers from them.

---

## 3. State Substrate — Where the Plan Lives

This is the load-bearing architectural divergence.

**Liza** — State lives in `.liza/state.yaml`, a file-backed YAML blackboard serialized through file locks. Pipeline configuration lives in `pipeline.yaml` and Go code. The blackboard is flat, human-readable, and version-controlled. Every agent reads and writes through the `liza` CLI, which validates transitions against the state machine before committing them.

**Gastown** — State lives in a **Dolt SQL database** — a SQL database with Git-like version control semantics (branch, merge, diff). A single `dolt sql-server` process runs per town on port 3307. Every write is wrapped in `BEGIN` / `DOLT_COMMIT` / `COMMIT` atomically. All agents write to the `main` branch directly. Each rig gets its own database. The "Beads" system tracks work as structured SQL records with full event history.

| | Liza | Gastown |
|:--|:--|:--|
| State format | YAML file | SQL database (Dolt) |
| Concurrency control | File locks | SQL transactions |
| Versioning | Git (the file is in the repo) | Dolt (Git-semantics on the database itself) |
| Query model | CLI reads → YAML parse | SQL queries |
| Human readability | Direct file inspection | SQL queries or CLI wrappers |
| Scalability ceiling | File-lock contention at high agent counts | SQL transactions scale better with concurrency |

Gastown's choice of Dolt over flat files is the more sophisticated data layer — SQL queries, atomic transactions, built-in versioning. The trade-off is operational complexity: a database server process must be running, and debugging state requires SQL rather than reading a YAML file. Liza's flat file is simpler to inspect and requires no running daemon, but file-lock serialization limits concurrent write throughput.

---

## 4. Agent Architecture & Roles

**Liza** — 13 roles across 4 pipeline phases (Specification, Architecture, Coding, Integration). Every activity is dual: a doer and a reviewer in an adversarial pair. Roles are functional positions with strict, supervisor-enforced boundaries. Agents are long-lived concurrent processes on isolated worktrees, leased and heartbeat-monitored. A handful of agents per sprint, each durable.

**Gastown** — A deeper role hierarchy with infrastructure and worker tiers:

- **Infrastructure agents**: Mayor (global coordinator), Deacon (daemon supervisor, health monitoring), Dog (maintenance worker dispatched by Deacon)
- **Per-rig supervisors**: Witness (polecat lifecycle manager, stuck-agent detection), Refinery (merge queue processor)
- **Workers**: Polecats (ephemeral task workers with persistent identities), Crew (human developer workspace)

Gastown's agent hierarchy is deeper — but much of that depth is a direct consequence of ZFC, not independent capability. Because Gastown delegates all decisions to agents, supervisory functions that Liza handles within the Go binary (health monitoring, stuck-agent detection, merge queue processing, maintenance) must be performed by dedicated LLM agent roles (Witness, Deacon, Dog, Refinery). Liza's mechanical supervisor is code that cannot hallucinate; it doesn't need a "Witness" role because the Go binary *is* the witness. Gastown's deeper hierarchy is the cost of its architectural choice — it's solving a problem Liza doesn't have.

| | Liza | Gastown |
|:--|:--|:--|
| Worker model | Long-lived, leased, heartbeat-monitored | Persistent identity, ephemeral sessions |
| Supervision | Go binary (deterministic) | LLM agents (Witness, Deacon) |
| Scale target | Handful per sprint | 20-30 concurrent agents |
| Session management | Terminal sessions, manual or TUI | tmux-managed, automatic lifecycle |
| Worker isolation | Git worktrees (structural) | Git worktrees (structural) |

Both use git worktrees for isolation — a genuine convergence. The difference is in supervision: Liza's supervisor is deterministic code that cannot hallucinate; Gastown's supervisory agents (Witness, Deacon) are LLMs that could, in principle, misinterpret a situation. Gastown's counter-argument via ZFC is that delegating judgment to agents is the point — the code provides raw data, the model decides.

---

## 5. Task Decomposition — From Goal to Work Units

**Liza** — Adversarially reviewed decomposition at every level. The Orchestrator reads a goal and creates planning tasks. Each doer (Epic Planner, US Writer, Architect, Code Planner) writes `output[]` subtask definitions that the paired reviewer validates before the supervisor mechanically fans out downstream tasks from the approved output. A bad decomposition is rejected before it propagates. For complex goals (>3 functional areas), the Orchestrator creates multiple sequential planning tasks chained with `depends_on`. The decomposition is the plan — and because every level is adversarially reviewed, the plan is challenged before any code is written.

**Gastown** — The `mol-idea-to-plan` formula orchestrates a substantial decomposition pipeline:

1. **Intake**: An agent structures the user's idea into a draft PRD.
2. **PRD Review**: Six polecats review the PRD in parallel (requirements, gaps, ambiguity, feasibility, scope, stakeholders).
3. **Human Clarification**: Consolidated questions presented to the user — a human gate.
4. **Generate Plan**: Six polecats design the implementation in parallel (API, data, UX, scale, security, integration).
5. **PRD Alignment Rounds**: Three rounds of two polecats each, aligning the plan with the PRD iteratively.
6. **Plan Self-Review Rounds**: Three rounds of two polecats each, reviewing the plan for internal quality (completeness, sequencing, risk, scope-creep, testability, coherence).
7. **Create Beads**: The refined plan is converted into individual beads with dependencies wired via `bd dep add`.
8. **Verify Beads**: Three sequential passes compare the plan to the created beads, filling gaps.

| | Liza | Gastown |
|:--|:--|:--|
| Decomposition author | Multiple doer/reviewer pairs per phase | Crew worker + parallel polecat swarms |
| Review of decomposition | Adversarial (reviewer validates before fan-out) | Self-review rounds (polecats reviewing polecats) |
| Review binding? | Yes (reviewer verdict blocks fan-out) | No (review rounds are iterative, not gating) |
| Human gate | Between sprints (checkpoint review) | After PRD review (clarification questions) |
| Dependency management | `depends_on` in blackboard, supervisor-enforced | `bd dep add`, convoy-managed |
| Decomposition phases | Goal → Epics → US → Architecture → Code Plans | Idea → PRD → Plan → Beads |

Both systems take decomposition seriously — neither expects the user to pre-enumerate tasks. The architectural difference follows the same pattern as everything else: Liza's decomposition is adversarially reviewed with binding verdicts at each level (a bad epic plan is rejected before user stories are written). Gastown's decomposition uses parallel review swarms and iterative self-review rounds — more agents thrown at the problem, but the reviews are corrective rather than gating. A decomposition that survives six reviewers and three alignment rounds is likely good, but nothing mechanically blocks a flawed one from producing beads.

Gastown's decomposition pipeline is heavier in agent-count (six parallel reviewers, three alignment rounds, three self-review rounds, three verification passes — potentially 20+ polecat invocations for a single decomposition). Liza's is lighter per-phase but spans more phases (specification → architecture → coding, each with one doer/reviewer pair). The cost trade-off: Gastown spends heavily on one decomposition burst; Liza spreads the cost across multiple adversarial passes over the sprint lifecycle.

---

## 6. Trust & Behavioral Control

**Liza** — The defining differentiator. A behavioral contract addresses 55+ documented LLM failure modes (sycophancy, phantom fixes, scope creep, test corruption, hallucinated completions). A tiered rule system: Tier 0 invariants are never violated (no unapproved state change, no fabrication, no test corruption, no unvalidated success). The Go supervisor enforces validation rules mechanically — task state machine, approval-gated merges, commit-SHA verification against the reviewed state. Trust is *suppression of known failure modes by code*.

**Gastown** — Behavioral governance is a hybrid of prompt-level contracts and infrastructure verification:

- **Role contracts**: Each agent role has explicit contracts in markdown templates — what it must do, what it must not do, prohibited actions. For example, polecats must not push directly to main or skip verification steps.
- **Failure mode awareness**: Formula definitions include explicit "Failure Modes" sections with prescribed actions for each (build fails, blocked on external, context filling, unsure what to do).
- **Anti-hallucination**: The Witness template explicitly states: "Hallucination kills trust. If you claim to have done something without actually doing it, the entire system breaks. Each step is mechanical and verifiable."
- **GUPP violation detection**: The Deacon monitors for agents that have work on their hook but aren't progressing, and notifies the Witness for remediation.

The trust boundary sits in fundamentally different places:

- **Liza**: Trust is mechanically enforced — the Go supervisor validates state transitions, blocks forbidden operations, and nothing merges without an independent reviewer's binding verdict. The agent cannot bypass the gate because the gate is code.
- **Gastown**: Trust is established through behavioral contracts (prompt-level) and verified through infrastructure (quality gates, capability ledger, oversight agents). The Go code deliberately avoids making decisions (ZFC) — it provides raw signals and lets the agent decide. The Refinery runs tests and lints before merging, but the `quality-review` step (LLM-based review) is currently **measurement-only and does not block merges**.

This is the sharpest philosophical divergence. Liza says "agents will misbehave; block them with code." Gastown says "agents will misbehave; detect it and route around it." Liza prevents the bad outcome from happening; Gastown detects it after the fact and repairs.

The critical detail: Gastown's LLM-based quality review does not currently block merges. The blocking gates are mechanical (tests, lint, typecheck, build), which is the same baseline any CI system provides. Liza's adversarial reviewer verdict is a *binding* gate that blocks merge — a stronger guarantee on judgment-level issues that pass automated tests.

---

## 7. Judgment Allocation — The Architectural Inversion

This section names the structural problem that runs through Gastown's design.

LLM judgment is expensive, unreliable, and valuable. The architectural question is *where to spend it*. There are two kinds of decisions in a multi-agent coding system:

- **Infrastructure decisions**: Is this agent stuck? Should this merge proceed? Is the system healthy? Should work be retried? These are deterministic problems with observable inputs — process state, test exit codes, timeouts, git status. Code solves them reliably.
- **Judgment decisions**: Is this code correct? Is the design sound? Is this change secure? Does this implementation match the spec? These are the problems LLMs are uniquely suited for — they require understanding intent, context, and quality in ways that mechanical gates cannot.

Liza allocates judgment correctly: **code handles infrastructure, LLMs handle code review with binding authority.** The Go supervisor monitors health, enforces state transitions, manages merges, and detects failures — deterministically, without hallucination risk. The LLM reviewer examines code quality, correctness, and design — the judgment problem — and its verdict is the binding gate.

Gastown inverts this allocation. **LLM agents handle infrastructure (Witness monitors health, Deacon detects violations, Refinery processes merges, Dog does maintenance), while LLM code review is advisory and non-blocking.** The system puts expensive, unreliable judgment where cheap, reliable code would suffice, and withholds it where it would add the most value.

The consequences cascade:

- **Role proliferation**: Gastown needs Witness, Deacon, Dog, Mayor, and Refinery as LLM agents to supervise infrastructure — roles that exist because code doesn't handle supervision. Liza's single Go binary replaces all of them. The deeper role hierarchy is not richer capability; it's accidental complexity from the architectural choice.
- **Infrastructure complexity**: Point-to-point messaging (mail + nudge), a SQL database (Dolt), and structured escalation routing are needed to coordinate the coordination agents. A blackboard + mechanical code makes this coordination implicit — the shared state *is* the communication, no routing required.
- **Reliability ceiling**: When supervision is an LLM, the system's reliability ceiling is the model's reliability ceiling. The Witness template says "Hallucination kills trust" — but that warning is addressed to the very agent whose hallucination would be hardest to detect, because it *is* the detector.
- **The gap that matters most**: Code that passes tests but is architecturally wrong, subtly buggy, or poorly designed merges without a binding judgment gate. The one place where LLM judgment is irreplaceable — evaluating code quality beyond what automated checks can catch — is the one place Gastown makes it optional.

ZFC is a coherent principle ("let agents decide"), but coherence is not correctness. The question is not "should decisions be in code or in agents?" — it's "which decisions?" Gastown answers "all of them" and pays for it in complexity and reliability. Liza answers "the right ones" and gets a simpler, more reliable system with stronger guarantees where they matter most.

---

## 8. Review & Verification

**Liza** — Adversarial doer/reviewer pairs on every task. A separate reviewer examines the work and issues a *binding verdict* via PR-like interaction. Approval means merge eligibility. Commit-SHA verification prevents reviewing stale state. A configurable review quorum with provider-diversity preference reduces single-model bias.

**Gastown** — The Refinery manages a Bors-style bisecting merge queue:

1. Polecat completes work → `gt done` → pushes branch → submits Merge Request
2. Refinery picks next branch → rebases onto target → runs quality gates (tests, lint, typecheck, build)
3. If gates pass → merge and push. If gates fail → diagnose whether branch caused failure → reopen issue, send `FIX_NEEDED` to polecat
4. Optional `quality-review` step (LLM-based, configurable depth: quick/standard/deep) — currently measurement-only, does not block

| | Liza | Gastown |
|:--|:--|:--|
| Review model | Adversarial doer/reviewer pairs | Automated quality gates + optional LLM review |
| Review blocks merge? | Yes (binding verdict) | Tests/lint block; LLM review does not (measurement-only) |
| Provider diversity in review | Yes (deliberate cross-provider routing) | No (single-provider review) |
| Review scope | Full code review (correctness, design, security) | Quality gates (automated) + optional AI review (advisory) |
| Commit verification | SHA verification against reviewed state | Rebase verification (branch rebased before gates) |

Liza's review is a harder gate. Gastown's quality gates catch mechanical issues (broken tests, lint failures) but don't currently enforce judgment-level review. For code that passes tests but is architecturally wrong, poorly designed, or subtly buggy, Liza's adversarial reviewer is the stronger safety net.

---

## 9. Coordination & Communication

**Liza** — Coordination through the YAML blackboard — a broadcast, shared-state model. Every agent sees every task's full state, history, and PR-like review comments without explicit routing. Coordination is implicit through state transitions: agents don't need to know who to message or how to route requests — they read the blackboard and act on what they find. The blackboard is simultaneously the task board, the communication channel, and the audit trail. This makes situational awareness automatic: any agent can inspect any task's full history, review comments, and current state without being an explicit recipient.

**Gastown** — A point-to-point communication architecture with more primitives:

- **Mail system**: Persistent inter-agent messages stored as Beads (`issue_type='message'`) in Dolt. Supports threading via dependencies.
- **Nudge**: Lightweight, zero-Dolt-cost notifications for routine communications.
- **Hook**: A pinned Bead that serves as an agent's primary work queue — work is "slung" onto hooks.
- **Seance**: Agents can discover and query previous sessions via `.events.jsonl` logs, recovering context from predecessors.
- **Escalation**: Structured escalation via `gt escalate` with severity levels (CRITICAL, HIGH, MEDIUM), routing through Deacon → Mayor → Overseer.

Different coordination trade-offs, not a clear winner. Gastown has more communication *primitives* — direct messaging, lightweight notifications, session discovery, structured escalation — that enable point-to-point coordination patterns (agent-to-agent requests, clarifications, handoffs). Liza's blackboard is a more powerful *coordination substrate*: shared state gives every agent global visibility without routing decisions, and the PR-like review interaction (submission, feedback comments, verdict, revised submission) is structured communication through state, not alongside it. Gastown's model requires explicit routing (who do I message?); Liza's model makes coordination implicit (what changed on the board?). The gap is that Liza lacks Gastown's Seance (retrospective session discovery) and escalation routing, while Gastown lacks Liza's broadcast situational awareness.

---

## 10. Persistence & Recovery

**Liza** — Built for failure. Leases expire and tasks become reclaimable; crashed agents are recoverable (`liza recover-agent`, `liza recover-task`); the circuit breaker detects systemic failure; failure is a *state transition* (BLOCKED, REJECTED, SUPERSEDED) the supervisor records. Work persists across crashes and sessions on the blackboard.

**Gastown** — Also built for failure, with different mechanisms:

- **Dolt persistence**: All state survives crashes because it's in a SQL database with atomic commits.
- **Witness monitoring**: Detects stalled and zombie polecats, triggers recovery. Three operating states: Working, Stalled, Zombie.
- **Deacon**: Continuous daemon that runs patrol cycles, monitors system health, checks for GUPP violations.
- **Seance**: Session recovery — agents can query previous sessions' `.events.jsonl` logs to recover context.
- **Handoff**: `gt handoff` cycles agent sessions when they hit context limits.
- **Agent identity persistence**: Polecats have persistent identities across ephemeral sessions.

Both systems take crash recovery seriously — a shared concern that separates them from lighter multi-agent tools. The mechanisms differ: Liza recovers through supervisor-managed state transitions and lease expiry. Gastown recovers through database persistence, monitoring agents (Witness/Deacon), and session discovery (Seance). Gastown's Seance feature — allowing agents to query their predecessors' logs — is a capability Liza lacks; Liza's context handoff is structured but forward-only (notes on task completion), not retrospective.

---

## 11. Workflow Definition

**Liza** — Two workflow tiers. The full MAS pipeline is a declarative YAML pipeline with fixed phases (Specification → Architecture → Coding → Integration), where role pairs and transitions are defined in pipeline configuration and Go code. For lighter work, the **adversarial-pairing skill** provides a lightweight workflow coordinating doer/reviewer sessions through a Markdown blackboard — same adversarial principle, without the full pipeline machinery. Every MAS sprint flows through the configured phases; adversarial-pairing is used in Pairing mode for focused tasks that need review but not full decomposition.

**Gastown** — A Formula/Molecule system:

- **Formula**: TOML-based workflow template defining multi-step procedures. Examples: `mol-idea-to-plan` (idea → PRD), `mol-polecat-work` (task execution), `mol-refinery-patrol` (merge queue processing), `gastown-release` (release workflow).
- **Protomolecule**: Frozen template ready for instantiation.
- **Molecule**: Active workflow instance — durable chained Beads.
- **Wisp**: Ephemeral molecule for patrols and polecat work.

Gastown's Formula system offers more workflow variety — templates can be authored for different task types, parameterized, and composed. Liza has two tiers (full MAS pipeline + lightweight adversarial-pairing) but not the per-task-type composability Gastown's Formulas provide.

| | Liza | Gastown |
|:--|:--|:--|
| Workflow definition | YAML pipeline (MAS) + Markdown blackboard (adversarial-pairing) | TOML formulas (composable templates) |
| Workflow variety | Two tiers (full pipeline + lightweight pairing) | Multiple formula types per task kind |
| Workflow customization | Pipeline config (roles, phases, policies) | Formula authoring (steps, variables, gates) |
| Spec-driven decomposition | Goal → Epics → User Stories → Architecture → Code Plans → Code | MEOW: Beads + Epics + Formulas → Molecules |

---

## 12. Human Role

**Liza** — The human owns intent and acts as observer/circuit-breaker. Within a sprint, agents are autonomous; between sprints, the human reviews artifacts and steers via CLI. In Pairing mode, the human is an active collaborator with approval gates. Authority is a kill switch plus, in Pairing mode, approval gates.

**Gastown** — The human is a "Crew member" with their own long-lived workspace (a full git clone, not a worktree). The Mayor is the primary interface: "Tell the Mayor what you want to accomplish." The human defines tasks, monitors progress via dashboards, and intervenes when necessary. The `mol-idea-to-plan` formula includes a "human gate" after PRD review.

Both keep the human at the boundary rather than in every step. Liza's human steers between sprints and has a richer Pairing mode (Coach, Challenger, Spike, User Duck) for synchronous collaboration. Gastown's human role is more federated — the Mayor mediates, and the human can work alongside agents in their own Crew workspace. Liza doesn't have an equivalent of the Crew concept (a human workspace managed by the same system).

---

## 13. Provider Support

**Liza** — Multi-provider by design: Claude Code, Codex, Gemini, Kimi, Mistral. The behavioral contract improves agent quality but is an additive layer — the mechanical enforcement (state machine, merge gates, worktree isolation) constrains any model regardless. Models that can't follow the contract produce lower-quality reasoning but are still safely supervised. Provider diversity is a deliberate feature (diversity-preferring review quorum).

**Gastown** — Multi-provider with tiered integration depth:

- **Tier 0 (Zero Integration)**: Any CLI that runs in a terminal — basic tmux orchestration.
- **Tier 1 (Preset Registration)**: JSON config for full lifecycle management, resume, process detection.
- **Tier 2 (Hooks)**: Settings files/plugins for context injection, tool guards, mail delivery.
- **Tier 3 (Deep)**: Code and scripts for non-interactive mode, session forking, wrappers.

Gastown's tiered integration model is more inclusive — any terminal CLI can participate at Tier 0, with deeper capabilities unlocked at higher tiers. Liza's integration is binary: either the model handles the behavioral contract or it doesn't. Gastown explicitly optimizes for loose coupling (tmux + environment variables, no library imports); Liza optimizes for behavioral compliance (the contract is the integration test).

---

## 14. Scale & Cost

**Liza** — Cost is dominated by the behavioral contract (every agent pays for it in context) plus the multi-sprint lifecycle. Context tiers (Full → Working Set → Kernel) manage degradation. A handful of agents per sprint, sustained over time. The cost shape is *sustained*.

**Gastown** — Designed for higher concurrency (20-30 agents). The Scheduler governs polecat dispatch with configurable concurrency limits to prevent API rate exhaustion. `gt costs` tracks per-session costs in Beads. Cost tracking is a first-class feature with stop-hook integration for automatic recording. The cost shape is *higher-width, with explicit cost governance*.

Gastown's cost tracking and quota management (per-session tracking, automatic recording, scheduler-governed dispatch) is more mature than Liza's current offering. Liza plans per-agent/task cost tracking but hasn't shipped it. The operational difference is that Gastown can run significantly more concurrent agents (20-30 vs Liza's handful) while actively managing the cost and rate-limit implications.

---

## 15. Maturity & Adoption

**Liza** — `v0.8.0`, single primary author, created January 2026, Apache 2.0. Self-implementing since v0.4.0 (all major Liza changes are built using Liza's own multi-agent mode). ~35k LOC + ~92k lines of tests. Small but real adoption; design coherence from one author, with the corresponding single-maintainer continuity risk.

**Gastown** — `v0.9.0` (as of March 2026), created January 2026, MIT License. Go. Team size and star count not publicly documented. Ambitious scope: the Wasteland federation, tiered provider integration, OTel telemetry, Dolt-backed state, and the Formula/Molecule system suggest a larger engineering investment than a solo project, though the repo's contributor structure is not confirmed.

Both are early-stage (v0.x), created the same month, and written in Go. Neither has the community scale of CrewAI (45k stars) or BMAD (~45k stars). The comparison is between two independent, architecturally serious systems at similar maturity levels — making the design divergences more interesting than adoption counts.

---

## 16. Auditability & Continuous Improvement

**Liza** — Full audit trail as a design feature. The blackboard records every state transition, assignment, verdict, rejection reason, and rescoping event — a durable, queryable history of what each agent did and why. Agent logs capture full sessions. Two analysis skills operate on this trail: `/liza-logs` analyzes agent sessions at sprint boundaries (anomaly patterns, token usage, behavioral signals), and `/context-engineering` analyzes the prompt/output corpus (context budget use, duplicated or missing context, prompt drift, tool-output pressure, handoff quality). Sprint checkpoints and retrospectives feed findings back into the next sprint's configuration. The audit trail enables *continuous improvement*: each sprint teaches the system about itself, and the lessons are actionable because they're grounded in durable evidence.

**Gastown** — A different but substantial auditability stack:

- **Capability Ledger**: A permanent record of every agent's completions, handoffs, and logged events — what agents *did*, not what they claimed. Tracks demonstrated capability for routing work to proven agents. Formula compliance is visible in the ledger; skipping steps is detectable.
- **OpenTelemetry (OTel)**: All agent operations emitted as structured logs and metrics to any OTLP-compatible backend (VictoriaMetrics/VictoriaLogs by default). Session context via environment variables enables cross-agent correlation.
- **`.events.jsonl`**: Raw audit log of all activity events. Queryable by `gt seance` for session archaeology.
- **`gt doctor`**: Comprehensive diagnostic system with dozens of checks (workspace, infrastructure, cleanup, clone divergence, routing, lifecycle, Dolt health) and auto-fix support.
- **A/B model testing**: Supports comparing different AI models on similar tasks and tracking completion rates and quality.

| | Liza | Gastown |
|:--|:--|:--|
| Audit trail | Blackboard (YAML, durable, human-readable) | Capability Ledger + `.events.jsonl` (Dolt + JSONL) |
| Analysis tooling | `/liza-logs` + `/context-engineering` (built-in skills) | External OTLP backends (PromQL/logsql queries) |
| Feedback loop | Sprint checkpoints → configuration → next sprint | Capability Ledger → work routing; Seance → context recovery |
| Diagnostics | Circuit breaker, `liza analyze` | `gt doctor` (dozens of checks + auto-fix) |
| Telemetry | Agent logs (session-level) | OTel (structured metrics + logs to external backends) |

Both take auditability seriously — a shared trait that separates them from lighter tools. The shapes differ: Liza's analysis is *built-in* (skills that run inside agent sessions, producing actionable findings at sprint boundaries). Gastown's telemetry is *exported* (structured data emitted to external backends for querying). Liza's approach is self-contained; Gastown's is more standard (OTel) but requires external infrastructure.

Gastown's `gt doctor` is a genuine capability Liza lacks — a systematic diagnostic system that detects configuration drift, stale processes, orphaned sessions, and dozens of other operational issues with auto-fix support. Liza's `liza validate` and `liza analyze` cover some of this territory but with narrower scope.

---

## 17. Unique Gastown Features

Not all of these are gaps in Liza. Several exist to mitigate problems that Gastown's own architecture (ZFC, LLM-based supervision) creates — problems Liza's blackboard + mechanical supervisor doesn't have. The distinction matters.

### Genuine innovations — ideas worth learning from

1. **Seance (session query interface)**: Agents can discover and interrogate previous sessions' event logs to recover a predecessor's reasoning and decisions. Liza's blackboard already provides durable task-level archaeology (every state transition, review comment, and verdict is recorded and queryable by any agent), and agent output logs capture full sessions — so the *data* exists. The gap is the *query mechanism*: Gastown's `gt seance` lets an agent ask "what did my predecessor conclude about X?" directly, while Liza's agents must piece it together from blackboard history and handoff notes.
2. **Wasteland federation**: A federated work coordination network linking Gas Town instances through DoltHub. Multi-dimensional reputation stamps. Nothing comparable in Liza or any other MAS in this survey.
3. **Formula/Molecule workflow system**: Composable, parameterized workflow templates for different task types. Liza has two tiers (full MAS pipeline + lightweight adversarial-pairing) but not per-task-type composability.
4. **Cost tracking**: First-class per-session cost tracking with automatic stop-hook recording. Liza's is planned but not shipped.
5. **`gt doctor` diagnostic system**: Dozens of automated checks (workspace, infrastructure, clone divergence, stale processes, orphaned sessions) with auto-fix support. Liza's `liza validate` and `liza analyze` cover narrower territory.
6. **OTel telemetry**: Structured metrics and logs emitted to any OTLP-compatible backend. Standard observability infrastructure.
7. **Tiered provider integration**: Any CLI works at Tier 0; deeper integration unlocked progressively. More inclusive than Liza's binary contract-capable/not model.
8. **A/B model testing**: Infrastructure for comparing different AI models on similar tasks and tracking quality/completion rates.

### ZFC consequences — solutions to self-inflicted problems

These features exist because Gastown's architectural choices create coordination problems that Liza's blackboard + mechanical supervisor avoids entirely.

9. **Point-to-point messaging (mail + nudge)**: Needed because ZFC distributes supervision across LLM agents that must communicate with each other. Liza's shared blackboard makes coordination implicit through state — no routing, no threading, no message delivery concerns.
10. **Dog agents**: Infrastructure maintenance workers dispatched by the Deacon. Liza's Go binary handles maintenance deterministically — no need for a separate LLM agent role to do housekeeping.
11. **Dolt SQL state**: The database adds queryability and concurrency, but much of its complexity (daemon management, transaction discipline, routing configuration, health checks) exists to support 20-30 agents that only exist because supervision is itself agent-performed. Liza's YAML + file locks are simpler at the scale its mechanical supervisor requires.
12. **Crew workspaces**: Managed human workspace within the agent system. Liza's Pairing mode serves the same purpose — human-agent collaboration — without needing to manage the human's workspace as another entity in the orchestration graph.

---

## 18. Unique Liza Features Gastown Lacks

1. **Adversarial doer/reviewer pairs with binding verdicts**: Every task is reviewed by an independent reviewer whose verdict blocks or enables merge. Gastown's quality-review is measurement-only.
2. **Failure mode catalog (55+)**: Documented LLM failure modes with specific mechanical countermeasures. Gastown has role-specific failure modes in formulas but no systematic catalog.
3. **Tiered invariant system (Tier 0-3)**: Explicit hierarchy of which rules never bend, which require waivers, and which degrade gracefully. Gastown has no equivalent tier system.
4. **Code-enforced state machine with forbidden transitions**: The Go supervisor validates every state transition against a formal state machine. Gastown delegates transition judgment to agents (ZFC).
5. **Provider-diversity review**: Deliberate cross-provider routing to break correlated model blind spots. Gastown's review is single-provider.
6. **Pairing mode with collaboration postures**: Coach, Challenger, Spike, User Duck — structured human-agent collaboration modes. Gastown's human interaction is mediated through the Mayor.
7. **Behavioral contract as additive quality layer**: The contract improves agent reasoning (structured analysis, assumption tracking, struggle protocol) without being load-bearing for safety. Remove it and mechanical enforcement still holds — the contract makes agents better, the code makes them safe.
8. **Commit-SHA verification**: The reviewer's verdict is bound to a specific commit SHA, preventing review of stale state.
9. **Integration phase**: A dedicated post-coding phase (Integration Analyst + Integration Reviewer) that examines cross-task interactions after individual tasks merge.
10. **Explicit context degradation tiers**: Full → Working Set → Kernel, with defined re-read protocols at each level.

---

## 19. Where They Overlap (The Convergence)

The convergence is striking — two independent Go CLIs, created the same month, reaching for the same structural primitives:

- **Git worktree isolation** as the concurrency boundary for parallel agent work.
- **Structured state persistence** (YAML blackboard / Dolt database) surviving crashes and sessions.
- **Role-based agent hierarchies** with distinct responsibilities and boundaries.
- **Multi-provider support** — both treat the agent CLI as a replaceable, loosely-coupled component.
- **Crash recovery** as a first-class architectural concern, not an afterthought.
- **Quality gates before merge** — both require tests/lint/build to pass before code lands.
- **CLI-mediated agent interaction** — agents act through the system CLI, not direct API calls.
- **Human at the boundary** — approve-then-observe, not per-action queues.
- **Go as implementation language** — both chose Go for the supervisor/orchestrator.
- **Spec-driven decomposition** — both decompose high-level goals into trackable work units.

The independent convergence on git worktrees + structured state + role hierarchies + CLI mediation suggests these are load-bearing design decisions for multi-agent coding systems, not arbitrary choices.

---

## 20. Where They Diverge Most

The fundamental divergence is **who makes decisions** — but the sharper question (see §7) is **which decisions**.

**Liza** keeps infrastructure decisions in code and gives LLMs binding authority on judgment decisions. The Go supervisor handles health monitoring, state transitions, and merge authority — deterministic work. The LLM reviewer handles code quality assessment — judgment work — and its verdict is the gate.

**Gastown** keeps all decisions in agents (ZFC). Infrastructure supervision (health, merges, maintenance) is performed by LLM agents. Code quality review — the one place LLM judgment is irreplaceable — is advisory and non-blocking.

The result is not a symmetric trade-off between "more rigid" and "more flexible." Gastown's judgment allocation is inverted: it spends LLM judgment on deterministic problems code can solve reliably, and withholds it from the judgment problem where it would add the most value. The consequences — role proliferation, infrastructure complexity, a reliability ceiling set by the model — are costs of this inversion, not inherent features of multi-agent orchestration.

Liza's failure mode is real: over-constraint and rigidity in its MAS pipeline (though the lightweight adversarial-pairing skill provides a second tier for focused work). But Gastown's failure mode is architectural, not just operational — the system is complex because it solves problems its own design choices create.

---

## 21. Framework Failure Modes

**Liza** fails when:
- Work requires flexible, ad-hoc orchestration that doesn't fit either the MAS pipeline or adversarial-pairing.
- High agent counts are needed — file-lock contention and the handful-per-sprint model limit concurrency.
- Operational setup cost is disproportionate for lightweight tasks.

Note: the behavioral contract is an additive layer, not load-bearing for safety. Remove it and the mechanical enforcement — state machine, merge gates, worktree isolation, review verdicts — still holds. A model that ignores the contract is constrained by the Go supervisor regardless. The contract makes agents more thoughtful; it doesn't make the system safe. The code does.

**Gastown** fails when:
- LLM supervisory agents (Witness, Deacon) misjudge situations — hallucinate health, miss subtle failures. The system's reliability ceiling is the model's reliability ceiling.
- Judgment-level code quality matters beyond what automated tests catch. The quality-review gate is advisory, not blocking.
- Provider diversity in review is needed to break correlated blind spots. Review is single-provider.
- The Dolt database adds operational complexity (daemon management, SQL debugging) that simpler state stores avoid.
- Formal state machine guarantees are needed — ZFC delegates transition decisions to agents, so forbidden transitions are conventions, not code-enforced rules.

---

## 22. What Each Could Steal

### What Liza could steal from Gastown

Genuine ideas, not mitigation patterns for an architecture Liza doesn't share.

1. **Session query interface**: Liza already has the data (blackboard history + agent output logs), but lacks a query mechanism like Gastown's `gt seance` that lets an agent directly interrogate a predecessor's reasoning. A `liza session-query` command surfacing relevant prior session context would close this gap without new data infrastructure.

2. **Cost tracking as first-class state**: Per-session cost tracking stored in the work ledger, with automatic stop-hook recording, is operational infrastructure Liza should ship.

3. **`gt doctor`-style diagnostic system**: A comprehensive, auto-fixing diagnostic command that checks workspace health, detects configuration drift, finds orphaned processes/sessions, and validates infrastructure integrity. Liza's `validate` and `analyze` cover a subset of this.

4. **Wasteland-style federation concept**: The idea that completed work earns portable, multi-dimensional reputation stamps across instances is novel. Nothing comparable exists in any MAS in this survey.

### What Gastown could steal from Liza

1. **Binding adversarial review**: Gastown's quality-review is measurement-only. Making an independent reviewer's verdict a *blocking* gate — nothing merges without it — would close the gap between "tests pass" and "the code is actually correct." This is the heart of what Liza enforces.

2. **Failure mode catalog with mechanical countermeasures**: Gastown has role-specific failure modes in formulas, but no systematic catalog mapping each LLM failure mode to a specific countermeasure. Liza's 55+ catalog is a defensible asset.

3. **Code-enforced state machine**: Gastown delegates transition decisions to agents (ZFC). For critical transitions — merge authority, review verdicts, task state changes — mechanical enforcement would provide guarantees that prompt-level contracts cannot. ZFC is the right default; selective code enforcement at high-stakes boundaries would strengthen it.

4. **Provider-diversity review**: Gastown's review is single-provider. Routing review to a different model family would make review genuinely adversarial across architectures, breaking correlated blind spots.

5. **Tiered invariant hierarchy**: Gastown has no explicit hierarchy of which conditions halt the system unconditionally versus merely warn. Liza's Tier 0-3 model is a clear template for deciding, in a 30-agent town, what must stop everything versus what can fail quietly.

6. **Integration phase**: A dedicated post-merge phase examining cross-task interactions. Gastown's Refinery verifies individual merges but has no equivalent of Liza's Integration Analyst examining how separately-merged tasks interact.

---

## 23. Layering & Integration — Can They Compose?

Unlike Dynamic Workflows (which lives inside an agent CLI Liza already drives), Gastown and Liza are peer systems at the same architectural layer — both are external orchestrators wrapping agent CLIs. Direct composition is awkward: you wouldn't run Liza supervising a Gastown-managed town or vice versa.

The realistic integration path is **idea exchange, not system layering**:

- Liza adopting Gastown's communication and state patterns (Dolt, mail, Seance) without ZFC.
- Gastown adopting Liza's enforcement patterns (binding review, state machine, tier system) without abandoning ZFC everywhere — applying mechanical enforcement selectively at high-stakes boundaries.
- A shared standard for agent session events (`.events.jsonl` or equivalent) that both systems could produce and consume, enabling cross-system session discovery.

---

## 24. Bottom Line

Gastown is the comparator that most resembles Liza architecturally — same language, same month, same structural primitives (worktrees, roles, state persistence, CLI mediation, crash recovery, multi-provider). The convergence validates that these are load-bearing design decisions for multi-agent coding systems.

The divergence is not symmetric. Gastown's ZFC principle ("all decisions to agents") produces an architectural inversion (§6): LLM judgment is spent on infrastructure supervision where code would be more reliable, and withheld from code review where it would add the most value. Much of Gastown's complexity — the deep role hierarchy, point-to-point messaging, SQL database, escalation routing — exists to coordinate problems that a blackboard + mechanical supervisor doesn't create. The system is sophisticated, but a significant portion of that sophistication is self-inflicted.

**Liza's durable advantages** are where Gastown's inversion hurts most: **binding adversarial review (not advisory), a documented failure mode catalog with mechanical countermeasures, code-enforced state machine transitions (not prompt conventions), provider-diversity review, a tiered invariant hierarchy, and a simpler architecture that solves coordination through shared state rather than agent-mediated messaging**.

**Gastown's genuine contributions** — ideas worth learning from regardless of architectural disagreement: **SQL-backed state for higher concurrency, composable workflow templates (Formulas), session archaeology (Seance), federated work coordination (Wasteland), first-class cost tracking, and tiered provider integration that lets any CLI participate**.

The sharpest lesson from this comparison is not about Gastown specifically — it's about ZFC as a design principle. "Let agents decide everything" is a coherent philosophy, but coherence is not correctness. The right question is not "code or agents?" but "which decisions?" Infrastructure supervision is a deterministic problem; code solves it simply and reliably. Code quality assessment is a judgment problem; LLMs solve it uniquely. Liza allocates judgment to where it's irreplaceable. That's not just a different trade-off — it's a better architecture.
