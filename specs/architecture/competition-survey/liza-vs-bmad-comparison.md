---
date: 2026-04-20
perspective: Authored by Liza maintainers. Hosted in Liza's own repo — weigh framing accordingly. Version-specific and adoption claims are point-in-time snapshots; both projects iterate quickly.
---

# Liza vs BMAD Method Comparison

## Source Snapshot

- **Liza**: local repository HEAD `bbabc18c77860b55cd4063a6c7f27a9821b6d84a`; public GitHub API checked 2026-04-20 21:58 UTC for stars/forks/commits/tags/latest release.
- **BMAD**: public GitHub API checked 2026-04-20 21:57 UTC for stars/forks/contributors/commits/releases/latest release; raw `CHANGELOG.md`, `docs/reference/agents.md`, and `src/core-skills/bmad-party-mode/SKILL.md` checked against `bmad-code-org/BMAD-METHOD`.
- **Diagnosis Design proof point**: non-public Diagnosis Design repo checked 2026-04-21; its README documents the resulting FastAPI API, Go/Cobra CLI, and Vite/React/TanStack Query web UI, and its `.liza/` directory contains the run artifacts.

## 1. Identity & Positioning

**Liza** — "Disciplined Multi Coding Agent System." Behavioral enforcement for autonomous multi-agent coding. Written in Go (~34k LOC + ~90k lines of tests). Apache 2.0 license. Single primary author. Stack-agnostic by design: `GUARDRAILS.md` G1.1 prohibits hardcoding language, build system, or tooling assumptions into runtime behavior — target projects may be in any language. The README places Liza alone in the behavioral-enforcement category of the multi-agent coding landscape. ~150 stars, 23 forks, 1,069 commits, 15 public tags. Current public release: v0.7.0. Self-implementing its own features since v0.4.0.

**BMAD** — "Breakthrough Method for Agile AI-Driven Development." Comprehensive agile development methodology framework with AI agents as expert collaborators. Written in JavaScript/Node.js. MIT license. Trademarked. ~45.2k stars, ~5.4k forks, 132 contributors, 1,826 commits, 29 releases as of the source snapshot. npm package (`npx bmad-method install`). V6.3.0 shipped April 10, 2026 (consolidated three agent personas into Developer). Part of a broader ecosystem with official modules (BMad Builder, Test Architect, Game Dev Studio, Creative Intelligence Suite). Documentation site in 5 languages. Discord community. YouTube channel.

---

## 2. Core Philosophy

**Liza** starts from the premise that LLM agents are unreliable by default. The system was born from direct experience with agents deceiving — altering tests to pass, fabricating completions, silently drifting from scope. The behavioral contract was developed incrementally as countermeasures to actually-observed misbehaviors. The design philosophy: "Systems that optimize for immediate output generate *muda* — defects, rework, and correction loops. By optimizing for trust, quality, and auditability, Liza eliminates these wasted cycles." Trust is engineered mechanically, not assumed. The contract applies a cost gradient: errors caught in specs cost less than errors caught in code, errors caught in code cost less than errors caught in tests — the spec system front-loads understanding so agents don't discover requirements by failing.

**BMAD** starts from the premise that AI tools "do the thinking for you, producing average results" and instead offers agents as "expert collaborators who guide you through a structured process to bring out your best thinking." The focus is on methodology — grounding AI interactions in agile best practices. The design philosophy is structured context engineering: each phase produces documents that inform the next, so agents always have clear context. Quality comes from following good process, not from constraining agent behavior.

The two systems rest on different epistemic legitimacy claims. Liza grounds its countermeasures in explicit academic failure-mode research — MAST taxonomy (Berkeley 2025), AgentIF benchmark, Da et al. 2023, Xia et al. 2024 — and cross-references each rule to the failure-mode category it addresses. BMAD grounds its process in established agile methodology, Amazon's Working Backwards, and industry-standard practices like ADRs and PRDs. Both have traction; they appeal to different audiences. Engineering leads comfortable with behavioral research find Liza's failure-mode catalog compelling; product leads comfortable with agile practice find BMAD's workflow lineage compelling.

