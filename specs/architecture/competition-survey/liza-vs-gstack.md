---
date: 2026-05-22
perspective: Authored by Liza maintainers. Hosted in Liza's own repo — weigh framing accordingly. Version-specific and adoption claims are point-in-time snapshots; both projects iterate quickly.
---

# Liza vs gstack Comparison

## Source Snapshot

- **Liza**: local repository HEAD `f7224558`; public GitHub API checked 2026-05-22 for stars/forks. ~218 stars, 33 forks.
- **gstack**: local shallow clone at v1.43.2.0; public GitHub API checked 2026-05-22 for stars/forks. ~100.7k stars, ~15k forks. Created 2026-03-11. 552 open issues.

---

## 1. Identity & Positioning

**Liza** — "Hardened Multi-Agent Coding System." Behavioral enforcement for autonomous multi-agent coding. Written in Go (~35k LOC + ~92k lines of tests). Apache 2.0 license. Single primary author. Stack-agnostic by design. ~218 stars, 33 forks. Current public release: v0.7.0. Self-implementing its own features since v0.4.0.

**gstack** — "Garry's Stack." A skill library that turns Claude Code into a virtual engineering team — 52 slash-command skills covering the full product lifecycle from ideation (`/office-hours`) through architecture (`/plan-ceo-review`, `/plan-eng-review`) to code review (`/review`), QA (`/qa`), security audit (`/cso`), and release (`/ship`, `/land-and-deploy`). Built by Garry Tan (Y Combinator CEO). TypeScript/Bun. MIT license. ~100.7k stars, ~15k forks. Created March 2026. v1.43.2.0. The project also ships a persistent headless Chromium browser daemon for browser-driven QA and automation.

The two projects occupy different niches. Liza is a runtime orchestrator with code-enforced supervision. gstack is a skill library with a browser runtime. Liza is infrastructure; gstack is tooling layered onto existing infrastructure (Claude Code, Codex, Cursor, etc.).

---

## 2. Core Philosophy

**Liza** starts from the premise that LLM agents are unreliable by default. The behavioral contract was developed incrementally as countermeasures to actually-observed misbehaviors — agents altering tests to pass, fabricating completions, silently drifting from scope. Trust is engineered mechanically, not assumed. The contract applies a cost gradient: errors caught in specs cost less than errors caught in code, errors caught in code cost less than errors caught in tests. The design philosophy: "Systems that optimize for immediate output generate muda — defects, rework, and correction loops."

**gstack** starts from the premise that "a single person with AI can now build what used to take a team of twenty." The philosophy is captured in ETHOS.md with three principles: (1) **Boil the Lake** — AI makes completeness cheap, so always do the complete thing; (2) **Search Before Building** — check if someone already solved it before designing from scratch; (3) **User Sovereignty** — AI recommends, users decide. gstack frames the current moment as a "Golden Age" where the engineering barrier is gone and what remains is taste, judgment, and willingness. The compression ratio table (3x for research to 100x for boilerplate) shapes every build-vs-skip decision.

The epistemic difference is fundamental. Liza treats agent behavior as the primary failure surface and constrains it through rules grounded in academic failure-mode research (MAST taxonomy, AgentIF benchmark). gstack treats agent behavior as the success surface and amplifies it through structured prompts that encode domain expertise (CEO thinking, engineering review, QA methodology, security auditing). Liza asks "how will the agent fail?" and builds defenses. gstack asks "what would 23 specialists do?" and builds personas.

---

## 3. Agent Architecture

**Liza** — 1 Orchestrator + 12 other roles across 3 pipeline phases, defined in `roles.go` and configured in `pipeline.yaml`. Every activity is dual: a doer and a reviewer. Roles are functional positions in a pipeline with strict boundaries enforced by the Go supervisor. Allowed-operations gating per role: a coder cannot perform planner operations. Agents are real concurrent processes running in separate terminal sessions on isolated git worktrees.

