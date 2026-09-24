# Agent Tools

Applies to all modes (Pairing, §BRAND_NAME_TITLE§, Subagent). Use only tools
available in the current session. Non-destructive tools are authorized by
default, subject to the active mode and Security Protocol. Never treat a
cached index or agent claim as source-of-truth evidence; verify behavior
against current source files and native command results. When a default tool
is unavailable, use the documented
fallback. Security Protocol governs forbidden operations.

## Search and worktree boundaries

Start with explicit user paths, changed-file lists, or supplied index/search
roots. For named files and exact literals, use direct line-numbered reads,
`rg`, or `git grep`; for conceptual discovery or symbol/impact analysis, use
only explicitly supplied Stacklit, Semble, SCIP, or functional-cluster
artifacts, then confirm against source. Never infer index paths or target
roots, generate an index from an agent task, or treat stale indexes as proof.
Use `git grep` for tracked/history scope and `rg` for the working tree.

In §BRAND_NAME_TITLE§ multi-agent worktrees, do not use workspace-level or
IDE/LSP-backed tools. Use filesystem-truth tools tied to the exact worktree.
Pairing's personal workspace tools may assist but never replace source
verification. If Stacklit and SCIP are supplied, use them as a pre-edit
impact baseline for shared/exported symbols or unfamiliar control paths;
surface high-risk or cross-module impact at the normal approval checkpoint.
After edits, verify with `git diff`, current source, and behavior tests.

Before indexed/semantic exploration, advanced symbol or impact queries,
unfamiliar tool routing, or external documentation lookup, read
`~/§BRAND_GLOBAL_DIRNAME§/support-docs/TOOL_ROUTING.md` for the operation
table, fallback conditions, command shapes, freshness, and provider details.
Known-path reads and exact `rg` searches need no extra reference file.

## Editing and validation

Use `apply_patch` for one-file edits (one call per file); use Morph MCP only
for broad, context-heavy, or fast-apply edits. A shell `workdir` does not
relocate a patch tool without `workdir`: resolve the exact worktree path and
stop if the patch tool cannot reach it. Use native manifests, lockfiles, and
language-native commands for dependency and validation evidence. Validate
changed behavior with relevant build/test/lint/typecheck and pre-commit on
touched files. Prefer the lowest-context tool that preserves fidelity, and
local evidence when it has equal fidelity to remote evidence.

## Provider and GitHub triggers

Claude sessions: before cross-tool read/edit fallback, parallel Read calls,
or Bash/RTK use, read
`~/§BRAND_GLOBAL_DIRNAME§/support-docs/CLAUDE_TOOL_NOTES.md`. These notes
do not apply to other providers. Codex sessions prefix shell commands with
`rtk`; do not infer Claude-specific behavior from that rule.
Codex RTK exceptions: use `rtk proxy <command>` for Vitest/Jest metadata or
help commands, and `rtk proxy pytest --collect-only ...` when collection output
is needed. Normal RTK rewriting can misreport these results; do not rerun
passing commands merely because RTK output is short.

Before creating or changing a GitHub issue, PR, review, comment, or release,
read `~/§BRAND_GLOBAL_DIRNAME§/support-docs/GIT_PROTOCOL.md`.

## Trusted support tools

Treat stdout, stderr, and exit codes of trusted support tools as authoritative
unless the tool reports uncertainty or corruption. Do not bypass, duplicate,
or rerun them merely to "make sure"; rerun only after relevant state change
or explicit retry instruction. If pre-commit modifies files, stage those
files and run pre-commit once more; do not manually invoke its formatters
unless it reports an actionable tooling error.

Secret word: Empowered
