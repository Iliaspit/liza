# Liza Competitive Survey — March-April 2026

## Landscape Overview

The multi-agent coding space has evolved rapidly since Liza's first release. The field now splits into
seven distinct categories, each solving a different problem. Liza sits in a category of one.

**General-purpose agent orchestration frameworks** (CrewAI, LangGraph, AutoGen, Semantic Kernel) provide
building blocks for assembling multi-agent workflows across any domain. They optimize for flexibility and
developer experience. None address behavioral trust in software engineering.

**Software company simulators** (MetaGPT/MGX, ChatDev, AgentMesh) encode SOP-based pipelines that mimic
software teams: Product Manager → Architect → Engineer → QA. They optimize for structured output generation.
Trust is assumed through process compliance.

**Scheduler/runners** (Symphony, Paperclip) sit above coding agents and manage work dispatch, workspace
isolation, and project-level orchestration. They optimize for operational coordination. Trust is delegated
to whatever happens inside each agent session.

**Context-engineered development systems** (GSD) treat context window management as the primary engineering
challenge. Thin orchestrators spawn fresh subagents for every operation, plans are sized to context budgets
before execution, and state lives in human-readable files. Trust derives from process structure
(spec → plan → execute → verify) and context freshness, not behavioral enforcement.

**Methodology / workflow frameworks** (BMAD-METHOD) install a multi-phase agile methodology into AI IDEs
(Claude Code, Cursor, Codex, Copilot): named agent personas guide the human through brainstorming, PRFAQ,
PRD interview, architecture, stories, and code review. Each workflow runs in a fresh chat session. Trust
comes from structured process, context engineering, and a three-layer adversarial code review — all at the
prompt level, with no mechanical merge gate.

**AI engineering workflow suites** (gstack) install broad specialist skill packs and power tools into
AI coding assistants. They optimize for solo-builder leverage across product discovery, design, code
review, QA, security, browser automation, and release. Trust comes from expert prompt depth, outside
voices, and selected host hooks, not from supervisor-owned task state or merge authority.

**Behavioral enforcement systems** (Liza). One entry. A hybrid hardened architecture: deterministic
Go supervisors enforce state transitions, role boundaries, merge authority, and TDD gates mechanically,
while LLM agents handle judgment under a behavioral contract addressing 55+ failure modes. One orchestrator
plus 12 other roles span three pipeline phases (specification → coding → integration), organized as adversarial doer/reviewer pairs
with configurable review quorum and provider-diversity preference/gates. Declarative YAML pipeline schema
drives role context, skills, permissions, and prompt construction. Checkpoint-gated phase transitions
ensure human review of planning output before coding begins. Optimizes for trust through mechanical
constraint of agent failure modes.

---

## Direct Competitors

### BMAD-METHOD

**What it is**: "Breakthrough Method for Agile AI-Driven Development" — npm-distributed methodology framework installed into AI IDEs (Claude Code, Cursor, Codex, Copilot). JavaScript/Node.js, MIT licensed. ~45.2k stars, ~5.4k forks, 132 contributors, 1,826 commits, 29 releases as of 2026-04-20. Trademarked by BMad Code, LLC.

**Current state (April 2026)**: V6.3.0 shipped April 10, 2026 — consolidated three previous agents (Barry/Quick-Flow, Quinn/QA, Bob/SM) into a single Developer persona (Amelia). Active Discord, YouTube channel, docs in 5 languages (EN, FR, ZH, CS, VI). Module ecosystem: BMM core, BMad Builder, Test Architect, Game Dev Studio, Creative Intelligence Suite. Corporate sponsorship. Rapid iteration.

**Philosophy**: Quality comes from methodology and context engineering. "AI tools do the thinking for you, producing average results; our agents are expert collaborators who guide you through a structured process." Each phase produces documents that inform the next. Grounded in agile methodology and Amazon's Working Backwards.

**Trust model**: Structured process + context engineering. `bmad-code-review` runs three parallel adversarial reviewers (Blind Hunter with diff-only context, Edge Case Hunter with project read, Acceptance Auditor with spec) and triages findings into decision-needed / patch / defer / dismiss. Implementation readiness check returns PASS/CONCERNS/FAIL before implementation. But enforcement is prompt-level — no mechanical gate prevents merge without approval.

**Where it falls short vs Liza**:
- No code-enforced role boundaries (prompt-level only)
- No failure mode catalog or academic failure-mode grounding
- No mechanical merge gate — reviewers are advisory, not binding
- No crash recovery (fresh-chat-per-workflow loses mid-task state if session dies)
- Sequential by design; no concurrent multi-agent execution on the same codebase
- No inter-workflow state; planning artifacts are re-read at each workflow entry (token cost)

**What's interesting about it**:
- Upstream planning breadth (brainstorming, market/domain/technical research, PRFAQ, PRD interview, UX design) is deeper than Liza's goal-document entry point — BMAD's Analysis phase reaches altitudes above Liza's vision input
- Three-layer parallel adversarial review is the most developed prompt-level adversarial mechanism surveyed
- Triage buckets (decision-needed / patch / defer / dismiss) are cleaner than binary approve/reject
- Scale-adaptive tracks (Quick Flow / BMad Method / Enterprise) match planning depth to project complexity

