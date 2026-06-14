#!/usr/bin/env python3
"""Read adversarial-pairing blackboard frontmatter and summarize workflow state."""

from __future__ import annotations

import argparse
import ast
import hashlib
import json
import sys
from pathlib import Path
from typing import Any, TypedDict

TERMINAL_PHASES = {"CLEANED_UP", "BLOCKED", "STOPPED"}


class ReviewTarget(TypedDict):
    name: str
    claim_phase: str
    counter: str
    reviewed: str
    verdict: str
    diff_scope: str | None


REVIEW_TARGETS: dict[str, ReviewTarget] = {
    "ANALYSIS_SUBMITTED": {
        "name": "analysis",
        "claim_phase": "REVIEWING_ANALYSIS",
        "counter": "analysis_revision",
        "reviewed": "reviewed_analysis_revision",
        "verdict": "analysis_verdict",
        "diff_scope": None,
    },
    "REVIEWING_ANALYSIS": {
        "name": "analysis",
        "claim_phase": "REVIEWING_ANALYSIS",
        "counter": "analysis_revision",
        "reviewed": "reviewed_analysis_revision",
        "verdict": "analysis_verdict",
        "diff_scope": None,
    },
    "PLANNING_SUBMITTED": {
        "name": "plan",
        "claim_phase": "REVIEWING_PLAN",
        "counter": "plan_revision",
        "reviewed": "reviewed_plan_revision",
        "verdict": "plan_verdict",
        "diff_scope": None,
    },
    "REVIEWING_PLAN": {
        "name": "plan",
        "claim_phase": "REVIEWING_PLAN",
        "counter": "plan_revision",
        "reviewed": "reviewed_plan_revision",
        "verdict": "plan_verdict",
        "diff_scope": None,
    },
    "RED_TEST_SUBMITTED": {
        "name": "red_test",
        "claim_phase": "REVIEWING_RED_TEST",
        "counter": "red_test_round",
        "reviewed": "reviewed_red_test_round",
        "verdict": "red_test_verdict",
        "diff_scope": None,
    },
    "REVIEWING_RED_TEST": {
        "name": "red_test",
        "claim_phase": "REVIEWING_RED_TEST",
        "counter": "red_test_round",
        "reviewed": "reviewed_red_test_round",
        "verdict": "red_test_verdict",
        "diff_scope": None,
    },
    "CODE_SUBMITTED": {
        "name": "code",
        "claim_phase": "REVIEWING_CODE",
        "counter": "code_review_round",
        "reviewed": "reviewed_code_round",
        "verdict": "code_verdict",
        "diff_scope": "staged",
    },
    "REVIEWING_CODE": {
        "name": "code",
        "claim_phase": "REVIEWING_CODE",
        "counter": "code_review_round",
        "reviewed": "reviewed_code_round",
        "verdict": "code_verdict",
        "diff_scope": "staged",
    },
    "FOLLOWUP_REVIEW": {
        "name": "code",
        "claim_phase": "FOLLOWUP_REVIEW",
        "counter": "code_review_round",
        "reviewed": "reviewed_code_round",
        "verdict": "code_verdict",
        "diff_scope": "unstaged",
    },
}

DOER_WORK_ACTIONS = {
    "ANALYZING": "write or revise root-cause analysis",
    "ANALYSIS_CHANGES_REQUESTED": "revise root-cause analysis",
    "PLANNING": "write or revise the plan",
    "PLAN_CHANGES_REQUESTED": "revise the plan",
    "RED_TESTING": "write or revise the red test or reproduction",
    "RED_TEST_CHANGES_REQUESTED": "revise the red test or reproduction",
    "CODING": "implement from the approved plan in the recorded worktree",
    "CODE_CHANGES_REQUESTED": "address code-review comments as unstaged follow-up changes",
    "READY_TO_COMMIT": "commit, rebase, merge, and clean up after final review approval",
    "COMMITTED": "merge the topic commit into the base branch",
    "MERGED": "clean up the dedicated worktree and merged topic branch",
}


def read_frontmatter(path: Path) -> tuple[str, bytes]:
    data = path.read_bytes()
    lines = data.splitlines(keepends=True)
    if not lines or lines[0].strip() != b"---":
        raise ValueError("blackboard does not start with YAML frontmatter")

    frontmatter_lines: list[bytes] = []
    for line in lines[1:]:
        if line.strip() == b"---":
            return b"".join(frontmatter_lines).decode("utf-8"), data
        frontmatter_lines.append(line)

    raise ValueError("blackboard frontmatter is missing closing ---")