---

## 3. Agent Roles

**Liza** — 1 Orchestrator + 12 other roles across 3 pipeline phases, defined in `roles.go` and configured in `pipeline.yaml` (524 lines: 13 roles + 6 pair configs + 3 sub-pipelines with role-gated allowed-operations). Every activity is dual: a doer and a reviewer. Shipped role pairs include: Epic Planner / Epic Plan Reviewer, US Writer / US Reviewer, Code Planner / Code Plan Reviewer, Coder / Code Reviewer, Architect / Architecture Reviewer, and the Integration phase roles (Integration Analyst / Integration Reviewer). Planned but not yet shipped: Sprint Analyzer, Security Auditor / Security Audit Reviewer. Roles are not personas with names — they are functional positions in a pipeline with strict boundaries enforced by the Go supervisor. The pipeline supports allowed-operations gating per role: a coder cannot perform planner operations.

**BMAD** — Six named agent personas after v6.3.0 consolidation (CHANGELOG PRs #2177, #2179, #2186): Analyst (Mary), Product Manager (John), Architect (Winston), Developer (Amelia), UX Designer (Sally), Technical Writer (Paige). V6.3.0 removed three previous agents — Barry (Quick-Flow Dev), Quinn (QA), Bob (Scrum Master) — folding their responsibilities into Developer (Amelia). Each agent has a personality, two-letter menu triggers (e.g., Developer: `DS`, `QD`, `CR`; Architect: `CA`, `IR`), and specific workflows.

---

## 4. Workflow & Pipeline Structure

**Liza** — 3 phases in the autonomous pipeline: **(1) Specification** (vision → epics → user stories), **(2) Coding** (architecture docs + code planning + implementation), **(3) Integration** (integration analysis + fixes after all tasks merge). Adversarial doer/reviewer pairs at every step within each phase, interacting like PR reviews — submission, feedback comments, verdict, revised submission, until approval. Pipeline phases are connected by `liza proceed`. Entry points are configurable (`--entry-point detailed-spec` to skip the spec phase). Multi-sprint: agents are fully autonomous within a sprint, the human steers between sprints via the CLI.

**BMAD** — 4 phases: **(1) Analysis** (optional — brainstorming, market/domain/technical research, product brief, PRFAQ via Amazon's Working Backwards), **(2) Planning** (PRD creation via structured interview, UX design), **(3) Solutioning** (architecture with ADRs, epics/stories informed by architecture, implementation readiness check: PASS/CONCERNS/FAIL), **(4) Implementation** (sprint planning, story creation, dev story, code review, course correction, sprint status, retrospective). Plus a Quick Flow parallel track for small/clear tasks. Each workflow runs in a fresh chat session. Three planning tracks based on project complexity — Quick Flow, BMad Method, and Enterprise — with the docs noting that story counts are guidance, not rigid thresholds.

---

## 5. Trust & Behavioral Control

**Liza** — The defining differentiator. A behavioral contract addresses 55+ documented LLM failure modes: sycophancy, phantom fixes, scope creep, test corruption, hallucinated completions, and dozens more. The contract's failure mode catalog lives in `CONTRACT_FAILURE_MODE_MAP` and cross-references the MAST taxonomy (14 modes from 1,600+ traces, AgentIF benchmark, Da et al. 2023, Xia et al. 2024) — evidence that the contract isn't arbitrary but maps to empirically documented agent behaviors. Each failure mode has a countermeasure. Countermeasures are encoded as rules. Rules form a contract.

The INVARIANTS.md file defines a tiered structure: Tier 0 invariants are never violated — no unapproved state changes, no fabrication, no test corruption (T0.3), no unvalidated success. The file specifies field requirements per task state (a table of required/optional/forbidden fields per transition), forbidden transitions, and concurrency invariants including CAS (compare-and-swap) merge and three-phase claim protocol.

The contract operates through "negative space design" — it defines what's forbidden; what remains is where judgment lives. Enforcement is both contract-level (rules agents must follow) and code-level (the Go supervisor mechanically checks invariants). Task state machine has 43+ validation rules enforced in Go. Approval-gated merges — coders cannot merge; only the supervisor merges after code reviewer approval. Commit SHA verification prevents reviewing stale state.

Notable invariant: provider diversity quorum — ≥2 distinct LLM providers required for multi-reviewer quorum (INVARIANTS §6). This mechanically prevents single-provider bias in review decisions.

**BMAD** — Addresses agent quality through structured process and context engineering rather than behavioral constraints. Adversarial review exists as a general BMAD technique, but the current `bmad-code-review` workflow is more specific: it uses Markdown step files, launches three parallel review layers (Blind Hunter, Edge Case Hunter, Acceptance Auditor), triages findings into decision-needed / patch / defer / dismiss, and has a clean-review path when all layers pass with no remaining findings. This is more structured than a simple "find problems" instruction, but it remains prompt-level discipline — the reviewer's authority is advisory, not mechanically enforced as a merge gate.

BMAD's "Preventing Agent Conflicts" approach relies on architecture documentation (ADRs, naming conventions, standards) as shared context so agents make consistent decisions. The two systems locate the quality lever differently: Liza treats agent behavior itself as the primary failure surface and constrains it mechanically; BMAD treats methodology and context as the primary lever and relies on good process to produce good outcomes.

---

## 6. Coordination Architecture

**Liza** — Classical blackboard architecture. A shared YAML file (`.liza/state.yaml`) tracks goals, tasks, assignments, status, and history. All state is explicit and visible. Agents claim tasks, update status, and hand off work through the blackboard. Key mechanisms: time-bounded leases (not just heartbeats) — stale agent tasks become reclaimable only after lease expiration. DRAFT tasks (planner writes DRAFT, finalizes to UNCLAIMED — coders cannot claim half-written tasks). Atomic operations via file locking (`flock`). Activity history in `.liza/log.yaml`. Agents work on isolated git worktrees (`.worktrees/task-N/`). Communication is through the Liza CLI, not direct agent-to-agent interaction. Context handoff as blackboard event — structured positive/negative findings on every task completion.

**BMAD** — Sequential workflow execution with document-based context passing. Each phase produces markdown documents (`PRD.md`, `architecture.md`, `epics.md`, `sprint-status.yaml`) that become context for the next phase. Each workflow runs in a fresh chat session — agents don't maintain persistent state across sessions. `project-context.md` serves as a constitution for implementation decisions. No blackboard or shared-state mechanism between simultaneously running agents.

Party Mode deserves specific attention: it does **not** default to one LLM instance role-playing multiple characters. The current `bmad-party-mode/SKILL.md` explicitly spawns real subagents via the IDE's Agent tool by default — each agent is a real subagent with independent thinking. The single-LLM roleplay mode is `--solo`, opt-in. Party Mode with real subagents is closer to actual multi-agent coordination than a "one model wearing hats" framing would suggest.

The *session longevity* implications differ materially. Liza's agents are task-bounded processes running against a durable blackboard — each agent instance lives for one task, reconstructing context from `.liza/state.yaml`, the activity log, and specs at spawn time. Durability lives in the blackboard, not in agent processes. Context exhaustion within a task triggers the `liza handoff` protocol: the current agent writes a structured handoff note, a fresh agent instance picks up from the blackboard position. Work spanning many hours on a single task is supported because state survives agent turnover. BMAD runs each workflow in a fresh chat session with no inter-session state; context is reconstructed only through the markdown artifacts each workflow reads and writes. Long-running work is structurally decomposed into short workflow invocations, each re-reading the same artifacts. This means Liza amortizes context across agent restarts via the blackboard while BMAD re-pays the context cost every workflow.

---

## 7. Concurrency & Isolation

**Liza** — True parallel execution with isolation as a core architectural feature. Multiple agents run simultaneously in separate terminal sessions, each working in their own git worktree. The blackboard + locking mechanism prevents conflicts. Merge authority is structural — only the supervisor merges approved work to the integration branch. The TUI (`liza tui`) displays live state across all agents. Agent crash recovery (`liza recover-agent`, `liza recover-task`) handles failures. Circuit breaker analysis (`liza analyze`) detects systemic issues.

**BMAD** — Sequential by design at the workflow level. Each workflow runs in a fresh chat. The "Preventing Agent Conflicts" documentation explicitly addresses the problem of multiple agents making conflicting decisions, but the solution is shared documentation (ADRs), not concurrent execution infrastructure. No workspace isolation, no merge coordination protocol. However, Party Mode's default subagent spawning (§6) provides a form of concurrent multi-agent interaction within a single session — though this is conversational collaboration, not parallel task execution on separate codebases.

---

## 8. Failure Handling & Recovery

**Liza** — Hypothesis exhaustion: if two different coders fail the same task, the task framing is presumed wrong — the planner must rescope, cannot just reassign unchanged. Rescoping audit trail: original task becomes SUPERSEDED with explicit reason; new tasks reference what they replace. No silent rewrites. Bounded failure over prolonged negotiation. Agent crash recovery via CLI (`liza recover-agent`, `liza recover-task`). Sprint-level log analysis (`/liza-logs`) cross-correlates logs across agents to identify frictions — from misconfiguration in early setups to regressions from provider CLI updates in mature deployments. Anomaly logging with a documented typology.

**BMAD** — `bmad-correct-course` workflow handles significant mid-sprint changes. `bmad-retrospective` captures lessons learned after epic completion. Implementation readiness check (`bmad-check-implementation-readiness`) acts as a gate before implementation: PASS/CONCERNS/FAIL decision. `bmad-checkpoint-preview` provides guided, concern-ordered human review of commits, branches, or PRs. No equivalent to hypothesis exhaustion, no mechanical rescoping protocol, no crash recovery for agent processes. The approach is methodological — course-correct through process, not mechanical enforcement.

---

## 9. Specification & Planning

**Liza** — Specs are the durable memory of the system. "Every restart is a new mind with old artifacts." The pipeline decomposes goals through adversarial doer/reviewer pairs at each stage: vision → epics → user stories → architecture docs → code plans → implementation. Contract rule: "Spec first, code second, doc third." Agents cannot claim tasks for under-specified work (triggers BLOCKED, not guessing). Reviewers reject work that doesn't match spec, not just work that fails tests. Many-to-one transitions consolidate sibling tasks (e.g., N user stories → 1 architecture task). Automatic task decomposition based on complexity with dependency management for parallel execution. Entry point is a human-authored vision document; the README provides guidance on writing effective goal documents. Strong on decomposition discipline but lighter on upstream discovery — no built-in brainstorming, market research, PRFAQ, or UX design workflows.

**BMAD** — Significantly more comprehensive on upstream planning — a clear BMAD strength over Liza's lighter entry point. Analysis phase offers brainstorming (with guided facilitation coaching), market/domain/technical research, product brief, and PRFAQ (Amazon's Working Backwards, 5-stage coached workflow with subagent architecture). Planning phase has structured PRD creation (multi-step interview process with the PM agent producing FRs/NFRs, vision/differentiators, executive summary) and UX design (user journeys, component strategy, design patterns). Solutioning phase adds architecture with ADRs and implementation readiness gate. V6 improvement: epics and stories are now created after architecture, so architecture decisions directly inform story breakdown. Scale-adaptive: Quick Flow produces only a tech-spec for simple tasks; BMad Method track produces PRD + Architecture + UX; Enterprise adds Security + DevOps. The framework covers ideation-to-deployment.

The positioning question is not "who starts highest" but "what's the minimum human input that reliably produces working code." BMAD answers with iterative PM-agent interviews across phases — the human participates at each workflow. Liza answers with one front-loaded goal document authored via pairing (Coach mode for surfacing WHY, Challenger mode for stress-testing WHAT), then mechanical pipeline execution with no human in the loop between goal and merged code. As evidence, a non-public Diagnosis Design Liza run produced a complete three-tier application (FastAPI backend, Go CLI, React web UI) from a ~200-line goal document describing the method.

---

## 10. Code Review & Quality

**Liza** — Externally validated completion replaces self-certification. A coder agent cannot mark their own work complete. A reviewer examines the work and issues a binding verdict. Approval means merge eligibility. Rejection means specific, actionable feedback and another loop. The coder-reviewer interaction follows PR review dynamics — submission, feedback comments, verdict, revised submission. Approval and merge traceability: task state records `approved_by` and `merge_commit`. Commit SHA verification: reviewer verifies the SHA before examining work — no reviewing stale state.

**BMAD** — Code review as a structured multi-step workflow (`bmad-code-review`). Three parallel review layers: **Blind Hunter** (reviews code without access to original reasoning — information asymmetry by design), **Edge Case Hunter** (traces boundary conditions and unhandled paths), and **Acceptance Auditor** (validates against story acceptance criteria). Findings are normalized, deduplicated, and triaged into decision-needed / patch / defer / dismiss categories. The workflow can produce a clean review when all layers pass with no remaining findings, or produce action items for the human to apply, defer, or walk through. This is the most developed adversarial mechanism in BMAD — more than a generic "find problems" instruction. But enforcement remains at the prompt level: there is no mechanical gate preventing merge without approval.

---

## 11. Testing

**Liza** — TDD is both a contract rule and a code-enforced check. INVARIANTS §6: "Code tasks must include tests (TDD: tests first, then implementation); waiver requires explicit `tdd_not_required`." The Go supervisor checks this at review submission time (`submit_review.go`). The behavioral contract prohibits test corruption as Tier 0 invariant T0.3 — agents cannot greenwash tests by altering them to make failing code pass. Testing is integrated into the coding phase, not a post-implementation workflow.

**BMAD** — Two testing paths. Built-in QA workflow (`bmad-qa-generate-e2e-tests`) runs after epic completion: detect framework, identify features, generate API tests (status codes, response structure, happy path + error cases), generate E2E tests (semantic locators, visible-outcome assertions), run and verify. Test Architect (TEA) module provides 9 enterprise-grade workflows: test design, ATDD, automate, test review, traceability, NFR assessment, CI setup, framework scaffolding, release gate. TEA supports risk-based prioritization (P0-P3) and requirements traceability for regulated/compliance environments. Testing methodology is more comprehensive but runs post-implementation, not inline.

---

## 12. Human Role

**Liza** — The human owns intent and acts as observer/circuit-breaker, not an approval bottleneck. "Authority is exercised through a kill switch, not an approval queue." Within a sprint, agents are fully autonomous. Between sprints, the human reviews produced artifacts, provides continuous improvement feedback, and steers the next sprint via the Liza CLI. The TUI provides real-time visibility. The human can pause/resume the system, add tasks, trigger sprint checkpoints. The goal: humans focus on strategy and product vision while agents handle execution.

**BMAD** — The human is central and deeply in the loop. BMad agents "guide you through a structured process to bring out your best thinking in partnership with the AI." The PM agent interviews you for requirements. The architect guides you through technical decisions. The developer implements stories you've approved. `bmad-help` is an interactive guide telling you what to do next — it inspects your project state and recommends the appropriate workflow. The human participates actively in each workflow and each phase. This is a fundamentally different human-AI relationship than Liza's observer model.

---

## 13. Tooling & CLI

**Liza** — Full CLI: `liza setup`, `liza init`, `liza tui`, `liza agent <role>`, `liza validate`, `liza status`, `liza proceed`, `liza pause/resume`, `liza stop/start`, `liza sprint-checkpoint`, `liza recover-agent`, `liza recover-task`, `liza analyze`, `liza add-task`, `liza get tasks`. TUI with live system state, agent spawning with role autocompletion. Multi-provider setup: `liza setup --claude --codex --gemini --mistral`. Agent-specific tool configuration via `~/.liza/AGENT_TOOLS.md` (user-customizable). Log analysis skill (`/liza-logs`) for cross-agent friction identification. Recommends shell-proxy for ~90% token savings on command output, and fast-apply for semantic codebase edits.

**BMAD** — npm-based installer (`npx bmad-method install`) with interactive and non-interactive modes. Skills invoked by name in IDE (e.g., `bmad-create-prd`). Works with Claude Code, Cursor, Codex CLI. `bmad-help` interactive guide that detects installed modules and project state. Module system for extensions with community marketplace. GitHub Copilot installer generates enriched `.agent.md` and `.prompt.md` files. Multi-language documentation (English, Vietnamese, Chinese, French, Czech). `bmad uninstall` with selective component removal.

*Token economics* diverge sharply. Liza ships a shell-proxy (RTK) that compresses tool output (git, go, pytest, etc.) by ~90% before it hits the model, and recommends fast-apply for semantic edits to avoid full-file reads. BMAD's fresh-chat-per-workflow architecture means planning artifacts (PRD, architecture, epics, stories, project-context) are reloaded as context at each workflow entry, and its multi-agent workflows (three-reviewer code review, multi-stage PRFAQ) multiply prompt tokens by the number of concurrent agents. For adopters running these at scale on their own budget, this is a first-order practical difference that compounds over a sprint.

---

## 14. LLM Provider Support

**Liza** — Multi-provider via setup flags: Claude Code, Codex, Gemini, Mistral. Agent-specific activation with skill symlinks and contract config per provider. Model capability assessment documented — the contract is a capability test requiring meta-cognitive machinery (parsing instructions as executable specifications, observing state, pausing at gates). Not all models pass. Provider diversity quorum: ≥2 distinct providers required for multi-reviewer quorum, mechanically preventing single-provider bias in review decisions.

**BMAD** — IDE-agnostic — works with any AI coding assistant supporting custom system prompts or project context. Claude Code recommended. Also supports Cursor, Codex CLI, GitHub Copilot. Cross-platform agent team support in V6. No model-specific behavioral requirements — the approach relies on process and context rather than behavioral constraints that some models might fail.

---

## 15. Extensibility

**Liza** — Skills system (skills directory with per-role capabilities). Behavioral contract customizable per project via project guardrails. Pipeline configuration via YAML (`pipeline.yaml`). Agent tools configuration per user (`~/.liza/AGENT_TOOLS.md`). Open-source (Apache 2.0) but no formal module/plugin ecosystem or marketplace.

**BMAD** — Extensive module ecosystem. BMad Builder (BMB) for creating custom agents, workflows, and modules. Community marketplace via installer with three-tier selection: official, community (category drill-down from marketplace index), custom URL. Current official modules: BMM (core, 34+ workflows), BMad Builder, Test Architect (TEA), Game Dev Studio (BMGD), Creative Intelligence Suite (CIS). V6 skills architecture converts everything to SKILL.md entrypoints. npm distribution. Universal source support for custom modules (GitHub, GitLab, Bitbucket, self-hosted, local paths). Formal contribution guidelines.

---

## 16. Maturity & Adoption

**Liza** — ~150 stars, 23 forks, 1,069 commits, 15 public tags, 2 public branches. Single primary author. Current public release v0.7.0. Self-implementing its own features since v0.4.0 — all major changes after that version are implemented using Liza's own multi-agent mode. External validation: Soufiane Keli (Octo Technology VP Software Engineering) placed Liza at L4 (Collaborative Agent Networks) on his 5-level AI maturity model, explicitly grouping Liza alongside BMAD and BEADS at the same maturity level. Demo video available. Active development.

**BMAD** — ~45.2k stars, ~5.4k forks, 132 contributors, 1,826 commits, 29 releases as of the source snapshot. npm package with semantic versioning. Documentation site with tutorials in 5 languages. Discord community. YouTube channel. Active sponsorship model (Buy Me a Coffee, corporate sponsorship). Multiple Medium articles and community write-ups. Trademark protection. Rapid iteration — V6 stable release plus ongoing v6.3.x updates.

*Governance profile* is a meaningful differentiator beyond the raw numbers. Liza depends on a single primary author — high design coherence, tight invariant discipline, but a correspondingly low bus factor and no formal community consensus mechanism. BMAD has distributed governance with 130+ contributors, Discord moderation, corporate sponsors, and trademark protection — broader survival surface and more institutional backing, at the cost of slower convergence on breaking architectural changes. Adopters weighing long-term bet stability should read these differently.

---

## 17. Documentation & Observability

**Liza** — Blackboard provides full state visibility into agent execution. Activity log (`.liza/log.yaml`) records all events. Agent logs are recorded and analyzed at sprint boundaries via `/liza-logs` — cross-correlates logs across agents to identify frictions (setup issues, provider CLI regressions, context budget growth, tool failure patterns). Token optimization analysis from logs. Approval and merge traceability on every task (`approved_by`, `merge_commit`). Rescoping audit trail (SUPERSEDED with reason, new tasks reference predecessors). User-facing docs are currently in `specs/` and `docs/` directories.

**BMAD** — Comprehensive documentation site (docs.bmad-method.org) organized in four sections: tutorials, how-to guides, explanations, reference. Workflow map with visual diagrams. `sprint-status.yaml` tracks story progress at sprint level. `project-context.md` captures implementation conventions. `bmad-help` provides interactive project-state-aware guidance. Multi-language translations. The documentation focuses on human-readable project artifacts (PRD, architecture, epics) and methodology guidance, not on agent execution observability or runtime diagnostics.

---

## 18. Operating Model — Cross-Dimension Synthesis

**Execution substrate** — BMAD is primarily a method/workflow layer installed into an AI IDE: agents, skills, prompts, workflows, and project artifacts. Liza is a runtime substrate: CLI, supervisor processes, blackboard state, locks, leases, worktrees, validation, recovery, and merge authority. BMAD teaches the AI assistant how to collaborate; Liza wraps agents in executable control infrastructure.

**Context economy** — BMAD reduces context pressure through fresh chats, just-in-time workflow step files, project artifacts, and document sharding. Liza deliberately spends context on behavioral contracts, approval gates, stop conditions, and role-specific constraints, then manages degradation with explicit context tiers. BMAD economizes context to preserve task focus; Liza invests context to shape agent behavior.

**Customization philosophy** — BMAD invites extension: custom agents, custom workflows, official modules, community marketplace sources, and BMad Builder as a meta-module. Liza is configurable, but its value depends on preserving hard invariants. BMAD optimizes adaptability; Liza optimizes constraint preservation.

**Artifact lifecycle** — BMAD's durable outputs are mainly human-readable product artifacts: PRD, architecture, UX spec, epics, stories, sprint status, and project context. Liza also uses durable specs, but adds operational execution state: task claims, leases, review commits, approvals, merge commits, histories, handoffs, and anomalies. BMAD's artifacts support planning continuity; Liza's artifacts support runtime auditability and recovery.

**Failure ownership** — BMAD expects an active human to filter false positives, resolve ambiguity, and decide next workflow moves. Liza routes failure into machine-visible states: BLOCKED, REJECTED, SUPERSEDED, crash recovery, circuit-breaker analysis. BMAD makes failure a collaboration moment; Liza makes failure a state transition.

**Scope of ambition** — BMAD covers the broader product lifecycle: ideation, market/domain/technical research, PRFAQ, UX, documentation, test strategy, custom modules, and implementation. Liza is narrower but deeper: autonomous coding execution with state integrity, adversarial review pairs, supervised merges, and recovery mechanics.

---

## 19. Where They Overlap

Both systems share several architectural concepts:

- Adversarial review (Liza: mechanically enforced doer/reviewer pairs; BMAD: three-layer parallel reviewers at the prompt level)
- Sprint-based development cycles
- Spec/story-driven implementation
- Code review as a quality gate
- Role specialization of agents
- Document-based context engineering
- Retrospectives for continuous improvement
- Architecture-informed story decomposition

Liza's README explicitly acknowledges BMAD in its acknowledgments for "role templates and workflow patterns." Octo Technology's assessment places both at the same L4 maturity level.

---

## 20. Framework Failure Modes

Both systems have intended weaknesses — places where the design choice that makes them strong elsewhere makes them worse here. Named symmetrically so the comparison doesn't read as one-sided.

**Liza** fails when:
- The work is small or exploratory. The full MAS pipeline — detailed-spec authoring, architecture docs, and integration phase — is disproportionate to trivial changes. Per-task ceremony (DoR/DoD, approval gates, adversarial pairs) stays valuable at any size; what over-invests is the sub-pipeline invocation. Pairing mode is Liza's answer for this envelope; MAS mode should not be reached for work that doesn't earn the sub-pipeline cost.
- The model can't hold the contract. The contract requires meta-cognitive machinery — parsing instructions as executable specifications, observing own state, pausing at gates. Weaker, smaller, or latency-optimized models fail the contract as a capability test; they pass tokens but miss the invariants. Provider choice is constrained.
- Requirements are genuinely unclear upstream. Liza has no native product-discovery capability (no brainstorming, market research, PRFAQ, UX workflows) — if the vision document is thin, the pipeline decomposes thin input into thin output, and the adversarial pairs can only reject against a spec that doesn't exist.
- Operational setup cost is disproportionate. Multi-agent mode coordinates multiple terminal sessions and git worktrees, and multi-reviewer quorum needs multiple provider credentials; recommended tooling such as RTK adds another per-machine setup step. Throwaway experiments or single-task work don't earn back the setup investment.
- Single-maintainer risk. Design coherence comes from one author; the flip side is no formal contribution pipeline, no contributor consensus on direction, and no continuity guarantee.

**BMAD** fails when:
- Agents bypass the prompt-level discipline. Adversarial reviewers, readiness gates, and role separation are prompt instructions — there is no mechanical enforcement. A model that ignores halt instructions or review workflow sequencing produces visibly compliant but actually-broken output, and nothing blocks a merge.
- Work must persist across sessions. Fresh-chat-per-workflow means no inter-workflow state beyond what's written to disk; a debugging session that spans workflows loses mid-task reasoning.
- An agent process crashes. No blackboard, no leases, no recover protocol — if a chat session dies mid-workflow, the work in that session is lost, not reassignable.
- Multiple agents must execute in parallel on the same codebase. The framework is sequential at the workflow level; concurrent multi-agent execution on separate tasks is not a supported coordination pattern (Party Mode is conversational, not parallel work execution).
- The human isn't available. BMAD assumes an active human collaborator at each workflow. Long-running autonomous execution without a human in the loop is not the design target.

These aren't bugs — they're boundary conditions. Each framework is strong inside its envelope and degraded outside it. Choosing between them is partly choosing which envelope matches the work.

---

## 21. Where They Diverge Most

The fundamental difference is the trust boundary and the execution substrate.

**BMAD** trusts agents when given good context and structured process. Quality comes from methodology — agile best practices, progressive document creation, scale-adaptive planning, multi-layer adversarial review. The framework optimizes the human-AI collaboration experience. The human thinks alongside the AI.

**Liza** does not trust agents and mechanically constrains workflow behavior through a tiered invariant system backed by ~34k LOC of Go enforcement code and ~90k lines of tests. Quality comes from suppressing the 55+ empirically documented ways agents fail, with cross-references to academic failure-mode taxonomies. The framework optimizes for mechanically enforced workflow correctness and externally reviewed autonomous execution. The supervisor is code, not prompts. Provider diversity quorum prevents single-model blind spots.

This maps to different use cases: BMAD is stronger for human-in-the-loop development where the AI helps you think through problems across the full product lifecycle — from brainstorming through architecture to implementation. Liza is stronger for autonomous multi-agent execution where you want agents to produce code without continuous human intervention, with mechanically enforced workflow constraints and external review against known failure modes.

They are architecturally complementary. BMAD's upstream methodology (analysis, PRD, architecture, UX) feeds naturally into Liza's downstream execution (spec decomposition, adversarial coding, mechanical review, supervised merges). BMAD optimizes the human-AI collaboration experience across the product lifecycle; Liza optimizes mechanically enforced workflow correctness in autonomous multi-agent execution.
