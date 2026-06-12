# Event-Journal Migration — Status & Concerns

Status of the strangler migration toward an event-journal architecture, and
the concerns a reviewer (and the next implementer) must know before continuing.
This is a **work-in-progress migration**, not a finished system: `state.yaml`
remains the source of truth. Everything here is additive and reversible.

## Why this migration

Liza's orchestration complexity is largely compensation for one early choice: a
mutable, denormalized `state.yaml` that ~55 commands mutate in place. Because
that model *permits* invalid states, the system grew an immune system to repair
them after the fact (a 2k-line `statevalidate`, a transcript-scrubbing
`statehygiene`, ~10 repair commands, five overlapping supervisor progress
trackers, lazy read-path migrations, duplicated ownership/lease fields).

The redesign inverts the foundation: an **append-only event journal** with
derived state, **one** progress policy, **one** scheduler, and a write funnel
that makes invalid states *unwritable* instead of *repairable*.

## What has landed (all behind `state.yaml`-as-truth; additive)

| Area | Package / file | Notes |
|------|----------------|-------|
| Append-only journal | `internal/journal` | Plain JSONL (`.liza/journal.jsonl`), **no SQLite** (per code-owner constraint). fsync, field-size caps, torn-tail tolerance. |
| Shadow derivation | `internal/db/blackboard.go` | Every `Modify`/`Write` derives typed events by diffing before/after; appended inside the state lock. No call-site changes. |
| Op provenance | `Blackboard.ModifyOp(op, fn)` | Named operations flow into journal `Op` field + lock diagnostics. ~70 write sites migrated to named ops. |
| Progress policy | `internal/agent/progress.go` | Five overlapping trackers (exit42/crash/spin/runtime-failure/success-no-progress) → one `progressLedger` + one threshold table. |
| Structural invariants | `internal/taskinvariants` | Per-task field rules extracted as a leaf package; **enforced fail-closed at the write funnel** for named ops (no-worse-than-before semantics). |
| Claims as entities | `internal/models/claim.go`, `internal/ops/claim_records.go` | First-class claim records, dual-written alongside legacy ownership fields. `SweepExpiredClaims` primitive. Contradiction warnings in `statevalidate`. |
| Journal rotation | `internal/journal/rotate.go` | Archives at threshold with a snapshot event seeding the projection; projection stays equivalent across rotation. Unbounded growth solved. |
| Scheduler core | `internal/scheduler` | Pure `Compute(state, resolver, now) → Plan` for doer/review/merge/reclaim work. Surfaced read-only via `liza schedule`. **Not yet wired into the live loop.** |
| Verification | `liza journal --verify`, `liza validate` | Folds journal → reconstructed view; warns on divergence across task statuses, claim holders, and system singletons (sprint/circuit-breaker/goal/mode). |

Two real bugs were found *by* the fail-closed write funnel during migration:
stale/missing `base_commit` on rejected-task reclaim, and `release-claim`
stranding submitted tasks as unreviewable zombies. Both fixed.

## Concerns / risks (read before continuing)

### 1. The journal events are lossy diffs, not snapshots — this is load-bearing
The derived events capture status *changes*, claim grant/release, and appended
anomalies — **not full task bodies** (description, worktree, base_commit,
history, output). Consequences:

- A literal "fold the journal back into `state.yaml`" is **impossible today**.
  `journal.Reconstruct` deliberately covers only the fully-captured dimensions
  (task statuses, claim holders, system singletons) and documents the gap.
- Therefore "make the journal the source of truth" is **not** a precursor step —
  the precursor (a complete, lossless per-field event vocabulary) *is the bulk
  of the flip itself*. Do not under-scope it.

### 2. The scheduler core is proven but NOT live
`scheduler.Compute` is pure and unit-tested against the production pipeline
config, and exposed read-only via `liza schedule`. The supervisor still
self-schedules per-agent (fsnotify wait loops, 8 orchestrator wake triggers,
reviewer-PreWork merges). Rewiring the hot path is the remaining risk and
**cannot be verified by unit tests alone** — it needs real multi-agent runs.

### 3. Dual-write means two sources that can drift
Claims are dual-written with legacy `AssignedTo`/`ReviewingBy`/lease fields.
`statevalidate` warns (not errors) on contradiction. Until legacy fields are
removed (a later strangler phase), a write path that updates one and not the
other silently diverges. The `liza validate` claim-divergence check is the
backstop; keep it green.

### 4. Orchestrator wake detection was NOT consolidated into the scheduler
It lives in `internal/agent/workdetection.go` and is referenced across `agent`,
`commands`, and `prompts`. Moving it is a cross-package change deferred to keep
increments safe. The scheduler `Plan` is therefore incomplete: it omits
orchestrator readiness. Consolidate before the scheduler goes live.

### 5. Write-funnel enforcement is intentionally permissive (for now)
`ModifyOp` fail-closed validation is skipped for generic `modify`/`write` ops
and for projects without a frozen pipeline config, and uses no-worse-than-before
semantics (touching an already-invalid task is allowed). These fail-open paths
should ratchet closed as legacy writers disappear and at-rest corruption is
extinguished — but doing so prematurely will reject legitimate repair operations.

### 6. Verification surface vs. rotation
`liza journal --verify` (task statuses) relies on the rotation snapshot seed.
The broader claim/singleton checks have **no** snapshot seed, so they fold over
`ReadAllIncludingArchives` (full history). If rotation archiving is ever
changed, re-confirm both paths stay equivalent.

## Remaining roadmap (each its own focused effort)

1. **Live scheduler rewiring** — replace per-agent self-scheduling with one
   dispatcher off `scheduler.Compute`; first fold orchestrator wake detection in
   (concern #4). Verify with real agent runs.
2. **Complete event vocabulary → journal as source of truth** — emit lossless
   per-field task events, add full `apply()`, demote `state.yaml` to a rendered
   read-only view. Keep `liza validate` divergence silent throughout. (concern #1)
3. **Ratchet the funnel closed** — drop fail-open paths as legacy writers vanish;
   declare op preconditions; shrink `statevalidate` to a debug backstop. (concern #5)
4. **Remove duplicated ownership/lease fields** once claims are the sole source. (concern #3)
5. **Stage-parameterized lifecycle** — collapse the 26+ status cross-product into
   one lifecycle × stage; cheapest once the journal is authoritative (event
   upcaster maps old status names during replay).

## Invariants the migration must keep

All of `INVARIANTS.md` still holds. The redesign's job is to make invalid states
*unrepresentable*, not to relax any invariant. The most binding ones for this
work: Tier-0 contract rules, doer/reviewer structural separation, commit-SHA
verification, three-phase claim TOCTOU protection, blackboard-as-source-of-truth
(the journal *becomes* that blackboard; it does not bypass it).