def parse_frontmatter(text: str) -> dict[str, Any]:
    try:
        import yaml  # type: ignore[import-untyped]
    except ImportError:
        return parse_frontmatter_fallback(text)

    loaded = yaml.safe_load(text)
    if not isinstance(loaded, dict):
        raise ValueError("frontmatter must be a YAML mapping")
    return loaded


def parse_frontmatter_fallback(text: str) -> dict[str, Any]:
    logical_lines = [
        (len(line) - len(line.lstrip(" ")), line.strip())
        for line in text.splitlines()
        if line.strip() and not line.lstrip().startswith("#")
    ]
    root: dict[str, Any] = {}
    stack: list[tuple[int, Any]] = [(0, root)]

    for line_index, (indent, stripped) in enumerate(logical_lines):
        if "\t" in stripped:
            raise ValueError("fallback YAML parser does not accept tabs")
        while len(stack) > 1 and indent < stack[-1][0]:
            stack.pop()

        parent = stack[-1][1]
        if stripped.startswith("- "):
            if not isinstance(parent, list):
                raise ValueError(f"list item without list parent: {stripped}")
            parent.append(parse_fallback_list_item(stripped[2:].strip()))
            continue

        key, separator, value = stripped.partition(":")
        if separator == "":
            raise ValueError(f"expected key: value entry, got: {stripped}")
        if not isinstance(parent, dict):
            raise ValueError(f"mapping entry without mapping parent: {stripped}")

        key = key.strip()
        value = value.strip()
        if value:
            parent[key] = parse_scalar(value)
            continue

        next_container: list[Any] | dict[str, Any]
        next_line = logical_lines[line_index + 1] if line_index + 1 < len(logical_lines) else None
        if next_line is not None and next_line[0] > indent and next_line[1].startswith("- "):
            next_container = []
        else:
            next_container = {}
        parent[key] = next_container
        child_indent = next_line[0] if next_line is not None and next_line[0] > indent else indent + 2
        stack.append((child_indent, next_container))

    return root


def parse_fallback_list_item(value: str) -> Any:
    if looks_like_mapping_entry(value):
        raise ValueError(
            "fallback YAML parser supports only scalar list items; mapping-valued list items require PyYAML"
        )
    return parse_scalar(value)


def looks_like_mapping_entry(value: str) -> bool:
    if (value.startswith('"') and value.endswith('"')) or (value.startswith("'") and value.endswith("'")):
        return False
    return value.endswith(":") or ": " in value


def parse_scalar(value: str) -> Any:
    if value in {"null", "Null", "NULL", "~"}:
        return None
    if value in {"true", "True", "TRUE"}:
        return True
    if value in {"false", "False", "FALSE"}:
        return False
    if value.startswith("[") and value.endswith("]"):
        inner = value[1:-1].strip()
        if not inner:
            return []
        return [parse_scalar(part.strip()) for part in inner.split(",")]
    if (value.startswith('"') and value.endswith('"')) or (value.startswith("'") and value.endswith("'")):
        try:
            return ast.literal_eval(value)
        except (SyntaxError, ValueError):
            return value[1:-1]
    try:
        return int(value)
    except ValueError:
        return value


def resolve_invocation(
    role_or_reviewer_id: str | None, role: str | None, agent_id: str | None
) -> tuple[str | None, str | None]:
    if role_or_reviewer_id is None:
        return role, agent_id
    if role_or_reviewer_id == "doer":
        return "doer", agent_id or "doer"
    if role_or_reviewer_id == "reviewer":
        return "reviewer", agent_id
    if role_or_reviewer_id.startswith("reviewer-"):
        return "reviewer", agent_id or role_or_reviewer_id.removeprefix("reviewer-")
    raise ValueError("--role-or-reviewer-id must be doer, reviewer, or reviewer-<id>")


def agent_entry_template(role: str) -> dict[str, Any]:
    return {
        "role": role,
        "status": "IDLE" if role == "reviewer" else "DRAFT",
        "last_seen": None,
        "reviewed_analysis_revision": None,
        "analysis_verdict": None,
        "reviewed_plan_revision": None,
        "plan_verdict": None,
        "reviewed_red_test_round": None,
        "red_test_verdict": None,
        "reviewed_code_round": None,
        "code_verdict": None,
    }


