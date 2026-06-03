# Cluster 0081 - Indexing Activation for Setup and Init

## Commit Set
- `f10328b6` - feat(indexing): activate optional pairing indexes
- `8da09dc4` - docs(agent-tools): centralize optional index guidance
- `c41d55fc` - fix(pairing-index): restore lifecycle index hook behavior

## Gap Commits
- `7c9b9f29` - docs: link optional indexing tools
- `9a1cbd70` - docs: clarify indexing activation timing

## Source Material
- `specs/goals/20260602-indexing-activation.md`

## Intent Hypothesis
Split optional repository-index activation between global setup guidance and project-local init artifacts.

## Architectural Signals
- `liza setup` installs generic optional-tool routing guidance
- `liza init` installs project-local pairing index hooks and safety artifacts
- Pairing SCIP command planning and override support
- lifecycle hook dispatch through generated index hook scripts
- prompt text trimmed to session-specific metadata plus global routing references

## Reconstructed Context
- Trigger: temporary/manual indexing activation mixed global guidance with repo-specific hook and path state.
- Alternatives: manual setup, repo-specific `AGENT_TOOLS.md`, duplicate prompt guidance, unified Pairing/MAS activation path.
- Rationale: setup owns generic global guidance; init owns repo-specific activation artifacts and prompt/session metadata.
- Tradeoffs: more init and hook-planning complexity; operators must set env gates before setup/init consumes them.

## Candidate Decision Date
2026-06-02

## Status
ADR generated: `specs/architecture/ADR/0081-indexing-activation-for-setup-and-init.md`

## Confidence
0.95 (high)