**Market position**: Closest philosophical neighbour. Same domain (software engineering with multi-agent coordination), opposite architectural bet. BMAD trusts agents when given good methodology; Liza mechanically constrains them. Liza's README acknowledges BMAD for "role templates and workflow patterns." Octo Technology places both at L4 maturity. Architecturally complementary — BMAD's upstream methodology (analysis, PRD, architecture, UX) feeds naturally into Liza's downstream execution (spec decomposition, adversarial coding, mechanical review, supervised merges). [Full comparison](liza-vs-bmad-comparison.md).

---

### gstack

**What it is**: Garry Tan's AI engineering workflow suite: 52 generated slash-command skills plus
Bun/TypeScript helper tools and a persistent Chromium browser daemon. It installs into Claude Code,
Codex CLI, OpenCode, Cursor, Factory Droid, Slate, Kiro, Hermes, GBrain, and related hosts. MIT
licensed, ~100.7k stars / ~15k forks as of 2026-05-22, v1.43.2.0 in the local shallow clone.

**Current state (May 2026)**: Extremely high adoption velocity, helped by Garry Tan's distribution
as YC CEO. The repo ships product/planning skills (`/office-hours`, `/plan-ceo-review`,
`/plan-eng-review`, `/autoplan`), implementation and review skills (`/review`, `/codex`,
`/investigate`), browser and QA workflows (`/browse`, `/qa`, `/qa-only`), design workflows
(`/design-shotgun`, `/design-html`, `/design-review`), release/deploy workflows (`/ship`,
`/land-and-deploy`, `/canary`), memory (`/learn`), and safety guards (`/careful`, `/freeze`,
`/guard`). The browser daemon is the most substantial hard subsystem.

**Philosophy**: "A single person with AI can now build what used to take a team of twenty." gstack's
ETHOS.md centers on Boil the Lake, Search Before Building, and User Sovereignty. Quality comes from
giving one active human a large bench of specialist workflows and fast feedback loops.

**Trust model**: Prompt-level methodology plus selected mechanical hooks. `/freeze` blocks Edit/Write
outside a directory, `/careful` warns on destructive Bash patterns, the browser daemon has token and
tunnel isolation, and review skills dispatch specialist subagents / Codex outside voices. But there is
no blackboard, lease, supervisor-owned task state, or mechanical merge gate.

**Where it falls short vs Liza**:
- No code-enforced role boundaries across task lifecycle
- No supervisor-owned task state, leases, or crash recovery
- No isolated worktrees or merge authority separated from the active session
- No binding doer/reviewer state machine; review is advisory even when rigorous
- Subagents and Codex outside voices contribute findings, but the parent session decides
- Safety hooks are host/tool-surface dependent (`/freeze` does not constrain Bash writes)

**What's interesting about it**:
- Best-in-class browser/QA substrate among surveyed systems
- Generated skill docs (`SKILL.md.tmpl` -> `SKILL.md`) reduce drift between tool commands and agent instructions
- Declarative host adapter configs are a clean pattern for multi-host prompt/tool adaptation
- Product/design/DX/release workflow breadth reaches above and below Liza's current MAS pipeline
- Per-project learnings and design taste memory are pragmatic, if subordinate to durable specs

**Market position**: High-traction adjacent/direct competitor. Like BMAD, gstack is prompt/workflow-first,
but broader and more tool-heavy; unlike GSD, it optimizes human-in-the-loop solo leverage rather than
context-budgeted autonomous execution. Architecturally complementary: gstack's browser, QA, design, and
release capabilities could sit behind Liza's supervisor gates. [Full comparison](liza-vs-gstack.md).

---

### MetaGPT / MGX

**What it is**: Python framework (64k+ stars) encoding standardized operating procedures into multi-agent
pipelines. Five fixed roles (Product Manager, Architect, Project Manager, Engineer, QA). Commercial
evolution is MGX, a hosted no-code platform launched Feb 2025.

**Current state (March 2026)**: Open-source repo hasn't had a release since v0.8.1 (April 2024). Team
energy shifted entirely to MGX. Product Hunt reviews are polarized — praise for fast prototyping, recurring
complaints about instability, data loss, weak context handling, and costly credit burn. MGX recently
rebranded to "Atoms." The promised MetaGPT v1.0 open-source release hasn't materialized.

**Philosophy**: `Code = SOP(Team)`. Encode how a software company works and agents will follow the process.

**Trust model**: Structured outputs as trust proxy. Each agent produces formatted artifacts (PRDs, UML,
API specs). Executable feedback loop retries up to 3 times on failure. No behavioral enforcement —
the system assumes agents follow the SOP if described clearly enough.

**Where it falls short vs Liza**:
- No code-enforced role boundaries (prompt-level only)
- No failure mode catalog or countermeasures
- No provider compatibility testing
- No anti-gaming mechanisms
- No crash recovery or multi-sprint continuity
- No context pressure management
- Retry-based failure handling (same agent tries again) vs structural prevention

**What Liza took from it**: Structured artifact requirements between roles are valuable. The spec phase
pipeline formalizes this. Academic positioning (ICLR oral) provides legitimacy. The MGX model validates
the market for a hosted enterprise product.

**Market position**: Low direct overlap. MGX targets no-code web app generation, not enterprise
software engineering. The open-source project appears stalled. Brand recognition (64k stars,
ICLR paper) keeps it visible.