def summarize(path: Path, role: str | None, agent_id: str | None) -> dict[str, Any]:
    if not path.exists() and role == "reviewer":
        return summarize_missing_blackboard(path, role, agent_id)
    frontmatter_text, raw_data = read_frontmatter(path)
    frontmatter = parse_frontmatter(frontmatter_text)

    agents = frontmatter.get("agents") or {}
    if not isinstance(agents, dict):
        raise ValueError("frontmatter agents field must be a mapping")

    required_reviewers = normalize_string_list(frontmatter.get("required_reviewers"))
    worktree = normalize_worktree(frontmatter.get("worktree"))
    worktree_path = normalize_worktree(frontmatter.get("worktree_path"))
    registered_reviewers = sorted(
        agent for agent, entry in agents.items() if isinstance(entry, dict) and entry.get("role") == "reviewer"
    )
    phase = str(frontmatter.get("phase") or "")
    review = summarize_review(phase, frontmatter, agents, required_reviewers)
    agent_summary = summarize_agent(role, agent_id, agents, required_reviewers)
    next_action = decide_next_action(phase, frontmatter, review, agent_summary)
    terminal = phase in TERMINAL_PHASES or is_legacy_committed(frontmatter)

    return {
        "path": str(path),
        "sha256": hashlib.sha256(raw_data).hexdigest(),
        "phase": phase,
        "terminal": terminal,
        "yolo": bool(frontmatter.get("yolo")),
        "work_type": frontmatter.get("work_type"),
        "worktree": worktree,
        "worktree_path": worktree_path,
        "audit": {
            "repo_root": frontmatter.get("repo_root"),
            "base_branch": frontmatter.get("base_branch"),
            "base_sha": frontmatter.get("base_sha"),
            "topic_branch": frontmatter.get("topic_branch"),
            "commit_sha": frontmatter.get("commit_sha"),
            "merged_at": frontmatter.get("merged_at"),
            "merged_into": frontmatter.get("merged_into"),
            "worktree_removed": frontmatter.get("worktree_removed"),
        },
        "counters": {
            "analysis_revision": frontmatter.get("analysis_revision"),
            "plan_revision": frontmatter.get("plan_revision"),
            "red_test_round": frontmatter.get("red_test_round"),
            "code_review_round": frontmatter.get("code_review_round"),
        },
        "required_reviewers": required_reviewers,
        "registered_reviewers": registered_reviewers,
        "missing_required_agent_records": [reviewer for reviewer in required_reviewers if reviewer not in agents],
        "unrequired_registered_reviewers": [
            reviewer for reviewer in registered_reviewers if reviewer not in required_reviewers
        ],
        "working_agents": agent_ids_with_status(agents, {"WORKING", "REVIEWING"}),
        "waiting_agents": agent_ids_with_status(agents, {"WAITING", "IDLE"}),
        "agent": agent_summary,
        "review": review,
        "next": next_action,
    }


def summarize_missing_blackboard(path: Path, role: str | None, agent_id: str | None) -> dict[str, Any]:
    return {
        "path": str(path),
        "sha256": None,
        "phase": "MISSING",
        "terminal": False,
        "yolo": False,
        "work_type": None,
        "worktree": None,
        "worktree_path": None,
        "audit": {},
        "counters": {
            "analysis_revision": None,
            "plan_revision": None,
            "red_test_round": None,
            "code_review_round": None,
        },
        "required_reviewers": [],
        "registered_reviewers": [],
        "missing_required_agent_records": [],
        "unrequired_registered_reviewers": [],
        "working_agents": [],
        "waiting_agents": [agent_id] if agent_id else [],
        "agent": {
            "id": agent_id,
            "role": role,
            "status": "WAITING",
            "exists": False,
            "is_required": False,
            "needs_registration": False,
            "needs_required_registration": False,
            "registration_entry": agent_entry_template("reviewer"),
        },
        "review": None,
        "next": {
            "actor": "reviewer",
            "action": "wait for the doer to create the blackboard, then register",
            "handoff_to": [agent_id or "reviewer"],
        },
    }


def normalize_string_list(value: Any) -> list[str]:
    if value is None:
        return []
    if not isinstance(value, list):
        raise ValueError("required_reviewers must be a list")
    return [str(item) for item in value]


def is_legacy_committed(frontmatter: dict[str, Any]) -> bool:
    if frontmatter.get("phase") != "COMMITTED":
        return False
    lifecycle_fields = {"commit_sha", "merged_at", "merged_into", "worktree_path", "worktree_removed"}
    return lifecycle_fields.isdisjoint(frontmatter.keys())


def normalize_worktree(value: Any) -> str | None:
    if value is None:
        return None
    if not isinstance(value, str) or not value.strip():
        raise ValueError("worktree must be null or an absolute path")
    if not Path(value).is_absolute():
        raise ValueError(f"worktree must be an absolute path when set: {value}")
    return value


