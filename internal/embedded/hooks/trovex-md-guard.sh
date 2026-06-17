#!/usr/bin/env bash
# trovex-md-guard — Claude Code PreToolUse hook (write side).
#
# Routes Markdown writes through trovex MCP tools instead of the local disk so
# every agent shares one indexed source of truth. Blocks Write/Edit/MultiEdit to
# a *.md file unless its path is listed in .trovexignore, and tells the agent to
# use the trovex_write MCP tool instead.
#
# Degrades to ALLOW when the trovex server is unreachable or jq is missing — a
# trovex outage must never brick the agent.
set -euo pipefail

TROVEX_URL="${TROVEX_URL:-http://localhost:8765}"

allow() { exit 0; }

command -v jq >/dev/null 2>&1 || allow

input="$(cat)"
tool="$(printf '%s' "$input" | jq -r '.tool_name // empty')"
case "$tool" in Write | Edit | MultiEdit) ;; *) allow ;; esac

file="$(printf '%s' "$input" | jq -r '.tool_input.file_path // empty')"
[ -n "$file" ] || allow
case "$file" in *.md | *.mdx | *.markdown) ;; *) allow ;; esac

# .trovexignore — exempt paths stay as real files on disk.
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

# Graceful degradation: trovex down → don't block.
curl -fsS -m 2 "$TROVEX_URL/healthz" >/dev/null 2>&1 || allow

reason="trovex centralizes docs — use the trovex_write MCP tool instead of writing '$file' to disk. To keep this file on disk, add its path to .trovexignore."

jq -cn --arg r "$reason" '{
  hookSpecificOutput: {
    hookEventName: "PreToolUse",
    permissionDecision: "deny",
    permissionDecisionReason: $r
  }
}'
exit 0
