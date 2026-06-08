---
date: 2026-06-08
perspective: Drafted by Opus 4.8 and revised by Opus 4.6. Hosted in Liza's own repo — weigh framing accordingly. Dynamic Workflows is a first-party Anthropic feature in research preview; its surface is changing fast, so version-specific claims are point-in-time snapshots. Liza facts are from the local repository; Dynamic Workflows facts are from Anthropic's official documentation and launch blog (see Source Snapshot).
---

# Liza vs Claude Code Dynamic Workflows Comparison

## Source Snapshot

- **Liza**: local repository HEAD on `main`; current release tag `v0.8.0`. Go, ~35k LOC + ~90k lines of tests. Apache 2.0. Single primary author. Created January 2026. Stack- and provider-agnostic by design.
- **Claude Code Dynamic Workflows**: first-party Anthropic feature, **research preview**. Requires Claude Code v2.1.154+. Available on all paid plans (Pro via a `/config` toggle), the Anthropic API, Amazon Bedrock, Google Cloud Vertex AI, and Microsoft Foundry. Facts drawn from the official docs (`code.claude.com/docs/en/workflows`) and the launch blog (`claude.com/blog/introducing-dynamic-workflows-in-claude-code`), checked 2026-06-08.

Unlike the gstack and BMAD comparisons in this folder, this is a near head-to-head. Dynamic Workflows (hereafter **DW**) is Anthropic's own multi-agent orchestration primitive, and it independently converged on the two ideas at Liza's core: **parallel agent fan-out** and **adversarial cross-review before results are trusted**. The interesting question is therefore not "do they do the same thing?" — partly, yes — but "where do they put the orchestration plan, and what enforces trust?"

---

## 1. Identity & Positioning

**Liza** — "Hardened Multi-Agent Coding System." A standalone Go CLI that supervises long-lived role agents (built on top of agent CLIs like Claude Code, Codex, Gemini, Mistral) running concurrently on isolated git worktrees, coordinated through a file-backed blackboard. Liza is *infrastructure that wraps agents in control machinery*. It is provider-agnostic: the agents it drives are interchangeable.

**DW** — A feature *inside* Claude Code. A dynamic workflow is "a JavaScript script that orchestrates subagents at scale": Claude writes the script for the task you describe, and a runtime executes it in the background while your session stays responsive. DW is *a capability of the agent runtime itself*. It is first-party and Claude-only: the orchestrator and the workers are all Claude.

The positioning gap matters. Liza sits *above* the agent CLI and treats the model as a replaceable, distrusted component. DW lives *inside* the agent CLI and treats the model as both the orchestrator (it writes the script) and the workforce (it runs the subagents). Liza is an external supervisor; DW is the runtime extending its own reach.

---

## 2. Core Philosophy

**Liza** starts from the premise that LLM agents are unreliable by default. The behavioral contract was developed incrementally as countermeasures to actually-observed misbehavior — agents altering tests to pass, fabricating completions, silently drifting from scope. Trust is engineered mechanically, not assumed. The supervisor is code; the invariants are enforced by Go, not by prompts the model is asked to honor.

**DW** starts from the premise that "some problems are too big for one pass by a single agent, especially in complex, legacy codebases." The motivating use cases are codebase-wide bug hunts, large migrations, and language ports — work whose size, not whose risk, is the binding constraint. DW's answer is to let Claude *plan dynamically*, decompose the task, fan it out, and apply a repeatable quality pattern (independent agents adversarially reviewing each other's findings) so the result is more trustworthy than a single pass.

Both systems rely on adversarial verification as their core quality mechanism — this is the deepest convergence. The difference is in what *backs* it. Liza's adversarial review is a *mechanically gated state transition*: a separate reviewer's binding verdict is the only thing that unlocks merge authority, the verdict is recorded on a durable blackboard that survives crashes, the supervisor enforces the gate in code, and the reviewer can optionally be a different model from a different provider to break correlated blind spots. DW's adversarial review is a *quality pattern encoded in the orchestration script*: independent agents try to refute each other's findings and the script folds in what survives. This is the same idea — but Liza's version is strictly stronger because the verdict is durable, mechanically enforced, and provider-diverse, while DW's is ephemeral (lives in script variables), self-judged (the script decides what passes), and single-model (all agents are Claude). The same mechanism, at two different levels of guarantee.

---

## 3. Orchestration Substrate — Who Holds the Plan

This is the load-bearing difference, so it gets its own section.

