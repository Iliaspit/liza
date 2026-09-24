# Claude Tool Notes

Claude sessions read this before switching read/edit tool families, parallel
Read calls, or Bash/RTK use. These rules do not apply to other providers.

When Claude reads a file with one tool family and then edits it with another,
tool/model state can drift. If an MCP edit tool is unavailable and native
editing is used, re-read the file with the native tool family immediately
before editing.

Parallel Read calls fail as a group if any one errors. Before fanning out,
use Glob to check existence first; then read only files that exist. Do not
combine the existence check and reads in one batch.

RTK (Rust Token Killer) is a trusted token-optimized CLI proxy. Short output
is not weaker evidence: content is complete and exit codes are unaltered.
Do not bypass RTK to get "full" output, invoke `rtk proxy` except for the
specific workarounds below, read RTK tee files at
`~/.local/share/rtk/tee/*.log`, or rerun a passing command because its output
looked short. A Claude PreToolUse hook rewrites most Bash commands to
`rtk <command>` transparently.

Until rtk-ai/rtk#1922 and rtk-ai/rtk#925 merge:
- Do not run Vitest/Jest metadata or non-run commands through RTK rewrite,
  such as `npx vitest --version`, `vitest --help`, or `rtk vitest --version`.
  Prefer package scripts or `npm exec -- vitest run ...`,
  `pnpm exec -- vitest run ...`, or `./node_modules/.bin/vitest run ...`.
  For metadata/help, use the narrow exception `rtk proxy <command>`.
- Avoid `rtk pytest --collect-only` and rewritten `pytest --collect-only`;
  they can report collected tests as "No tests collected". When collection
  output is needed, use `rtk proxy pytest --collect-only ...`.