**gstack** — 23 specialist personas and 8 "power tools" (utility skills), all invoked as slash commands within a single Claude Code session. Key specialists: Office Hours (product interrogation), CEO Review, Eng Review, Design Review, DevEx Review, Review (code review), QA, CSO (security), Ship, Land-and-Deploy, Canary, Investigate (debugging), Retro (retrospective), Autoplan (end-to-end planning), and iOS-specific skills. Each skill is a self-contained Markdown file (1,000–3,000 lines) with a preamble (bash setup), detailed instructions, checklists, and templates. Skills are generated from `.tmpl` templates via `bun run gen:skill-docs`.

The architectural difference: Liza's roles are concurrent agents with supervisor-owned task state and merge authority. gstack's personas are primarily prompt-driven within a single session, though several skills (`/review`, `/autoplan`, `/ship`) dispatch real subagents via Claude Code's Agent tool or invoke Codex as an outside voice. The distinction is not "no spawned agents" — it is that gstack's subagents have no supervisor-owned state, no lease, and no merge authority. They contribute findings; the parent session decides. gstack's `/ship` skill (3,093 lines) is the longest — it orchestrates code review, test verification, and PR creation in a single session.

The system shapes reflect this:

```text
Liza (state-first):                        gstack (workflow-first):

Human/specs                                User invokes slash command
  → Liza Go supervisor                      → generated SKILL.md preamble + workflow
  → .liza/state.yaml blackboard             → optional host hooks / helper binaries
  → per-task .worktrees/                     → browser, review, design, QA, release tools
  → doer/reviewer agent CLIs                 → local/project gstack state
  → supervisor-enforced merge
```

Liza externalizes workflow state into a supervisor-owned state machine. gstack internalizes workflow state into executable prompt documents plus helper state under `~/.gstack`.

---

## 4. Workflow & Pipeline Structure

**Liza** — 3 phases in the autonomous pipeline: (1) Specification (vision → epics → user stories), (2) Coding (architecture docs + code planning + implementation), (3) Integration (integration analysis + fixes after all tasks merge). Adversarial doer/reviewer pairs at every step. Pipeline phases connected by `liza proceed`. Multi-sprint: agents are fully autonomous within a sprint, the human steers between sprints.

**gstack** — No fixed pipeline. Skills are composable but human-sequenced. The README suggests a workflow: `/office-hours` → `/plan-ceo-review` → `/review` → `/qa` → `/ship`. The `/autoplan` skill (1,810 lines) is the closest to an end-to-end pipeline — it generates implementation plans that feed downstream skills. But each invocation is a standalone session; there is no persistent state machine connecting them. The human decides what to run next.

This reflects the philosophical split. Liza automates the entire development pipeline with mechanical transitions. gstack provides specialist tools the human orchestrates manually. Liza's pipeline is a factory; gstack's skills are a workbench.

---

## 5. Trust & Behavioral Control

**Liza** — The defining differentiator. A behavioral contract addresses 55+ documented LLM failure modes: sycophancy, phantom fixes, scope creep, test corruption, hallucinated completions. The contract has a tiered rule system: Tier 0 invariants are never violated (no unapproved state changes, no fabrication, no test corruption). The Go supervisor enforces 43+ validation rules mechanically. Task state machine, approval-gated merges, commit SHA verification. Configurable review quorum with provider-diversity preference at merge time; diversity may degrade when no diverse reviewer is available.

**gstack** — Behavioral control is lighter and optional. Three safety skills exist: `/careful` (warns before destructive commands like `rm -rf`, `DROP TABLE`, `force-push`), `/freeze` (restricts edits to a specified directory), and `/guard` (combines both). These use Claude Code's hook system — `PreToolUse` hooks run shell scripts that check commands before execution. The review skill includes a structured checklist (SQL safety, LLM trust boundary violations, conditional side effects), and the CSO skill runs OWASP + STRIDE audits. But none of these are mechanically enforced state-machine constraints — they're prompt-level discipline and optional hook-based guards.