**Liza** externalizes the plan into a **supervisor-owned state machine**. The Go binary holds task state in `.liza/state.yaml` (the blackboard), serialized through file locks. Roles, transitions, review policies, and timeouts live in `pipeline.yaml` and `roles.go`. The plan is *data the supervisor enforces*; the LLM never holds it.

**DW** externalizes the plan into **an LLM-written JavaScript script**. Claude writes the orchestration — the loop, the branching, the fan-out, the verification — as code, and a runtime executes it in an isolated environment separate from the conversation. Intermediate results live in *script variables*, not the model's context window, which is what lets DW coordinate hundreds of agents without blowing the context budget. The script is written to a file under `~/.claude/projects/`, so you can read it, diff it against a prior run, edit it, and relaunch.

Both move the plan out of the turn-by-turn context window — a genuine convergence, and a real advance over plain subagents (where every result lands back in context). The difference is *what kind of artifact the plan is and who authored it*:

| | Liza | Dynamic Workflows |
| :-- | :-- | :-- |
| Plan author | Human (pipeline config) + agents (decomposition) | Claude, per task (generated script) |
| Plan form | Declarative state machine (YAML + Go) with dynamic task topology | Imperative script (JavaScript) |
| Plan lifetime | Persistent across crashes/sessions | Per-session; resets on exit |
| Enforcement | Supervisor validates every transition | Runtime executes; script self-governs |
| Repeatable unit | The pipeline shape (consistent every sprint); the task graph within it (dynamic per goal) | The generated script (savable as a command) |

Liza's pipeline shape is fixed (the role-pair sequence is declared in YAML), but the *task graph within it is dynamically generated*: the Orchestrator reads a goal and decomposes it into tasks, each doer writes `output[]` subtask definitions that the reviewer validates, and the supervisor mechanically creates downstream tasks from approved output. The decomposition is adversarially reviewed at every level — a bad decomposition is rejected before it fans out. DW's plan is bespoke code generated for one task — more flexible per-run, but not adversarially reviewed before execution (only the plan approval gate). Liza trades per-run flexibility for structural guarantees and reviewed decomposition; DW trades those guarantees for unconstrained per-task orchestration.

---

## 4. Agent Architecture & Fan-out

**Liza** — 1 Orchestrator + 12 other roles across 3 pipeline phases (specification, implementation, integration). Every activity is dual: a doer and a reviewer. Roles are functional positions with strict, supervisor-enforced boundaries (a coder cannot perform planner operations). The task graph is **dynamically decomposed**: the Orchestrator reads a goal and creates tasks; each doer (Epic Planner, US Writer, Code Planner) writes `output[]` subtask definitions that the paired reviewer validates, and the supervisor then mechanically fans out downstream tasks from the approved output. For complex goals (>3 functional areas), the Orchestrator creates multiple sequential planning tasks chained with `depends_on`. Agents are *real, long-lived concurrent processes* in separate terminal sessions, each on its own git worktree. The number of live agents is modest (a handful of roles per sprint) but each is durable, leased, and heartbeat-monitored.

**DW** — Ephemeral, high-cardinality fan-out. A single run spawns **up to 16 concurrent agents** (fewer on CPU-limited machines) and **up to 1,000 agents total per run** (a hard cap that bounds runaway loops). Subagents are short-lived workers the script spawns, coordinates, and discards; their results accumulate in script variables. The Bun port (Jarred Sumner, Zig→Rust, ~750k lines, 99.8% of the test suite passing, ~11 days) reportedly ran "hundreds of agents in parallel with two reviewers on each file."

The architectural contrast is **few-and-durable vs many-and-ephemeral**, combined with **where decomposition happens**. Liza's agents are persistent role-holders with identity, leases, and recovery; they *produce* the task graph dynamically (each doer proposes subtasks, each reviewer validates them, the supervisor instantiates them), and the system is sized for sustained autonomous execution across sprints. DW's agents are transient workers executing a task graph that Claude wrote up front as a script. Liza scales in *time* (long-running, restartable, dynamically decomposing). DW scales in *width* (massive single-run parallelism on a pre-planned script).

---

## 5. Trust & Behavioral Control