---

### CrewAI

**What it is**: General-purpose multi-agent orchestration framework (45k+ stars, 100k+ certified
developers, VC-backed). Two primitives: Crews (autonomous role-playing agents) and Flows (event-driven
deterministic workflows). Python, YAML-based.

**Current state (March 2026)**: Active development — v1.9.0 latest stable, regular releases. Added
Agent-to-Agent (A2A) task execution, MCP server integration, structured output support. Enterprise
product (AMP Suite) offers tracing, observability, control plane. AWS Prescriptive Guidance integration.
Strong ecosystem with DeepLearning.ai courses and enterprise relationships.

**Philosophy**: Agent-centric. Design agents with roles, goals, backstories. Let them collaborate
through natural language. Flexibility and developer experience above all.

**Trust model**: Optimistic with retry. Guardrails are post-hoc validation functions (Python or
LLM-based string checks) that run after agent output. Retry up to N times on failure. Hierarchical
manager mode documented but doesn't work as intended — executes sequentially, not as true delegation
(confirmed by independent Towards Data Science analysis).

**Where it falls short vs Liza**:
- Guardrails are output-oriented, not behavior-oriented
- Cannot detect process violations (test mutation, scope creep during execution)
- No failure mode catalog
- No code-enforced role boundaries
- No provider compliance testing
- "Managerial overhead" measured at ~3x tokens for simple tasks vs LangChain
- No context pressure management or crash recovery

**What Liza took from it**: Flows architecture (deterministic backbone + autonomous agents) is the
right production pattern. YAML-based definition lowers barrier to entry. AMP Suite's
tracing/observability is a good model for enterprise positioning. A2A task delegation is worth watching.

**Market position**: Strong in general-purpose orchestration. Most likely framework enterprise teams
try first for any multi-agent use case. When they attempt serious software engineering and hit
behavioral failures, that's Liza's entry point. The competitive risk is if CrewAI adds domain-specific
guardrails for coding — but their general-purpose philosophy works against this.

---

### OpenAI Symphony

**What it is**: Elixir-based scheduler/runner released March 2026 (engineering preview). Polls Linear
for issues, creates per-issue workspace directories, spawns Codex agents. Built on BEAM/OTP for
fault tolerance and concurrency. Open source (Apache 2.0 initially reported, some sources say MIT).

**Current state (March 2026)**: Just released. Engineering preview — explicitly not production-ready.
Reference implementation in Elixir, but the spec (SPEC.md) is designed for multi-language implementation.
Active buzz on tech media. Positions itself as "scheduler, runner, tracker reader" — deliberately
narrower than a full orchestration framework.

**Philosophy**: "Manage work, not agents." Transform project management into automated execution.
Harness engineering — make codebases legible to agents, then let agents run autonomously per issue.
WORKFLOW.md keeps agent policy versioned with code.

**Trust model**: Minimal. "Proof of Work" concept (CI passes, walkthroughs before merge) but the spec
explicitly says it "does not require a single approval, sandbox, or operator-confirmation policy."
Trust posture is implementation-dependent and must be documented. No behavioral enforcement.
Agent self-certifies — no review loop, no role pairs.

**Where it falls short vs Liza**:
- No code review loop (agent self-certifies)
- No behavioral contract or failure mode awareness
- No role pairs (single-stage agent run per issue)
- In-memory state in some implementations (lost on crash)
- Codex-only in reference implementation
- No multi-sprint continuity
- No adversarial architecture

**Ideas worth adopting** (not yet implemented):
- Per-state concurrency limits (`max_concurrent_agents_by_state`) — cap reviewers vs coders
- Lifecycle hooks (pre_run, post_run, on_error) — maps to planned pre/post hooks
- Stall detection (agent alive but not progressing) — complements lease-based heartbeat
- Dispatch priority enforcement (sort candidates by priority before dispatch)

**Market position**: High in mindshare, low in direct competition. Symphony is OpenAI's answer to
"what do we build on top of Codex?" It will attract massive adoption through OpenAI's distribution.
But it's a scheduler, not an orchestration system — it doesn't solve trust, review, or behavioral
enforcement. The risk is that Symphony becomes the default starting point and teams never discover
they need what Liza provides. Liza positions as what you add when Symphony isn't enough.

---

### Paperclip

**What it is**: Open-source Node.js + React orchestration platform for "zero-human companies."
14k stars in days. Org charts, budgets, governance, goal alignment, agent coordination. Multi-tenant.
Agent-agnostic (Claude Code, Codex, Cursor, etc.).

**Current state (March 2026)**: Just launched, trending fast. Riding the "zero-person company" narrative.
Active community. MIT licensed. Embedded PostgreSQL. Well-structured repo.

**Philosophy**: "Your AI agents need a company, not better prompts." Models organizational structure —
org charts, budgets, approval gates, delegation chains. The corporate apparatus, minus the humans.
Completely unopinionated about agent runtimes.

**Trust model**: Governance-oriented. Budget caps (auto-pause at limit), approval gates (human operates
as "board of directors"), audit trails (append-only, no edits/deletions). But no behavioral enforcement
within agent sessions. Trust is organizational (who can do what, how much they can spend) not
behavioral (how they do the work).