The trust boundary is in fundamentally different places. Liza constrains agent behavior through tiered invariants enforced by code and a state machine. gstack trusts agents when given good prompts and adds optional guardrails for destructive operations. Liza's approach costs context budget (the contract is loaded into every session); gstack's approach costs nothing until a safety skill is explicitly invoked.

A specific limitation worth noting: `/freeze` blocks `Edit` and `Write` tool calls outside the allowed directory, but does not constrain `Bash` writes (e.g. `echo > file`, `cp`, `mv`). It is a scope guard for the Edit/Write tools, not a true write sandbox. This is representative of the broader pattern — gstack's safety mechanisms are useful guardrails, not hermetic enforcement.

---

## 6. Coordination Architecture

**Liza** — Classical blackboard architecture. A shared YAML file (`.liza/state.yaml`) tracks goals, tasks, assignments, status, and history. Time-bounded leases, DRAFT tasks, atomic operations via file locking. Agents work on isolated git worktrees. Communication through the Liza CLI. Context handoff as blackboard event.

**gstack** — No coordination architecture between skills. Each slash command runs in a single Claude Code session with no shared state beyond the filesystem. The `/context-save` and `/context-restore` skills provide manual session continuity — save current context, load it in a new session. The `/learn` system persists operational learnings to `~/.gstack/projects/{slug}/learnings.jsonl` and surfaces them in future sessions. But there is no blackboard, no task state machine, no inter-agent communication protocol.

The `/pair-agent` skill is notable — it enables a remote AI agent to connect to and drive the local browser via an ngrok tunnel with scoped token auth. This is inter-agent coordination, but for browser access, not task management.

---

## 7. Concurrency & Isolation

**Liza** — True parallel execution with isolation as a core architectural feature. Multiple agents run simultaneously in separate terminal sessions, each in their own git worktree. The blackboard + locking mechanism prevents conflicts. Merge authority is structural. TUI displays live state across all agents.

**gstack** — Primarily sequential at the skill level. One skill runs at a time in one Claude Code session, though skills like `/review` and `/autoplan` dispatch subagents internally. The preamble tracks active sessions (`~/.gstack/sessions/`) and shows a session count — when 3+ sessions exist, skills display project/branch context to help the user distinguish windows. But this is awareness, not coordination. Subagents contribute findings to the parent session; there is no supervisor-owned task state, workspace isolation, or merge coordination across them. The `pair-agent` skill enables two agents to share a browser, but they don't share a codebase or coordinate tasks.

---

## 8. Browser & QA

**gstack** — The clear differentiator. A persistent headless Chromium daemon (Bun.serve + Playwright) runs as a long-lived process with sub-second command latency (~100-200ms after first start). Features: persistent cookies and login sessions across commands, cookie import from local browsers (Chrome, Arc, Brave, Edge), responsive layout testing, screenshot capture, form interaction, dialog handling, and a dual-listener tunnel architecture for remote agent access. The browser powers `/qa` (automated QA flows), `/browse` (general navigation), and the iOS skills. Security model includes bearer token auth, localhost-only binding, and separate local/tunnel HTTP listeners with different endpoint allowlists.

**Liza** — No browser integration. QA and browser testing are outside Liza's scope. Liza's agents work with code, specs, and tests — not with running applications.

This is the strongest complementarity point between the two projects. Liza's agents could benefit from browser-driven QA capabilities; gstack's browser daemon could benefit from orchestrated multi-agent execution.

---

## 9. Specification & Planning

**Liza** — Specs are the durable memory of the system. The pipeline decomposes goals through adversarial doer/reviewer pairs at each stage: vision → epics → user stories → architecture docs → code plans → implementation. Many-to-one transitions consolidate sibling tasks. Automatic task decomposition based on complexity with dependency management for parallel execution. Entry point is a human-authored vision document.

