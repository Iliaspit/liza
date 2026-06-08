# Architectural Issues

Persistent record of issues identified by architectural analysis skills.

**Skills that contribute here:**
- `systemic-thinking` — Systemic coherence and risk analysis
- `software-architecture-review` — Code-level architectural patterns and smells

## Update Policy

1. Keep unresolved concerns in their thematic sections (load-bearing, tensions, smells, etc.).
2. When an issue is fixed, record it in `Completed Fixes` and `Fixed (Traceability)` with commit references.
3. Do not delete resolved issues from this document without preserving traceability metadata.
4. If a resolved issue is removed from an active section, add/update its `Fixed (Traceability)` entry in the same change.
5. `Fix Details` keeps the long-form rationale; `Fixed (Traceability)` is the canonical index for historical closure.
6. Keep the **Open Issues Summary** table in sync when adding, resolving, or re-prioritizing issues.

## Table of Contents

- [Update Policy](#update-policy)
- [Open Issues Summary](#open-issues-summary)
- [Structural Load-Bearing Elements](#structural-load-bearing-elements)
  - [Mode Selection Trigger Coupled to Prompt Lexeme](#mode-selection-trigger-coupled-to-prompt-lexeme)
  - [Role Pair Field as Single Point of Configuration Truth](#role-pair-field-as-single-point-of-configuration-truth)
- [Systemic Tensions](#systemic-tensions)
  - [MCP Cross-Layer Read Dependency](#mcp-cross-layer-read-dependency)
  - [Role-Boundary Severity Drift](#role-boundary-severity-drift)
  - [Merge Execution Authority Split](#merge-execution-authority-split)
  - [Sprint Completion Signal Diverges from Active Scope](#sprint-completion-signal-diverges-from-active-scope)
  - [Task Type Registry Only Supports Coding Workflows](#task-type-registry-only-supports-coding-workflows)
  - [Prompts Layer Imports Business Logic](#prompts-layer-imports-business-logic)
  - [Commands Layer Imports Agent Runtime](#commands-layer-imports-agent-runtime)
  - [Orchestrator Role Dissolution Without Replacement](#orchestrator-role-dissolution-without-replacement)
  - [Cross-Pair Knowledge Required by Single-Pair Reviewers](#cross-pair-knowledge-required-by-single-pair-reviewers)
- [Feedback Loops](#feedback-loops)
  - [Supervisor Wait-Claim-Spawn Loop](#supervisor-wait-claim-spawn-loop)
  - [Contract Complexity vs Context Pressure](#contract-complexity-vs-context-pressure)
  - [Issue Registry Resolution Drift](#issue-registry-resolution-drift)
  - [Contract-Driven Safety vs Structural Enforcement Asymptote](#contract-driven-safety-vs-structural-enforcement-asymptote)
- [Assumptions](#assumptions)
  - [Implicit Orchestrator Provenance Default](#implicit-orchestrator-provenance-default)
  - [Spec Maturity Dependency](#spec-maturity-dependency)
  - [Well-Formed Blackboard State](#well-formed-blackboard-state)
  - [Single-Goal Data Model Constrains Applicability](#single-goal-data-model-constrains-applicability)
  - [Orchestrator State Change Verification is Non-Binding](#orchestrator-state-change-verification-is-non-binding)
  - [`one-to-one` Transition Child Field Generation Unspecified](#one-to-one-transition-child-field-generation-unspecified)
- [Stress Points](#stress-points)
  - [Filesystem/Git I/O Contention](#filesystemgit-io-contention)
  - [Cache Coherence Gap in Multi-Process Deployments](#cache-coherence-gap-in-multi-process-deployments)
  - [Manual Sprint Transitions as Scaling Bottleneck](#manual-sprint-transitions-as-scaling-bottleneck)
- [Fragility](#fragility)
  - [Cross-Script State Mutation](#cross-script-state-mutation)
  - [Bootstrap Artifact Path Drift](#bootstrap-artifact-path-drift)
  - [File-Based Spec References Without Version Anchors](#file-based-spec-references-without-version-anchors)
  - [Review Lease Orphaning Without Automatic Reclamation](#review-lease-orphaning-without-automatic-reclamation)
  - [SetTaskOutput spec_ref Validation Gap](#settaskoutput-spec_ref-validation-gap)
- [Blind Spots](#blind-spots)
  - [Contract Effectiveness Self-Certification](#contract-effectiveness-self-certification)
  - [Initialization Completion Unverifiable](#initialization-completion-unverifiable)
  - [No Source Type for Pre-Implementation Spec Findings](#no-source-type-for-pre-implementation-spec-findings)
  - [Prompt-Build-to-Execution State Drift](#prompt-build-to-execution-state-drift)
  - [No Feedback Signal for Specification Quality](#no-feedback-signal-for-specification-quality)
  - [No Reverse Data Channel in Inter-Pair Transitions](#no-reverse-data-channel-in-inter-pair-transitions)
  - [Retrospective Findings Don't Feed Forward to Next Sprint](#retrospective-findings-dont-feed-forward-to-next-sprint)
  - [Sprint Metrics Lossy at Sprint Boundary](#sprint-metrics-lossy-at-sprint-boundary)
- [Trajectory](#trajectory)
  - [Blackboard Growth Without Pruning](#blackboard-growth-without-pruning)
  - [Role Addition Accelerates Contract Complexity Pressure](#role-addition-accelerates-contract-complexity-pressure)
  - [Anomaly Detail Validation Incomplete](#anomaly-detail-validation-incomplete)
  - [Task Type Registry is Partial Abstraction](#task-type-registry-is-partial-abstraction)
  - [Spec Corpus Lacks Lifecycle Management](#spec-corpus-lacks-lifecycle-management)
  - [Metrics Collection Without Query Interface](#metrics-collection-without-query-interface)
  - [No Query Layer](#no-query-layer)
- [Cascades](#cascades)
  - [Sub-Pipeline Expansion Multiplies Every Existing Issue](#sub-pipeline-expansion-multiplies-every-existing-issue)
  - [Fan-Out Amplifies Decomposition Errors Across Pipeline Stages](#fan-out-amplifies-decomposition-errors-across-pipeline-stages)
- [Accepted v1 Limitations](#accepted-v1-limitations)
  - [Orchestrator as Single Semantic Interpreter](#orchestrator-as-single-semantic-interpreter)
  - [Supervisor as Single Correctness Gate](#supervisor-as-single-correctness-gate)
  - [Spec Completeness vs Reality](#spec-completeness-vs-reality)
  - [Code Reviewer Structural Accountability Gap](#code-reviewer-structural-accountability-gap)
  - [Hypothesis Exhaustion Without Root Cause](#hypothesis-exhaustion-without-root-cause)
  - [Restart/Lease Churn Under Load](#restartlease-churn-under-load)
  - [Human Availability as Bottleneck](#human-availability-as-bottleneck)
  - [Supervisor Contention](#supervisor-contention)
  - [Dual Contract Delivery Paths](#dual-contract-delivery-paths)
  - [Self-Reported Validation](#self-reported-validation)
  - [Kill Switch Granularity](#kill-switch-granularity)
- [Structural Debt](#structural-debt)
  - [Decompose proceed.go (1,200 LOC)](#decompose-proceedgo-1200-loc)
  - [Decompose init.go (854 LOC)](#decompose-initgo-854-loc)
  - [Decompose supervisor.go (831 LOC)](#decompose-supervisorgo-831-loc)
- [Completed Fixes](#completed-fixes)
- [Fixed (Traceability)](#fixed-traceability)
- [Fix Details](#fix-details)

## Open Issues Summary

| Priority | Category | Issue |
|----------|----------|-------|
| **high** | LOAD-BEARING | [Mode Selection Trigger Coupled to Prompt Lexeme](#mode-selection-trigger-coupled-to-prompt-lexeme) |
| **high** | LOAD-BEARING | [Orchestrator as Single Semantic Interpreter](#orchestrator-as-single-semantic-interpreter) |
| **high** | LOAD-BEARING | [Supervisor as Single Correctness Gate](#supervisor-as-single-correctness-gate) |
| **high** | TENSION | [Role-Boundary Severity Drift](#role-boundary-severity-drift) |
| **high** | TENSION | [Code Reviewer Structural Accountability Gap](#code-reviewer-structural-accountability-gap) |
| **high** | FEEDBACK | [Contract Complexity vs Context Pressure](#contract-complexity-vs-context-pressure) |
| **high** | STRESS POINT | [Filesystem/Git I/O Contention](#filesystemgit-io-contention) |
| **high** | STRESS POINT | [Cache Coherence Gap in Multi-Process Deployments](#cache-coherence-gap-in-multi-process-deployments) |
| **high** | FRAGILITY | [Dual Contract Delivery Paths](#dual-contract-delivery-paths) |
| **high** | LOAD-BEARING | [Role Pair Field as Single Point of Configuration Truth](#role-pair-field-as-single-point-of-configuration-truth) |
| **high** | TENSION | [Orchestrator Role Dissolution Without Replacement](#orchestrator-role-dissolution-without-replacement) |
| **high** | FEEDBACK | [Contract-Driven Safety vs Structural Enforcement Asymptote](#contract-driven-safety-vs-structural-enforcement-asymptote) |
| **high** | CASCADE | [Sub-Pipeline Expansion Multiplies Every Existing Issue](#sub-pipeline-expansion-multiplies-every-existing-issue) |
| **high** | BLIND SPOT | [No Feedback Signal for Specification Quality](#no-feedback-signal-for-specification-quality) |
| **high** | BLIND SPOT | [No Reverse Data Channel in Inter-Pair Transitions](#no-reverse-data-channel-in-inter-pair-transitions) |
| **high** | CASCADE | [Fan-Out Amplifies Decomposition Errors Across Pipeline Stages](#fan-out-amplifies-decomposition-errors-across-pipeline-stages) |
| **high** | TENSION | [Cross-Pair Knowledge Required by Single-Pair Reviewers](#cross-pair-knowledge-required-by-single-pair-reviewers) |
| **medium** | ASSUMPTION | [`one-to-one` Transition Child Field Generation Unspecified](#one-to-one-transition-child-field-generation-unspecified) |
| **medium** | BLIND SPOT | [Retrospective Findings Don't Feed Forward to Next Sprint](#retrospective-findings-dont-feed-forward-to-next-sprint) |
| **medium** | STRESS POINT | [Manual Sprint Transitions as Scaling Bottleneck](#manual-sprint-transitions-as-scaling-bottleneck) |
| ~~medium~~ | ~~TENSION~~ | ~~[MCP Cross-Layer Read Dependency](#mcp-cross-layer-read-dependency) — resolved: MCP removed (`90c132d5`)~~ |
| **medium** | TENSION | [Merge Execution Authority Split](#merge-execution-authority-split) |
| **medium** | TENSION | [Sprint Completion Signal Diverges from Active Scope](#sprint-completion-signal-diverges-from-active-scope) |
| **medium** | TENSION | [Task Type Registry Only Supports Coding Workflows](#task-type-registry-only-supports-coding-workflows) |
| **medium** | TENSION | [Spec Completeness vs Reality](#spec-completeness-vs-reality) |
| **medium** | FEEDBACK | [Supervisor Wait-Claim-Spawn Loop](#supervisor-wait-claim-spawn-loop) |
| **medium** | FEEDBACK | [Issue Registry Resolution Drift](#issue-registry-resolution-drift) |
| **medium** | FEEDBACK | [Hypothesis Exhaustion Without Root Cause](#hypothesis-exhaustion-without-root-cause) |
| **medium** | FEEDBACK | [Restart/Lease Churn Under Load](#restartlease-churn-under-load) |
| **medium** | ASSUMPTION | [Implicit Orchestrator Provenance Default](#implicit-orchestrator-provenance-default) |
| **medium** | ASSUMPTION | [Well-Formed Blackboard State](#well-formed-blackboard-state) |
| **medium** | ASSUMPTION | [Orchestrator State Change Verification is Non-Binding](#orchestrator-state-change-verification-is-non-binding) |
| **medium** | ASSUMPTION | [Human Availability as Bottleneck](#human-availability-as-bottleneck) |
| **medium** | STRESS POINT | [Supervisor Contention](#supervisor-contention) |
| **medium** | FRAGILITY | [Cross-Script State Mutation](#cross-script-state-mutation) |
| **medium** | FRAGILITY | [Bootstrap Artifact Path Drift](#bootstrap-artifact-path-drift) |
| **medium** | FRAGILITY | [File-Based Spec References Without Version Anchors](#file-based-spec-references-without-version-anchors) |
| **medium** | FRAGILITY | [Review Lease Orphaning Without Automatic Reclamation](#review-lease-orphaning-without-automatic-reclamation) |
| **medium** | FRAGILITY | [State Validation Composition Gap](#state-validation-composition-gap) |
| **medium** | FRAGILITY | [SetTaskOutput spec_ref Validation Gap](#settaskoutput-spec_ref-validation-gap) |
| **medium** | FRAGILITY | [INVARIANTS.md §7 Clean Worktree Not Enforced at Submission](#invariantsmd-7-clean-worktree-not-enforced-at-submission) |
| **medium** | FRAGILITY | [INVARIANTS.md §6 Provider Diversity Not Enforced at Verdict Time](#invariantsmd-6-provider-diversity-not-enforced-at-verdict-time) |
| **medium** | BLIND SPOT | [Contract Effectiveness Self-Certification](#contract-effectiveness-self-certification) |
| **medium** | BLIND SPOT | [Initialization Completion Unverifiable](#initialization-completion-unverifiable) |
| **medium** | BLIND SPOT | [Sprint Metrics Lossy at Sprint Boundary](#sprint-metrics-lossy-at-sprint-boundary) |
| **medium** | ACCEPTED v1 | [Self-Reported Validation](#self-reported-validation) |
| **low** | ASSUMPTION | [Spec Maturity Dependency](#spec-maturity-dependency) |
| **low** | ASSUMPTION | [Single-Goal Data Model Constrains Applicability](#single-goal-data-model-constrains-applicability) |
| **low** | BLIND SPOT | [No Source Type for Pre-Implementation Spec Findings](#no-source-type-for-pre-implementation-spec-findings) |
| **low** | BLIND SPOT | [Prompt-Build-to-Execution State Drift](#prompt-build-to-execution-state-drift) |
| **low** | TRAJECTORY | [Blackboard Growth Without Pruning](#blackboard-growth-without-pruning) |
| **low** | TRAJECTORY | [Role Addition Accelerates Contract Complexity Pressure](#role-addition-accelerates-contract-complexity-pressure) |
| **low** | TRAJECTORY | [Anomaly Detail Validation Incomplete](#anomaly-detail-validation-incomplete) |
| **low** | TRAJECTORY | [Task Type Registry is Partial Abstraction](#task-type-registry-is-partial-abstraction) |
| **low** | TRAJECTORY | [Spec Corpus Lacks Lifecycle Management](#spec-corpus-lacks-lifecycle-management) |
| **low** | TRAJECTORY | [Metrics Collection Without Query Interface](#metrics-collection-without-query-interface) |
| **low** | TENSION | [Prompts Layer Imports Business Logic](#prompts-layer-imports-business-logic) |
| **low** | TENSION | [Commands Layer Imports Agent Runtime](#commands-layer-imports-agent-runtime) |
| **low** | TRAJECTORY | [No Query Layer](#no-query-layer) |
| **low** | ACCEPTED v1 | [Kill Switch Granularity](#kill-switch-granularity) |
| **medium** | STRUCTURAL DEBT | [Decompose proceed.go (1,200 LOC)](#decompose-proceedgo-1200-loc) |
| **medium** | STRUCTURAL DEBT | [Decompose init.go (854 LOC)](#decompose-initgo-854-loc) |
| **medium** | STRUCTURAL DEBT | [Decompose supervisor.go (831 LOC)](#decompose-supervisorgo-831-loc) |

**Counts:** 17 high, 31 medium, 14 low — 62 open issues total. *(2026-04-13: MCP Cross-Layer Read Dependency resolved — MCP removed)*

---

## Structural Load-Bearing Elements

Single points of failure with no redundancy or validation mechanism.

### Mode Selection Trigger Coupled to Prompt Lexeme

**Skill:** systemic-thinking
**Category:** LOAD-BEARING

**Issue:** Mode selection in CORE depends on detecting specific bootstrap wording (`"You are a Liza ... agent"` for Liza mode, `MODE: SUBAGENT` for subagent mode). The prompt template (`base_prompt.tmpl`) currently generates matching wording (`"You are a Liza {{.Role}} agent"`), so there is no active mismatch. However, because gate semantics and approval behavior branch entirely on this lexical detection, the coupling is load-bearing despite being outside the blackboard/state machine. A template edit, prompt builder refactor, or alternative CLI integration that changes the wording would silently change mode without any structural guard.

**Implication:** Mode detection correctness depends on convention alignment between two independently maintained artifacts (CORE.md detection table and prompt templates). No compile-time or runtime check validates this alignment.

**Current mitigation:** Prompt template output currently matches CORE.md detection patterns. Builder tests (`builder_test.go`) assert the expected prefix, providing regression coverage.

**Future options:**
- Add explicit mode declaration outside free-text prompt (e.g., structured field/environment variable)
- Add startup self-check that fails fast when expected mode and detected mode diverge
- Record detected mode in blackboard state for runtime observability

### Role Pair Field as Single Point of Configuration Truth

**Skill:** systemic-thinking
**Category:** LOAD-BEARING
**Related:** [Mode Selection Trigger Coupled to Prompt Lexeme](#mode-selection-trigger-coupled-to-prompt-lexeme)

**Issue:** The sub-pipelines spec introduces `role_pair` on tasks as the mechanism that replaces `task.type` for claimability, state resolution, and transition determination. State-machines.md notes that `role_pair` "supersedes" the type field. The pipeline YAML declares role-pairs, and `role_pair` on each task links to its entry. A task's lifecycle (what states are valid, who can claim it, what "approved" means, whether it's sprint-terminal) is derived from a YAML string referencing a configuration section. A typo, a renamed role-pair, or a pipeline config mismatch makes a task unclaimable, unresolvable, or invisible to the supervisor. The configuration is loaded per-operation from disk (architecture review pass 13: "pipeline config loaded per-operation from 14 ops files"), so there's no cached schema validation catching a bad config before tasks are stuck.

**Implication:** Pipeline configuration correctness becomes the single most consequential validation in the system, yet it's validated at point-of-use rather than at initialization, making misconfiguration a runtime failure across every task operation simultaneously.

**Current mitigation:** `liza validate` checks task states against known valid states. Pipeline YAML is loaded by `pipeline.LoadFrozen()`.

**Future options:**
- Validate pipeline config at `liza init` time and cache the validated schema
- Add compile-time or startup-time consistency check between pipeline YAML and role constants
- Reject tasks with unknown `role_pair` values at creation time, not at claim time

---

## Systemic Tensions

Design contradictions that create structural friction.

**Future options:**
- Extract `ops.RegisterAgent` / `ops.UnregisterAgent` for agent lifecycle
- Accept the split as intentional: ops owns task lifecycle, agent owns agent lifecycle (document the boundary)

### MCP Cross-Layer Read Dependency — RESOLVED

**Skill:** systemic-thinking
**Category:** TENSION → RESOLVED
**Resolved:** MCP server removed in `90c132d5` (ADR-0057). The cross-layer dependency no longer exists — `commands` now has a single consumer (CLI). The underlying "No Query Layer" trajectory issue remains but is lower-pressure with only one presentation layer.

**Original issue:** MCP read-only handlers imported `commands` (CLI presentation layer), creating a cross-layer dependency. Resolved by removing MCP entirely — agents now interact via CLI `--json` mode.

### Role-Boundary Severity Drift

**Skill:** systemic-thinking
**Category:** TENSION

**Issue:** Vision-level contract text classifies role-boundary violations as Tier 0 (contract termination), while the active Multi-Agent mode contract classifies the same class of violations as Tier 1. The same behavioral breach therefore has two incompatible severities across authoritative artifacts.

**Implication:** Violation handling semantics become document-dependent, so recovery behavior can vary by which artifact an agent or operator treats as canonical.

**Current mitigation:** None structural; conflict is resolved ad hoc by whichever document is consulted first.

**Future options:**
- Align role-boundary severity to a single tier across all mode and vision artifacts
- Add consistency checks in contract maintenance workflow for severity-classified rules
- Publish one canonical severity table referenced by all contracts

### Merge Execution Authority Split

**Skill:** systemic-thinking
**Category:** TENSION

**Issue:** Task/worktree protocols state that Code Reviewer executes merge on approval, while role/supervision architecture defines Code Reviewer as read-only and supervisor as the merge executor. Merge authority and operational responsibility are split across documents.

**Implication:** Integration ownership is ambiguous, weakening accountability and making post-incident reconstruction of merge authority less reliable.

**Current mitigation:** Runtime flow appears supervisor-driven in architecture documents, so practical execution tends to converge despite specification drift.

**Future options:**
- Normalize all protocol docs to one merge authority model
- Record merge executor identity explicitly in task history for auditability
- Add validation/docs linting to flag authority contradictions across artifacts

### Task Type Registry Only Supports Coding Workflows

**Skill:** systemic-thinking
**Category:** TENSION
**Related:** [Task Type Registry is Partial Abstraction](#task-type-registry-is-partial-abstraction)

**Issue:** The task workflow registry (`taskWorkflows` in `internal/models/state.go`) contains exactly one entry: `coding` maps to `[coder, code_reviewer]`. The claimability logic in `IsClaimable()` derives role eligibility from this registry. Adding any upstream role that produces specifications (e.g., turning vision docs into PRDs) requires either: (a) creating a new task type with its own workflow, or (b) operating outside the task system entirely.

**Implication:** Pre-implementation specification work cannot be tracked, assigned, or validated through the same mechanisms as code tasks. The blackboard has no structural concept of "spec production" as work that progresses through states with agents assigned to it.

**Current mitigation:** Specification work is assumed to be complete before `liza init` runs, produced by humans or external processes.

**Future options:**
- Extend task type registry with `specification` or `prd` type and corresponding workflow
- Create explicit `spec_writer` role constant and claimability rules
- Document that spec production operates outside Liza workflow (human-only phase)

### Sprint Completion Signal Diverges from Active Scope

**Skill:** systemic-thinking
**Category:** TENSION

**Issue:** Sprint governance allows sprint completion when only the original planned task list is terminal, while replacement tasks created by rescoping may still be active. The completion signal is therefore cadence-based rather than work-closure-based.

**Implication:** A sprint can report completion while unresolved implementation risk remains in flight under replacement tasks.

**Current mitigation:** Pipeline-created children (from `ExecuteAvailableTransitions`, executed by orchestrator PreWork after planning checkpoint is resumed) are automatically added to `sprint.scope.planned[]`, preventing premature sprint completion when planning tasks transition to coding tasks. Orchestrator-created replacement tasks (rescoping) are still NOT automatically added — humans must manually update scope or wait for all active tasks. Sprint governance protocol (`sprint-governance.md`) documents this as expected behavior.

**Remaining gap:** Orchestrator rescoping (SUPERSEDED → replacement tasks) does not auto-update scope. This is intentional — rescoping is a human-guided decision, and auto-inclusion could mask scope creep.

**Future options:**
- Add an alternate completion criterion based on all active (planned + replacement) tasks
- Separate cadence checkpoint status from true work-closure status

### Prompts Layer Imports Business Logic

**Skill:** software-architecture-review
**Category:** TENSION
**Coupled with:** [No Query Layer](#no-query-layer)

**Issue:** `internal/prompts/builder.go` imports `internal/ops` to call three functions: `LoadDetectionContext` (pipeline config lookup), `GetLatestScopeExtensions` (scope-extension file search), and `IsPlanningPair` (role-pair classification). This creates a `prompts → ops` dependency — the prompt-building layer (presentation) reaches into the business-logic layer for data it needs to assemble prompt context. The dependency inverts the expected flow where `ops` or `agent` would push context *into* prompt building, not have prompts pull from ops.

**Implication:** Prompt assembly becomes coupled to ops implementation details. Changes to pipeline config loading or scope-extension storage require touching the prompts package. The dependency also makes `prompts` harder to test in isolation.

**Current mitigation:** The three functions are pure queries (no side effects beyond file reads), so the coupling is dormant — it creates no mutation-path risk.

**Future options:**
- Have the caller (agent) resolve these values and pass them into prompt builder as parameters
- Extract the three functions to a shared query package that both `ops` and `prompts` can import
- Accept the coupling and document `prompts → ops` as an intentional read-path shortcut

### Commands Layer Imports Agent Runtime

**Skill:** software-architecture-review
**Category:** TENSION
**Coupled with:** [No Query Layer](#no-query-layer), [MCP Cross-Layer Read Dependency](#mcp-cross-layer-read-dependency)

**Issue:** `internal/commands/status.go` imports `internal/agent` to call `agent.DetectOrchestratorWakeTriggers()` — a pure state-query function that examines blackboard state to determine whether orchestrator should wake. This creates a `commands → agent` dependency where the CLI presentation layer reaches into the agent runtime layer for a query function. The function itself is a stateless query (reads tasks and checks conditions), but its location in the agent package creates a boundary crossing.

**Implication:** The CLI layer gains a compile-time dependency on agent internals. As agent evolves (new wake conditions, supervisor changes), the commands package must track those changes. This is a specific instance of the broader pattern where query logic lives wherever it was first needed rather than in a shared query layer.

**Current mitigation:** `DetectOrchestratorWakeTriggers` is a pure function with no side effects — it reads state and returns a struct. The coupling is limited to one call site (`commands/status.go:282`).

**Future options:**
- Move `DetectOrchestratorWakeTriggers` to `models/diagnostics.go` (already partially serves as query home)
- Have the status command call through `ops` rather than reaching into `agent` directly
- Extract to a query layer alongside the MCP cross-layer read functions

---

### Orchestrator Role Dissolution Without Replacement

**Skill:** systemic-thinking
**Category:** TENSION
**Related:** [Orchestrator as Single Semantic Interpreter](#orchestrator-as-single-semantic-interpreter)

**Issue:** The sub-pipelines spec renames Planner to Orchestrator, strips its planning responsibility into Code Planner, and reduces the Orchestrator to "creating a task for the Code Planner." Yet roles.md retains the Orchestrator with 8 substantial capabilities: rescoping tasks, resolving blocked reviews, hypothesis exhaustion handling, discovery conversion, goal alignment tracking, spec gap management, multi-blocked-task sequencing, and systemic finding disposition. The Orchestrator's wake triggers (blocked task, hypothesis exhaustion, integration failure, immediate discovery, systemic finding) all remain active. The vision describes the Orchestrator as "Single Semantic Interpreter" — the role carrying the entire semantic burden. The sub-pipeline evolution dissolves the planning function but doesn't redistribute the semantic interpretation function. A Code Planner plans code; it doesn't rescope failed tasks, resolve blocked reviews, or convert discoveries to tasks. The Orchestrator's post-Phase-1 residual responsibilities (all non-planning duties) are now its entire purpose, and they are exactly the capabilities no other role can perform.

**Implication:** The Orchestrator's post-Phase-1 identity is undefined: it's no longer a planner (that's Code Planner) but still the only role capable of system-level judgment, with no spec for how its reduced scope interacts with its expanded responsibilities.

**Current mitigation:** Orchestrator role definition in roles.md still describes all capabilities. The tension is between roles.md (full capability set) and the sub-pipelines spec (minimal Orchestrator).

**Future options:**
- Spec the Orchestrator's post-Phase-1 identity explicitly: what it does, what it doesn't, and why
- Redistribute semantic interpretation capabilities to other roles where possible
- Accept the Orchestrator as the system-level coordination role (not a planner) and rename/respec accordingly

### Cross-Pair Knowledge Required by Single-Pair Reviewers

**Skill:** systemic-thinking
**Category:** TENSION
**Related:** [No Feedback Signal for Specification Quality](#no-feedback-signal-for-specification-quality), [No Reverse Data Channel in Inter-Pair Transitions](#no-reverse-data-channel-in-inter-pair-transitions)

**Issue:** Reviewers validate both the doer's artifact and its `output[]` decomposition into downstream tasks. Artifact review is within-domain ("is this plan sound?"). Decomposition review is cross-domain ("will these `output[]` entries make good inputs for the next pair?"). The Epic Plan Reviewer must understand what makes a good US Writer input. The Code Plan Reviewer must understand what makes a good Coder input. This cross-pair knowledge is embedded in a single-pair role. The reviewer is the only quality gate before `output[]` fans out into N downstream tasks via `liza proceed`. The sub-pipelines spec acknowledges this ("decomposition review is harder than artifact review") and defers it to reviewer skill prompts (Phase 2), meaning the quality gate's effectiveness depends entirely on prompt engineering, not structural enforcement.

**Implication:** The reviewer's decomposition judgment is the single most consequential quality gate in the pipeline — it controls fan-out scope and shape — yet it operates on cross-pair knowledge that the role's structural position doesn't naturally provide.

**Current mitigation:** Sub-pipelines spec §Known System Properties acknowledges "output[] decomposition quality" as a known trade-off. Phase 2 defers reviewer prompt templates with decomposition-specific checklists.

**Future options:**
- Downstream pair signals decomposition quality back to the reviewer pair (structural feedback, not just anomaly logging)
- Decomposition validation as a separate gate from artifact review (two verdicts per submission)
- Iteration cap calibration per role-pair to bound the cost of bad decomposition before detection

---

## Feedback Loops

Self-reinforcing patterns that can amplify failures.

### Supervisor Wait-Claim-Spawn Loop

**Skill:** systemic-thinking
**Category:** FEEDBACK

**Issue:** Supervisor's "wait → claim → spawn → restart" loop is tightly coupled with lease timing and work availability. Under slow tasks or transient failures, the loop can become self-reinforcing, cycling agents without progressing state.

**Implication:** System can be active but not advancing, with increasing log noise and human overhead.

**Future options:**
- Supervisor state machine with explicit "stalled" detection
- Alert on N cycles without state change
- Automatic pause after repeated no-progress cycles

### Contract Complexity vs Context Pressure

**Skill:** systemic-thinking
**Category:** FEEDBACK

**Issue:** The contract is the mechanism that suppresses agent failure modes. It competes for the same resource agents need to do work: context tokens. CORE.md is 800 lines. Add the mode contract (~200 lines), AGENT_TOOLS.md (94 lines), initialization reads (REPOSITORY.md, specs, lessons, collaboration continuity), skill files when loaded (100-300 lines each), the blackboard state, and the task's spec — a fresh session starts with 1500+ lines of governance before any work begins. The tier architecture and kernel appendix address degradation after it happens, but the fundamental dynamic is self-defeating: each new clause added to prevent a failure mode consumes context that makes other clauses harder to follow. The system's safety margin shrinks as its safety mechanisms grow.

**Implication:** The contract will hit a ceiling where adding another clause to prevent failure mode N+1 degrades compliance with clauses 1 through N, and no tier architecture can compensate because the contract must be loaded before tiers can be evaluated.

**Future options:**
- Contract compression (semantic deduplication, remove examples that models don't need)
- Conditional loading (only load clauses relevant to current role/task type)
- Structural enforcement replacing behavioral rules (move more rules into Go code, reducing contract size)
- Measure contract-to-work ratio across sessions to detect the ceiling empirically

### Issue Registry Resolution Drift

**Skill:** systemic-thinking
**Category:** FEEDBACK

**Issue:** The architectural issues registry is treated as the durable source of resolved-vs-open architectural risk, but its own resolution claims can diverge from live internal behavior. A prior example was the `submit-for-review` `commit_sha` item: the registry treated it as resolved while internal CLI/ops surfaces still required caller-provided SHA. That creates a reinforcing loop where planning and review work trusts the registry, then inherits stale assumptions, then perpetuates stale status.

**Implication:** Architectural debt tracking becomes self-invalidating: "resolved" no longer means the risk is absent in current runtime surfaces.

**Current mitigation:** Manual source verification during reviews can detect the mismatch, but only when a reviewer re-audits internals.

**Update:** The `submit-for-review` example was addressed on 2026-04-25 by allowing worktree-local commit refs (`HEAD`) and defaulting omitted refs to `HEAD`. The meta-risk remains: registry resolution claims need validation artifacts.

**Future options:**
- Add automated checks that verify each "resolved" entry against current code contracts
- Require a validation artifact (test/doc/assertion) link for every resolved architectural issue
- Add a `REGRESSED` status class to avoid binary resolved/unresolved drift

### Contract-Driven Safety vs Structural Enforcement Asymptote

**Skill:** systemic-thinking
**Category:** FEEDBACK
**Related:** [Contract Complexity vs Context Pressure](#contract-complexity-vs-context-pressure)

**Issue:** The architecture has two enforcement strategies operating simultaneously. The contract (CORE.md, ~800 lines) suppresses agent failure modes through behavioral rules loaded into context. The Go binary (`liza`) enforces structural invariants through code (supervisor-assigns-work, lease management, state machine validation, `ops` layer). ADR-0011 and ADR-0030 explicitly chose structural enforcement over behavioral compliance. Yet the system's most critical safety properties — Tier 0 invariants (no fabrication, no test corruption, no unapproved state change) — remain entirely behavioral. They cannot migrate to structural enforcement because they require semantic judgment. The structural enforcement strategy that works brilliantly for workflow mechanics (claiming, leasing, merging) is architecturally impossible for the safety properties that matter most. Each new structural enforcement (ADR-0030 guardrails, state validation extraction) removes a low-cost rule from the contract, leaving a higher concentration of the hard-to-enforce ones.

**Implication:** The contract will asymptotically approach a core of pure behavioral rules that cannot be structurally enforced, and that residue is precisely the set the system was designed to solve.

**Current mitigation:** The tier architecture (Tier 0 hard invariants vs Tier 2-3 best-effort) explicitly acknowledges that some rules are non-negotiable and others degrade gracefully — but doesn't address the dynamic where the non-negotiable set becomes the entire contract.

**Future options:**
- Accept the asymptote explicitly: document which rules are permanently behavioral and invest in detection rather than prevention (e.g., post-hoc audit of fabrication)
- Structural proxies for semantic properties (e.g., require validation command output in commit metadata to structurally enforce "no unvalidated success")
- Adversarial testing to measure behavioral rule compliance under context pressure empirically

---

## Assumptions

Implicit dependencies that constrain system behavior.

### Implicit Orchestrator Provenance Default

**Skill:** systemic-thinking
**Category:** ASSUMPTION

**Issue:** Task-creation provenance assumes a synthetic orchestrator identity when none is provided. Both MCP `handleAddTask` and `ops.AddTask` default missing agent identity to `orchestrator-1`, so write attribution can be generated without proving who initiated the mutation.

**Implication:** Multi-orchestrator operation collapses to a synthetic single actor in audit trails, reducing accountability and weakening post-incident reconstruction of planning decisions.

**Current mitigation:** CLI and MCP can provide explicit `agent_id`, but omission silently falls back to the default identity.

**Future options:**
- Make orchestrator identity mandatory for task-creation mutations
- Distinguish system-authored vs agent-authored mutations with explicit provenance fields
- Add validation rejecting task-creation events with defaulted identity in multi-orchestrator mode

### Spec Maturity Dependency

**Skill:** systemic-thinking
**Category:** ASSUMPTION

**Issue:** "Specs substantially complete before work" ties throughput to spec maturity and creates dependency on continuous human availability for spec evolution.

**Implication:** When specs incomplete or human constrained, throughput collapses rather than degrading gracefully.

**Addressed by:** BLOCKED resolution via `human_notes`.

### Well-Formed Blackboard State

**Skill:** systemic-thinking
**Category:** ASSUMPTION

**Issue:** CLI commands assume blackboard fields (current_task, review_lease_expires, integration_branch) are present and well-formed. Limited defensive handling for corrupted or partial state.

**Implication:** Single malformed entry can cascade into systemic stop conditions across all roles.

**Current mitigation:** `liza validate` checks invariants.

**Future options:**
- Schema validation on every state read
- Auto-repair for common corruption patterns
- Quarantine malformed entries rather than fail-stop

### Single-Goal Data Model Constrains Applicability

**Skill:** systemic-thinking
**Category:** ASSUMPTION

**Issue:** The blackboard schema has exactly one `goal` section, one `sprint` section, and a flat `tasks` array. This single-goal, single-sprint data model is documented as v1 scope (`specs/functional/1 - Liza.md`), but the structural implications are broader than the scoping language suggests. It prevents: concurrent goals (feature work alongside tech debt), multi-sprint planning (seeing the backlog beyond current sprint), hierarchical task relationships (epics containing stories), and project-level metrics (cross-sprint trends). The "every restart is a new mind with old artifacts" philosophy compounds this — there is no memory of previous sprints beyond what's manually archived, and no mechanism to learn from past sprint metrics because each sprint overwrites the metrics section.

**Implication:** Liza is structurally a single-feature-at-a-time system, and this constraint is embedded in the data model rather than documented as a design choice — teams discovering this limit will face a schema migration, not a configuration change.

**Future options:**
- Document as explicit v1 limitation in vision and deployment docs
- Sprint history array (append completed sprints rather than overwriting)
- Goal array with per-goal task filtering
- Backlog section separate from active sprint scope

### Orchestrator State Change Verification is Non-Binding

**Skill:** systemic-thinking
**Category:** ASSUMPTION

**Issue:** After orchestrator execution completes, `verifyOrchestratorStateChanges()` (`agent/systemctl.go`) logs a warning if expected state changes weren't made, but takes no corrective action. The supervisor continues the loop. However, the orchestrator re-invocation depends on `waitForWork` → `DetectOrchestratorWakeTriggers()`, which checks actual state conditions (unassigned tasks, anomalies, blocked tasks, etc.). If no wake triggers exist, the orchestrator waits indefinitely rather than looping. The infinite loop scenario requires persistent wake triggers that the orchestrator fails to resolve — e.g., a blocked task the orchestrator cannot unblock, or an anomaly it cannot interpret.

**Implication:** The system assumes orchestrators will eventually resolve wake triggers. An orchestrator stuck on an unresolvable trigger (spec ambiguity it cannot bypass, anomaly pattern it cannot interpret) will repeatedly execute without triggering escalation, consuming API tokens and time without progress signals. The failure mode is narrower than "any stuck orchestrator" but still lacks detection.

**Current mitigation:** None explicit. Circuit breaker anomaly patterns may eventually detect the orchestrator loop, but only if anomalies are logged.

**Future options:**
- Escalate to human after N consecutive orchestrator executions without state change
- Require orchestrator to document progress or blocking reason on each wake
- Add orchestrator-specific circuit breaker for no-op execution patterns

### `one-to-one` Transition Child Field Generation Unspecified

**Skill:** systemic-thinking
**Category:** ASSUMPTION

**Issue:** The `us-to-coding` pipeline transition uses `one-to-one` cardinality. The sub-pipelines spec states that the child task's `desc`, `done_when`, and `scope` describe "the next phase's work" and are "generated from the transition definition." But the transition definition in the YAML contains only `name`, `from`, `to`, `trigger`, and `cardinality` — no child field templates or generation rules. For `per-subtask` cardinality, the child fields come from `output[]` entries (concrete, validated mechanism). For `one-to-one`, the source of child task content is unspecified. Whether `liza proceed` uses a hard-coded template, an LLM call, or manual input is an open design question in the Phase 2 spec.

**Implication:** The cross-sub-pipeline data flow (`epic-spec-subpipeline` → `coding-subpipeline`) relies on a mechanism that has no concrete specification, making it the weakest link in the end-to-end pipeline data chain.

**Current mitigation:** Phase 2 is not yet implemented. The gap is in the design spec, not in running code.

**Future options:**
- Add a `child_template` section to one-to-one transition definitions with field generation rules
- Require `output[]` for all cardinalities (one-to-one simply requires exactly one entry)
- Have `liza proceed` prompt the human for child task fields on one-to-one transitions

---

## Stress Points

Bottlenecks that emerge under load.

### Filesystem/Git I/O Contention

**Skill:** systemic-thinking
**Category:** STRESS POINT

**Issue:** Worktree creation, review assignment, and merge operations funnel through filesystem and git in same repo. Primary shared resource for all roles. The flock-based locking protects `state.yaml` consistency but does NOT serialize git operations — two agents can concurrently run `git worktree add` and `git merge` against the same repo. In practice this works because worktree operations are scoped to different directories and merge operations are single-threaded per branch, but no formal exclusion mechanism prevents concurrent git state corruption.

**Implication:** I/O contention or git state anomalies become first systemic bottleneck as task volume increases. Concurrent merges to the integration branch are the highest-risk scenario.

**Current mitigation:** Supervisor serializes merge operations per-agent (only one merge per supervisor loop iteration). Different agents operate in separate worktrees.

**Future options:**
- Worktree pool pre-creation
- Git operations queuing (serialization mutex for integration branch merges)
- Separate integration repo for merges

### Cache Coherence Gap in Multi-Process Deployments

**Skill:** systemic-thinking
**Category:** STRESS POINT

**Issue:** The `Blackboard` provides `ReadCached()` for performance, using mtime comparison to detect file changes. However, the cache is process-local and keyed by mtime alone. In a multi-process deployment (multiple `liza agent` instances, CLI commands, and MCP server), one process's cache invalidation doesn't propagate to others. Two processes can hold different cached versions of state simultaneously because there's no cross-process cache coherence mechanism—only file locking for writes.

**Implication:** Under concurrent load or multi-terminal operation, agents may make decisions based on stale state without any error signal, potentially causing claim races or missed work detection that the file locking was designed to prevent.

**Current mitigation:** Cache invalidation happens on write, so single-process deployments are consistent. File locking ensures write serialization.

**Future options:**
- Remove caching in favor of always reading under lock (simplest, performance cost)
- Add cache versioning or generation counter in state.yaml
- Document that `ReadCached()` is unsafe for multi-process use

### Manual Sprint Transitions as Scaling Bottleneck

**Skill:** systemic-thinking
**Category:** STRESS POINT
**Related:** [Human Availability as Bottleneck](#human-availability-as-bottleneck)

**Issue:** Inter-pipeline transitions require human intervention at every boundary. With 4 role-pairs in series, a single feature passes through 4 human gates. The existing "Human Availability as Bottleneck" issue documents human as circuit breaker and escalation point, but the sub-pipeline architecture makes human intervention structural rather than exceptional: every inter-pair transition requires it. The compound effect on throughput is that the system's maximum velocity is capped by human response time multiplied by the number of pipeline stages, regardless of how many agents are deployed.

**Implication:** The system's throughput ceiling shifts from "agent capacity" to "human gate frequency × pipeline depth" — adding more agents doesn't help when the bottleneck is the mandatory human touch between every role-pair.

**Current mitigation:** The Planning Transition Gate reduces this from a two-step flow to a single checkpoint: the orchestrator checkpoints with `checkpoint_trigger: PLANNING_COMPLETE`, the human reviews and resumes, and child tasks are created automatically by the orchestrator's PreWork. This eliminates the separate `liza proceed` step for pipeline-configured transitions. Human review at sprint boundaries remains the trust mechanism.

**Future options:**
- Allow configurable auto-proceed for low-risk transitions (e.g., code-plan → coding when plan approval rate is high)
- Batch multiple pipeline transitions into single human review gate
- Async notification with timeout-based auto-proceed for non-critical pipelines

---

## Fragility

Partial failure modes with unclear recovery.

### Cross-Script State Mutation

**Skill:** systemic-thinking
**Category:** FRAGILITY

**Issue:** State mutation distributed across CLI commands (`liza claim-task`, `liza wt-merge`, `liza clear-stale-review-claims`) with shared transactional boundary via the Go binary's internal locking. Cross-command assumptions about state shape are type-checked at compile time.

**Implication:** Partial failure in any command can leave blackboard logically consistent but operationally stuck.

**Future options:**
- State machine validation after each operation
- Transaction log for rollback capability
- Centralized state mutation through single entry point

### Bootstrap Artifact Path Drift

**Skill:** systemic-thinking
**Category:** FRAGILITY

**Issue:** Initialization and navigation artifacts reference paths that no longer align: Pairing initialization requires `docs/USAGE.md` even though only split usage docs exist, and multiple docs still point to `specs/build/0 - Vision.md` while the current spec index canonizes `specs/build/1 - Vision.md`. The bootstrap/read path is therefore partially identity-drifted.

**Implication:** Session bootstrap and orientation become dependent on ad hoc path discovery, creating non-deterministic context loading across agents and sessions.

**Current mitigation:** Repository-level discovery (`REPOSITORY.md`, `specs/README.md`) allows humans/agents to recover missing paths manually.

**Future options:**
- Introduce stable alias files for canonical bootstrap paths
- Add link/path integrity checks in CI for contract and spec references
- Generate initialization read lists from a single manifest rather than hardcoded paths

### File-Based Spec References Without Version Anchors

**Skill:** systemic-thinking
**Category:** FRAGILITY

**Issue:** The `spec_ref` field in tasks and goal uses file paths (e.g., `specs/retry-logic.md`, optionally with `#section` anchors). The anchors refer to headings within the file, not to versions of the file. Git tracks file history, but `spec_ref` contains no commit SHA, no version identifier, and no content hash. When a task cites `specs/api.md#pagination`, it references whatever content currently exists at that heading.

**Implication:** Spec drift during task execution is undetectable. A PRD produced by a spec-authoring agent and consumed by the Orchestrator can change between when the Orchestrator decomposes it and when the Coder implements the resulting tasks. The blackboard's `spec_changes` log tracks that changes occurred, not which tasks were affected by which changes.

**Current mitigation:** Code Reviewer validates against "current spec version" and logs `spec_changed` anomaly if material changes detected.

**Future options:**
- Include commit SHA or content hash in `spec_ref`
- Track `spec_version` at task creation and warn on divergence
- Generate spec snapshots when tasks are created

### Review Lease Orphaning Without Automatic Reclamation

**Skill:** systemic-thinking
**Category:** FRAGILITY

**Issue:** Review leases expire based on `review_lease_expires` timestamp. Stale leases need periodic clearing to prevent tasks from getting stuck in REVIEWING with expired leases, particularly after crash scenarios where the agent cannot execute graceful cleanup.

**Implication:** After a crash (no graceful shutdown), tasks can remain stuck in REVIEWING until the next reviewer wait-loop iteration clears stale claims.

**Current mitigation:** Three clearing mechanisms now exist: (a) reviewer registration (`registration.go`) auto-clears stale claims on agent startup, (b) `liza clear-stale-review-claims` command for manual invocation, and (c) reviewer wait-loop (`waitforwork.go`) calls `ops.ClearStaleReviewClaims` before each poll iteration for all reviewer roles (code-reviewer, code-plan-reviewer, epic-plan-reviewer, us-reviewer). Signal handling (`SIGINT`/`SIGTERM`) triggers `unregisterAgent()` which atomically releases active review claims on graceful exit. The remaining gap is narrow: crash scenarios where no reviewer agent is running to execute the wait-loop clearing.

**Future options:**
- Add watcher-based automatic lease expiration (transition to READY_FOR_REVIEW on expiry)
- Include stale lease check in work detection diagnostics

### State Validation Composition Gap

**Skill:** software-architecture-review
**Category:** FRAGILITY

**Issue:** The `statevalidate` package has 55.1% statement coverage — the lowest of any functional package. All entry-point validators are at 0%: `ValidateStateFile`, `ValidateAgentInvariants`, `ValidateAnomalies`, `validateRequiredFields`, `validateAgentInvariants`, `validateHandoff`, `validateDiscovered`, `validateAnomalies`, `checkSpecFileExists`. Inner validators that these compose (`validateTaskStates` 92.9%, `validateDependencies` 91.3%, `checkCircular` 81.8%) are well-covered. This creates a "tested parts, untested whole" pattern in the data-integrity package.

**Implication:** The composition logic — which validators run, in what order, how errors aggregate — is untested. A bug in `ValidateStateFile` (the top-level entry point invoked by `liza validate` CLI and MCP `liza_validate` tool) could skip validators entirely without detection. Since `liza validate` is the primary mechanism for detecting blackboard corruption, this gap affects the system's self-healing capability.

**Direction:** Table-driven tests calling `ValidateStateFile` with various malformed states (missing required fields, invalid agent invariants, malformed anomalies, broken handoff entries, missing spec files) would cover the composition layer cheaply. The inner validators' existing coverage means only the wiring needs testing.

### SetTaskOutput spec_ref Validation Gap

**Skill:** software-architecture-review
**Category:** FRAGILITY
**Related:** [`one-to-one` Transition Child Field Generation Unspecified](#one-to-one-transition-child-field-generation-unspecified)

**Issue:** `ops/set_task_output.go` validates `desc`, `done_when`, and `scope` as non-empty but does NOT validate `spec_ref`. By contrast, `ops/proceed.go:validateOutputEntry` (line 405-418) requires all four fields including `spec_ref`. An agent can successfully set output entries with empty `spec_ref` via `liza_set_task_output`, then `liza proceed` fails at transition time with "output[N] missing spec_ref".

This is a different code path from the `extractOutputEntries` MCP handler validation gap — that's the MCP extraction layer accepting empty fields; this is the ops-layer validation allowing empty `spec_ref` through.

**Implication:** The failure is delayed — by the time the human runs `liza proceed`, the agent's work is already merged and the sprint is checkpointed. Fixing requires manual state editing or re-running the planning pair. The validation asymmetry between set and consume operations creates a "write now, fail later" pattern.

**Direction:** Add `spec_ref` validation to `SetTaskOutput`, consistent with `validateOutputEntry`. Alternatively, if `spec_ref` should be optional for some transition types, make `validateOutputEntry` configurable per transition.

### INVARIANTS.md §7 Clean Worktree Not Enforced at Submission

**Skill:** software-architecture-review
**Category:** FRAGILITY
**Related:** [Contract-Driven Safety vs Structural Enforcement Asymptote](#contract-driven-safety-vs-structural-enforcement-asymptote)

**Issue:** INVARIANTS.md §7 states: "Clean sync: before READY_FOR_REVIEW, working tree must be clean (no staged, unstaged, or untracked files)." The `submit_review.go` submission path validates commit SHA match, TDD enforcement, pre-execution checkpoint, rebase onto integration, and re-validates status/assignment under lock — but performs no `git status` or working tree cleanliness check. The invariant is documented but not enforced.

**Implication:** A coder can submit work with uncommitted changes in the worktree. The reviewer sees only the committed code (via `review_commit`), but the coder may have relied on uncommitted files during local testing. If the coder's tests passed partly due to uncommitted state, the reviewer approves code that may fail on integration. This is a concrete instance of the structural enforcement asymptote — the invariant document promises a protection the code doesn't deliver.

**Direction:** Add a clean worktree check (`git status --porcelain` in the worktree) to `submit_review.go` before the rebase step. This matches the invariant's intent and catches a real failure mode. Alternatively, if clean worktree enforcement is intentionally deferred (e.g., to allow configuration files), narrow the invariant in INVARIANTS.md and the worktree-management spec.

### INVARIANTS.md §6 Provider Diversity Not Enforced at Verdict Time

**Skill:** software-architecture-review
**Category:** FRAGILITY
**Related:** [Contract-Driven Safety vs Structural Enforcement Asymptote](#contract-driven-safety-vs-structural-enforcement-asymptote)

**Issue:** INVARIANTS.md §6 states: "Quorum enforcement: approval count tracked, provider diversity required (≥2 distinct providers for multi-reviewer quorum)" and attributes enforcement to `submit_verdict.go`. The code has the building blocks (`task.HasProviderDiversity()` in task.go:278, `resolver.ProviderDiversity()` in resolver.go:287) but `submit_verdict.go` only checks `ApprovalCount() < effectiveQuorum` (line 237) — it never evaluates provider diversity.

Provider diversity is actually enforced at two different points:
1. Reviewer claim-filtering (`claim_reviewer_task.go:filterDoerProviderDiversity`) — prevents a reviewer with the same provider as the doer from claiming
2. Merge-readiness evaluation (`agent/claiming.go:151`) — soft "preferred" enforcement that checks `HasProviderDiversity()` before proceeding with merge

Neither matches the invariant's description of hard enforcement during verdict submission.

**Implication:** If two reviewers from the same provider approve a task, the quorum count is met and the task transitions to APPROVED. The diversity check at merge-readiness uses "preferred" semantics (can be overridden if no diverse reviewers are available). The invariant document implies stronger enforcement than exists. This creates a false confidence surface for security-sensitive review workflows where provider diversity is a compliance requirement.

**Direction:** Either (a) add diversity evaluation to `submit_verdict.go` — when `ProviderDiversity` is configured and quorum > 1, verify `HasProviderDiversity()` before transitioning to APPROVED, or (b) update INVARIANTS.md §6 to accurately describe the current enforcement model: diversity is ensured at claim-time (doer-reviewer) and preferred at merge-time (reviewer-reviewer), not enforced at verdict time.

---

## Blind Spots

Unacknowledged forces or gaps the system doesn't model.

### Contract Effectiveness Self-Certification

**Skill:** systemic-thinking
**Category:** BLIND SPOT

**Issue:** The contract's failure mode coverage is self-certified. The failure mode map (`CONTRACT_FAILURE_MODE_MAP.md`) claims 55/55 "Strong" coverage with 0 Partial and 0 Gap. This assessment is produced by the same process that writes the contract — there is no independent validation that clauses actually suppress the failure modes they claim to cover. The map references line numbers from a prior contract version ("Last updated: Contract v3 (882 lines)") while the current contract is 800 lines — every line reference is stale. The maintenance protocol ("check which failure modes the affected clause covers") is a process rule enforced by the same behavioral compliance the contract is designed to compensate for. There is no test suite, no simulation, no adversarial probing of whether the 55 coverage claims hold under context pressure, novel model versions, or multi-agent interaction.

**Implication:** The 55/55 coverage claim provides confidence without evidence — the map may be accurate, or it may be a snapshot of aspirational intent that has drifted from reality as the contract evolved.

**Future options:**
- Adversarial testing: deliberately trigger each failure mode and verify the contract suppresses it
- Automated line-number maintenance (extract clause IDs instead of line numbers)
- Periodic red-team exercises using the failure mode map as a checklist

### Initialization Completion Unverifiable

**Skill:** systemic-thinking
**Category:** BLIND SPOT

**Issue:** The contract requires a complex initialization sequence: mode detection → read mode contract → read project files → build 6 mental models → role-specific initialization. Completion of this sequence is entirely self-reported. There is no structural verification that an agent actually read what it was supposed to read, built the models it was supposed to build, or internalized the constraints. In multi-agent mode, the supervisor verifies agent registration (identity, lease) but not contract compliance. An agent that skips initialization or partially completes it enters the same state machine as a fully initialized agent. The "compaction checkpoint" and "working set" mechanisms handle mid-session degradation but assume initialization was complete — if it wasn't, the agent starts in a degraded state without any detection signal.

**Implication:** Contract compliance depends on a bootstrap sequence that cannot be verified from outside the agent — a model that partially follows initialization instructions produces no observable difference from one that follows them completely, until a violation occurs.

**Future options:**
- Initialization checklist emitted as structured output (supervisor verifies before accepting agent as ready)
- Canary questions: supervisor tests agent's knowledge of key contract clauses before allowing work
- Reduce initialization surface by embedding more rules in supervisor-enforced structural mechanisms

### No Source Type for Pre-Implementation Spec Findings

**Skill:** systemic-thinking
**Category:** BLIND SPOT

**Issue:** The `discovered` section in `state.yaml` tracks findings logged by agents during work. The `source` field documents who/what produced the finding: `null` (implementation discovery by Coder) or `systemic-thinking` (analytical finding by Code Reviewer). There is no source value for findings produced during specification authoring—ambiguous requirements, conflicting constraints in vision docs, or SMART criteria violations identified before implementation begins. Note: the source taxonomy is documented but not enforced at runtime — `models/state.go` and `validate.go` do not reject arbitrary source values, so a spec-authoring agent could technically use any string. The gap is in the documented taxonomy and semantic clarity, not structural enforcement.

**Implication:** Specification-quality issues discovered by a spec-authoring agent would need to use `null` (misleading—implies Coder found it) or `systemic-thinking` (misleading—implies analytical review of existing code), or use an undocumented ad hoc value that other agents won't recognize. The discovery taxonomy cannot represent "this finding blocks the PRD, not the implementation."

**Current mitigation:** Spec-quality issues are assumed to be resolved by humans before the blackboard is initialized.

**Future options:**
- Add `spec-authoring` or `prd-validation` as valid `source` values
- Add `urgency: blocks_spec` to distinguish spec-blockers from implementation-blockers
- Track spec-production work separately from implementation tasks

### Prompt-Build-to-Execution State Drift

**Skill:** systemic-thinking
**Category:** BLIND SPOT

**Issue:** The supervisor builds and saves the prompt file (`agent/supervisor.go:250-259`) before executing the agent. The prompt is constructed from state read at claim time, but the agent execution happens in a separate subprocess that may read different state. There's no mechanism to ensure the prompt content remains consistent with the state the agent actually operates on.

**Implication:** When debugging failures, the saved prompt may not represent the actual state the agent operated on, making post-hoc analysis less reliable. However, since agents read live state via MCP tools during execution (not from the prompt file), the prompt is an initial context artifact, not the runtime truth — the actual impact on agent behavior is low.

**Current mitigation:** Agents read current state via MCP/tools during execution, not from the prompt file. The prompt provides initial context and orientation but is not authoritative for state-dependent decisions. Prompts are timestamped and saved for debugging.

**Future options:**
- Include state version/checksum in prompt header for comparison
- Snapshot state.yaml at prompt build time alongside the prompt
- Add prompt-state consistency verification to post-execution diagnostics

### No Feedback Signal for Specification Quality

**Skill:** systemic-thinking
**Category:** BLIND SPOT
**Related:** [Spec Completeness vs Reality](#spec-completeness-vs-reality), [No Source Type for Pre-Implementation Spec Findings](#no-source-type-for-pre-implementation-spec-findings)

**Issue:** The system has rich feedback loops for code quality: Code Reviewer rejects code, rejection metrics are tracked, rubber-stamping is detected, iteration limits catch coder loops, circuit breaker catches systemic patterns. Phase 2 adds spec-writing roles (Epic Planner, US Writer) with their own reviewers. But the feedback loop for specification quality — the signal that a spec is good enough to produce correct code — doesn't exist. The spec-to-code handoff is a one-way gate (`liza proceed`), and the only signal that a spec was bad is: coders fail. There is no metric, anomaly type, or circuit breaker pattern that measures "how often do approved specs produce blocked tasks?" The `spec_gap` anomaly type exists but is logged by the Orchestrator when discovering gaps during planning — not by the Coder when discovering an approved spec was insufficient. There's no path from "Coder blocks because spec was wrong" back to "Spec Reviewer approved a bad spec."

**Implication:** As the system adds spec-writing pipelines, the spec reviewer's accuracy is the least measurable and most consequential quality dimension — a bad spec amplifies through every downstream role, and the system has no signal to detect this before coders burn cycles.

**Current mitigation:** The `spec_changed` anomaly type lets Code Reviewer note spec drift, but this tracks change, not quality. Human checkpoint review provides periodic correction.

**Future options:**
- Track spec provenance on tasks: which spec version was approved, which role approved it, and whether downstream tasks succeeded or blocked
- Add `spec_quality_gap` anomaly type for Coders to log "approved spec was insufficient for implementation"
- Compute spec reviewer accuracy metric: ratio of downstream task success to spec approvals
- Circuit breaker pattern for repeated spec-caused blocks across tasks from the same approved spec

### No Reverse Data Channel in Inter-Pair Transitions

**Skill:** systemic-thinking
**Category:** BLIND SPOT
**Related:** [No Feedback Signal for Specification Quality](#no-feedback-signal-for-specification-quality), [Cross-Pair Knowledge Required by Single-Pair Reviewers](#cross-pair-knowledge-required-by-single-pair-reviewers)

**Issue:** The `output[]` mechanism is the sole structured data path between role pairs. Data flows strictly forward: doer writes `output[]`, reviewer validates, `liza proceed` stamps child tasks. When a downstream pair discovers the decomposition was wrong — scope too broad, `done_when` untestable, dependencies miscounted — the only recourse is BLOCKED → orchestrator SUPERSEDE → rescope. No structured signal reaches the upstream pair that created the decomposition. The upstream doer and reviewer receive no data about how their `output[]` performed. Discoveries and anomalies live in state.yaml, but they're addressed by the orchestrator, not routed to the pair that originated the flawed output. The `spec_quality_gap` anomaly type (proposed under "No Feedback Signal") would provide some signal, but even that routes to the orchestrator, not the upstream pair.

**Implication:** Decomposition quality is a learned skill that cannot improve within the system because the feedback loop is severed at the pair boundary — the entity that could learn (the upstream pair) never sees the signal.

**Current mitigation:** Human checkpoint review provides periodic observation of decomposition quality. Retrospective protocol identifies patterns but actions are human-directed, not agent-routed.

**Future options:**
- Add structured reverse-channel entries on child tasks (e.g., `parent_feedback: {quality: good|poor, reason: ...}`) populated by downstream doer on completion or blocking
- Route `spec_quality_gap` anomalies to the upstream pair's reviewer, not just the orchestrator
- Compute upstream pair accuracy metric: ratio of downstream child task success to upstream `output[]` entries approved

### Retrospective Findings Don't Feed Forward to Next Sprint

**Skill:** systemic-thinking
**Category:** BLIND SPOT
**Related:** [No Feedback Signal for Specification Quality](#no-feedback-signal-for-specification-quality)

**Issue:** Sprint retrospective data — identified patterns, spec gaps, root causes, corrective actions — is archived with the completed sprint to `.liza/archive/sprint-N.yaml`. Agents in sprint N+1 read `state.yaml`, which contains only lightweight `sprint_history[]` summaries (id, number, status, dates, tasks_done). The detailed retrospective that should inform the next sprint's task creation, scope calibration, and risk awareness is structurally inaccessible to agents unless they explicitly read archive files — which no initialization protocol requires. The human is expected to translate retrospective actions into spec updates, but the *patterns* (iteration count distributions, anomaly frequencies, decomposition quality signals) are lost to agent context.

**Implication:** Each sprint starts with the same naive optimism as the first — agents cannot calibrate based on prior sprint experience because the retrospective data exits the agent-readable surface at archival time.

**Current mitigation:** Human translates retrospective actions into spec updates or task adjustments manually. `spec_changes` in state.yaml provides some continuity, but this tracks changes, not the reasoning behind them.

**Future options:**
- Include a `lessons` summary field in `sprint_history[]` entries (top 3 patterns from retrospective, machine-readable)
- Add agent initialization step to read the most recent archived sprint's retrospective
- Carry forward unresolved retrospective actions as structured items in state.yaml rather than archiving them with the sprint

### Sprint Metrics Lossy at Sprint Boundary

**Skill:** software-architecture-review
**Category:** BLIND SPOT
**Related:** [Retrospective Findings Don't Feed Forward to Next Sprint](#retrospective-findings-dont-feed-forward-to-next-sprint), [Metrics Collection Without Query Interface](#metrics-collection-without-query-interface)

**Issue:** `applySprintAdvance` (advance_sprint.go:100-107) reduces the full 12-field `SprintMetrics` struct to a single `TasksDone` count in `SprintSummary`. Fields lost from active state: `TasksTotal`, `TasksAbandoned`, `AvgReviewIterations`, `MaxReviewIterations`, `ReviewApprovalRate`, `TasksBlockedCount`, `AnomaliesLogged`, `AvgTaskDurationMinutes`, `ContextExhaustionCount`, `IntegrationFailures`, `SupersededCount`. These are preserved only in the archive YAML on disk (`.liza/archive/sprint-N.yaml`).

`BuildOrchestratorContext` (builder.go:127) passes `state.SprintHistory` to orchestrator templates, but those summaries carry only `TasksDone`. The orchestrator's planning for sprint N+1 cannot reference sprint N's review iteration rates, block frequency, or anomaly counts without reading archive files — which no prompt template instructs.

This is distinct from "Retrospective Findings Don't Feed Forward" (which covers qualitative retrospective findings) and "Metrics Collection Without Query Interface" (which covers the absence of a unified query layer). This issue is specifically about the quantitative metrics data being structurally discarded at the sprint boundary.

**Implication:** The system collects detailed sprint performance metrics during execution, then discards all but one at the sprint boundary. Scope calibration decisions (how many tasks to plan, iteration cap tuning, risk assessment) lack quantitative foundation from prior sprints.

**Current mitigation:** Full sprint data is archived to disk. Humans can inspect archives manually. The `SprintSummary` struct was intentionally kept lightweight per ADR-0028.

**Future options:**
- Extend `SprintSummary` with a small set of decision-relevant metrics (e.g., `AvgReviewIterations`, `ReviewApprovalRate`, `TasksBlockedCount`)
- Add a `MetricsSummary` sub-struct to `SprintSummary` carrying the top 4-5 planning-relevant fields
- Have orchestrator templates reference the previous sprint's archive file (deterministic path: `.liza/archive/sprint-N.yaml`)

---

## Trajectory

Long-term concerns about system evolution.

### Blackboard Growth Without Pruning

**Skill:** systemic-thinking
**Category:** TRAJECTORY

**Issue:** System optimizes for accountability via append-only logs, explicit states, and anomaly logging. No clear pruning or partition strategy in v1.

**Implication:** As task volume grows, coordination cost and cognitive load rise nonlinearly. System becomes harder to operate without additional tooling.

**Future options:**
- Archive completed sprints to separate files
- Prune history older than N days
- Split blackboard by concern (tasks, agents, anomalies)

### Role Addition Accelerates Contract Complexity Pressure

**Skill:** systemic-thinking
**Category:** TRAJECTORY
**Related:** [Contract Complexity vs Context Pressure](#contract-complexity-vs-context-pressure)

**Issue:** The "Contract Complexity vs Context Pressure" feedback loop (documented in this file) notes that CORE.md is 800 lines and each new clause consumes context agents need for work. Adding a fourth role requires: role definition in `roles.md`, SKILL.md for the role, task type workflow extension, and initialization sequence updates. The contract is the mechanism that suppresses agent failure modes, but the safety margin shrinks with each role added.

**Implication:** Each new role added to the system brings the contract closer to the ceiling where "adding another clause to prevent failure mode N+1 degrades compliance with clauses 1 through N." The tier architecture and kernel appendix handle mid-session degradation, but initialization grows monotonically.

**Current mitigation:** None structural; the existing feedback loop documents the concern but offers no relief mechanism.

**Future options:**
- Conditional contract loading (only load role-relevant sections)
- Structural enforcement replacing behavioral rules (more logic in Go, less in contract)
- Measure contract-to-work ratio empirically before adding roles

### Anomaly Detail Validation Incomplete

**Skill:** code-review
**Category:** FRAGILITY

**Issue:** `liza validate` enforces required detail fields for 5 of 15 anomaly types (`retry_loop`, `trade_off`, `external_blocker`, `assumption_violated`, `system_ambiguity`). The remaining 10 types — including `reviewer_loop` (requires `count`, `command_pattern`) and `review_exhaustion` (requires `reviewers_failed`, `common_blocker`) — pass validation with empty details. The spec (`blackboard-schema.md:770`) and prompt templates (`shared_reference.tmpl`) both declare required fields for all types.

**Implication:** Agents can write structurally valid but informationally empty anomalies. Circuit breaker pattern detection and retrospective analysis degrade when detail fields are missing.

**Future options:**
- Add cases for all 10 missing types in `validateAnomalies()` (`internal/commands/validate.go:360`)
- Generate validation from a single type→fields declaration (eliminate spec/code/template as three separate lists)

### Task Type Registry is Partial Abstraction

**Skill:** code-review
**Category:** TRAJECTORY

**Issue:** The task type workflow registry (`taskWorkflows` in `internal/models/state.go`) maps `TaskType` → ordered role sequence, but the mapping of role → claimable statuses is hardcoded in `IsClaimable`'s switch statement. The registry captures *which* roles participate but not *how* they participate (i.e., which statuses each role can claim from). Adding a new role requires modifying the switch, which undermines the "new types add rows to the registry" premise.

**Implication:** When a second task type arrives (e.g., `specification` with a `spec_reviewer` role), the claimable-status mapping will need resolving — either by extending the registry to include status rules, or by accepting the switch as the canonical location for claiming semantics.

**Current mitigation:** TODO comment on the switch in `IsClaimable`. Only one task type exists today, so there's no split in practice.

**Future options:**
- Extend registry to map `(TaskType, role)` → `[]TaskStatus` (claimable statuses)
- Keep the switch but validate it against registry entries at init time
- Accept the split as intentional separation of concerns (registry = participation, switch = claiming rules)

### Spec Corpus Lacks Lifecycle Management

**Skill:** systemic-thinking
**Category:** TRAJECTORY

**Issue:** The spec-first design requires specifications before implementation, blocks work on missing specs, and logs spec changes to the blackboard. But the specification corpus has no lifecycle management. Specs are created, updated, and appended to — never deprecated, archived, or retired. `spec-mapping.yaml` is already 59KB for a single project (Liza itself). Over multiple sprints and goals, the spec corpus grows monotonically. Agents must read relevant specs on session initialization; as specs accumulate, "relevant" becomes harder to determine and the read cost grows. There is no mechanism to mark a spec as superseded by implementation, no staleness detection for specs whose referent code has changed, and no pruning trigger in the sprint governance protocol.

**Implication:** For long-lived projects, the spec corpus becomes a maintenance burden that consumes human bandwidth proportional to project age — the opposite of the "reduce human workload" goal.

**Future options:**
- Spec status field (active, superseded, archived) with archival workflow
- Staleness detection: flag specs not referenced by any task in N sprints
- Spec pruning as part of sprint retrospective checklist
- Hierarchical spec organization with summary documents to reduce agent read cost

### Metrics Collection Without Query Interface

**Skill:** systemic-thinking
**Category:** TRAJECTORY

**Issue:** The system collects rich metrics—file lock timing (`filelock/metrics.go`), sprint metrics (`models/state.go:406-420`), diagnostic data (`models/diagnostics.go`)—but there's no unified query layer to access them. The MCP server exposes individual tools for specific queries, but operators cannot ask "show me agent performance over time" or "what's the current lock contention rate?" without writing custom code.

**Implication:** Operational visibility requires ad hoc tooling or direct state.yaml inspection. The investment in metrics instrumentation doesn't translate to operational insight because the data is fragmented and inaccessible through standard interfaces.

**Current mitigation:** Individual metrics are accessible via specific commands (`liza inspect metrics`, MCP tools). Sprint metrics visible in `liza status`.

**Future options:**
- Unified query interface aggregating all metric sources
- Time-series storage for historical metric analysis
- Dashboard generation from collected metrics

### No Query Layer

**Skill:** systemic-thinking
**Category:** TRAJECTORY
**Coupled with:** [MCP Cross-Layer Read Dependency](#mcp-cross-layer-read-dependency)

**Issue:** The system has a clear mutation layer (`ops`) but no query layer. Complex read operations are implemented wherever first needed: `models` has `FindTask`, `IsClaimable`, `CountClaimableTasks`, `AllPlannedTasksTerminal`; `agent/workdetection.go` has `DetectOrchestratorWakeTriggers`; `commands` has `InspectCommand` (parametric query with format control), `StatusCommand` (dashboard aggregation), `ValidateCommand` (invariant checking); `db` has `Read`, `ReadCached`, `ReadRaw`, `GetTask`, `GetAgent`. The pattern: each package implements the queries it needs, and cross-package query reuse happens through the wrong seam (`mcp` → `commands`). The `models/diagnostics.go` file represents a partial move toward a query layer (work detection functions extracted from agent), but the extraction stopped there. The three consumers (CLI, MCP, agent supervisor) each need overlapping but different views of state — formatted text, structured JSON, and in-memory assessment — but share no query infrastructure.

**Implication:** As the system's query surface grows (new MCP resources, dashboard enhancements, diagnostic tools), either the `mcp` → `commands` dependency deepens or query logic gets duplicated across consumers.

**Future options:**
- Extract query functions to `ops` or a new `queries` package returning structured data (each presentation layer formats independently)
- Promote `models/diagnostics.go` as the canonical query home and migrate state queries from `commands` and `agent`
- Accept `commands` as the shared query+formatting layer and document or rename to reflect its dual role

---

## Cascades

Failure propagation and compound interaction effects.

### Sub-Pipeline Expansion Multiplies Every Existing Issue

**Skill:** systemic-thinking
**Category:** CASCADE
**Related:** [Contract Complexity vs Context Pressure](#contract-complexity-vs-context-pressure), [Role Addition Accelerates Contract Complexity Pressure](#role-addition-accelerates-contract-complexity-pressure), [Supervisor Contention](#supervisor-contention), [Cache Coherence Gap in Multi-Process Deployments](#cache-coherence-gap-in-multi-process-deployments), [Filesystem/Git I/O Contention](#filesystemgit-io-contention)

**Issue:** The sub-pipelines spec (Phase 2: Epic Planner → Epic Plan Reviewer → US Writer → US Reviewer → Code Planner → Code Plan Reviewer → Coder → Code Reviewer) adds 4 new roles to the current 4. Every issue in this registry that scales with role count compounds: "Contract Complexity vs Context Pressure" (8 roles × role-specific contract sections), "Role Addition Accelerates Contract Complexity Pressure" (acknowledged trajectory), "waitForXWork" refactored to generic callback pattern (no longer scales with role count), "Supervisor Contention" (8 concurrent supervisors vs current 3-4), "Cache Coherence Gap" (more processes, more stale reads), "Filesystem/Git I/O Contention" (more worktrees, more concurrent git operations). The architecture review (pass 13) notes production code grew 11% faster than tests during pipeline expansion. The compound effect is unmeasured: the 45+ open architectural issues are documented as independent concerns, yet under 8-role deployment they interact — supervisor contention amplifies lease churn, which amplifies restart frequency, which amplifies cache coherence gaps.

**Implication:** Phase 2 deployment activates interaction effects between issues that were individually tolerable at 3-role scale; the risk profile is combinatorial, not additive.

**Current mitigation:** Phase 1 (code-planning pair) provides empirical data at 5-role scale before Phase 2 doubles it. Issues are individually documented with mitigations.

**Future options:**
- Model interaction effects between high-priority issues before Phase 2 deployment
- Establish performance baselines at current scale to detect degradation during expansion
- Prioritize structural fixes for issues that compound (supervisor contention, cache coherence) before adding roles
- Consider phased Phase 2 deployment (add one role-pair at a time, measure impact)

### Fan-Out Amplifies Decomposition Errors Across Pipeline Stages

**Skill:** systemic-thinking
**Category:** CASCADE
**Related:** [Sub-Pipeline Expansion Multiplies Every Existing Issue](#sub-pipeline-expansion-multiplies-every-existing-issue), [Cross-Pair Knowledge Required by Single-Pair Reviewers](#cross-pair-knowledge-required-by-single-pair-reviewers), [No Reverse Data Channel in Inter-Pair Transitions](#no-reverse-data-channel-in-inter-pair-transitions)

**Issue:** Each `per-subtask` transition multiplies the task count. In the full pipeline, 1 goal produces N epics, each epic produces M user stories, each US produces P code plans, each plan produces Q coding tasks. A decomposition error at stage K propagates as a multiplicative factor across all downstream stages. Sprint serialization means each stage runs to completion before the next begins — the first signal that an epic was wrongly framed arrives from the coding sprint, 3 sprint boundaries later, having generated potentially dozens of mis-scoped tasks that each consumed agent time. The human reviews at each sprint boundary, but reviews a *completed* sprint — the error is visible only in retrospect, after the fan-out has occurred. Unlike "Sub-Pipeline Expansion Multiplies Every Existing Issue" (which concerns *system-level* issues scaling with role count), this concerns *domain-level* errors (wrong decomposition) amplifying through the fan-out topology of per-subtask transitions.

**Implication:** The cost of decomposition errors grows multiplicatively with pipeline depth while detection remains constant (single reviewer at each stage), creating a structural risk gradient that steepens as the pipeline lengthens.

**Current mitigation:** Sprint boundary human review provides a checkpoint between stages. Epic Plan Reviewer checklist (Phase 2 spec) includes decomposition quality gates. Sub-pipelines spec §Known System Properties acknowledges "output[] decomposition quality" as harder than artifact review.

**Future options:**
- Early sampling: downstream pair processes one `output[]` entry as a pilot before the full fan-out is committed
- Decomposition cost estimator: flag `output[]` entries that would produce >N downstream tasks for human review before `liza proceed`
- Cross-sprint error attribution: when downstream tasks block, trace back to the upstream `output[]` entry and accumulate error counts per upstream pair

---

## Accepted v1 Limitations

### Self-Reported Validation

**Skill:** systemic-thinking

**Issue:** Coder runs validation and reports result. Code Reviewer trusts claim without re-execution.

**Why accept:** Re-execution requires Code Reviewer to run in different worktree, understand commands, handle environment differences.

**Mitigation:** Code Reviewer can request re-run if suspicious.

### Kill Switch Granularity

**Skill:** systemic-thinking

**Issue:** Kill switches (PAUSE/ABORT) affect all agents. Can't surgically stop one misbehaving agent.

**Why accept:** Per-task kills add complexity. Rare failure mode.

**Future option:** Per-task pause via `liza pause --task task-{id}`.

### Orchestrator as Single Semantic Interpreter

**Skill:** systemic-thinking
**Category:** LOAD-BEARING

**Issue:** Orchestrator carries the entire semantic burden. It decomposes goals, interprets failure signals, resolves blocked reviews, converts discoveries to tasks, and maintains goal alignment. All other roles execute mechanical functions (implement spec, validate against spec) while Orchestrator alone interprets intent. No second opinion, no validation mechanism, no structural redundancy.

**Implication:** Orchestrator drift compounds silently across all tasks until human checkpoint reveals accumulated misalignment. Correction costs scale with drift duration.

**Current mitigation:** Human checkpoints provide periodic correction opportunities.

**Future options:**
- Orchestrator self-review before finalizing task decomposition
- Second Orchestrator instance for cross-validation on critical decisions
- Automated coherence checks against vision.md

### Supervisor as Single Correctness Gate

**Skill:** systemic-thinking
**Category:** LOAD-BEARING

**Issue:** System depends on supervisors (`liza agent`) performing correct pre-claim/assignment for all roles. This single gate defines whether tasks can proceed and whether agents stay within protocol. Correctness is concentrated in a single control loop that is neither redundant nor independently validated.

**Implication:** A supervisor bug, crash loop, or misconfiguration stalls the entire system and blocks recovery without manual intervention.

**Current mitigation:** Supervisor is implemented in the `liza` Go binary with type-safe error handling. `liza validate` catches invalid states.

**Future options:**
- Supervisor health check endpoint
- Redundant supervisor with leader election
- Agent self-validation of claim state on startup

### Spec Completeness vs Reality

**Skill:** systemic-thinking
**Category:** TENSION

**Issue:** The vision positions specs as the mechanism for context survival ("if it's not written down, it doesn't exist") while stating "Liza v1 assumes specs substantially complete before work" and excluding "domains where requirements emerge through implementation."

Incomplete specs—normal in real projects—trigger a reinforcing loop: coders block on spec gaps, Orchestrator logs spec_gap anomalies, human must update specs, system pauses. The spec-first design shifts work from agents to humans while promising to reduce human workload.

**Implication:** System selects for a narrow project profile (complete specs, solo developers) rather than adapting to common project conditions.

**Current mitigation:** BLOCKED resolution via `human_notes`. Orchestrator reads human_notes on wake.

**Future options:**
- Spike mode for spec discovery
- Orchestrator-assisted spec drafting from coder discoveries
- Graceful degradation when specs incomplete (proceed with explicit assumptions)

### Code Reviewer Structural Accountability Gap

**Skill:** systemic-thinking
**Category:** TENSION

**Issue:** The Code Reviewer has binding approval/rejection authority but no structural accountability for verdict quality. The contract specifies detection of reviewer dysfunction in two modes: rubber-stamping (>95% approval rate metric, `MULTI_AGENT_MODE.md`) and abandonment (review exhaustion — 2 reviewers exit without verdict). However, these are contract-specified behaviors, not structurally enforced in the supervisor flow — the supervisor does not compute approval rates or detect review exhaustion patterns at runtime. The system cannot detect a third, more damaging mode: incorrect verdicts with plausible reasoning. A reviewer that rejects valid work forces full implement-review cycles before the Orchestrator evaluates (governed by `effectiveCoderIterationLimit` and `effectiveReviewCycleLimit` in `iteration_limits.go`), and the Orchestrator's assessment is itself the unvalidated judgment of the single semantic interpreter. A reviewer that approves flawed work is invisible unless integration tests catch it — but the system doesn't mandate integration tests on the integration branch. The power asymmetry is structural: Coders must address every rejection point-by-point, but there's no mechanism for Coders to challenge a rejection except by re-implementing and re-submitting. Note: with current LLM-based reviewers, over-rejection (spurious rejections with plausible reasoning) is the empirically dominant failure mode, making the iteration limit the most exercised circuit breaker in practice.

**Implication:** Code review quality is the least observable dimension of system health, yet it gates all task completion — the system optimizes for reviewer throughput signals while reviewer accuracy remains unmeasured.

**Future options:**
- Reviewer accuracy metric (compare rejected items against final merged state)
- Coder appeal mechanism (structured objection triggers Orchestrator evaluation before 5 cycles)
- Post-merge validation on integration branch (automated tests catch reviewer misses)

**Decision:** Integration Reviewer role. Would also catch incompatible changes made within the various merged worktrees.

### Hypothesis Exhaustion Without Root Cause

**Skill:** systemic-thinking
**Category:** FEEDBACK

**Issue:** Hypothesis exhaustion rule (two coders fail = must rescope) forces Orchestrator intervention but doesn't require root cause identification. Orchestrator may split task-3 into task-3a/task-3b without diagnosing why two coders failed. If underlying cause is spec ambiguity or architecture flaw, new tasks encounter same obstacle.

Circuit breaker theoretically catches this via spec_gap_cluster, but pattern detection uses exact string matching—different coders may describe same issue differently.

**Implication:** System may cycle through rescope iterations without converging, consuming time and compute on task churn rather than progress.

**Future options:**
- Similarity matching for anomaly clustering (semantic, not exact)
- Escalate to human after N rescopes of same original task

### Restart/Lease Churn Under Load

**Skill:** systemic-thinking
**Category:** FEEDBACK

**Issue:** Protocol restarts agents on exit 42 and uses leases/heartbeats for coordination. Under load or long-running operations, lease pressure and restart frequency can amplify each other. The restart loop is assumed stabilizing but can become self-sustaining when work exceeds lease windows.

**Implication:** Under stress, system enters churn state—progress stops but resource usage and log noise increase.

**Current mitigation:** Grace periods on lease checks. Context self-diagnosis triggers graceful abort.

**Future options:**
- Adaptive lease duration based on task complexity
- Supervisor watchdog with timeout detection
- Exponential backoff on repeated restarts

### Human Availability as Bottleneck

**Skill:** systemic-thinking
**Category:** ASSUMPTION

**Issue:** Human is circuit breaker, escalation point, spec author, checkpoint reviewer, and resolution authority for deadlocks. Sprint governance states hard checkpoints pause agents indefinitely awaiting human action, and transition checkpoints still require human action before downstream task creation. The "solo developers, small teams" deployment context is load-bearing, not merely scope-limiting.

If human attention becomes bottleneck (competing priorities, vacation, scaling), system has no degradation path. All escalation paths terminate at same person with no delegation.

**Implication:** Human availability constrains throughput more than agent capacity, inverting goal of reducing human bandwidth as bottleneck.

**Future options:**
- Timeout with automatic abort after N hours without human response
- Delegation mechanism for escalation routing
- Async human review queue with SLA tracking

### Supervisor Contention

**Skill:** systemic-thinking
**Category:** STRESS POINT

**Issue:** Supervisor-only worktree creation and claim handling centralize concurrency control and state transitions. All contention and race resolution concentrated in single process. Coders and Reviewers fully dependent on its throughput and correctness.

**Implication:** Supervisor contention becomes primary bottleneck when scaling beyond small task counts.

**Future options:**
- Partition by task ID for parallel claim handling
- Optimistic claiming with conflict resolution
- Dedicated claim coordinator separate from agent supervisor

### Dual Contract Delivery Paths

**Skill:** systemic-thinking
**Category:** FRAGILITY

**Issue:** Contracts reach agents through two paths: symlinks from repo root (development: `CLAUDE.md → ~/.liza/CORE.md → contracts/CORE.md`) and installed copies (`liza setup` writes to `~/.liza/`). Changes to contracts in the repo don't propagate to installed copies until `liza setup --force` is run. The Go binary embeds contracts at build time (`internal/embedded/`); installed copies are from the last `setup` run; symlinks resolve at read time. A running system can have three contract versions active simultaneously: the embedded version (used by prompt templates), the installed version (in `~/.liza/`), and the repo version (via symlinks). `liza setup` writes version metadata into installed contracts, providing partial version tracking, but there is no compatibility check between binary version and installed contract version, and `state.yaml`'s `version: 1` field is inert. `liza validate` validates state schema, not contract consistency. Note: agent prompts are built from Go templates (`internal/prompts/templates/`), not from embedded contract markdown directly — the embedded copies serve `liza setup`, not runtime prompt construction.

**Implication:** Contract drift between delivery paths is silent — agents may operate under different behavioral rules than the system operator believes are active, with no error signal.

**Partial mitigation (P1.4, commits `47e5597`, `bab9a78`):** `TestArtifactConsistency` in `internal/embedded/consistency_test.go` performs byte-exact comparison of repo master files against their embedded copies (contracts, skills, claude-settings.json, mcp.json). Wired into `make lint` via `make check-embedded`. This catches repo→embedded drift at build time, but does not address embedded→installed (`~/.liza/`) drift or runtime version mismatch.

**Remaining gaps:**
- Installed copies (`~/.liza/`) can still drift from both repo and embedded versions
- No runtime compatibility check between binary version and installed contract version
- `state.yaml`'s `version: 1` field remains inert

**Future options:**
- Content hash in contract files, verified at agent startup
- `liza validate` checks embedded vs installed contract consistency
- Single delivery path (eliminate duplication, choose symlinks or embedding)

---

## Structural Debt

### Decompose proceed.go (1,200 LOC)

**Skill:** code-quality-assessment
**Category:** RECOMMENDATION (P1.8 — escalated)

**Issue:** `internal/ops/proceed.go` at 1,200 LOC is 2.4x the Go ecosystem norm (500 LOC). Contains 26 functions spanning 6 distinct responsibility clusters: transition execution, many-to-one cohort management, phase-gate dependency propagation, topo-sorted execution with SCC cycle detection, crash recovery, and child task construction. Recommended for decomposition at the 2026-03-24 assessment (then 816 LOC); grew 47% without being addressed.

**Implication:** Navigability, reviewability, and testability degrade as file size increases. The 6 responsibility clusters share minimal state — extraction is straightforward but grows harder as the clusters interleave further.

**Direction:** Extract by responsibility cluster: transition execution, cohort management, child construction, graph algorithms, crash recovery, and the available-transitions engine. Each cluster has clear boundaries already. After structural decomposition, evaluate design-level improvements (P3.4).

### Decompose init.go (854 LOC)

**Skill:** code-quality-assessment
**Category:** RECOMMENDATION (P1.9)

**Issue:** `internal/commands/init.go` at 854 LOC with `InitCommandWithConfig` at 315 LOC. Sequential phases (project detection, symlink setup, config generation, interactive prompts) are inlined in a single function. Recommended for decomposition at the 2026-03-24 assessment (then 753 LOC); grew 13%.

**Implication:** Brownfield init evolution is constrained by the monolithic function. Adding new detection steps or interactive prompts requires understanding the full 315 LOC function.

**Direction:** Extract each phase into its own function or file. Phases are sequential with clear data boundaries.

### Decompose supervisor.go (831 LOC)

**Skill:** code-quality-assessment
**Category:** RECOMMENDATION (P1.10 — new 2026-04-13)

**Issue:** `internal/agent/supervisor.go` grew from 588→831 LOC (+41%) with 24 functions mixing: restart tracking (exit-42 + crash + spinning trackers, ~300 LOC), CLI execution (`DefaultCLIExecutor`, ~150 LOC), and the supervisor loop (`RunSupervisor` at 287 LOC).

**Implication:** Restart tracking is a self-contained subsystem with its own types and no shared mutable state with the supervisor — yet it's co-located, making the supervisor file harder to navigate and the trackers harder to test in isolation.

**Direction:** Extract restart tracking (3 tracker types + helpers) into `restart_tracking.go`. Extract `DefaultCLIExecutor` into a dedicated file. Reduces supervisor.go from 831 to ~380 LOC.

---

## Completed Fixes

---

## Fixed (Traceability)

| Issue | Category | Verified |
|-------|----------|----------|
| [MCP Admin Handler Authorization Gap](#mcp-admin-handler-authorization-gap) | FRAGILITY | 2026-03-11 — all handlers now use declarative `roleChecker` in `server_registration.go` |
| [Unbounded Integration Test Execution](#unbounded-integration-test-execution) | STRESS POINT | 2026-03-11 — `exec.CommandContext` with `DefaultIntegrationTestTimeout` (10m) + process group kill |
| [MCP Cross-Layer Read Dependency](#mcp-cross-layer-read-dependency) | TENSION | 2026-04-13 — MCP server removed (`90c132d5`, ADR-0057); single consumer (CLI) eliminates cross-layer concern |

---

## Fix Details