def agent_ids_with_status(agents: dict[str, Any], statuses: set[str]) -> list[str]:
    return sorted(
        agent for agent, entry in agents.items() if isinstance(entry, dict) and str(entry.get("status")) in statuses
    )


def summarize_agent(
    role: str | None,
    agent_id: str | None,
    agents: dict[str, Any],
    required_reviewers: list[str],
) -> dict[str, Any] | None:
    if role is None and agent_id is None:
        return None
    if role == "reviewer" and not agent_id:
        return {
            "id": None,
            "role": "reviewer",
            "status": None,
            "exists": False,
            "is_required": False,
            "needs_registration": True,
            "needs_required_registration": False,
            "registration_entry": agent_entry_template("reviewer"),
        }

    resolved_id = agent_id or role
    entry = agents.get(str(resolved_id))
    if isinstance(entry, dict):
        exists = True
        status = entry.get("status")
    else:
        exists = False
        status = None
    is_required = role == "reviewer" and str(resolved_id) in required_reviewers

    return {
        "id": resolved_id,
        "role": role,
        "status": status,
        "exists": exists,
        "is_required": is_required,
        "needs_registration": role == "reviewer" and not exists,
        "needs_required_registration": role == "reviewer" and exists and not is_required,
        "registration_entry": None if exists or role != "reviewer" else agent_entry_template("reviewer"),
    }


def summarize_review(
    phase: str,
    frontmatter: dict[str, Any],
    agents: dict[str, Any],
    required_reviewers: list[str],
) -> dict[str, Any] | None:
    target = REVIEW_TARGETS.get(phase)
    if target is None:
        return None

    current_counter = frontmatter.get(str(target["counter"]))
    reviewed_field = str(target["reviewed"])
    verdict_field = str(target["verdict"])
    missing: list[str] = []
    stale: list[str] = []
    pending: list[str] = []
    approved: list[str] = []
    changes_requested: list[str] = []
    comments: list[str] = []

    for reviewer in required_reviewers:
        entry = agents.get(reviewer)
        if not isinstance(entry, dict):
            missing.append(reviewer)
            pending.append(reviewer)
            continue
        verdict = entry.get(verdict_field)
        reviewed_counter = entry.get(reviewed_field)
        if reviewed_counter != current_counter:
            stale.append(reviewer)
            pending.append(reviewer)
            continue
        if verdict == "APPROVED":
            approved.append(reviewer)
        elif verdict == "CHANGES_REQUESTED":
            changes_requested.append(reviewer)
        elif verdict == "COMMENT":
            comments.append(reviewer)
        else:
            pending.append(reviewer)

    complete = bool(required_reviewers) and not pending
    return {
        "target": target["name"],
        "claim_phase": target["claim_phase"],
        "counter_field": target["counter"],
        "counter": current_counter,
        "reviewed_field": reviewed_field,
        "verdict_field": verdict_field,
        "diff_scope": target["diff_scope"],
        "complete": complete,
        "approved": complete and len(approved) == len(required_reviewers),
        "changes_requested": complete and bool(changes_requested),
        "comment_only": complete and bool(comments) and not changes_requested,
        "pending_reviewers": pending,
        "missing_reviewers": missing,
        "stale_reviewers": stale,
        "approved_reviewers": approved,
        "changes_requested_reviewers": changes_requested,
        "comment_reviewers": comments,
    }


