# Cluster 0096 - Catalog-Backed Provider Registry

## Commit Set
- 61adbb10 - feat(providers): add catalog-backed provider registry

## Reconstructed Context
- Trigger: hardcoded provider paths created lifecycle drift and complexity.
- Alternatives: none formally considered beyond the hardcoded baseline.
- Rationale: declarative provider metadata reduces cross-cutting provider additions.
- Tradeoffs: schema, cache, and catalog correctness maintenance.

## Status
ADR generated: specs/architecture/ADR/0096-catalog-backed-provider-registry.md
