#!/usr/bin/env bash
# trovex-md-read-guard — Claude Code PreToolUse hook (read side).
#
# Nudges agents to retrieve docs via trovex MCP tools (trovex_search,
# trovex_read) instead of reading .md files raw off disk — reading raw means
# stale content and no chunk-level token savings.
# .trovexignore exempts files that should be read raw (README, CLAUDE.md, etc.).
# Degrades to ALLOW when the trovex server is unreachable or jq is missing.
set -euo pipefail

TROVEX_URL="${TROVEX_URL:-http://localhost:8765}"

allow() { exit 0; }

command -v jq >/dev/null 2>&1 || allow

input="$(cat)"
tool="$(printf '%s' "$input" | jq -r '.tool_name // empty')"
case "$tool" in Read) ;; *) allow ;; esac

file="$(printf '%s' "$input" | jq -r '.tool_input.file_path // empty')"
[ -n "$file" ] || allow
case "$file" in *.md | *.mdx | *.markdown) ;; *) allow ;; esac

# .trovexignore — exempt paths are read raw off disk.
root="$(git -C "$(dirname "$file")" rev-parse --show-toplevel 2>/dev/null || pwd)"
ignore="$root/.trovexignore"
if [ -f "$ignore" ]; then
  rel="${file#"$root"/}"
  while IFS= read -r pat || [ -n "$pat" ]; do
    [ -z "$pat" ] && continue
    case "$pat" in \#*) continue ;; esac
    # shellcheck disable=SC2254
    case "$rel" in $pat) allow ;; esac
    # shellcheck disable=SC2254
    case "$file" in $pat) allow ;; esac
  done <"$ignore"
fi

# Graceful degradation: trovex down → don't block reads.
curl -fsS -m 2 "$TROVEX_URL/healthz" >/dev/null 2>&1 || allow

reason="trovex centralizes docs — use trovex MCP tools (trovex_search or trovex_read) instead of reading '$file' raw. To read this file directly, add its path to .trovexignore."

jq -cn --arg r "$reason" '{
  hookSpecificOutput: {
    hookEventName: "PreToolUse",
    permissionDecision: "deny",
    permissionDecisionReason: $r
  }
}'
exit 0