def decide_next_action(
    phase: str,
    frontmatter: dict[str, Any],
    review: dict[str, Any] | None,
    agent: dict[str, Any] | None,
) -> dict[str, Any]:
    if phase in TERMINAL_PHASES:
        return {"actor": "none", "action": "stop polling; phase is terminal", "handoff_to": []}
    if is_legacy_committed(frontmatter):
        return {"actor": "none", "action": "stop polling; legacy COMMITTED phase is terminal", "handoff_to": []}
    if phase == "COMMITTED" and agent is not None and agent.get("role") == "reviewer":
        return {
            "actor": "none",
            "action": "reviewer may stop polling; doer owns merge and cleanup",
            "handoff_to": [],
        }

    if agent is not None and agent.get("needs_registration"):
        return {
            "actor": "reviewer",
            "action": "self-register under agents.<id> and add the same id to required_reviewers in one locked write",
            "handoff_to": [agent.get("id") or "reviewer"],
        }

    if agent is not None and agent.get("needs_required_registration"):
        return {
            "actor": "reviewer",
            "action": "add this reviewer id to required_reviewers in one locked write with status/last_seen update",
            "handoff_to": [agent.get("id")],
        }

    if review is not None:
        if not frontmatter.get("required_reviewers"):
            return {
                "actor": "doer",
                "action": ("wait for reviewer registration or obtain explicit no-review approval before advancing"),
                "handoff_to": ["doer"],
            }
        if review["pending_reviewers"]:
            return {
                "actor": "reviewer",
                "action": f"review current {review['target']} artifact",
                "handoff_to": review["pending_reviewers"],
            }
        if review["approved"]:
            return {
                "actor": "doer",
                "action": f"advance after approved {review['target']} review",
                "handoff_to": ["doer"],
            }
        if review["changes_requested"]:
            return {
                "actor": "doer",
                "action": f"move to changes-requested handling for {review['target']}",
                "handoff_to": ["doer"],
            }
        return {
            "actor": "doer",
            "action": f"resolve COMMENT verdicts for {review['target']}; no approval predicate is satisfied",
            "handoff_to": ["doer"],
        }

    if phase == "DRAFT":
        if frontmatter.get("yolo"):
            action = "begin analysis or planning from the blackboard contents"
        else:
            action = "ask the user before leaving DRAFT for analysis or planning"
        return {"actor": "doer", "action": action, "handoff_to": ["doer"]}

    if phase == "ANALYSIS_APPROVED":
        return {"actor": "doer", "action": "begin planning", "handoff_to": ["doer"]}
    if phase == "PLAN_APPROVED":
        if frontmatter.get("red_test_required"):
            action = "begin red-test work"
        elif frontmatter.get("worktree"):
            action = "begin coding in the recorded worktree"
        else:
            action = "create or select the dedicated worktree before coding"
        return {"actor": "doer", "action": action, "handoff_to": ["doer"]}
    if phase == "RED_TEST_APPROVED":
        action = "begin coding in the recorded worktree" if frontmatter.get("worktree") else "create or select worktree"
        return {"actor": "doer", "action": action, "handoff_to": ["doer"]}
    if phase in DOER_WORK_ACTIONS:
        return {"actor": "doer", "action": DOER_WORK_ACTIONS[phase], "handoff_to": ["doer"]}

    return {"actor": "unknown", "action": f"inspect unsupported or malformed phase {phase}", "handoff_to": []}


def render_text(summary: dict[str, Any]) -> str:
    lines = [
        f"path: {summary['path']}",
        f"sha256: {summary['sha256']}",
        f"phase: {summary['phase']}",
        f"terminal: {str(summary['terminal']).lower()}",
        f"yolo: {str(summary['yolo']).lower()}",
        f"worktree: {summary['worktree']}",
        f"required_reviewers: {', '.join(summary['required_reviewers']) or '(none)'}",
        f"registered_reviewers: {', '.join(summary['registered_reviewers']) or '(none)'}",
    ]
    agent = summary.get("agent")
    if agent:
        lines.append(
            "agent: "
            f"id={agent['id']} role={agent['role']} status={agent['status']} "
            f"exists={str(agent['exists']).lower()} required={str(agent['is_required']).lower()}"
        )
    review = summary.get("review")
    if review:
        lines.append(
            "review: "
            f"target={review['target']} counter={review['counter']} complete={str(review['complete']).lower()} "
            f"pending={','.join(review['pending_reviewers']) or '(none)'} "
            f"approved={','.join(review['approved_reviewers']) or '(none)'} "
            f"changes={','.join(review['changes_requested_reviewers']) or '(none)'}"
        )
    next_action = summary["next"]
    lines.append(
        "next: "
        f"actor={next_action['actor']} handoff_to={','.join(next_action['handoff_to']) or '(none)'} "
        f"action={next_action['action']}"
    )
    return "\n".join(lines) + "\n"


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--path", required=True, type=Path, help="Markdown blackboard path to inspect")
    parser.add_argument(
        "--role-or-reviewer-id",
        help="Invocation role value such as doer, reviewer, reviewer-codex, or reviewer-claude",
    )
    parser.add_argument("--role", choices=["doer", "reviewer"], help="Agent role when not using --role-or-reviewer-id")
    parser.add_argument("--agent-id", help="Stable agent id when role is reviewer without reviewer-<id>")
    parser.add_argument("--json", action="store_true", help="Emit machine-readable JSON")
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        role, agent_id = resolve_invocation(args.role_or_reviewer_id, args.role, args.agent_id)
        summary = summarize(args.path, role, agent_id)
    except Exception as err:
        print(f"error: {err}", file=sys.stderr)
        return 1

    if args.json:
        print(json.dumps(summary, sort_keys=True, default=str))
    else:
        print(render_text(summary), end="")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
