# Task Execution Protocol

Read before implementation planning or other state-changing task work, before
using the fast path, and before declaring a changed candidate complete. CORE
and the selected mode annex retain authority over approval, checkpoints,
security, and stopping.

## Readiness and impacts

Confirm understanding and scope; compare 2–3 options for non-trivial work and
scale architectural analysis to complexity. Declare `Doc Impact: none |
[affected docs]` and `Test Impact: none — existing tests cover | [tests to
write/update]` before execution. API/interface changes affect usage docs;
behavior affects specs; new capabilities affect feature docs; config affects
setup docs. Claiming no doc impact requires searching related docs/specs and
checking documented siblings. Claiming no test impact requires existing
coverage of the changed behavior; justify any new behavior without tests.

If clarification reveals scope ambiguity, propose a spec in `specs/` and await
approval (spec → code → docs). In Spike mode, the spec is the work; propose
iterative updates rather than a pre-code gate. Keep one intent per task; propose
splitting feature + refactor.

## Implementation and validation

Trace the touched flow before choosing the smallest *correct* solution. Prefer
no new component when none is needed, then native platform or stdlib, sound
existing code, installed dependencies, and finally minimal custom code. This
is a tie-breaker, not a rigid hierarchy. Never simplify away trust-boundary
checks, data-loss prevention, security, accessibility, requested behavior, or
necessary investigation. Check available libraries before adding a dependency
or writing 30+ lines for a generic need.

Match existing file and directory conventions. Keep refactors separate from
functional changes; one intent per commit. Remove only items made unused by
this change, not pre-existing dead code or other owners' work. A claimed
prerequisite must name what fails without it. Before ≥10 lines of utility-like
code, search for existing patterns, reuse or extract where sound, and propose
a shared location before writing a new utility inline.

For multi-file changes, list all files, make the planned edits, run pre-commit
on all modified files, and fix its issues before tests, DONE, or new work.
Pre-commit also precedes tests for single-file work. Validation must exercise
the changed behavior; record exact commands and outputs and externalize
necessary understanding in docs, specs, or comments.

Before a change, check dependent modules, schema/migration, security,
performance, and retry/idempotency effects. Classify it as Reversible, Costly,
or Irreversible; warn unless Reversible. For trivial/local work, check quickly
and ask if unsure; for medium work, use the full checklist and note unknowns;
for costly/irreversible work, deeply trace and obtain explicit sign-off per
item. If rationale changes during execution, stop at the next safe point,
explain what changed and why, and re-checkpoint if scope or risk changed.
Continue only within approved scope. A violation is not discovery.

Security preflight before execution: confirm no credential file was read
without authorization; no hardcoded secret; external input validation; SQL
and command-injection prevention; safe deserialization; sanitized downstream
output; unchanged auth/authz; known dependency vulnerabilities checked; and
existing security invariants preserved.

## Fast path

Trivial, zero-risk changes may bypass formal DoR/DoD ceremony. Debugging has
its own fast path. Eligible only for a single-file, single-intent change with
an established precedent, no assumptions, and reversibility in under one
minute. Never use this path for control flow, try/except, validation, parsing,
error handling, deletions not explicitly marked as dead code, or an
assumption-dependent change. It still requires the Intent Gate, mode-specific
gate artifact, passing pre-commit, and applicable tests.

## Completion and partial completion

Standard deliverables are code, tests, and docs. A Spike delivers a complete
spec with code scaffolding and relaxed code gates. Research delivers findings,
not code. Re-read the diff for P0–P2 security, correctness, and data-integrity
issues. Every changed line must trace to approved intent, validation, doc
impact, or change-caused cleanup. Ask whether you would approve it and whether
a future reader will understand it; fix concerns before presenting and
escalate P0–P2 issues to the Code Review Protocol.

If incomplete, report `PARTIAL COMPLETION: N/M`, completed items, and each
remainder's issue/status: Blocked (dependency, missing info, tool failure),
Descoped (narrowed scope), or Deferred by choice (explicit rationale). Deferral
triggers CORE Rule 7's Post-Hoc Discovery Protocol.

When deferring, making trade-offs, or accepting concerns, record deliberate
debt in `TECH_DEBT.md` with what, why, and a payback trigger; without a
trigger it is not debt. For a small local simplification without project-level
debt, state its ceiling and upgrade trigger inline.
