---
date: 2026-06-15
perspective: Authored in Liza's own repo, but written as a deliberately fair, fact-grounded comparison. Each contested theme was argued by an adversarial pro-Liza advocate and an adversarial pro-gascity advocate, then reconciled by a neutral arbitrator instructed to reward genuine advantages, drop overstatements from both sides, and converge. The central honesty constraint here is a DUAL FRAME (see below) — Liza and gascity are different *kinds* of artifact, so any single-axis headline misrepresents one of them. Where a theme genuinely favors one system, it says so; where it is a trade-off, it says that.
method: Adversarial debate + arbitration over 7 contested themes (21 agents). gascity facts verified against the local SDK clone at v1.1.0-1252-g322dc987 (HEAD 322dc987, 2026-06-15); Liza facts from the local repository at v0.8.0. Load-bearing SDK claims (internal/git surface, reviewquorum wiring, pack layout, refinery prompt, Controller) were re-verified in source, not docs alone.
---

# Liza vs gascity (Gas City) Comparison

## The Dual Frame (read this first)

Liza and gascity are **different kinds of thing**, and comparing them fairly requires holding two frames at once. Collapsing to one frame is the single biggest way this document could mislead.

- **Frame A — framework vs framework:** Liza-the-orchestrator vs gascity-the-SDK. Here the question is *"which is the better orchestration toolkit — what can the technology express, and how does it age?"* On this axis **gascity wins decisively**: zero hardcoded roles, pack composition, a formula grammar with multiple topologies, a broad provider matrix, and a Kubernetes-style reconciler. It ships a complete, SHA-pinned **gastown reference pack** that reconstructs an entire shipping product as pure configuration — proof the genericity is real, not aspirational. Liza is essentially **one opinionated topology** (a doer+reviewer assembly line).

- **Frame B — assembled product vs product:** Liza vs *"gascity + the gastown pack"* — what a user actually runs for "trustworthy autonomous coding **today**." On this axis **Liza wins**: its code-backed floor (an approval-gated, SHA-bound merge gate and state-machine checks) is **guaranteed by construction in Go**, while its Tier-0 behavioral invariants are enforced by the contract's halt/reset semantics. The equivalent in the gascity world is **assembled from pack config and is agent-executed/advisory**. Even the pack's real mechanical merge gate (the refinery) is not an independent SHA-bound semantic verdict — the refinery prompt itself says *"All merge/conflict decisions are made by you, not Go code."*

Neither frame is "primary." "What runs trustworthy today" and "what the technology fundamentally is / how it ages" are both legitimate questions answering different needs; naming one primary smuggles in a value judgment. So the rule applied throughout: **credit gascity for genericity/authoring (an SDK-level, verified strength) and Liza for built-in enforcement (a product-level, verified strength) — and never let either borrow the other's axis.** "Can be configured to resemble Liza" is not "enforces Liza's guarantees." Equally, "ships one hardened pipeline" is not "is the whole category."

One artifact-identity note up front, because it recurs: **gascity (the SDK) and Gas Town (the product) are two distinct repositories by the same author.** gascity lives at `…/esciara/gascity` (`cmd/gc`, v1.1.0-1252-g322dc987, MIT, © Steve Yegge), and is where "zero hardcoded roles," the bundled gastown pack, and "the SDK ships no mutating git ops" hold. Gas Town the product (`github.com/steveyegge/gastown`, `cmd/gt`, role literals in Go) is a separate clone. Per gascity's own docs, **Gas City is the successor that generalizes Gas Town.** Claims below are about gascity unless stated otherwise.

---

## Source Snapshot

