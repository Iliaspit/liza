# Cluster 0068 - Optional Repository Indexing with SCIP and Stacklit

## Commit Set
- `2e626ba1` - feat: add scip-search support (#80)
- `f28de468` - fix(scipsearch): infer TypeScript index roots
- `a16a2a5a` - fix(scipsearch): infer Python index roots
- `8d50b5bd` - fix(scipsearch): remove --skip-tests option for scip-go
- `e4848776` - fix(init): skip scip-search validation when disabled
- `68e693b2` - feat(stacklit): add optional Stacklit indexing
- `6f1d3b42` - feat(stacklit): allow ignored runtime indexes
- `ba003785` - chore(stacklit): include ai summary in derive hints

## Gap Commits
- `08b5189b` - docs(prompts): unify index query routing
- `e4ef50ca` - docs(indexing): clarify SCIP setup guidance

## Intent Hypothesis
Make repository indexes prompt-safe, opt-in navigation aids for MAS worktrees and Pairing startup context.

## Architectural Signals
- New `internal/scipsearch` package
- New `internal/ops/scip_indexing.go`
- `LIZA_ENABLE_SCIP_SEARCH` plus `config.scip_search` activation contract
- Generated SCIP indexes under `.liza/scip/`
- Worktree creation, submit-for-review, reviewer recovery, direct worktree creation, and orchestrator wake refresh indexes
- Prompt builder injects explicit `scip-search --index` paths and language-specific capabilities
- New `internal/stacklit` package
- New `internal/ops/stacklit_indexing.go`
- `LIZA_ENABLE_STACKLIT` activation gate
- Prompt builder injects explicit Stacklit index paths
- Task worktree generation requires `stacklit.json` to be tracked or ignored
- `stacklit derive --ai-summary` becomes the expected first-pass orientation command

## Reconstructed Context
- Trigger: MAS agents need low-token repository orientation and precise structural navigation before source reads, especially in isolated task worktrees.
- Alternatives: rely on plain `rg`/file reads, use workspace-level tools, use SCIP only, or use Stacklit only.
- Rationale: SCIP provides precise symbol/reference/package navigation, while Stacklit provides compact repository orientation; explicit index paths keep agents anchored to filesystem-truth artifacts.
- Tradeoffs: external indexing tools remain optional dependencies, generated indexes can become stale, and task worktrees need cleanliness safeguards.
- Related decisions: extends prompt context and worktree isolation decisions by adding bounded repository-index inputs.

## Candidate Decision Date
2026-05-21

## Status
ADR generated: `specs/architecture/ADR/0068-optional-repository-indexing-with-scip-and-stacklit.md`

## Confidence
0.85 (high)
