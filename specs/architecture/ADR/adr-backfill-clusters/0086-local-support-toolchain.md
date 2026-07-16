# Cluster 0086 - Local Support Toolchain

## Commit Set
- 88b7eb35 - feat(toolchain): add local support tool installer

## Reconstructed Context
- Trigger: complex setup was skipped by new users and later complementary tools were easy to miss.
- Alternatives: none formally considered.
- Rationale: profile-based local CLI installation; MCP and provider setup remain user/project specific.
- Tradeoffs: maintain catalog and installers; source fallback requires git and Go.

## Status
ADR generated: specs/architecture/ADR/0086-local-support-toolchain.md