**Liza** — Both adversarial verification *and* mechanical enforcement, layered. The behavioral contract addresses 55+ documented LLM failure modes (sycophancy, phantom fixes, scope creep, test corruption, hallucinated completions). A tiered rule system: Tier 0 invariants are never violated (no unapproved state change, no fabrication, no test corruption, no unvalidated success). The adversarial doer/reviewer pair is at the heart of the trust model — but the reviewer's verdict is not advisory; it is a *mechanically enforced state transition* backed by the durable blackboard. The Go supervisor enforces the gate: no merge without an approved verdict, commit-SHA verification against the reviewed state, and optional provider-diversity preference so the reviewer can be a fundamentally different model. Adversarial verification is *the mechanism*; the supervisor and blackboard are *what makes it binding*.

**DW** — Also adversarial verification, but without mechanical enforcement. The quality pattern is that independent agents review each other's findings and the script keeps what survives cross-checking (`/deep-research` literally votes on each claim and filters out the ones that don't survive). This is a real and valuable mechanism — but it lives in an ephemeral script, not on a durable blackboard; the script (not an external supervisor) decides what passes; and all agents are the same model family, so correlated blind spots survive cross-checking. Critically, the **subagents a workflow spawns always run in `acceptEdits` mode with file edits auto-approved**, regardless of your session's permission mode. The only mandatory human gate is *before the run starts* (approve the plan); after that, file edits flow without per-edit confirmation.

Both rely on adversarial verification as the core quality mechanism. The difference is in what backs it. Liza's is durable (recorded on the blackboard, survives crashes), enforced (the supervisor blocks merge without approval), and optionally diverse (different provider for the reviewer). DW's is ephemeral (lives in script runtime variables), self-judged (the script decides acceptance), and single-model (Claude reviewing Claude). For a read-only audit or research sweep, DW's self-judging model is well-matched — the output is a report, not a merge. For autonomous code that lands in a branch, Liza's enforced gate is the stronger guarantee — and DW's auto-approved edits are the larger surface.

---

## 6. Coordination & State

**Liza** — Classical blackboard architecture. A shared YAML file tracks goals, tasks, assignments, status, and history. Time-bounded leases, DRAFT tasks, atomic operations via file locking, isolated worktrees. Coordination is *durable shared state* that any agent or the operator can read and that survives process death.

**DW** — Coordination is **in-script**. The plan, the branching, and the intermediate results are held in the JavaScript runtime's variables. There is no shared task list visible to outside processes (contrast agent teams, which use a shared task list); there is no blackboard file. The runtime tracks each agent's result as the run progresses — but as runtime state, not as a persisted, externally-queryable artifact. The `/workflows` view reads this live state to show phases, agent counts, and token totals.

Liza's coordination state is a first-class, on-disk, lockable artifact — the system of record. DW's coordination state is the live memory of a running script — rich while the run is alive, gone when it ends. This directly drives the next two sections.

---

## 7. Concurrency & Isolation

**Liza** — Isolation is a core architectural feature. Each agent works in its own git worktree; the blackboard plus file locking prevents conflicting state writes; merge authority is structural and serialized. Two coders never edit the same working tree; integration happens through a supervised merge protocol.

**DW** — High concurrency, **less explicit workspace isolation**. Up to 16 agents run at once, and they read, write, and run commands directly (the script itself has no filesystem or shell access — the agents do the work; the script only coordinates). The docs describe the Bun port as "two reviewers on each file," implying file-level partitioning is how concurrent writers are kept from colliding — a *discipline encoded in the generated script*, not a worktree-level guarantee from the runtime. There is no mention of per-agent worktree isolation; the subagents inherit your session's tool allowlist and edit in `acceptEdits` mode.

