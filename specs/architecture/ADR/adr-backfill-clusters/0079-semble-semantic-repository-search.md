# Cluster 0079 - Semble Semantic Repository Search

## Commit Set
- `8716af67` - feat(semble): surface pairing session context
- `ff8130be` - feat(semble): integrate semble semantic search

## Source Material
- `specs/goals/20260601-integrate-semble.md`

## Intent Hypothesis
Add optional local semantic repository search for conceptual discovery before exact symbols or modules are known.

## Architectural Signals
- New `internal/semble` package
- Semble activation through `LIZA_ENABLE_SEMBLE`
- Offline readiness validation and init-time prewarm
- Pairing SessionStart Semble guidance
- MAS prompt Semble guidance with explicit target roots
- Worktree `.sembleignore` preparation and shared private-exclude handling

## Reconstructed Context
- Trigger: Stacklit and SCIP did not cover natural-language discovery before agents knew the right search term.
- Alternatives: text search only, Morph MCP primary path, Semble MCP, remote Git URL indexing, required Semble dependency.
- Rationale: Semble fills semantic discovery while staying optional, local, offline-gated, and evidence-not-proof.
- Tradeoffs: optional dependency and model/cache readiness complexity; safety checks before root search.

## Candidate Decision Date
2026-06-01

## Status
ADR generated: `specs/architecture/ADR/0079-semble-semantic-repository-search.md`

## Confidence
0.95 (high)
