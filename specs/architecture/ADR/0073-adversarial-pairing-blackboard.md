# 73 - Adversarial Pairing Blackboard

## Status

ACCEPTED

## Context

Pairing mode is human-supervised and synchronous. Multi-agent mode is fully blackboard-coordinated. There is a useful middle shape: two or more separate agent terminals acting as doer and reviewer(s) for a pairing-style task, with a human still present, but without booting the full Liza multi-agent machinery.

Those sessions need shared coordination: role registration, current phase, staged review rounds, verdicts, worklog, and terminal stop behavior. Ad hoc chat or external notes are too loose; the full `.liza/state.yaml` blackboard is heavier than needed.

## Decision

Add an `adversarial-pairing` skill built around a Markdown blackboard.

The skill defines:
- doer and reviewer protocols
- frontmatter polling
- self-registration flow
- review-round staging
- reviewer verdicts
- terminal STOPPED behavior
- git-worktree execution requirements
- RCA and red-test gates
- multi-reviewer review phase handling

Blackboard writes use a skill-local helper, `blackboard_write.py`, which applies a sidecar `flock` and SHA-256 compare-and-swap checks. The writer makes worklog updates before status or phase signals so observers do not see an advanced state without the corresponding explanation.

## Consequences

Positive:
- Pairing sessions can split doer/reviewer work across terminals without having to copy/paste between terminal or entering full multi-agent mode.
- No need to commit a spec upfront
- Coordination remains inspectable in one Markdown artifact.
- The locked writer prevents common concurrent-write corruption.
- Review and follow-up rounds have explicit phase semantics.
- The mechanism is skill-local and does not complicate the core Liza state model.

Trade-offs:
- The protocol is manually invoked and depends on agent compliance.
- Provider permissions and sandbox behavior need setup.
- The Markdown blackboard is lighter than full `.liza/state.yaml` validation.
- This creates another collaboration workflow to maintain alongside Pairing and Multi-Agent modes.

## Alternatives Considered

1. Use full Liza multi-agent mode.

Rejected because the need is smaller: coordinated pairing terminals, not a full sprint blackboard with autonomous agent lifecycle.

2. Keep both roles in one terminal.

Rejected because separate terminals let doer and reviewer maintain independent context and scrutiny.

3. Use informal notes.

Rejected because review rounds and status changes need locking, ordering, and a shared protocol.

## Relationship to Prior Decisions

Complements ADR-0004 (Dual-Mode Contract Architecture), ADR-0015 (Subagent Mode First-Class), and ADR-0046 (Review Quorum) without creating another core execution mode. It is a skill-level coordination mechanism.

---
*Reconstructed from commits 00d57671..b3cee812 and follow-up adversarial-pairing docs (2026-05-28 to 2026-05-30)*
