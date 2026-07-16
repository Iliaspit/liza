# Cluster 0088 - Aggregate SCIP Indexes Across Language Roots

## Commit Set
- c950158b - feat(scip): aggregate multi-root indexes

## Reconstructed Context
- Trigger: complex multi-root repositories need indexing most; the prior behavior produced no index.
- Alternatives: none formally considered.
- Rationale: generate per-root indexes and aggregate them into a repository-relative result.
- Tradeoffs: per-root generation and aggregation complexity.

## Status
ADR generated: specs/architecture/ADR/0088-aggregate-scip-indexes-across-language-roots.md