- **Liza**: local repository on `main`; release tag `v0.8.0`. Go, ~61k non-test LOC + ~144k lines of tests (~2.4:1 test-to-code). Apache 2.0. Single primary author (plus agent-generated commits via Liza's own MAS mode). Created January 2026. Stack- and provider-agnostic by design.
- **gascity**: local clone at `v1.1.0-1252-g322dc987` (HEAD `322dc987`, 2026-06-15). Go, MIT (© Steve Yegge). gascity's *own* orchestration code (`internal/` + `cmd/`) is ~354k non-test LOC + ~561k test (~1.6:1); the whole-tree figure is several times larger but inflated by bundled embedded-Dolt fixtures and in-repo git-worktree copies, so it overstates gascity's own surface. **Essentially single-author** as well (Steve Yegge, with many agent-generated commit identities). The larger surface reflects a different *kind* of artifact — an SDK + control plane + bundled state substrate — not a bigger team.

Both are essentially solo-author Go projects. The interesting question is therefore not maturity or governance (neither earns a community-hardening edge) but **design intent**: gascity deliberately refuses to be a finished product (no decision logic in Go, no hardcoded roles), and Liza deliberately *is* one (a hardened, opinionated pipeline with code-enforced guarantees).

---

## 1. Identity & Positioning

**Liza** — "Hardened Multi-Agent Coding System." A standalone Go CLI that supervises long-lived role agents running concurrently on isolated git worktrees, coordinated through a file-backed YAML blackboard. Liza wraps agents in control machinery and treats LLMs as unreliable components to be mechanically constrained where it matters. Its defining features are the **behavioral contract** (55 catalogued failure modes, 14 mapped to the MAST taxonomy, with tiered countermeasures) and a **binding adversarial review gate** with commit-SHA verification. It is **an opinionated product**: you install it and you get this pipeline.

**gascity** — An **orchestration-builder SDK** that extracts Gas Town's infrastructure into a configurable toolkit whose central invariant is **ZERO HARDCODED ROLES** ("if a role name appears in Go source, it's a bug") layered with **ZFC (Zero Framework Cognition)**: "Go handles transport, not reasoning; if a line of Go contains a judgment call, it's a violation." It is **a substrate, not a finished product** — a CI engine to Liza's single pipeline. Gas Town becomes "one pack among many" running on it, and the SDK ships that pack (`cmd/gc/.gc/system/packs/gastown/`) SHA-pinned and bundled offline, so `gc init` resolves a curated, versioned product as pure config.

The positioning gap is a **category gap**, not a slogan war. Liza answers "give me a trustworthy coding pipeline." gascity answers "give me the toolkit to build *any* coding-agent orchestration, of which a Liza-like pipeline is one expressible point." Both use code where code is reliable and models where judgment is irreducible — but gascity draws the code/model line far more aggressively toward the model (Go may transport and detect, never judge), while Liza is willing to compile a mechanical *backstop* for its highest-stakes guarantees.

---

## 2. Core Philosophy — Enforcement Floor vs Zero Framework Cognition

**Liza** starts from the premise that LLM agents are unreliable by default, and that the cheapest place to contain that unreliability is a **compiled enforcement floor**. The behavioral contract was developed incrementally as countermeasures to observed misbehavior — agents altering tests to pass, fabricating completions, drifting from scope. The contract is an *additive* quality layer; the load-bearing safety lives in Go: the supervisor validates every state transition, and **nothing merges without an approved, commit-SHA-verified reviewer verdict** (`internal/ops/wt_merge.go`). Delete `CORE.md` and all 55 failure-mode clauses and that floor still holds.

**gascity** organizes around a coherent engineering thesis:

- **ZFC (Zero Framework Cognition)**: the Go layer transports content and detects observable facts (session alive? heartbeat stale?); agents do all the interpreting. Even "if stuck then restart" is framework intelligence to be moved into the prompt — replaced by objective failure-*counting* plus evidence-before-restart.
- **Bitter Lesson alignment**: every primitive must become *more* useful as models improve, so nothing in the substrate may be a heuristic that ages against the model.
- **GUPP** (propulsion: "if there is work on your hook, you must run it" — liveness over caution) and **NDI** (Nondeterministic Idempotence: sessions die, persistent beads survive; reliability through redundancy, not prevention).

Two corrections keep this fair. First, **ZFC is a relocation of the floor, not its removal.** gascity keeps hard, model-independent invariants in Go that it refuses to delegate: a counting circuit breaker (`internal/scheduler/capacity` — failures ≥ max → Erlang/OTP-style quarantine), a process-table health patrol with zero LLM references, ff-only landing, idempotency keys, a hard `max_iterations` cap, and a crash-adoption barrier. The accurate phrasing is *"no code-quality / work-correctness **judgment** in Go,"* not "no coded decision rails at all." Second, the "no role name in Go" doctrine is **aspirational shorthand**: dozens of non-test Go files reference role names in comments and `gc init` scaffolding that *emits* a sample config. The *doctrine* (no role-conditioned reasoning in Go) holds; it is not a grep-clean guarantee.

The honest epistemic difference is about **which failure class each guarantee covers**, and they do not overlap:

- **Liza's guarantee is a MERGE-SOUNDNESS floor:** "an unapproved or different-than-reviewed change does not become durable shared state." Its semantic half (the reviewer verdict) is model-dependent; the *binding* of that verdict to the exact commit is compiled and model-independent.
- **gascity's guarantee is a LIVENESS floor:** "work eventually completes despite failing runs." It is genuinely model-invariant, but says nothing about a latent defect that merged green.

The Bitter-Lesson critique ("heuristics age badly") splits cleanly: it **does not reach** Liza's Go-backed merge invariant — "don't merge what no independent reviewer approved at this exact SHA" is the same structural kind of rule as git's own refusal to fast-forward a diverged ref, and it does not decay as models improve. It **does reach** Liza's *additive contract* parts (the bespoke regex repair pass for LLM-authored YAML, context-tier downgrades, prompt clauses tuned to 2026-era models), which will become friction as models stop committing those failures. gascity's "ages better" claim is correct about the substrate and overstated if applied to Liza's merge-soundness invariant.

**Verdict — trade-off, goal-relative.** Liza's coded floor is sounder for a trustworthy mainline at human-supervisable scale (one bad merge is expensive). gascity's opinion-free substrate ages better and scales further (the model is the bottleneck, and nothing in the substrate depreciates). It is **not** "rigid vs flexible" — both keep hard rails in Go; they disagree specifically about whether the **merge/review boundary** belongs there.

---

## 3. State Substrate — Where the Plan Lives

**Liza** — State lives in `.liza/state.yaml`, a file-backed YAML blackboard serialized through file locks. The write path is crash-safe despite being "just a file": flock + temp-file + fsync + atomic rename + read-modify-write under lock. The entire dependency closure is pure-Go (`gofrs/flock`, `yaml.v3`, `fsnotify`, `cobra`, charm) — **no SQL driver, no daemon, no listening port, no external binary.** State is one human-readable file you can `cat`/`grep`/`diff`/hand-edit, and recovery can be literally editing the file.

**gascity** — State lives in **Beads**, with **four store providers**, which makes it a *strict superset* of Liza's option space:

- `bd` (default) — an external `dolt sql-server` daemon (SQL with Git-like versioning: branch/merge/diff; durable per-row history).
- `native` — in-process embedded Dolt ("DoltLite"), no daemon.
- `file` — a single JSON file, no dependencies (this matches Liza's zero-ops floor almost exactly).
- `exec:` — a pluggable external provider.

So gascity can *be* Liza-shaped (the `file` provider) and then graduate to SQL-queryable per-row history by changing a provider setting rather than the orchestrator — a dial Liza does not offer. The honest seam: the **default** is the heavier `bd`/Dolt-daemon path, and the no-daemon `native` path documents a single-writer limitation — so gascity's high-concurrency story mainly requires the server mode, i.e. you cannot fully "have it both ways" (Liza-light *and* high-concurrency) on one lightweight substrate.

| | Liza | gascity |
|:--|:--|:--|
| State format | YAML file | Beads — 4 providers (Dolt server / embedded Dolt / JSON file / exec) |
| Concurrency control | Single global file lock + whole-file rewrite | Per-row SQL transactions (server mode) |
| Operational footprint | Zero (no daemon, no port, no external binary) | Zero with `file`; default `bd` needs a Dolt daemon + binaries |
| Versioning/history | None built-in (`.liza` is committable, not auto-committed) | Durable per-row history (server/embedded Dolt) |
| Query model | Load YAML into a Go struct, walk in-process | SQL queries, cross-rig aggregation |
| Scaling shape | Caps concurrency by construction | Independent row commits scale to many writers (server mode) |

**Verdict — trade-off, persona-decided.** For Liza's design center (one human, a handful of agents, one project), the YAML blackboard's zero-ops and hand-auditability are a real, verified advantage on *every* sprint. gascity wins **substrate flexibility**: it can collapse to Liza's zero-ops floor *and* scale past it, a move Liza cannot make in reverse (it cannot become SQL-queryable). One fragility Liza's file approach pays: it ships a bespoke regex repair pass for LLM-authored YAML indentation — a class of bug a typed schema does not have.

---

## 4. Agent Architecture & Roles

**Liza** — 13 roles across 4 pipeline phases (Specification, Architecture, Coding, Integration), but they are one conceptual machine — a doer+reviewer adversarial pair — instanced across phases, declared in `pipeline.yaml` plus a small `roles.go`. (Precisely: one mandatory, unique orchestrator + six doer/reviewer pairs across four sub-pipelines.) Agents are long-lived concurrent processes on isolated worktrees, leased and heartbeat-monitored. Supervision and recovery are centralized and **deterministic** in the Go binary (lease expiry, circuit breaker, `liza recover-agent`).

**gascity** — Roles are **pure configuration**; the SDK hardcodes none. The bundled gastown pack defines **six role directories** (`boot`, `deacon`, `mayor`, `polecat`, `refinery`, `witness`) plus a *patched* `dog` (a maintenance role) and an *inline* `crew` (a human workspace declared in `city.toml`, not an agent directory). What runs the fleet is the **Controller**: a Kubernetes-style **level-triggered reconciler** (`cmd/gc/city_runtime.go`) that each tick diffs desired-vs-running sessions and starts/wakes/stops to converge. Liveness is purely mechanical (process table: `pgrep`/`tmux`/`lsof`/`/proc`, never agent status files); crash-loops are handled by *counting* into an Erlang/OTP quarantine; a crash-adoption barrier reattaches live sessions on startup. The Controller makes **no code-quality or correctness judgment** — it only keeps the fleet alive and the session count satisfied.

| | Liza | gascity |
|:--|:--|:--|
| Roles | 13, baked into pipeline + supervisor | Zero in Go; defined per pack (gastown pack = 6 dirs + patched dog + inline crew) |
| Worker model | Long-lived, leased, heartbeat-monitored | Persistent identity, ephemeral sessions (pack-defined) |
| Supervision judgment | Go binary (deterministic) | Controller is mechanical; *behavioral* supervision lives in pack prompts |
| Re-skinnable topology | No (roles/pairing are fixed) | Yes (a different pack = a different system on the same engine) |
| Worker isolation | Git worktrees (structural) | Git worktrees (structural) |

**Verdict — split.** On **genericity** (Frame A) gascity is in a different category: the same engine hosts gastown, a security-audit pack, or any user pack with **zero Go changes** — Liza cannot be re-skinned into a different role topology. On **a code-enforced authority model** (Frame B) Liza is simpler and stronger: one binary, one lock-guarded state object, one place transitions are validated, with deterministic supervision that needs no watchdog-of-watchdog. gascity's Controller is excellent mechanical engineering, but it is **orthogonal** to the merge-safety question — keeping the fleet alive is not judging whether a change should land.

---

## 5. Task Decomposition — From Goal to Work Units

**Liza** — Adversarially reviewed decomposition at every level. The Orchestrator reads a goal and creates planning tasks; each doer (Epic Planner, US Writer, Architect, Code Planner) writes `output[]` subtask definitions that the paired reviewer validates *before* the supervisor mechanically fans out downstream tasks. A bad decomposition is **rejected before it propagates**, because every level carries a binding verdict.

**gascity** — Decomposition is **a formula, not a built-in.** The gastown pack's `mol-idea-to-plan` is one representative example (intake → draft PRD → parallel PRD review legs → human-clarification gate → parallel plan designers → alignment/self-review rounds → create beads → bead-verification passes) — potentially 20+ agent invocations for one decomposition. But it is *one expressible shape*; a user can author an entirely different decomposition as a formula. The review rounds are **corrective, not gating** (polecats reviewing polecats), and the default bare-bead path (`mol-polecat-work`) ships *self-review only* and pushes directly to main.

| | Liza (default pipeline) | gascity (`mol-idea-to-plan`, as an example) |
|:--|:--|:--|
| Decomposition author | Doer/reviewer pairs per phase | Crew worker + parallel polecat swarms |
| Review of decomposition | Adversarial — reviewer validates before fan-out | Self-review / alignment rounds |
| Review binding? | Yes (verdict blocks fan-out) | No (corrective, not gating) |
| Customizable? | Yes (pipeline config) | Yes (it's a formula — rewrite it freely) |

**Verdict — trade-off.** Liza's decomposition has binding verdicts at each level (a flawed epic plan is rejected before user stories are written). gascity's example formula throws more agents at the problem with corrective review, and is **freely re-authorable** in a way Liza's fixed topology is not. Both are user-editable — gascity via formula authoring, Liza via pipeline config — but only Liza *guarantees* the gate.

---

## 6. Trust & Behavioral Control

This theme conflates two separable things, and the honest split runs along that seam.

**Liza's *durable* control is narrow and real:** a set of merge/state invariants enforced in compiled Go, plus Tier-0 behavioral invariants enforced by contract halt/reset semantics. Verified in `internal/ops/wt_merge.go`: `MergeWorktree` refuses any task not in an approved state, requires a `review_commit`, normalizes it to a full SHA, and aborts (`IntegrationFailedError`/HEADMismatch) if the worktree HEAD ≠ the approved commit — re-validated under lock. That guarantee ("an unapproved or different-than-reviewed change does not become durable shared state") holds even if the agent ignores every prompt clause, and it ships as the out-of-the-box default.

**But the "55-mode, MAST-mapped, tier-structured contract" is mostly a *behavioral-contract* artifact, not a mechanical control system.** By Liza's own "remove the contract and the floor holds" framing, ~50 of the 55 modes are **prompt/process obligations** enforced by the agent contract rather than by Go runtime checks. (The coverage map header even marks itself "Not for agent consumption — zero runtime context cost.")

**gascity's behavioral layer is a genuine, content-rich peer on that advisory plane** — not a strawman. The gastown pack carries the "Idle Polecat Heresy"/approval-fallacy framing, per-role "The Failure Mode We're Preventing" propulsion blocks, "Do NOT adopt an identity from files" anti-hallucination, evidence-before-restart, TDD discipline with explicit prohibitions, and a reputational Capability Ledger. On a per-clause basis these are as sharp as Liza's, and some target failure modes Liza addresses more abstractly. Where a *hard* rule is wanted, the pack builds a **structural** one (the refinery owns merges, so polecats cannot push to main) rather than pretending prose is code.

The trade-offs are symmetric and real:

- **Liza wins centralization & auditability:** one reader can reason about behavioral *completeness* from two files, the coverage map answers "which documented failure modes are we NOT covering?", MAST ties modes to external literature, and the **tier hierarchy gives graceful degradation** (Tier-0 invariants are re-read from the Kernel tier under compaction, so the five hard rules survive when process-quality clauses are shed). A pack overlay, by contrast, can silently weaken a fragment with nothing to detect the gap.
- **gascity wins targeting & evolvability:** each rule lives at the role that needs it (no 55-row monolith in every agent's context), and the whole contract is **config** — `pack.toml` with imports, semver/SHA pins, agent patches, and per-rig overlays. An operator can tighten one rig, loosen a fragment for a trusted senior model, or SHA-pin a known-good contract **without recompiling** a single-author binary. This is the more Bitter-Lesson-aligned posture for the model-improvement trajectory.

**Verdict — trade-off with one clear sub-win.** Liza wins the hard **merge-safety floor** decisively (compiled, default-on; the bare gascity SDK has nothing at this layer — the floor only arrives via the gastown pack, and even then it is structural/prose-enforced, not a SHA-bound semantic verdict). On the ~50 advisory modes the two are near-peers (prose vs prose), Liza ahead on organization, gascity ahead on locality and evolvability. The decisive, narrow truth: Liza's win is a **merge-gate + completeness-instrument** win, *not* a "whole contract enforced in code" win; gascity's parity is an **advisory-behavior** parity, *not* a merge-floor parity.

---

## 7. Judgment Allocation

LLM judgment is expensive, unreliable, and valuable; the architectural question is *where to spend it.* The honest framing is a precise **SPLIT, not an "inversion."** Both systems run mechanical pre-merge test/lint/build gates and both keep liveness/scheduling deterministic. The single difference is **one judgment — "is this code correct?" — at one boundary — entry into durable shared state.**

**Liza** places a **binding** semantic verdict there and enforces the *placement* (not the verdict) in compiled Go. The "judged commit IS the merged commit" guarantee is real and code-backed (`wt_merge.go`: approval gate, SHA normalization, HEAD-mismatch tamper detection, TOCTOU re-validation under lock, compare-and-swap ref update with rollback). This is principled: the one irreducibly-fuzzy call is delegated to a swappable model, while the only thing frozen in Go — the *binding* — is the Bitter-Lesson-safe part (a smarter reviewer is worthless if the system can still merge a different commit than the one reviewed).

**gascity** spends determinism precisely where determinism is trustworthy (process-table liveness, crash-loop counting, level-triggered reconciliation, convergence-loop bookkeeping with a hard iteration cap) and **refuses to dress an LLM opinion as a mechanical guarantee.** Its convergence design cleanly separates the mechanical *loop* (Go owns iteration counting + idempotency + crash recovery) from the delegated *verdict* (a gate mode: manual / condition-shell-script exit code / hybrid). An exit code is deterministic, reproducible, and auditable in a way an uncalibrated LLM verdict is not — strictly more trustworthy for any correctness class you can actually express as an executable.

**Two honest bounds keep this fair:**

1. **Liza's win is bounded by its own default.** The coder→code-reviewer pair that actually gates the *code* merge defaults to **quorum 1 with no provider diversity** (`pipeline.yaml:475-478`) — so at its shipped setting the binding code verdict is a single opinion and provider separation is not mechanically required by the coding-pair policy. Operators can still make doer/reviewer model separation easy through `LIZA_DEFAULT_DOER_CLI` and `LIZA_DEFAULT_REVIEWER_CLI`. The quorum-2 + (hard) provider-diversity guarantees apply to the **spec / code-planning / arch / integration** pairs, not the coding gate. A binding gate that is confidently wrong manufactures false assurance — a failure mode gascity's refusal-to-assert structurally avoids.
2. **gascity's "gap" is narrower than "no gate."** The bare SDK genuinely has *no* correctness judgment before durable state (`internal/git` has no mutating merge/commit/push ops; `reviewquorum` is unwired; `mol-polecat-work` pushes direct-to-main). But the **assembled product** closes most of it: `mol-refinery-patrol` is a real mechanical Bors-style gate (rebase → run tests → merge-push on pass, reject/`FIX_NEEDED` on fail). The surviving, precise gap is **"no SHA-bound *independent semantic* verdict at the durability boundary,"** not "no gate." For regressions the test suite doesn't cover, gascity-as-product relies on test coverage.

**Verdict — split with one clear edge each.** On the contested axis (*does merged code carry a binding semantic verdict tied to the exact commit?*) Liza owns it: it is the only system of the two that puts a binding semantic verdict before durability and binds it to the exact commit in compiled Go — "one reviewer, bound" strictly dominates "no reviewer, unbound." gascity wins the opposite goal profile: higher concurrency, a model-agnostic re-derivable substrate, and exploratory / throwaway / open-a-PR (human-reviewed-downstream) workflows where a mandatory binding gate is overhead and verification-by-executable is the more honest correctness primitive. (NDI does **not** close this: it guarantees *delivery*, not *correctness* — a wrong-but-idempotent change converges reliably to the wrong durable state.) One economic caveat against "gascity should just move that spend to the merge gate": a binding per-activity LLM review at high concurrency is a throughput tax its design center cannot afford.

---

## 8. Review & Verification

**Liza** — Adversarial doer/reviewer pairs on every task; a separate reviewer issues a **binding verdict**, and approval means merge eligibility. Commit-SHA verification binds the verdict to the exact reviewed commit (judged == merged), with a compare-and-swap ref update (`internal/git/merge.go`, `RefConflictError`/retry) preventing a clobber under concurrency — equivalent serialization to a merge-slot lock, but with no external store to depend on. A dedicated **Integration phase** (integration-analyst + integration-reviewer) re-reviews cross-task interactions after individual merges — a second-order semantic gate catching emergent breakage that per-branch gating structurally cannot. **Caveat (carried honestly):** the *code-review* step defaults to quorum 1 with no provider diversity; the robust multi-reviewer/diversity guarantees apply by default to the spec/arch/integration pairs.

**gascity** — In the bare SDK there is **no merge gate at all** (`internal/git` exposes only read/probe helpers — `SameCommit`, `HasUnpushedCommits`). The gate is the gastown pack's **refinery**: a single-owner, Bors-style processor that rebases onto a fresh target, runs the project's mechanical gates, and merges-and-pushes **only on green**, mechanically reopening a failing branch to the pool. It is deterministic and reproducible — but it is **agent-executed**: the refinery prompt states verbatim, *"You are the decision maker. All merge/conflict decisions are made by you, not Go code,"* and its test/merge steps are guarded by FORBIDDEN-text, not by code that can refuse. Its strongest review hook (a human PR-approval gate) is **off by default**; `reviewquorum` is unwired. Optional batch-then-bisect (upstream Gas Town) can localize a breaking commit, a capability Liza lacks.

| | Liza | gascity (+ gastown pack) |
|:--|:--|:--|
| Review model | Binding adversarial doer/reviewer | Mechanical rebase→test gate, agent-executed |
| Review blocks merge? | Yes (binding, SHA-bound verdict) | Tests block; no independent semantic verdict; optional human PR approval (off by default) |
| Provider diversity | Hard filter on spec/arch/integration; off for code-review step | No |
| Judged == merged | SHA-bound, compiled | Single-owner structure closes the tamper window mechanically, not by binding a verdict |
| Cross-task review | Dedicated Integration phase | Per-branch only; rides on the test suite |

**Verdict — genuine trade-off; Liza safer at the exact moment of merge.** On the literal question — *does merged code carry a correctness signal beyond "tests pass"?* — Liza is genuinely stronger and it is the **default you must work to remove**: it is the only system here putting a binding, independent, SHA-bound semantic verdict on the critical path, plus a cross-task integration pass. But that verdict is an uncalibrated LLM opinion (binding a possibly-wrong verdict makes it *attributable*, not *correct*), it is the most token-expensive topology, and on the highest-volume gate (code review) its default is a single reviewer with no provider-diversity requirement in policy, though doer/reviewer CLI separation is easy to configure. gascity trades that for a deterministic, reproducible, higher-throughput floor and makes the extra (human or script) verdict opt-in exactly where it earns its keep. Decision rule: if "only an approved, exact reviewed commit becomes durable, by default, including cross-task, even if someone weakens the config" is the requirement → Liza. If "maximize verified-green throughput on a reproducible floor and place strong verdicts surgically" is the requirement → gascity.

---

## 9. Coordination & Communication

**Liza** — Coordination through the YAML blackboard: a **broadcast, shared-state** model. Every agent sees every task's full state, history, and PR-like review comments without explicit routing. Coordination is implicit through state transitions — the blackboard is simultaneously the task board, the communication channel, and the audit trail. Situational awareness is automatic.

**gascity** — A **point-to-point** architecture with more primitives, because supervision is distributed across communicating agents: **Mail** (persistent inter-agent messages as beads), **Nudge** (lightweight tmux notifications), **Hook** (a pinned bead serving as an agent's work queue), **Seance** (discover/query/resume a predecessor's session via event logs), **Escalate** (structured severities routed up the hierarchy), plus beads-native groups/queues/channels and convoy/cross-rig dependency notifications.

**Verdict — different trade-offs, not a clear winner.** gascity has more communication *primitives* enabling point-to-point patterns (requests, clarifications, handoffs, escalation) and **retrospective session query** (Seance), which Liza lacks. Liza's blackboard is a more powerful *coordination substrate* at its scale: shared state gives global visibility without routing decisions, and review interaction is structured communication *through* state. gascity requires explicit routing ("who do I message?"); Liza makes coordination implicit ("what changed on the board?").

---

## 10. Persistence & Recovery

**Liza** — Built for failure: leases expire and tasks become reclaimable; crashed agents/tasks are recoverable (`liza recover-agent`, `recover-task`); a circuit breaker detects systemic failure; failure is a *recorded state transition* (BLOCKED, REJECTED, SUPERSEDED). Recovery is centralized and deterministic — fewer moving parts, no watchdog-of-watchdog. Handoff is structured but forward-only (notes on completion), not retrospective.

**gascity** — Also built for failure, with richer mechanisms: NDI + persistent beads mean "sessions die, work survives"; the Controller's crash-loop counting → quarantine and crash-adoption barrier keep the fleet self-healing; Seance recovers a predecessor's context; session handoff cycles at context limits (PreCompact hooks); the projection of session state is a pure function over observed facts, so a wrong agent action is recoverable and auditable rather than silently durable.

**Verdict — both serious; gascity's is richer, Liza's is simpler.** gascity's Seance (querying a predecessor's reasoning) and its self-healing reconciler are capabilities Liza lacks. Liza's recovery is centralized and deterministic — no retrospective session archaeology, but no quarantine timers to tune and nothing to monitor.

---

## 11. Workflow Definition & Configurability

**This is the heart of Frame A, and gascity wins it decisively.**

**Liza** — Two tiers, essentially **one topology**. The full MAS pipeline is a declarative YAML pipeline (roles, phases, transitions) configurable three ways: a global `~/.liza/pipeline.yaml`, a per-project frozen `.liza/pipeline.yaml`, and `--entry-point` to start mid-pipeline. For lighter work, the **adversarial-pairing skill** coordinates doer/reviewer sessions through a Markdown blackboard. It is opinionated around a doer+reviewer assembly line — and crucially, **review is the grammar's atomic unit**: `internal/pipeline/config.go` hard-rejects any role-pair whose reviewer does not resolve to a real role, so you *cannot author a doer-only pair* through the supported pipeline path.

**gascity** — A genuine small **composition language**:

- **Three validated formula topologies** — `workflow` (sequential steps with a `needs` DAG), `expansion` (parameterized template), `aspect` (parallel analysis passes). *(Upstream Gas Town adds a fourth, `convoy`; the gascity SDK validates three.)*
- **Typed inputs/vars**, single- and multi-parent **inheritance** (`extends`, with circular-extends detection), and **composition** (`compose`/`expand`).
- **Pack composition**: imports, semver/SHA pins, agent patches, per-rig **overlays** with CSS-like per-step overrides, and a 3-layer resolution (rig → town → embedded) that survives upgrades.
- **~23 bundled formulas**, a broad **provider-preset catalog** (~19 presets across ~10+ CLIs), progressive capability levels, and several runtime providers (tmux/subprocess/exec/acp…).

| | Liza | gascity |
|:--|:--|:--|
| Workflow definition | YAML pipeline + Markdown-blackboard skill | TOML formulas (3 topologies) + packs |
| Topologies | One (doer+reviewer chain) | Three (workflow/expansion/aspect) |
| Composition / inheritance / overlays | No (fork-and-edit the frozen pipeline) | Yes (`extends`, `compose`, per-step overlays, SHA pins) |
| Re-skin into a different system | No | Yes (a different pack = a different product) |
| Built-in review guarantee | **Yes** — binding review is the default floor | **No** — a formula need not include review; default path pushes direct-to-main |

**Verdict — split on two orthogonal axes, no overall winner.** **Expressiveness/genericity: gascity wins clearly** — more topologies, real composition and overlays, typed vars, and a demonstrated ability to reconstruct a whole product (gastown) as config. **Built-in guarantee: Liza wins clearly** — independent adversarial review is a Go-enforced merge precondition and the *default*; its grammar cannot even *name* an unreviewed-merge workflow through the supported path, whereas gascity's default expresses the unsafe option. A superset grammar raises the expressive ceiling **and** lowers the safe-default floor simultaneously; these are independently dialable. "A grammar that can express 'no review' is not more powerful *for the goal of trustworthy autonomous merges*; a single opinionated topology is not more powerful *for arbitrary orchestration authoring*" — which matters more is goal-relative. (Do not double-book: counting Liza's safety default as an authoring win, or gascity's breadth as a merge-trust win, mis-scores one axis against the other.)

---

## 12. Human Role

**Liza** — The human owns intent and acts as observer/circuit-breaker. Within a sprint, agents are autonomous; between sprints, the human reviews artifacts and steers via CLI. In **Pairing mode**, the human is an active collaborator with approval gates and structured postures (Coach, Challenger, Spike, User Duck).

**gascity** — The human is a **Crew member** with a long-lived workspace (a full git clone, pushing directly to main — "no feature branches"). Interaction is mediated through pack-defined roles (in the gastown pack, "tell the Mayor what you want"). Some formulas include explicit human gates (e.g. `mol-idea-to-plan`'s clarification step); a rig can opt into a human PR-approval merge gate.

**Verdict — both keep the human at the boundary; different shapes.** Liza's human steers between sprints with a richer synchronous Pairing mode (collaboration postures gascity has no equivalent of). gascity's is more federated — the human works alongside agents in their own managed Crew workspace, which Liza does not model as a first-class entity.

---

## 13. Provider Support

**Liza** — Multi-provider (Claude Code, Codex, Gemini, Kimi, Mistral). The behavioral contract improves agent quality but is additive; mechanical enforcement constrains any model regardless. Provider diversity is a deliberate, **hard** filter in the review quorum — but, per §8, only on the spec/arch/integration pairs, not the code-review step. Integration is effectively binary: a model handles the contract or it doesn't.

**gascity** — Providers are a **configurable data axis, not a Go enum**: builtin presets keyed by typed capability flags (e.g. `SupportsACP`, `SupportsHooks`), so adding an agent is adding a preset entry, not Go branching. A broad catalog (~19 presets across claude/gemini/codex/cursor/auggie/amp/opencode/copilot/pi/omp/kimi/grok/ollama…) with progressive capability tiers (terminal-via-tmux at the floor; deeper hooks/session-forking/wrappers above).

**Verdict — gascity is more inclusive here.** Any terminal CLI participates at the floor tier, with deeper capability unlocked progressively, and the provider set is pure config. Liza optimizes for behavioral compliance over breadth of integration.

---

## 14. Scale & Cost

**Liza** — Cost is dominated by the behavioral contract (every agent pays for it in context), the multi-sprint lifecycle, and the binding-review topology. Context tiers (Full → Working Set → Kernel) manage degradation. A handful of agents per sprint. Quota handling is **detect-and-shutdown** (`internal/agent/quota.go` converts a rate-limit pattern to a signal file that gracefully stops supervisors). **No shipped cost tracking, no usage-limit/account-rotation** — a real gap at fleet scale.

**gascity** — Designed for higher concurrency, with first-class machinery and a **governed-resource** model (per-tick wake/create budgets, per-agent `MaxActiveSessions`). At the assembled-product level it ships **cost visibility** (per-session recording, by-role/by-rig digests) **plus management levers**: an account-quota **rotation** subsystem (`gt quota rotate`, including preemptive rotation off a pool) that keeps a fleet alive under throttling, and **cost-tier steering** (route patrols/workers to cheaper models). **Important correction:** neither system *enforces* a budget — gascity's cost path is verified record/query/report only, with no halt/cap. So enforcement is a 0-0 draw.

**Verdict — gascity wins this dimension, but it is visibility + levers, not enforcement.** Cost attribution, account rotation, tier steering, and scheduler-governed dispatch are shipped infrastructure with no Liza equivalent (Liza ships *no* cost tracking and, on quota exhaustion, shuts down rather than failing over). Frame it precisely: gascity wins cost **visibility** and adds management **levers**; hard budget enforcement exists in neither.

---

## 15. Maturity & Adoption

**Liza** — `v0.8.0`, single primary author, January 2026, Apache 2.0. Self-implementing since v0.4.0. ~61k non-test LOC + ~144k test (~2.4:1; ~70% of the codebase is tests — verification depth, not API breadth). Small but real adoption; design coherence from one author, with the corresponding single-maintainer continuity risk.

**gascity** — `v1.1.0-1252-g322dc987`, MIT (© Steve Yegge), 2026. **Also essentially single-author** (with many agent-generated commit identities). ~354k non-test LOC + ~561k test in its own orchestration code — the larger surface reflects a broader *kind* of artifact (SDK + control plane + bundled state substrate + provider matrix), not a bigger team.

**Verdict — two solo-author, architecturally serious systems at similar early maturity.** Neither has the community scale of CrewAI or BMAD. gascity is materially ahead on *operational* and *expressive* surface (control plane, formula grammar, provider breadth, ops tooling); Liza is ahead on *correctness* surface (binding gate, failure-mode rigor, test density). Each side's strength is the other's gap; the design divergences are more interesting than the adoption counts.

---

## 16. Auditability & Continuous Improvement

**Liza** — Full audit trail as a design feature: the blackboard records every transition, assignment, verdict, rejection reason, and rescoping event (durable, human-readable YAML). Two analysis skills operate on the trail: `/liza-logs` (anomaly patterns, token usage, behavioral signals at sprint boundaries) and `/context-engineering` (context budget, duplicated/missing context, prompt drift). Sprint checkpoints feed findings into the next sprint's configuration. The feedback loop is **built-in and actionable**, but self-contained (grepping your own logs).

**gascity** — A broader, **exported** observability stack: a **Capability Ledger** (a permanent record of completions/handoffs used to route work to proven agents; formula-compliance is detectable), **OpenTelemetry** (OTLP logs+metrics to external backends), an internal event bus + decision-trace recorder, raw event logs queryable by Seance, and **`gc doctor`** (~45 check constructors spanning workspace/infra/Dolt health/lifecycle, with blocking-vs-advisory severity and autofix). A/B model-eval harness for comparing models on similar tasks.

| | Liza | gascity |
|:--|:--|:--|
| Audit trail | Blackboard (YAML, durable, human-readable) | Capability Ledger + event logs (Beads/Dolt + JSONL) |
| Analysis tooling | `/liza-logs` + `/context-engineering` (built-in skills) | OTLP backends + decision-trace + A/B harness |
| Diagnostics | Circuit breaker, `liza validate`/`analyze` (narrow) | `gc doctor` (~45 checks + autofix) |
| Telemetry | Agent logs (session-level) | OTel (metrics + logs to external backends) |

**Verdict — both serious; gascity broader on ops observability.** Liza's analysis is *built-in* and produces actionable findings at sprint boundaries; gascity's telemetry is *exported* (standard OTel, external backends) and its `gc doctor` is a genuine operational capability Liza lacks at that breadth. Liza's `validate`/`analyze` cover a narrower slice.

---

## 17. Notable gascity Capabilities

### Genuine, demonstrated strengths (Frame A)

1. **Zero-hardcoded-roles SDK** — the entire product surface is config; the same engine hosts gastown or any user pack with no Go changes. This *is* the genericity the comparison turns on.
2. **Formula/pack composition language** — typed vars, `extends`/`compose`, per-rig overlays, semver/SHA pins, 3-layer resolution. The most expressive workflow-authoring model in this survey.
3. **Kubernetes-style level-triggered Controller** — mechanical liveness (process table, never status files), Erlang/OTP crash-loop quarantine, crash-adoption barrier, typed session state machine. Excellent ops engineering (and a clean Bitter-Lesson/ZFC exemplar).
4. **Beads substrate with 4 providers** — collapses to a JSON file (Liza-light) or scales to SQL-queryable per-row history; a dial Liza lacks.
5. **Cost visibility + account rotation + cost-tier steering** — keeps a fleet alive under throttling; cuts spend by routing to cheaper models.
6. **OTel telemetry, `gc doctor` (~45 checks + autofix), A/B model-eval** — a real operational/observability stack.
7. **Seance** — retrospective session query/resume; interrogate a predecessor's reasoning.
8. **Broad provider matrix** — ~19 presets, capability-flag-typed, any terminal CLI at the floor.
9. **A bundled, SHA-pinned reference pack (gastown)** — proves the SDK's generality is not vaporware; it reconstructs a whole shipping product as config.

### Honest limits of that reach

- gascity's reach is **"can express," not "enforces."** No bundled pack today reconstructs Liza's specific trust stack (binding SHA-bound semantic verdict + Tier-0 behavioral-contract hierarchy). The gastown pack's refinery is mechanical-but-**agent-executed**, and the default `mol-polecat-work` pushes direct-to-main.
- The Controller's quality is **orthogonal** to merge-safety: it is pure liveness/scheduling and "makes no code-quality or work-correctness judgment."

---

## 18. Notable Liza Features gascity Lacks

1. **Binding adversarial review with commit-SHA verification** — an independent reviewer's verdict gates merge **by default**, in compiled Go; gascity's equivalent is assembled, agent-executed, and (for the semantic verdict) absent.
2. **Failure-mode catalog (55, 14 MAST-mapped) with a completeness instrument** — answers "which documented failure modes are we NOT covering?"; gascity has rich per-role behavioral prose but no central catalog, no MAST mapping, no coverage map.
3. **Tiered invariant system (Tier 0-3) with graceful degradation** — Tier-0 invariants survive context compaction via the Kernel tier; gascity's per-role prompts have no priority structure.
4. **Code-enforced state machine** — the Go supervisor validates every transition; gascity delegates transition judgment to agents (ZFC).
5. **Provider-diversity review as a hard filter** — on the spec/arch/integration pairs (not the code-review step); gascity's review is single-provider.
6. **Pairing mode with collaboration postures** — Coach, Challenger, Spike, User Duck.
7. **Dedicated Integration phase** — a post-merge review of cross-task interactions; gascity gates each branch in isolation.
8. **Explicit context-degradation tiers** — Full → Working Set → Kernel, with defined re-read protocols.
9. **Zero-ops by construction as the default** — pure-Go, single hand-editable file; gascity's *default* substrate is the heavier Dolt-daemon path (the `file` provider matches Liza but is not the default).

---

## 19. Where They Overlap (The Convergence)

Two solo-author Go projects by adjacent design lineages, reaching for the same primitives:

- **Git worktree isolation** as the concurrency boundary.
- **Structured state persistence** surviving crashes.
- **Role-based agent organization** with distinct boundaries.
- **Multi-provider support** — the agent CLI as a replaceable component.
- **Crash recovery** as a first-class concern.
- **Mechanical quality gates before merge** — both require tests/lint/build to pass.
- **CLI-mediated agent interaction.**
- **Human at the boundary** — approve-then-observe, not per-action queues.
- **Runtime-configurable workflows** — pipeline config (Liza) / formulas+packs (gascity), not hard-coded behavior.
- **Deterministic infrastructure invariants in Go** — both keep liveness/scheduling/idempotency mechanical; they differ on whether *merge/review* joins them.
- **Go as implementation language.**

The independent convergence on these suggests they are load-bearing decisions for multi-agent coding systems, not arbitrary choices.

---

## 20. Where They Diverge Most

The fundamental divergence is **what kind of artifact each chooses to be**, and downstream of that, **where each draws the line of Go's authority** and **what each guarantees**.

**Liza** is a **product** that spends determinism on an enforcement *floor* (state machine, forbidden operations, a binding merge gate) and spends LLM judgment on *code review with binding authority*. Its guarantee: an unapproved or different-than-reviewed change does not become durable shared state. Its design center is **correctness-bounded, human-supervisable-scale delivery** where a binding gate is affordable and one bad merge is expensive.

**gascity** is an **SDK** that, by ZFC, refuses to put *any* judgment in Go — Go transports and detects; agents (via packs) interpret. Its guarantee: work eventually completes despite failing runs (a liveness/idempotency floor), plus whatever a chosen pack assembles on top. Its design center is **a maximally general, well-aging substrate** on which high-concurrency, federated, throughput-oriented orchestrations can be built.

These are not symmetric "rigid vs flexible." Liza's real risk is over-constraint, a lower concurrency ceiling, and an additive contract layer that ages against improving models. gascity's real risk is that, in the bare SDK and even in the assembled product, **test-passing-but-wrong code can merge with no binding semantic gate**, and that its trust properties must be *correctly assembled* rather than being guaranteed by construction. Both risks are genuine; neither system is incoherent.

---

## 21. Framework Failure Modes

**Liza** struggles when:
- Work needs flexible, ad-hoc orchestration that fits neither the MAS pipeline nor adversarial-pairing — there is essentially one topology.
- High agent counts are needed — a single global state lock + whole-file rewrite caps concurrency by construction.
- Operational cost/quota governance at fleet scale matters — Liza ships none (no cost tracking, no account rotation).
- A workflow author wants composition, inheritance, or overlays — Liza offers fork-and-edit, not a grammar.
- *(Note: the behavioral contract is additive. A model that ignores it is still constrained by the Go supervisor's merge/state floor — but most contract modes are behavioral obligations rather than Go-backed runtime checks.)*

**gascity** struggles when:
- Judgment-level code quality matters beyond what tests catch — there is **no binding semantic merge verdict at any layer** (the bare SDK has no gate; the pack's gate is mechanical and agent-executed).
- Trustworthy-by-default-out-of-the-box is wanted — the safe configuration must be *assembled* (correct `pack.toml`, the right SHA pin, the refinery honoring its prompt), and the default `mol-polecat-work` pushes direct-to-main.
- A single reader needs to reason about behavioral completeness — the contract is distributed prose fragments with no central catalog, MAST mapping, or tier system, and an overlay can silently weaken a fragment.
- Provider diversity in review is wanted — review is single-provider.
- Zero-ops simplicity is wanted by default — the default Beads/`bd` path needs a Dolt daemon + binaries (which `gc doctor` exists partly to manage); the `file` provider matches Liza but is opt-in.

---

## 22. What Each Could Steal

### What Liza could steal from gascity

1. **A more composable workflow grammar** — formula-style inheritance/overlays/SHA-pins would extend Liza's two-tier model toward per-task reuse without forking pipelines.
2. **First-class cost + quota governance** — per-session cost tracking, account rotation, and rate-limit-aware dispatch are operational infrastructure Liza should ship.
3. **A `gc doctor`-style diagnostic engine with autofix** — Liza's `validate`/`analyze` cover a narrower slice.
4. **Session query (Seance)** — Liza has the data (blackboard history + logs) but no mechanism to interrogate a predecessor's reasoning directly.
5. **Exported OTel telemetry** — for running Liza as fleet infrastructure rather than a solo tool.
6. **A pluggable state substrate** — a `file`-vs-SQL provider dial would let Liza keep zero-ops by default and scale when needed.

### What gascity could steal from Liza

1. **A binding adversarial review option promoted to a blocking gate** — once calibration data exists, an independent, SHA-bound semantic verdict closes the "tests pass" vs "actually correct" gap the assembled product still has.
2. **Selective code-enforced enforcement at high-stakes boundaries** — for merge authority and review verdicts, a mechanical backstop (a doctor check, a tool-guard hook, or a pack-structural constraint) provides guarantees prompt conventions cannot. ZFC is a fine default; selective enforcement at the durability boundary would strengthen it without abandoning the principle.
3. **A failure-mode catalog with a completeness instrument** — a central, MAST-grounded map so an operator can answer "which modes are uncovered?" instead of assembling the picture from per-role prompts.
4. **A tiered invariant hierarchy** — an explicit priority structure so the load-bearing rules survive context compaction.
5. **Provider-diversity review** — route review to a different model family to break correlated blind spots.
6. **A safer default** — make the out-of-box dispatch include review / not push direct-to-main, so the trustworthy configuration is the one you must work to *leave*, not the one you must work to *reach*.

---

## 23. Layering & Integration — Can They Compose?

gascity and Liza sit at the same architectural layer — both external orchestrators wrapping agent CLIs — so direct composition is awkward (you would not run Liza supervising a gascity town or vice versa). But there is a real asymmetry: **gascity could, in principle, host a "Liza-like" pack** (a doer+reviewer pipeline expressed as formulas) — except it would reproduce Liza's *shape*, not its *guarantees*, since the binding SHA-bound semantic verdict and Tier-0 behavioral-contract hierarchy are exactly the parts ZFC declines to compile.

The realistic path is **idea exchange, not system layering**:

- Liza adopting gascity's substrate/observability/cost patterns (provider dial, OTel, cost tracking, session query) without adopting Dolt or ZFC wholesale.
- gascity adopting Liza's enforcement patterns (a promoted blocking review, selective structural enforcement, a tier system, a completeness map) at high-stakes boundaries without abandoning ZFC everywhere — routed through its own sanctioned mechanisms (doctor checks, hooks, pack-structural constraints) rather than judgment-in-Go.
- A shared standard for agent session events both systems could produce and consume, enabling cross-system session discovery.

---

## 24. Bottom Line

gascity is **a different kind of artifact** from Liza, so the fair verdict is explicitly two-framed:

- **As frameworks (Frame A), gascity wins decisively.** It is a substrate — a CI engine to Liza's single pipeline — with zero hardcoded roles, the most expressive workflow-authoring model in this survey (formulas, composition, overlays, SHA pins), a broad provider matrix, a clean level-triggered control plane, a pluggable state substrate, and real ops/observability tooling. It demonstrably reconstructs a whole product (gastown) as config and can express topology families Liza structurally cannot. If the question is "what is this technology, what range of systems can it express, and how does it age," gascity is the answer.

- **As products (Frame B), Liza wins.** For "what gives me trustworthy autonomous coding I can run **today**, with minimal ops," Liza's code-backed floor is **guaranteed by construction**: an approval-gated, SHA-bound, independent adversarial review gate as the default (the one place it puts judgment, and it puts it where tests can't reach), plus a code-enforced state machine. Its behavioral-contract layer adds a documented failure-mode catalog with a completeness instrument and a tiered invariant hierarchy, and it keeps a genuinely zero-ops footprint. Even **gascity + the gastown pack** does not reproduce Liza's binding independent semantic verdict — its best assembled gate is mechanical and agent-executed ("decisions made by you, not Go code").

The sharpest lesson is about **judgment allocation**, stated precisely. Both systems use code where code is reliable and models where judgment is irreducible — they disagree about which problems fall where, and about whether a *mechanical backstop* for the merge/review boundary is wisdom or a Bitter-Lesson violation. gascity's allocation is coherent and well-aging: gating merges on mechanical truth is correct, spending judgment on fuzzy fleet-liveness is defensible, and refusing to dress an LLM opinion as a guarantee avoids manufacturing false assurance. But it ships **no binding correctness judgment before a change becomes durable shared state**, at any layer — and that is a real gap for the "trustworthy today" use case. Liza's allocation puts binding judgment exactly at that boundary and compiles the *binding* (not the opinion) into Go — at the cost of throughput gascity's scale target can't pay per-activity, and with an honest soft spot of its own (the code-review gate defaults to a single reviewer with no provider-diversity requirement, though doer/reviewer CLI separation is easy to configure).

So the recommendation is workload-shaped, and the frame you are in decides which strength is "primary":

- **Building an orchestration system, or betting on how the technology ages?** gascity is the more capable, more general, better-aging foundation — and Liza-like trust can be *assembled* on it, with effort.
- **Running trustworthy autonomous coding today at human-supervisable scale, where one bad merge is expensive and ops should be near-zero?** Liza's enforced floor is the stronger out-of-the-box guarantee, and gascity's assembled equivalent does not yet match its binding semantic verdict.

Each is, in its own design center, the better choice — and neither frame may borrow the other's axis.
