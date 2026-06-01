# Cluster 0073 - Adversarial Pairing Blackboard

## Commit Set
- `00d57671` - feat(skills): add adversarial pairing blackboard
- `b3cee812` - feat(adversarial-pairing): add locked blackboard writer

## Gap Commits
- `35d308ad` - docs(adversarial-pairing): clarify coordination invariants
- `91bb232a` - docs(adversarial-pairing): document codex sandbox workaround
- `bc0fa6c9` - docs(adversarial-pairing): add pre-coding context hygiene
- `ce16c058` - fix(adversarial-pairing): configure claude permission
- `c93fdcaa` - docs(adversarial-pairing): clarify blackboard worktree and review ordering

## Intent Hypothesis
Support separate doer/reviewer pairing terminals with a lightweight shared coordination surface without switching to full multi-agent mode.

## Architectural Signals
- New `adversarial-pairing` skill
- Markdown blackboard with frontmatter polling, role protocols, review-round staging, and terminal STOPPED behavior
- Skill-local `blackboard_write.py` helper with sidecar flock and SHA-256 compare-and-swap checks
- Documentation for lock ordering, worktree requirements, and multi-reviewer phases

## Reconstructed Context
- Trigger: separate doer and reviewer terminals needed a coordination surface without switching the session into full Liza multi-agent mode.
- Alternatives: full multi-agent blackboard, one-terminal pairing, external notes, or ad hoc chat coordination.
- Rationale: a Markdown blackboard keeps coordination inspectable and portable, while a skill-local writer provides enough locking discipline for concurrent terminal use.
- Tradeoffs: the protocol is manually invoked, has provider permission requirements, and is lighter than the full state-machine guarantees of Liza mode.
- Related decisions: complements Pairing mode and Multi-Agent mode without creating a new core mode.

## Candidate Decision Date
2026-05-28

## Status
ADR generated: `specs/architecture/ADR/0073-adversarial-pairing-blackboard.md`

## Confidence
0.90 (high)