**Where it falls short vs Liza**:
- No behavioral contracts or failure mode awareness
- No code review loop
- No role-specific enforcement
- No TDD gates or mechanical validation
- "Not a code review tool" (their own words)
- Trust is budget/approval-based, not execution-based
- No context pressure management

**What's interesting about it**:
- Cost tracking per agent, per task, per project, per goal — Liza's planned economic instrumentation
  should match or exceed this
- Budget enforcement with auto-pause — worth considering for sprint-level cost caps
- Multi-tenant with data isolation — relevant for enterprise product architecture
- "If it can receive a heartbeat, it's hired" — agent-runtime agnostic, like Liza

**Market position**: Low direct overlap (different domain — business operations, not software
engineering). High as a narrative competitor. The "zero-human company" framing captures
imagination. Liza's counter-narrative: "zero-trust agent sessions" matters more
for enterprise software than "zero-human companies."

---

### Ruflo

**What it is**: Multi-agent coding framework with 60+ specialized agent types (coder, tester, architect,
security-architect, etc.), 215+ MCP tools, Q-learning-based task routing, and HNSW-indexed
persistent memory. Uses Claude hooks for pre/post execution checks. ReasoningBank does
trigram/Jaccard similarity matching to find relevant past patterns and route tasks to agents
with the best track record.

**Philosophy**: Specialize and route. Many narrow agent types, each optimized for a specific task
category, with ML-based routing to select the best agent for each job.

**Trust model**: Track-record based. Q-learning routes tasks to agents with the best historical
performance. ReasoningBank matches patterns from past successes. Trust is empirical (past results)
rather than structural (enforced constraints).

**Where it falls short vs Liza**:
- No behavioral contract or failure mode catalog
- No code-enforced role boundaries (Claude hooks are provider-specific)
- No adversarial doer/reviewer pairs
- 60+ agent types creates context overhead that composable skills avoid
- 215+ MCP tools clog context
- Swarm topologies, Byzantine consensus, Q-learning routers solve coordination problems
  that Liza's blackboard architecture doesn't have

**Ideas adopted:**
- Planner hints (skills, docs per task) — implemented via declarative pipeline schema
  (`skills-affinity`, `mandatory-docs`, `context-sections` per role-pair)

**Ideas worth adopting** (not yet implemented):
- Per-task model selection — planner assigns models by complexity, better than static tier routing
- Mechanical pre-review linting — deterministic checks before reviewer spawns (generalized
  beyond Claude-specific hooks to model-agnostic shell scripts)
- Sprint Analyzer concept — capitalizing on patterns at sprint boundaries via lesson-capture

**What Liza skipped:** Swarm topologies, Byzantine consensus, Q-learning routers, HNSW indexing,
agent-writes-tracker pattern. Adding infrastructure complexity to a system whose core bet is
behavioral simplicity would be self-defeating.

**Market position**: Same domain (multi-agent coding), opposite architectural bet. Ruflo optimizes
for breadth (many specialized agents, many tools, ML routing). Liza optimizes for depth (fewer
roles, behavioral enforcement, composable skills). The test is which approach produces more
reliable output on real codebases.

---

### GSD (Get Shit Done)

**What it is**: Context engineering and spec-driven development system for AI coding assistants
(37k+ stars, v1.25+). 15 specialized agents across research, planning, execution, checking, and
verification phases. Node.js CLI (`gsd-tools.cjs`) for deterministic operations. File-based state
in `.planning/` directory. Multi-runtime: Claude Code, OpenCode, Gemini CLI, Codex, GitHub Copilot,
Antigravity. MIT licensed.

**Current state (March 2026)**: Mature and actively maintained — version 1.25.1 with frequent
releases. Strong community (37k stars, 3k forks). Claims adoption by engineers at Amazon, Google,
Shopify, Webflow. Active Discord. npm distribution (`get-shit-done-cc`).

**Philosophy**: Context engineering solves quality. "Context rot" — quality degradation as context
windows fill — is the primary enemy. Thin orchestrators (10-15% context usage) spawn fresh subagents
(clean 200K windows) for every significant operation. Crucially, "orchestrators" are themselves LLM
agents with focused prompts — not deterministic supervisors. The entire stack is LLM-on-LLM:
orchestrator agents delegate to subagents. This contrasts with Liza's Go-on-LLM architecture where
deterministic supervisors mechanically enforce guarantees that agents cannot bypass. (File-path
passing and plan-to-context sizing are standard practice shared by both systems, not GSD
innovations.)

**Trust model**: Spec-driven with deviation rules. Executor has 4 auto-correction rules (auto-fix
bugs, auto-add missing critical functionality, auto-fix blocking issues, stop for architectural
decisions). 3 attempts per task before documenting and moving on. Pre-execution plan verification
(`gsd-plan-checker`), post-execution goal-backward verification (`gsd-verifier` checks outcomes
exist, not just tasks completed). Analysis paralysis guard (>5 consecutive reads without edits =
stop). No behavioral contract or failure mode catalog — trust is structural (fresh contexts prevent
degradation) and procedural (spec → plan → execute → verify).

**Where it falls short vs Liza**:
- No behavioral contract or failure mode catalog
- No code-enforced role boundaries (agent roles are prompt-level)
- No adversarial doer/reviewer pairs (checker/verifier are separate agents, not adversarial pairs
  reviewing the same work)
