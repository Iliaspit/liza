# ADR-0094: Entry-Point Input-Readiness Assessment

## Status

ACCEPTED

## Context

Starting a multi-agent run from an input at the wrong level of detail can force
agents to guess and produce poor-quality output.

## Decision

Add a read-only assessment that checks whether an input document is ready for
the selected general-objective, functional-spec, or technical-spec entry point.
It classifies entry-point fit, reports evidence-based blockers, and asks for
specific missing decisions without rewriting the source document.

## Consequences

- Liza can prevent low-quality runs caused by insufficient or misaligned input.
- The assessment works as a review before initialization.
- The workflow gains a maintained readiness rubric and can delay execution
  until critical inputs are resolved.

## Alternatives Considered

No formal alternatives were considered.

---

Reconstructed from commit 18046a64 (2026-07-05) and
skills/check-liza-input-readiness/SKILL.md. User context confirmed 2026-07-16.