**gstack** — Product-oriented planning through specialized skills. `/office-hours` (2,092 lines) conducts a product interrogation — six forcing questions, premise challenges, capability extraction, and implementation approach generation. `/plan-ceo-review` runs a 10-section strategic review. `/plan-eng-review` focuses on technical architecture. `/autoplan` generates full implementation plans. `/design-consultation` and `/design-shotgun` handle UI/UX. These skills encode years of startup product thinking (Garry Tan's background: Palantir, Posterous, Y Combinator).

The planning approaches are complementary. gstack excels at upstream product discovery — "what should we build and why?" with strategic challenge and product taste. Liza excels at downstream decomposition — "given what we're building, how do we break it into parallel tasks with adversarial review?" gstack's `/office-hours` output could naturally feed Liza's vision document entry point.

---

## 10. Code Review & Quality

**Liza** — Externally validated completion. A coder agent cannot mark their own work complete. A separate reviewer examines the work and issues a binding verdict via PR-like interaction. Approval means merge eligibility. Commit SHA verification prevents reviewing stale state. Provider-diversity preference at claim and merge time reduces single-model bias when diverse reviewers are available.

**gstack** — `/review` (1,788 lines) runs a structured pre-landing review analyzing the diff against the base branch. Checks include SQL safety, LLM trust boundary violations, conditional side effects, and structural issues. The CSO skill runs OWASP + STRIDE security audits. The review output is advisory — there is no mechanical gate preventing merge. The `/ship` skill bundles review + test verification + PR creation into one flow, but the human decides whether to proceed.

---

## 11. Testing

**Liza** — TDD is both a contract rule and a code-enforced check. INVARIANTS §6: "Code tasks must include tests." The Go supervisor checks test presence at review submission time. The contract forbids test corruption as a Tier 0 invariant; TDD ordering and reviewer gates make corruption visible, but semantic test corruption (subtly weakening assertions while preserving green) remains a review concern, not a mechanically decidable property. Testing is integrated into the coding phase.

**gstack** — Testing is primarily through the `/qa` skill (1,647 lines), which drives the headless browser through user flows, captures screenshots, and verifies state. This is end-to-end QA testing, not unit testing. The `/benchmark` skill runs performance benchmarks. gstack's own test suite uses Bun's test framework with three tiers: free static validation (<2s), LLM-as-judge evals (~$0.15/run), and E2E tests via `claude -p` (~$3.85/run). Diff-based test selection is notable — tests declare file dependencies and only run when relevant files change.

The testing approaches are complementary again: Liza enforces TDD at the unit/integration level; gstack provides browser-driven E2E QA. Both are valuable in different parts of the testing pyramid.

---

## 12. Human Role

**Liza** — The human owns intent and acts as observer/circuit-breaker. Within a sprint, agents are fully autonomous. Between sprints, the human reviews artifacts and steers the next sprint via CLI. Authority through a kill switch, not an approval queue.

**gstack** — The human is the central orchestrator. Every skill is manually invoked. The human decides what to run next, reviews output, and acts on recommendations. gstack's ETHOS.md makes this explicit: "AI models recommend. Users decide. This is the one rule that overrides all others." The Iron Man suit philosophy — augment the user, don't replace them.

This is a design choice, not a weakness. gstack targets founders and CEOs who want to stay hands-on. Liza targets situations where autonomous execution at scale is the goal.

---

## 13. Tooling & Infrastructure

**Liza** — Full CLI: `liza setup`, `liza init`, `liza tui`, `liza agent <role>`, `liza validate`, `liza status`, `liza proceed`, `liza pause/resume`, `liza sprint-checkpoint`, `liza recover-agent`, `liza recover-task`, `liza analyze`. TUI with live system state. Multi-provider setup. Agent-specific tool configuration. Log analysis skill.

**gstack** — Installation via git clone + `./setup` script. Skills installed to `~/.claude/skills/gstack/`. Auto-update check (throttled to once/hour). Config via `~/.gstack/config.yaml`. Team mode (`--team`) for shared repos. Dev mode for contributors. Skills generated from templates (`SKILL.md.tmpl` → `SKILL.md`). Operational learnings system. Telemetry (opt-in). Multi-host support: Claude Code, Codex CLI, OpenCode, Cursor, Factory Droid, Slate, Kiro, Hermes, GBrain — 10 hosts total, each with a typed TypeScript config file. Browser daemon with state file, auto-start/stop, and version auto-restart.

---

## 14. LLM Provider Support

**Liza** — Multi-provider via setup flags: Claude Code, Codex, Gemini, Mistral. Model capability assessment documented — the contract is a capability test requiring meta-cognitive machinery. Configurable review quorum with provider-diversity preference; diversity is best-effort when fewer providers are available.

**gstack** — 10 hosts supported via typed config files. Claude Code is primary. Setup auto-detects installed agents. Adding a new host is one TypeScript config file, zero code changes. Model overlays (`model-overlays/`) allow per-model prompt adjustments. No model capability requirements — skills are Markdown prompts that work with any model that follows instructions.

gstack's broader host support is a direct consequence of its lighter touch — Markdown skills port trivially across hosts. Liza's behavioral contract is a capability filter that narrows the usable model set.

---

## 15. Extensibility

**Liza** — Skills system with per-role capabilities. Pipeline configuration via YAML. Behavioral contract customizable per project via GUARDRAILS.md. Agent tools configuration per user. Open-source (Apache 2.0) but no formal plugin ecosystem.

**gstack** — Skills are Markdown files in directories. Adding a new skill means creating a directory with a `SKILL.md`. The `/skillify` skill can convert ad-hoc workflows into reusable skills. Template system (`SKILL.md.tmpl` → `SKILL.md`) with config-driven generation. Health dashboard (`bun run skill:check`). Dev mode with watch (`bun run dev:skill`). `CONTRIBUTING.md` documents the full contributor workflow. OpenClaw integration with ClawHub-distributed skills.

gstack's extensibility model is more accessible — a new skill is one Markdown file. Liza's extensibility requires understanding the pipeline, roles, and state machine.

---

## 16. Maturity & Adoption

**Liza** — ~218 stars, 33 forks. Single primary author. Created January 2026. 15 public tags. Current release v0.7.0. Self-implementing since v0.4.0. External validation: placed at L4 (Collaborative Agent Networks) on Octo Technology's 5-level AI maturity model. Apache 2.0.

**gstack** — ~100.7k stars, ~15k forks. Created March 2026. v1.43.2.0. 552 open issues. Built by Garry Tan (Y Combinator CEO) — the project benefits from significant personal reach and audience. MIT license. GitLab CI. CHANGELOG.md at 6,265 lines (715KB) showing rapid iteration. Active community with PRs referenced in commit messages. README includes an OpenClaw integration path.

The adoption gap (~460x stars) is partly a distribution effect — Garry Tan's platform as YC CEO provides reach that a single-author technical project cannot match. The project's rapid iteration (v1.43 in ~10 weeks) and community engagement are genuine indicators of traction.

---

## 17. Documentation & Observability

**Liza** — Blackboard provides full state visibility. Activity log records all events. Agent logs analyzed at sprint boundaries via `/liza-logs`. Token optimization analysis. Approval and merge traceability. Rescoping audit trail. Context engineering analysis via `/context-engineering`. Specs and architecture documents are comprehensive.

**gstack** — README.md (486 lines), ARCHITECTURE.md (435 lines), SKILL.md (962 lines), CLAUDE.md (878 lines), CONTRIBUTING.md (491 lines), ETHOS.md (164 lines), CHANGELOG.md (6,265 lines). Each skill's SKILL.md is thorough — the review skill alone is 1,788 lines. `BROWSER.md` (59.1k bytes) documents the browser extensively. Observability is session-local: skill telemetry, learnings JSONL, analytics. No cross-session execution observability comparable to Liza's log analysis.

---

## 18. Operating Model — Cross-Dimension Synthesis

**Execution substrate** — Liza is a runtime substrate: CLI, supervisor processes, blackboard state, locks, leases, worktrees, validation, recovery, and merge authority. gstack is a skill layer installed into AI coding assistants: Markdown prompts, a browser daemon, shell scripts, and config files. Liza wraps agents in executable control infrastructure; gstack gives agents expert knowledge and specialized tools.

**Context economy** — Liza deliberately spends context on behavioral contracts, approval gates, stop conditions, and role-specific constraints, then manages degradation with explicit context tiers. gstack spends context on per-skill expertise (a single skill can be 1,800+ lines of domain knowledge). Both are context-expensive but for different reasons: Liza buys behavioral safety; gstack buys domain depth.

**Scope of ambition** — gstack covers the broader product lifecycle: product thinking, strategic review, design, code review, QA, security, release, and browser automation. Liza is narrower but deeper: autonomous multi-agent coding with state integrity, adversarial review pairs, supervised merges, and recovery mechanics. gstack is horizontally broad; Liza is vertically deep.

**Failure ownership** — gstack expects an active human to orchestrate skills and act on their output. Liza routes failure into machine-visible states: BLOCKED, REJECTED, SUPERSEDED, crash recovery, circuit-breaker analysis. gstack makes failure a human decision; Liza makes failure a state transition.

**Customization philosophy** — gstack invites extension through its Markdown skill format and template system. One file, one skill. Liza is configurable but its value depends on preserving hard invariants — the behavioral contract is load-bearing, not optional. gstack optimizes accessibility; Liza optimizes constraint preservation.

---

## 19. Where They Overlap

- Both use Claude Code as a primary host
- Both provide slash-command skills for code review, debugging, and retrospectives
- Both care about code review quality (structured checklists, specific checks)
- Both support multiple AI providers/hosts
- Both are open source, built by individual makers
- Both have operational learning mechanisms (Liza: `/lesson-capture` + `lessons/`; gstack: `/learn` + learnings JSONL)
- Both address safety concerns (Liza: tiered behavioral contract; gstack: `/careful`, `/freeze`, `/guard` hooks)

---

## 20. Framework Failure Modes

**Liza** fails when:
- The work is small or exploratory. The MAS pipeline is disproportionate to trivial changes. Pairing mode is the answer for this envelope.
- The model can't hold the contract. The contract requires meta-cognitive machinery that weaker models fail as a capability test.
- Requirements are genuinely unclear upstream. No native product-discovery capability — if the vision document is thin, the pipeline decomposes thin input into thin output.
- Browser-driven QA is needed. No browser integration means no ability to verify deployed applications visually.
- Operational setup cost is disproportionate for throwaway experiments.

**gstack** fails when:
- Multiple agents must work in parallel on the same codebase with coordinated state. Some skills dispatch subagents, but there is no supervisor-owned task state, workspace isolation, or merge coordination across them.
- Mechanical enforcement of workflow correctness is needed. Skills are prompt-level discipline — a model that ignores instructions produces visibly compliant but broken output, and nothing blocks a merge.
- Long-running autonomous execution is the goal. Every skill requires human invocation and human decision-making. No unattended pipeline execution.
- Work must persist across sessions as supervised task state. No blackboard, no leases, no crash recovery — if a session dies, in-progress work is not automatically recoverable. Recovery relies on git/WIP commits, `/context-save` / `/context-restore`, and local gstack state, not on a supervisor-owned task lifecycle.
- Adversarial review with external validation is needed. Code review is advisory, not mechanically gated. A coder can self-certify their own work.
- Task decomposition and dependency management are needed. No automatic complexity-based decomposition, no dependency-aware parallel execution.
- The "Boil the Lake" ethos overfits to solo velocity. Completeness-at-all-costs is sound for a single builder shipping fast, but needs counterweight in teams where review load, maintainability, and blast radius dominate. Non-interactive automation like `/ship` increases leverage but also increases reliance on model compliance — more leverage, more consequence when the model drifts.

---

## 21. Where They Diverge Most

The fundamental difference is the unit of work and the trust boundary.

**gstack** trusts agents when given expert-level prompts and amplifies their capability through domain knowledge. A single skill file can contain 3,000 lines of specialist expertise — the equivalent of loading a senior engineer's playbook into the model's context. Quality comes from prompt depth. The human orchestrates.

**Liza** does not trust agents and mechanically constrains them through a tiered invariant system backed by ~35k LOC of Go enforcement code. Quality comes from suppressing empirically documented failure modes and requiring external validation at every step. The supervisor orchestrates.

This maps to different users and use cases. gstack is for the solo builder who wants to ship fast — a founder, CEO, or staff engineer who stays in the loop, makes every decision, and uses AI skills as a force multiplier. gstack's README self-positions around an 810x normalized pace improvement claim — one person doing the work of twenty. Liza is for situations where autonomous multi-agent execution must produce production-grade code with audit trail — where the human sets direction and the system executes with mechanical guarantees.

They are architecturally complementary. The useful integration direction is not "replace Liza with gstack" or "absorb all gstack skills into Liza." The cleaner direction: Liza remains the supervisor and source of truth for tasks, state, review authority, and merge authority. gstack-like capabilities become optional tools or role skills inside Liza tasks. Any imported gstack workflow that can mutate repository or deployment state must be mediated by Liza's state machine rather than executed as free-form skill automation.

The best lesson from gstack is not "add more slash commands." It is that agents become dramatically more useful when specialized workflows have excellent tools, fast feedback loops, and durable local memory. The best Liza-shaped version of that lesson is to put those capabilities behind Liza's existing mechanical gates instead of weakening the gates to chase convenience.

---

## 22. What Each Could Steal

### What Liza could steal from gstack

1. **A persistent browser capability as first-class infrastructure.** Liza's MAS would benefit from a hardened browser QA substrate with persistent state, fast commands, logs, screenshots, and scoped remote pairing. gstack's browser architecture is the most concrete reusable subsystem.

2. **Generated skill documentation.** gstack's template/resolver system (`SKILL.md.tmpl` → `SKILL.md`) keeps skill docs aligned with command registries and host differences. Liza has embedded skills and contracts; a generator could reduce drift for provider-specific variants and tool-surface adaptations.

3. **Host adapter configs.** Liza already supports multiple provider CLIs, but gstack's declarative host config pattern is a clean way to express path rewrites, frontmatter transforms, suppressed sections, runtime assets, and co-author trailers — one TypeScript file per host, zero code changes.

4. **Workflow verbs for human ergonomics.** `/qa`, `/ship`, `/canary`, `/document-release`, and `/design-review` are easy handles. Liza could expose equivalent high-level commands that map onto its existing supervisor machinery.

5. **Review readiness dashboard.** gstack's branch-local review log and ship preflight dashboard are pragmatic. Liza has richer state; it could present an even stronger sprint/branch readiness summary.

6. **Per-project learning and taste memory.** gstack's learnings JSONL and design taste profile are useful, but should sit below Liza's spec/invariant hierarchy so memory never overrides durable project truth.

### What gstack could steal from Liza

1. **Supervisor-owned state transitions.** gstack's workflows would be safer if review, ship, QA, and release state were externalized into a small deterministic state machine rather than held mostly in prompt execution.

2. **A blackboard for multi-session work.** `~/.gstack` stores useful memory, but Liza's task-level blackboard is stronger for explicit ownership, status, claims, and recovery.

3. **Doer/reviewer authority separation.** gstack has review skills and outside voices, but not Liza's structural rule that a separate reviewer approval unlocks merge authority.

4. **Lease and progress watchdog concepts.** Long-running agent sessions need more than logs. Liza's distinction between heartbeat and real progress is directly applicable to gstack's automated flows like `/ship`.

5. **Formal invariant tiers.** gstack has strong individual safety mechanisms, especially in browser security, but Liza's tiered invariant model is clearer for deciding what halts the system versus what merely warns.
