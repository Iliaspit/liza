#!/bin/bash
# SessionStart hook: provide repo index paths as startup context when this
# Pairing repository is managed by liza-index.

input=$(cat)

if [[ -n "${LIZA_AGENT_ID:-}" ]]; then
  exit 0
fi

json_val() {
  local key="$1" rest value ch escape=0
  local hex

  rest="${input#*\"$key\"}"
  if [[ "$rest" == "$input" ]]; then
    return 0
  fi

  rest="${rest#*:}"
  rest="${rest#"${rest%%[![:space:]]*}"}"
  if [[ "${rest:0:1}" != '"' ]]; then
    return 0
  fi
  rest="${rest:1}"

  value=""
  while [[ -n "$rest" ]]; do
    ch="${rest:0:1}"
    rest="${rest:1}"
    if (( escape )); then
      if [[ "$ch" == "u" && ${#rest} -ge 4 ]]; then
        hex="${rest:0:4}"
        rest="${rest:4}"
        case "$hex" in
          0022) value+='"' ;;
          0026) value+='&' ;;
          003c) value+='<' ;;
          003e) value+='>' ;;
          005c) value+='\\' ;;
          *) value+="u$hex" ;;
        esac
        escape=0
        continue
      fi
      value+="$ch"
      escape=0
      continue
    fi

    case "$ch" in
      \\) escape=1 ;;
      \") printf '%s' "$value"; return 0 ;;
      *) value+="$ch" ;;
    esac
  done
}

quote_for_shell() {
  local value="$1"
  printf "'%s'" "$(printf '%s' "$value" | sed "s/'/'\\\\''/g")"
}

json_escape() {
  local value="$1"
  value=${value//\\/\\\\}
  value=${value//\"/\\\"}
  value=${value//$'\n'/\\n}
  value=${value//$'\r'/\\r}
  value=${value//$'\t'/\\t}
  printf '%s' "$value"
}

repo_liza_index_hook_path() {
  local hook_path

  hook_path=$(git -C "$project_dir" rev-parse --git-path hooks/post-commit 2>/dev/null || true)
  if [[ -n "$hook_path" ]]; then
    case "$hook_path" in
      /*) printf '%s' "$hook_path" ;;
      *) printf '%s/%s' "$project_dir" "$hook_path" ;;
    esac
    return 0
  fi

  printf '%s/.git/hooks/post-commit' "$project_dir"
}

cwd=$(json_val cwd)
project_dir="${CLAUDE_PROJECT_DIR:-}"
if [[ -z "$project_dir" ]]; then
  if [[ -n "$cwd" ]]; then
    project_dir=$(git -C "$cwd" rev-parse --show-toplevel 2>/dev/null || printf '%s' "$cwd")
  else
    project_dir=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
  fi
fi

hook_path=$(repo_liza_index_hook_path)
if [[ ! -f "$hook_path" ]] || ! grep -q 'liza-index' "$hook_path" 2>/dev/null; then
  exit 0
fi

stacklit_path="$project_dir/stacklit.json"
if [[ -f "$stacklit_path" ]]; then
  shell_stacklit_path=$(quote_for_shell "$stacklit_path")
fi

scip_files=()
for scip_path in "$project_dir"/*.scip; do
  [[ -f "$scip_path" ]] || continue
  scip_files+=("$scip_path")
done

if [[ -z "${shell_stacklit_path:-}" && "${#scip_files[@]}" -eq 0 ]]; then
  exit 0
fi

context="Liza repository indexes detected. Pairing mode can use these explicit repo-root index paths. They are refreshed after commits and do not reflect uncommitted changes; verify against source files before editing. "

if [[ -n "${shell_stacklit_path:-}" ]]; then
  context+=" // Stacklit index: $stacklit_path
    //  stacklit derive --ai-summary -i $shell_stacklit_path
    //  stacklit find-module <query> -i $shell_stacklit_path
    //  stacklit get-module <module> -i $shell_stacklit_path
    //  stacklit get-dependencies <module> -i $shell_stacklit_path
    //  stacklit get-hints -i $shell_stacklit_path
    //  stacklit get-hot-files -i $shell_stacklit_path"
fi

if [[ "${#scip_files[@]}" -gt 0 ]]; then
  context+=" // SCIP indexes: "
  for scip_path in "${scip_files[@]}"; do
    language=$(basename "$scip_path" .scip)
    case "$language" in
      go) display_language="Go" ;;
      typescript) display_language="TypeScript" ;;
      python) display_language="Python" ;;
      *) display_language="$language" ;;
    esac
    shell_scip_path=$(quote_for_shell "$scip_path")
    context+=" // $display_language index: $scip_path
      // scip-search symbols --index $shell_scip_path --name Foo --name Bar
      // scip-search references --index $shell_scip_path --symbol '<exact-foo>' --symbol '<exact-bar>' --location-only"
    if [[ "$language" != "python" ]]; then
      context+=" // scip-search implementations --index $shell_scip_path --symbol '<exact-symbol>'"
    fi
  done
fi

context+=" // Orient with Stacklit first, then trace precisely with scip-search."

printf '{"hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":"%s"}}\n' "$(json_escape "$context")"