- LLM-on-LLM architecture — orchestrators are LLM agents, not deterministic supervisors.
  No hard trust boundary between orchestrator and subagent. Orchestrator can hallucinate,
  lose track, or deviate from its own coordination protocol
- Deviation rules are execution-time heuristics, not structural prevention
- No provider compliance testing (supports many runtimes but doesn't test behavioral compliance)
- "3 attempts then document and move on" — retry-based, not escalation-based
- No multi-sprint continuity (session-scoped, not sprint-scoped)
- Context engineering prevents degradation but doesn't prevent behavioral failure modes
  (scope creep, test mutation, authority violation) that occur with fresh context

**Ideas worth adopting** (not yet implemented):
- Multi-runtime transformation engine — single source definition automatically transformed for
  each runtime. Liza's `liza init` already adapts to runtimes but could learn from GSD's
  transformation approach

**What Liza skipped:** Developer profiling, automatic deviation rules. (GSD's "analysis paralysis
guard" — stop after >5 reads without edits — is a prompt-level heuristic. Liza's equivalent is
mechanical: the supervisor counts exit-42 restarts without progress, and hypothesis exhaustion
triggers planner rescoping after two coders fail the same task.) GSD adds orchestrator-level
context management (thin orchestrators, context budgets,
subagent spawning within workflows). Liza's single-task agents already get fresh context
by design — the problem GSD solves at the orchestrator layer doesn't arise.
Both are valid — but behavioral failures occur with fresh context too.

**Market position**: Highest-traction direct competitor (37k stars vs Liza's early stage). Same
domain (multi-agent software engineering), complementary core bet. GSD solves "context degrades
within long sessions" (context engineering for orchestrators and multi-step workflows). Liza
solves "agents misbehave even when fresh" (behavioral enforcement) — and Liza agents are already
short-lived (single task), so the context rot problem GSD targets is less acute. GSD's multi-runtime
support and npm distribution give it strong developer reach. The competitive risk is GSD adding
behavioral enforcement on top of context engineering. The opportunity is that GSD's architecture
(thin orchestrators, file-based state) would make it a natural consumer of Liza's supervision layer.

---

## Adjacent Frameworks (Not Direct Competitors)

### LangGraph

State-machine based framework from LangChain. Closest to Liza's architectural philosophy (explicit
states, transitions, conditional routing). Production-grade, model-agnostic. But general-purpose —
no software engineering domain expertise, no behavioral contracts. Used as infrastructure by teams
building custom agent systems. No threat as a product competitor; potential as infrastructure
Liza could build on (but Liza's Go CLI approach is deliberately simpler).

### AWF (AI Workflow Framework)

Go CLI for declarative YAML workflow orchestration. State-machine execution with conditional
transitions, parallel steps, loops, retries, and sub-workflow composition. Provider-agnostic
(Claude CLI, Gemini, Codex, OpenAI-compatible, Ollama). RPC-based plugin system. EUPL-1.2.
8 stars, single maintainer, early stage.

Audit-first trust model — dry-run preview, interactive step-by-step approval, JSONL audit trails
with secret masking. No sandbox, no behavioral enforcement. Agents execute shell commands with
user privileges; workflows are deterministic orchestration, not autonomous collaboration.

Architectural DNA shared with Liza (Go CLI, YAML-driven, state machines, provider-agnostic) but
different domain. AWF orchestrates arbitrary AI workflows; Liza enforces behavioral trust in
software engineering. AWF's workflow-as-state-machine pattern validates the approach but applies it
to task automation, not agent governance. No direct competitive overlap.

### AutoGen (Microsoft)

Multi-agent conversation framework with human-in-the-loop patterns. Supports asynchronous messaging,
event-driven and request/response patterns. Robust framework for custom agent systems, but requires
heavy engineering. Known issues: can get trapped in loops, limited interface, high token costs.
Not a product — an SDK for building products. No domain-specific trust mechanisms.

### Cline / RooCode

Terminal/editor-native coding agents. Single-agent, not multi-agent orchestration. Permissioned
actions, clear plans, model flexibility. Popular with individual developers for day-to-day coding.
Not competing with Liza's multi-agent supervision — competing with Claude Code and Cursor as
the agent runtime that Liza orchestrates.

### AgentMesh

Academic prototype (184 stars). Planner → Coder → Debugger → Reviewer. Python script under 1,000
lines. In-memory state. Honest about limitations. Validates the multi-role architecture thesis but
stops at "this could work." Case study was a CLI to-do app. No ongoing development visible.

### OpenSpec

Spec-driven development (SDD) framework (34k stars, 30 contributors, v1.2.0, MIT). NPM package
(`@fission-ai/openspec`) that installs slash commands into 24+ AI coding assistants (Claude Code,
Cursor, Cline, GitHub Copilot, Gemini CLI, etc.). TypeScript. By Fission AI.

**Philosophy**: "Agree on what to build before any code is written." Lightweight spec layer —
fluid and iterative, not waterfall. Brownfield-first: targets mature codebases where understanding
existing systems is the hard part. Specs live in the repo as version-controlled markdown, organized
by domain (`specs/`) with changes tracked as self-contained folders (`changes/`) containing
proposal, design, tasks, and delta-specs.

**How it works**: `/opsx:propose` generates a change folder with structured artifacts.
`/opsx:apply` implements tasks from spec. `/opsx:verify` validates completeness, correctness,
and coherence. `/opsx:archive` merges delta-specs into main specs. The delta-spec approach
(ADDED/MODIFIED/REMOVED relative to current specs) avoids conflicts when multiple changes coexist.

**Trust model**: Advisory, not enforcing. Verify surfaces warnings but doesn't block. Dependencies
are "enablers, not gates." Tasks are marked complete by the AI during implementation — no external
validation. No behavioral enforcement, no role boundaries, no adversarial review. Trust derives
from structured artifacts making intent visible, not from constraining execution.

**Relationship to Liza**: Complementary, not competitive. OpenSpec is a planning layer; Liza is
an execution system. OpenSpec solves "requirements live only in chat history" — the same problem
Liza's spec phase solves, but as a universal tool across 24+ agents rather than an integrated
pipeline phase. OpenSpec's specs could theoretically feed Liza's `--spec` input, though the
formats differ. The key architectural difference: OpenSpec trusts agents to follow specs once
written; Liza mechanically enforces that they do. OpenSpec positions against SpecKit ("thorough
but heavyweight") and Kiro ("powerful but locked in") — Liza's spec phase is closer to SpecKit's
thoroughness with OpenSpec's agent-agnostic reach.

**Spec altitude comparison**: OpenSpec, like most spec-driven tools, operates at the detailed
technical spec level — delta-specs describe precise changes to existing system specifications.
The human or agent must have already made product decisions before OpenSpec adds value. Liza's
goal document is deliberately higher-altitude: product decisions (what to build, for whom, how
it behaves) without implementation detail. The orchestrator then decomposes into tasks that agents
implement. OpenSpec's specs could feed Liza's `--spec` input, but they solve different problems:
OpenSpec structures what's already decided; Liza's pairing modes (Coach/Challenger) help decide
what to build in the first place.

**Market position**: High traction (34k stars) in the "spec-first" niche. Validates the thesis
that front-loading understanding reduces rework. No threat to Liza's multi-agent orchestration
or behavioral enforcement. Potential overlap only with Liza's spec phase — and even there, the
approaches differ (universal planning layer vs integrated adversarial pipeline).

### GitHub Spec Kit

Open-source toolkit (March 2026) for spec-driven development with AI coding agents. Four-stage
workflow: Specify → Plan → Tasks → Implement. Slash commands (`/specify`, `/plan`, `/tasks`)
steer the agent. Supports Copilot, Claude Code, Codex, Gemini CLI.

Closest to Liza's spec philosophy — starts from a high-level goal, not a detailed spec. But
it's a template/CLI toolkit, not an orchestration system. No supervision, no adversarial review,
no crash recovery, no behavioral enforcement. The agent generates the spec *and* executes it —
no separation between product decisions and implementation choices. Spec Kit solves "how to
structure the input"; Liza solves "how to ensure the output is trustworthy."

**Relationship to Liza**: Complementary. Spec Kit's structured goal → spec → plan → tasks
workflow could feed Liza's `--spec` input. The gap is everything after task decomposition —
supervision, review, enforcement, recovery — which Spec Kit doesn't address.

### Kiro (Amazon)

IDE-native AI agent generating a three-document specification system interactively from user
prompts. Agent asks clarifying questions, produces requirements, design, and task documents.
Single-agent execution from the resulting spec.

**Relationship to Liza**: Similar interactive spec refinement, but agent-led rather than
human-led. Kiro's agent drives the conversation; Liza's Coach mode has the human articulate
intent while the agent asks Socratic questions. Different control model — Kiro inverts
ownership of product decisions. Locked to the Kiro IDE.

### MAS²

Research paper (Sept 2025). Meta-level paradigm — a MAS that generates other MAS. Tri-agent
architecture: Generator → Implementor → Rectifier. Reports impressive benchmark numbers but
operates at architecture-selection level, not behavioral enforcement level. The Rectifier concept
(runtime monitoring) inspired Liza's planned anomaly detection. No implementation available.

---

## Competitive Dimensions Matrix

| Dimension | Liza | BMAD | gstack | CrewAI | Ruflo | GSD | Symphony | Paperclip |
|-----------|------|------|--------|--------|-------|-----|----------|-----------|
| **Spec input level** | High-level goal doc (product decisions only) → orchestrator decomposes. Human owns spec via pairing (Coach/Challenger). | Full lifecycle (brainstorming → PRFAQ → PRD → Architecture → Epics → Stories); PM agent interviews human | Product idea / plan / live app depending on skill; human sequences workflows | None (task descriptions) | None (task descriptions) | Detailed spec required → plan → execute → verify | Issue tracker (Linear) | Goal → delegated tasks |
| **Domain** | Software engineering (13 roles, 3 phases, declarative pipeline) | Software engineering + product discovery (6 named agents, 4 phases + Quick Flow track) | Software engineering + product/design/QA/release workflow suite | General-purpose | Software engineering (60+ agent types) | Software engineering (15 agents, 4 phases) | Task scheduling | Business operations |
| **Trust approach** | Behavioral contract (55+ failure modes) + review quorum + provider-diversity preference/gates | Structured process + context engineering; prompt-level discipline | Expert prompt workflows + outside voices + selected hooks | Post-hoc output validation | Track-record based (Q-learning) | Spec-driven + deviation rules | Implementation-dependent | Budget/approval governance |
| **Role enforcement** | Code-enforced (CLI-native Go supervisor, YAML-driven permissions) | Prompt-level (6 named personas, skill and workflow step files) | Prompt-level personas plus host hooks; no supervisor state | Prompt suggestion | Claude hooks (provider-specific) | Prompt-level (least-privilege tooling) | None (single-agent) | Org chart hierarchy |
| **Review loop** | Adversarial doer/reviewer pairs with quorum + provider-diversity gate/preference | 3 parallel reviewers (Blind Hunter / Edge Case Hunter / Acceptance Auditor) → triage buckets; advisory (no merge gate) | `/review` specialists + Claude/Codex outside voices; advisory | Optional manager mode (broken) | None (single-pass) | Checker + verifier (separate, not adversarial) | None | None |
| **Failure handling** | Structural prevention + escalation + checkpoint-gated transitions | `bmad-correct-course`, retrospective, readiness gate (PASS/CONCERNS/FAIL) | STOP points, 3-strike debugging, context-save/restore, local learnings | Retry on output failure | Pattern matching from past successes | 3 retries + document + move on | Implementation-dependent | Budget auto-pause |
| **Provider compliance** | Empirical matrix (5 providers), provider-diversity preference/gates | IDE-agnostic (Claude Code, Cursor, Codex, Copilot); no compliance testing | 10 host configs, model overlays; no behavioral compliance matrix | None published | Claude-only | Multi-runtime (6), no compliance testing | Codex-only | Agent-agnostic (no testing) |
| **Context management** | Tiered degradation, structured HandoffEvents, prompt templates | Fresh chat per workflow; markdown artifact re-reads | Per-skill deep context, local learnings, session tracking | Memory (short/long/entity) | HNSW-indexed persistent memory | Fresh subagent contexts, context budget enforcement | Per-issue workspace | Persistent sessions |
| **Crash recovery** | recover-agent, recover-task | None (fresh-chat architecture) | Manual context-save/restore, optional WIP checkpoints; no task recovery | None | None | File-based state, session resume, state reconstruction | BEAM supervision trees | Session persistence |
| **Cost tracking** | Planned (token-level) | None documented | Telemetry/model benchmark helpers; no governance budget model | None native | None | Model profiles (quality/balanced/budget), proxy metrics | None | Per-agent/task/project budgets |
| **Multi-sprint** | Yes (numbering, checkpoints, archive, replan) | `sprint-status.yaml` per sprint; no cross-sprint continuity | No MAS sprint lifecycle; branch/project learnings persist | No | No | No (session/phase-scoped) | Per-issue runs | Heartbeat-based scheduling |
| **Maturity** | Alpha MAS (three phases shipped), battle-tested pairing | Production (V6.3.0, April 2026) | Fast-moving, high adoption (v1.43.2.0, May 2026) | Production (v1.9.0) | Active development | Production (v1.25+) | Engineering preview | Just launched |
| **Stars** | Early | ~45.2k | ~100.7k | 45k | Growing | 37k | New | 14k |
| **License** | Apache 2.0 | MIT (trademarked) | MIT | MIT | MIT | MIT | Apache 2.0 | MIT |

---

## Spec-Driven Landscape

| System | Input altitude | Who owns product decisions | Decomposition | Adversarial review |
|--------|---------------|--------------------------|---------------|-------------------|
| **Liza** | High-level goal (problem, users, behavior, scope) | Human via pairing (Coach/Challenger) | Orchestrator decomposes into tasks | Doer/reviewer pairs with quorum |
| **BMAD** | Full lifecycle (brainstorming → PRFAQ → PRD → Architecture → Stories) | Human via conversational PM-agent interview | Phase workflows produce artifacts (PRD → Architecture → Epics → Stories) | Three-layer parallel reviewers at code stage (prompt-level) |
| **gstack** | Product idea / plan / live app depending on skill | Human via User Sovereignty; AI challenges and recommends | Human sequences skills; `/autoplan` chains reviews into plans | `/review` specialists + Codex/Claude outside voices, advisory |
| **Spec Kit** | High-level goal → agent-generated spec | Agent generates, human approves | Agent decomposes spec into tasks | None |
| **OpenSpec** | Detailed delta-specs on existing system | Human (spec assumed pre-decided) | Slash commands structure tasks | None (verify is advisory) |
| **Kiro** | Interactive 3-doc generation | Agent drives, human confirms | Agent decomposes from spec | None (single-agent) |
| **GSD** | Detailed spec required | Human (pre-written) | Planner sizes to context budget | Checker + verifier (not adversarial) |
| **MetaGPT** | Natural language request | Agent (PM role generates PRD) | PM → Architect → Engineer pipeline | None (sequential, not adversarial) |

The key differentiator: most tools either expect the spec already done (OpenSpec, GSD) or have the agent
write it (Spec Kit, MetaGPT, Kiro). Liza treats goal-setting as a synchronous human-agent collaboration
where the human makes product decisions and the agent helps surface gaps — then enforces those decisions
mechanically during execution.

---

## Key Trends (March-April 2026)

**The scheduler/orchestrator layer is commoditizing.** Symphony and Paperclip prove that dispatching
work to agents, managing workspaces, and tracking runs is becoming table stakes. This is not where
the moat is.

**"Zero-person company" is the dominant narrative.** Paperclip's rapid traction (14k stars in days)
shows the market appetite. The narrative is exciting but premature for enterprise — nobody will
trust fully autonomous agents on production codebases without behavioral guarantees.

**A2A (Agent-to-Agent) protocols are emerging.** CrewAI's A2A task execution, Google's A2A protocol
announcement. Standardization of how agents delegate to each other is coming. Liza's blackboard
pattern already solves this for its domain but should watch for interoperability expectations.

**Harness engineering is OpenAI's answer to trust.** Symphony's philosophy is "make codebases legible
to agents" rather than "constrain agent behavior." Liza's README addresses this directly: it "enforces
governance intrinsically — not through external scaffolding as Harness Engineering does." Harness
engineering is necessary but insufficient — a legible codebase doesn't prevent an agent from modifying
tests to make broken code pass. The constraint must be in the system, not in the codebase.

**Context engineering is becoming a discipline.** GSD's 37k stars prove that "prevent context rot"
resonates with developers. The insight is real — agent quality does degrade with context accumulation.
But context engineering and behavioral enforcement solve different problems. A fresh-context agent can
still mutate tests, violate scope, or skip review. The complete solution needs both.

**Workflow suites are becoming the adoption wedge.** gstack packages git worktrees, tmux sessions,
AI CLI orchestration, and review loops into a single operator surface. This suggests a competing route
to multi-agent adoption: make everyday coding workflows feel coordinated first, then layer deeper
architectural governance on top.

**Spec-driven development is converging — but at different altitudes.** GitHub Spec Kit, OpenSpec,
Kiro, Intent, and BMAD-METHOD all position around "spec before code." Most either expect a
detailed technical spec as input (OpenSpec, GSD) or have the agent generate one (Spec Kit,
Kiro, MetaGPT). BMAD covers the broadest altitude range — from brainstorming and PRFAQ at the
top through epics, stories, and code review at the bottom — but relies on continuous human
participation across a chain of workflows. Liza picks a narrower altitude and bets on autonomy:
the human authors a goal document capturing product decisions (problem, users, behavior, scope
boundaries) without implementation detail, and the pipeline decomposes autonomously under
adversarial review. The rule is explicit: "agents may make implementation choices but not
product decisions." A ~200-line goal document describing the "Diagnosis Design" method produced
a complete three-tier application in a single Liza run — Python FastAPI backend with
SQLAlchemy/Alembic persistence, Go Cobra CLI, and a Vite/React/TanStack-Query web UI — with no
human steering between the goal document and merged code; supporting run artifacts are in a non-public
Diagnosis Design repo. The positioning question is not "who
starts highest" but "what's the minimum human input that reliably produces working code": BMAD
answers with iterative PM-agent interviews across phases; Liza answers with one front-loaded
goal doc authored via pairing (Coach mode for surfacing WHY, Challenger mode for stress-testing
WHAT), then mechanical pipeline execution.

**Enterprise trust remains unsolved by everyone except Liza.** Every framework survey and comparison
article mentions guardrails as a desirable feature. Nobody has what Liza has — 55+ documented failure
modes with mechanical countermeasures, code-enforced role boundaries, adversarial review with
configurable quorum, provider-diversity gates, and checkpoint-gated phase transitions.
Recent additions (review quorum, provider diversity blocking, structured handoff events, declarative
pipeline schema) widen this gap further — these are trust mechanisms that require deep architectural
commitment, not features that can be bolted on.

---

## Code Quality Evidence

Claude Code Opus 4.6 produced a [code quality assessment](../code_quality_assessment.md) of the Liza codebase
(commit 972d6c26, March 2026). Key findings:

**Overall Rating: A (Excellent)**

**Metrics (updated March 22, 2026 — commit 54efbf2):**
- 27,926 lines of Go across 178 production files
- 71,839 lines of tests across 147 test files (2.57:1 test-to-production ratio)
- 1,247 test functions with table-driven subtests
- 4 direct dependencies (cobra, yaml.v3, flock, fsnotify) — radically minimal
- Zero TODOs in production Go code
- 50 Architecture Decision Records
- 115 specification files (16,425 lines)
- 21 skills (debugging, testing, code-review, epic-writing, etc.)
- 21 pre-commit hooks, E2E tests in CI, race detection enabled by default

**Subsystem ratings:**
- State Machine & Models: ★★★★★
- Git Operations: ★★★★★
- State Validation (43+ rules): ★★★★★
- Behavioral Contracts & Skills: ★★★★★
- Testing & Quality Infrastructure: ★★★★★
- Documentation & Specifications: ★★★★★
- Operations Layer: ★★★★☆
- MCP Server: ★★★★☆
- Agent Supervision: ★★★★☆
- CLI Commands: ★★★★☆

**Key quote from the assessment**: "Liza is a technically rigorous project that practices what it
preaches. The behavioral contracts that govern LLM agents are themselves enforced by well-tested
Go code with atomic state management, comprehensive validation, and race-free concurrency patterns."

**Primary challenge identified**: Not code quality but cognitive surface area — 35,000+ lines of
specifications, contracts, and skills create an extraordinary knowledge base that also presents
a steep learning curve for maintainers. This is the "easy to maintain, easy to onboard" claim that needs to be
backed by the convention-over-code pattern holding as the system grows.