The contrast: Liza prevents write collisions *structurally* (separate worktrees + locks + serialized merge). DW prevents them *by construction of the script* (partition the work so two agents don't fight over a file). When the script partitions cleanly, DW's approach is lighter. When it doesn't, Liza's worktree isolation is the safety net DW lacks.

---

## 8. Persistence & Recovery

**Liza** — Built for failure. Leases expire and tasks become reclaimable; crashed agents are recoverable (`liza recover-agent`, `liza recover-task`); the circuit breaker detects systemic failure; failure is a *state transition* (BLOCKED, REJECTED, SUPERSEDED) the supervisor records. Work persists across crashes and across sessions because it lives on the blackboard, not in any one process.

**DW** — Recovery is **session-scoped**. A run is resumable *within the same Claude Code session*: completed agents return cached results, the rest run live. But "if you exit Claude Code while a workflow is running, the next session starts the workflow fresh." There is no supervisor-owned task state that survives a process exit, no lease, no reassignment of an in-flight agent's work to another. Resilience comes from the work the agents commit (git, file writes), not from a recoverable orchestration record.

This is the sharpest operational divergence. Liza's whole reason for existing is that long-running autonomous work must survive crashes as supervised state. DW's resumability is a convenience within a live session, not a durability guarantee across them. For a one-shot batch run you babysit, that's fine. For unattended multi-day execution, it's the gap Liza was built to close.

---

## 9. Verification & Review

**Liza** — Externally validated completion. A coder cannot mark their own work complete; a separate reviewer examines it and issues a *binding verdict* via PR-like interaction. Approval means merge eligibility. Commit-SHA verification prevents reviewing stale state. A configurable review quorum with **provider-diversity preference** reduces single-model bias when diverse reviewers are available.

**DW** — Adversarial review *within the orchestration*. Independent agents work from different angles and others try to refute their findings; the script iterates until answers converge and folds in only what survives. `/deep-research` votes on each claim and drops the ones that fail cross-checking. The Bun port used "two reviewers on each file."

Both systems independently arrived at "don't trust a single pass; have an independent agent challenge it." Same core mechanism. The differences are durability, enforcement, and provenance:
- **Durability**: Liza's verdict is recorded on the blackboard and survives crashes — the approved/rejected state is a persistent fact. DW's cross-check results live in script runtime variables and vanish on session exit.
- **Enforcement**: Liza's review *is* the gate — the supervisor will not merge without it, mechanically. DW's review improves the answer but nothing external blocks acceptance — the script self-judges.
- **Provenance**: DW's reviewers are also Claude (same model family). Liza can deliberately route review to a *different provider* to break correlated blind spots — a guarantee DW structurally cannot offer, since it's Claude-only.

They're the same idea at two levels of guarantee. DW's review makes the output better. Liza's review makes acceptance conditional, durable, and optionally cross-model.

---

## 10. Human Role

**Liza** — The human owns intent and acts as observer/circuit-breaker. Within a sprint, agents are fully autonomous. Between sprints, the system pauses at checkpoints — but **human review is optional, not mandatory**. With `config.auto_resume: true` (toggled via the TUI `[y] yolo` shortcut), checkpoints and sprint completions are no longer human gates; the system rolls forward continuously. The human can intervene at any time via the blackboard, `liza pause`, or the kill switch — but doesn't have to. Trust rests on the mechanically enforced adversarial review and merge gates, not on the human watching.

**DW** — The human approves the **plan once**, then watches. Before a run, Claude shows the planned phases and asks for approval (with options to view or edit the raw script, and per-permission-mode variations on how often you're asked). During the run there is **no mid-run user input** — only an agent's own permission prompt (an un-allowlisted shell command, web fetch, or MCP tool) can pause it. For sign-off between stages, the docs advise running each stage as its own workflow. You can pause, resume, stop an agent, or stop the run from `/workflows`.

Both keep the human at the boundary rather than in every step, and both can run with no human involvement once launched. The difference is what happens when the human is absent: Liza's mechanically enforced doer/reviewer gates and durable blackboard continue to guarantee review quality and merge integrity — the adversarial structure holds whether or not a human is watching. DW's trust rests on the adversarial cross-check inside the script, which is real but ephemeral and self-judged. Liza's human is optional *because the mechanical gates are sufficient*; DW's human is optional *because you trusted the plan up front*.

---

## 11. Repeatability & Reuse

**DW** — Because the plan is a script, a run that did what you wanted can be **saved as a command** (`s` in `/workflows`) to `.claude/workflows/` (shared via the repo) or `~/.claude/workflows/` (personal), then invoked as `/<name>` forever after. Saved workflows accept input via an `args` global, so the same orchestration parameterizes over different targets (a question, a path list, issue numbers). "Codify the orchestration, then rerun it" is a clean model.

The value of reusable orchestration depends on the domain. For **operational tasks** — recurring audits, periodic security sweeps, regression hunts, deployment checklists — saved workflows are a clear win: the shape of the work is the same each time, only the target changes. For **software engineering** — implementing features, fixing bugs, evolving architecture — tasks rarely repeat identically. Each feature has different requirements, each bug has different root causes, each refactor has different structural constraints. A saved "implement a feature" workflow would be too generic to be useful; the decomposition *is* the work, not a reusable template. DW's repeatability is strongest in the operations/maintenance envelope and weakest in the greenfield-engineering envelope.

**Liza** — The pipeline *shape* is repeatable by construction (same role-pair sequence every sprint), while the *task graph inside it* is dynamically decomposed per goal. The shape repeats; the content doesn't — each goal produces a different topology of tasks through the adversarially-reviewed `output[]` mechanism. This is the right trade-off for software engineering, where the *process* (spec → plan → code → review → integrate) is stable but the *content* of each task is unique. Liza has no equivalent of saving an ad-hoc orchestration as a reusable command — but for its primary use case (feature development), the need is also smaller.

---

## 12. Provider & Platform Support

**Liza** — Multi-provider by design: Claude Code, Codex, Gemini, Mistral. The behavioral contract is explicitly a *capability filter* — it requires meta-cognitive machinery, so weaker models fail it as a capability test. Provider diversity is a feature (diversity-preferring review quorum); it's also a constraint (only contract-capable models qualify).

**DW** — Claude-only, but broad in *deployment surface*: CLI, Desktop, VS Code extension, headless `claude -p`, and the Agent SDK; backed by the Anthropic API, Bedrock, Vertex AI, and Foundry. There is no notion of provider diversity because every agent is Claude. Org controls exist (`disableWorkflows` in managed settings, an admin toggle, `CLAUDE_CODE_DISABLE_WORKFLOWS`).

Liza buys cross-provider robustness at the cost of a narrower usable-model set and external orchestration code. DW buys deep first-party integration and zero orchestration code at the cost of single-vendor, single-model-family coupling. If correlated model failure is your worry, Liza's diversity is the answer; if first-party integration and reach are your worry, DW's platform spread is.

---

## 13. Scale & Cost

**DW** — Designed for bursty, high-width runs: 16 concurrent / 1,000 total agents per run, with the caps explicitly framed as cost and runaway-loop bounds. The docs are candid that a run "can use meaningfully more tokens than working through the same task in conversation," recommend trial runs on a small slice, surface per-agent token usage live in `/workflows`, and suggest routing non-critical stages to a smaller model. Cost counts against your plan's usage and rate limits.

**Liza** — Cost is spread across long-lived role agents and is dominated by the always-loaded behavioral contract (every agent session pays for it) plus the multi-sprint lifecycle. Liza manages this with explicit context tiers (Full → Working Set → Kernel) and token-optimization analysis at sprint boundaries, not with a per-run agent cap. The cost shape is *sustained* rather than *bursty*.

Two different cost profiles: DW is a large, bounded, one-shot spend you can preview and cap; Liza is an ongoing spend you tier and monitor across a sprint. Neither is cheap; they're expensive in different rhythms.

---

## 14. Maturity & Adoption

**Liza** — `v0.8.0`, single primary author, created January 2026, Apache 2.0. Self-implementing since v0.4.0. Small but real adoption; design coherence from one author, with the corresponding single-maintainer continuity risk.

**DW** — **Research preview**, first-party Anthropic, requiring a recent Claude Code (v2.1.154+). Backed by Anthropic's distribution and engineering. The headline proof point is the Bun Zig→Rust port. "Research preview" is the honest caveat: the surface (keywords, caps, UI) is explicitly still moving — e.g. the trigger keyword changed from `workflow` to `ultracode` around v2.1.160.

The asymmetry is stark and obvious: a solo open-source project versus a vendor feature shipped by the company that makes the underlying model. That asymmetry is exactly why the substrate/trust differences below matter more than adoption counts — Liza's reason to exist is not to out-distribute Anthropic but to provide guarantees DW's design doesn't target.

---

## 15. Auditability & Continuous Improvement

**Liza** — Full audit trail as a design feature, not an afterthought. The blackboard records every state transition, assignment, verdict, rejection reason, and rescoping event — a durable, queryable history of what each agent did and why. Agent logs capture the full session. Two analysis skills operate on this trail: `/liza-logs` analyzes agent sessions at sprint boundaries (anomaly patterns, token usage, behavioral signals), and `/context-engineering` analyzes the prompt/output corpus (context budget use, duplicated or missing context, prompt drift, tool-output pressure, handoff quality). Sprint checkpoints and retrospectives feed findings back into the next sprint's configuration. The audit trail enables *continuous improvement*: each sprint teaches the system about itself, and the lessons are actionable because they're grounded in durable evidence, not in recall of a conversation that no longer exists.

**DW** — Observability is **live-run only**. The `/workflows` view shows phases, agent counts, per-agent token totals, and elapsed time as the run progresses, and you can drill into any agent's prompt, tool calls, and result. This is useful for monitoring a run in progress and for cost control. But when the run completes (or the session exits), the orchestration record is gone — there is no durable log of what each agent found, what was cross-checked, or what was filtered out. The script file persists (you can read and diff it), but the agent results do not. There is no equivalent of sprint-boundary analysis, prompt/output corpus analysis, or structured retrospectives that feed back into the next run.

The gap is stark: DW lets you watch a run; Liza lets you learn from it. For a one-shot batch, live monitoring may suffice. For sustained autonomous execution where each sprint should be better than the last, auditable history and analysis tooling are the mechanism that closes the improvement loop.

---

## 16. Operating Model — Cross-Dimension Synthesis

**Execution substrate** — Liza is an external runtime: a supervisor binary, blackboard state, locks, leases, worktrees, validation, recovery, merge authority. DW is an internal runtime feature: an LLM-written script executed in an isolated environment by the agent CLI itself. Liza wraps the agent in control machinery from outside; DW grows orchestration out of the agent from inside.

**Context economy** — Both deliberately keep intermediate results out of the orchestrator's context window (Liza on the blackboard; DW in script variables) — a shared insight. They differ on what *else* loads context: Liza pays for the behavioral contract in every agent; DW pays for the model writing and reasoning about the orchestration script.

**Failure ownership** — Liza routes failure into machine-visible, persistent states (BLOCKED, REJECTED, SUPERSEDED, crash recovery, circuit breaker) that survive process death. DW routes failure into in-session resumability and the work agents have already committed; a session exit loses the orchestration record. Liza makes failure a durable state transition; DW makes it a live-run condition.

**Trust model** — Both rely on adversarial verification as the core quality mechanism. The difference is what backs it. Liza's adversarial review is durable (on the blackboard), mechanically enforced (the supervisor blocks merge without approval), and optionally provider-diverse. DW's adversarial review is ephemeral (in script variables), self-judged (the script decides what passes), and single-model (Claude reviewing Claude), with file edits auto-approved once the run starts. Same mechanism; Liza's version is strictly stronger in enforcement, durability, and diversity.

**Auditability & learning loop** — Liza's blackboard, agent logs, and analysis skills (`/liza-logs`, `/context-engineering`) form a closed improvement loop: each sprint's execution is auditable after the fact, and the findings feed forward into the next sprint's configuration. DW's observability is live — you can watch per-agent progress and drill into any agent during the run — but when the run ends, the agent results vanish. There is no post-run audit trail, no structured analysis tooling, and no mechanism to feed findings from one run into the next. Liza lets you *learn from a run*; DW lets you *watch one*.

---

## 17. Where They Overlap (The Convergence)

This is the most striking comparison in the folder, because Anthropic independently shipped Liza's two foundational ideas:

- **Parallel multi-agent fan-out** as the way to handle work too big for one pass.
- **Dynamic task decomposition** — both break a goal into subtasks at runtime rather than requiring a human to pre-enumerate them (Liza via the Orchestrator + doer `output[]`; DW via Claude writing the script).
- **Adversarial cross-review** — independent agents challenging each other's findings — as the way to raise trust above a single pass.
- **Plan/results held outside the orchestrator's context window** — Liza on the blackboard, DW in script variables — so coordination scales without context blowup.
- **A human boundary at the edges, not in every step** — both prefer approve-then-observe over per-action queues.
- **Background, non-blocking execution** while the human stays free to work (Liza via detached agent processes + TUI; DW via the background runtime + `/workflows`).
- **Cost awareness for multi-agent runs** as a first-class operational concern.

That a frontier lab converged on the same primitives Liza built is the strongest external validation of Liza's architecture this folder records. The divergence is not *whether* to fan out and cross-review, but *what enforces the result and whether it survives a crash*.

---

## 18. Framework Failure Modes

**Liza** fails when:
- The work is a single large batch operation (one 500-file migration, one codebase-wide audit). The full MAS pipeline, worktrees, and multi-sprint machinery are heavier than the job needs — this is precisely DW's sweet spot. (Liza's adversarial-pairing skill offers a lightweight alternative that preserves doer/reviewer separation without MAS overhead, but it lacks DW's massive fan-out.)
- The model can't hold the contract. Provider choice is narrowed to contract-capable models.
- Requirements are unclear upstream. No native product discovery; thin vision in, thin decomposition out.
- Operational setup cost is disproportionate for a one-shot task — but this applies only to MAS mode (multiple terminals, worktrees, provider credentials). Pairing mode and the adversarial-pairing skill require no infrastructure beyond the agent CLI itself.
- Single-maintainer risk: no formal contribution pipeline or continuity guarantee.

**DW** fails when:
- Work must survive a crash or session exit as supervised state. Resumability is same-session only; exit starts fresh. No leases, no reassignment, no recover protocol — only what agents have committed persists.
- Mechanical enforcement of merge correctness is needed. Review is a script-level filter, not an enforced gate; subagents auto-approve file edits once the run is allowed. Nothing external blocks a bad result from landing.
- Provider diversity is the point. Every agent is Claude; correlated model blind spots can't be broken by routing review to a different vendor.
- Mid-run human steering is needed. No mid-run input by design; redirection means re-running stages as separate workflows.
- The orchestration itself must be audited and constrained ahead of time. The plan is generated per run by the model; you can read and edit it, but it is bespoke code, not a fixed, reviewed state machine.
- Sustained, multi-day autonomous execution across many tasks is the goal — DW is a burst primitive, not a long-running supervisor.
- Agents exhibit LLM failure modes — sycophancy, phantom fixes, test corruption, scope creep, hallucinated completions. DW has no behavioral contract, no tiered invariant system, and no mechanical countermeasures for the 55+ documented failure modes Liza addresses. Adversarial cross-review inside the script may catch *some* of these (one agent refuting another's phantom fix), but nothing *prevents* them structurally — a sycophantic reviewer will agree with a sycophantic doer, especially when both are the same model family. The absence of failure-mode-specific defenses is the deepest trust gap between the two systems.

These are boundary conditions, not bugs. DW is strong inside "one large batch, babysat to completion in a session." Liza is strong inside "sustained, crash-surviving, externally-gated autonomous execution across sprints."

---

## 19. Where They Diverge Most

The fundamental difference is **what authors and enforces the orchestration**.

**DW** lets the model author the plan as a script and trusts adversarial cross-review plus up-front approval to keep it honest. Flexibility is maximal — a fresh, task-shaped orchestration every time. Durability and external enforcement are minimal — the plan lives in a runtime that resets on exit, and the only hard gate is before launch. DW optimizes for *tackling one enormous task in a single, transparent, parallel burst*.

**Liza** keeps the pipeline *shape* as a declared state machine and mechanically enforces trust at every transition through ~35k LOC of Go and a tiered invariant contract — but the *task graph within the pipeline is dynamically decomposed* by agents, with each decomposition adversarially reviewed before it fans out. Per-run flexibility is lower than DW (you can't invent a novel orchestration shape on the fly); structural guarantees and decomposition quality are higher (every subtask definition passes through a reviewer gate). Durability and enforcement are maximal — state survives crashes, and nothing merges without an enforced external verdict, optionally from a different provider. Liza optimizes for *sustained autonomous execution that produces production-grade, externally-reviewed code with an audit trail*.

DW asks "how do we let Claude coordinate hundreds of copies of itself to crush a task it can't do in one pass?" Liza asks "how do we run distrusted agents autonomously without them merging broken or fabricated work?" Same primitives (fan-out, cross-review); opposite design centers (model-authored flexibility vs code-enforced guarantees).

---

## 20. Layering & Integration — Can Liza Ride on DW?

Unlike gstack/BMAD, DW is not a competing *layer* — it's a capability of an agent CLI Liza already drives. That opens an integration question the other comparisons don't have.

**DW as a fan-out primitive inside a Liza task.** A Liza coder agent (running Claude Code) could, in principle, invoke `ultracode` to fan out a within-task batch operation — a mechanical refactor across many files, a within-worktree audit — while remaining a single Liza task with a single lease, reviewed by a separate Liza reviewer and merged through the supervisor. DW would handle *intra-task width*; Liza would retain *inter-task state, isolation, review authority, and recovery*. This is the cleanest fit: DW's burst parallelism beneath Liza's durable supervision.

**Constraints that integration must respect.** DW's subagents run in `acceptEdits` with auto-approved file edits. Inside a Liza worktree that is acceptable *because the worktree is the isolation boundary and the Liza reviewer is still the merge gate* — the DW run produces a candidate diff, not a merge. The non-negotiable is that any DW-produced change still passes through Liza's enforced verdict and merge protocol; DW must not become a path that lands code without external review. DW also can't satisfy Liza's provider-diversity review (it's Claude-only), so a diversity-preferring reviewer should still sit above it.

**What does *not* compose.** DW cannot replace Liza's blackboard, leases, or crash recovery — its state is session-scoped. And Liza's pipeline cannot be reduced to a saved DW script without losing the supervisor's enforcement. The layering is "DW inside a Liza task," not "Liza expressed as DW."

---

## 21. What Each Could Steal

### What Liza could steal from DW

1. **Orchestration-as-savable-artifact for operational tasks.** DW's "save a successful run's script as a `/command`, parameterized by `args`" is a clean model for recurring operational work (audits, security sweeps, migration patterns). For software engineering — Liza's primary use case — the value is limited because tasks rarely repeat identically. But Liza could benefit from savable playbooks for the operational envelope (codebase-wide lint fixes, dependency upgrades, periodic hardening passes), flowing through the supervisor's gates.

2. **Massive within-task fan-out for batch operations.** Liza's strength is durable, few-and-long-lived agents; it has no answer for "edit 500 files in parallel inside one task." Borrowing DW's bounded burst model (concurrency cap + total cap + live per-agent token visibility) as an *intra-task* tool would let a single Liza task crush batch work it currently serializes.

3. **Pre-flight cost preview on a slice.** DW's advice to trial a workflow on one directory before the whole repo, with live per-agent token totals and a hard agent cap, is a pragmatic cost-control pattern Liza could surface for expensive sprints.

4. **A readable, diffable plan artifact.** DW writes its orchestration to a file you can read and diff between runs. Liza's plan is spread across YAML config and Go; a single human-readable "this is the orchestration this sprint will run" artifact (even generated) would aid auditability.

### What DW could steal from Liza

1. **Durable, crash-surviving orchestration state.** DW's session-scoped resumability is its biggest operational gap. A persisted run record — surviving session exit, with completed-agent results reattachable — would turn DW from a babysit-to-completion burst into something closer to unattended execution. Liza's blackboard + lease model is the reference design.

2. **An enforced merge gate, not just a cross-check.** DW raises answer quality with adversarial review but auto-approves edits and lets the script decide acceptance. A mechanical gate — *no agent-produced change merges without an independent verdict* — would close the "visibly compliant but broken, and nothing blocked it" failure mode. This is the heart of what Liza enforces.

3. **Provider-diversity review.** DW's reviewers are all Claude, so correlated blind spots survive cross-checking. Allowing a review stage to route to a different model family would make adversarial review genuinely adversarial across architectures.

4. **A tiered invariant model for what halts vs warns.** DW has plan approval and agent caps, but no explicit hierarchy of which conditions must halt a run unconditionally versus merely warn. Liza's Tier 0–3 model is a clear template for deciding, in a thousand-agent run, what is allowed to fail quietly and what must stop everything.

5. **Mid-run intervention beyond stop.** DW allows pause/stop but no redirection. Liza's blackboard lets an operator change course mid-flight without killing the run. Even a constrained "inject guidance for not-yet-started agents" channel would soften DW's all-or-nothing batch stance.

6. **A durable audit trail and analysis tooling.** DW's per-agent results vanish when the run completes. Persisting agent findings, cross-check outcomes, and filtered-out claims into a queryable log would make post-run analysis possible. Liza's agent logs + `/liza-logs` and `/context-engineering` skills are the reference: they close the loop between "what happened in this run" and "how do we make the next one better."

---

## 22. Bottom Line

DW is the comparator that most validates Liza's thesis and most pressures its niche at once. It validates the thesis because Anthropic, building from scratch, reached for the same primitives — parallel fan-out and adversarial cross-review with the plan held outside the context window. It pressures the niche because for *one large, well-bounded batch task you'll babysit to completion*, DW does inside Claude Code, with zero setup and first-party integration, much of what you'd otherwise stand up Liza for.

Liza's durable advantages are exactly where DW's design stops: **adversarial review backed by a durable blackboard and mechanical enforcement rather than ephemeral script self-judgment; state that survives a crash; provider-diversity review; an auditable history that enables continuous improvement; and sustained multi-task autonomy.** The sharpest strategic read is not "Liza vs DW" but "Liza *over* DW" — let DW be the burst-parallelism primitive inside a Liza task, and keep Liza as the supervisor that owns state, isolation, review authority, auditability, and recovery. The best lesson from DW is not "let the model write the orchestration freely." It is that a successful orchestration is an artifact worth capturing and rerunning — and the best Liza-shaped version of that lesson keeps the captured orchestration behind the same mechanical gates, rather than trusting a generated script to govern itself.
