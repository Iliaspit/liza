#!/usr/bin/env python3
"""Semantic operations for adversarial-pairing blackboards.

The raw writer replaces a complete Markdown file. This helper owns common
field-scoped workflow mutations under the lock so agents do not hand-author
full blackboards for routine state changes.
"""

from __future__ import annotations

import argparse
import json
import sys
import time
from collections.abc import Callable
from pathlib import Path
from typing import Any

import yaml  # type: ignore[import-untyped]
from blackboard_state import agent_entry_template, parse_frontmatter, summarize
from blackboard_write import acquire_flock, atomic_replace, sha256_bytes, write_owner_metadata

WorkflowMutator = Callable[[dict[str, Any], str], tuple[dict[str, Any], str]]


REVIEW_FIELDS = {
    "analysis": ("analysis_revision", "reviewed_analysis_revision", "analysis_verdict", "Root Cause Analysis"),
    "plan": ("plan_revision", "reviewed_plan_revision", "plan_verdict", "Plan Reviews"),
    "red-test": ("red_test_round", "reviewed_red_test_round", "red_test_verdict", "Red Tests"),
    "code": ("code_review_round", "reviewed_code_round", "code_verdict", "Code Review Rounds"),
}


ARTIFACT_PHASES = {
    "analysis": ("analysis_revision", "ANALYSIS_SUBMITTED", "Root Cause Analysis"),
    "plan": ("plan_revision", "PLANNING_SUBMITTED", "Plan Revisions"),
    "red-test": ("red_test_round", "RED_TEST_SUBMITTED", "Red Tests"),
    "code": ("code_review_round", "CODE_SUBMITTED", "Code Review Rounds"),
}


REVIEW_PHASES = {
    "analysis": {"ANALYSIS_SUBMITTED", "REVIEWING_ANALYSIS"},
    "plan": {"PLANNING_SUBMITTED", "REVIEWING_PLAN"},
    "red-test": {"RED_TEST_SUBMITTED", "REVIEWING_RED_TEST"},
    "code": {"CODE_SUBMITTED", "REVIEWING_CODE", "FOLLOWUP_REVIEW"},
}


SUBMIT_PHASES = {
    "analysis": {"DRAFT", "ANALYZING", "ANALYSIS_CHANGES_REQUESTED"},
    "plan": {"DRAFT", "PLANNING", "PLAN_CHANGES_REQUESTED", "ANALYSIS_APPROVED"},
    "red-test": {"PLAN_APPROVED", "RED_TESTING", "RED_TEST_CHANGES_REQUESTED"},
    "code": {"CODING"},
}


PLAN_WITH_RCA_SUBMIT_PHASES = {"ANALYSIS_APPROVED", "PLANNING", "PLAN_CHANGES_REQUESTED"}
FOLLOWUP_REVIEW_PHASES = {"CODE_CHANGES_REQUESTED", "CODE_SUBMITTED", "REVIEWING_CODE", "FOLLOWUP_REVIEW"}
READY_TO_COMMIT_PHASES = {"CODE_SUBMITTED", "REVIEWING_CODE", "FOLLOWUP_REVIEW"}


APPROVED_PHASES = {
    "analysis": "ANALYSIS_APPROVED",
    "plan": "PLAN_APPROVED",
    "red-test": "RED_TEST_APPROVED",
}


CHANGES_REQUESTED_PHASES = {
    "analysis": "ANALYSIS_CHANGES_REQUESTED",
    "plan": "PLAN_CHANGES_REQUESTED",
    "red-test": "RED_TEST_CHANGES_REQUESTED",
    "code": "CODE_CHANGES_REQUESTED",
}


def utc_now() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def split_document(data: bytes) -> tuple[dict[str, Any], str]:
    text = data.decode("utf-8")
    if not text.startswith("---\n"):
        raise ValueError("blackboard does not start with YAML frontmatter")
    end = text.find("\n---\n", 4)
    if end == -1:
        raise ValueError("blackboard frontmatter is missing closing ---")
    frontmatter = parse_frontmatter(text[4:end])
    body = text[end + len("\n---\n") :]
    return frontmatter, body


def render_document(frontmatter: dict[str, Any], body: str) -> bytes:
    dumped = yaml.safe_dump(frontmatter, sort_keys=False, default_flow_style=False)
    return f"---\n{dumped}---\n{body}".encode()


def append_to_section(body: str, section: str, text: str) -> str:
    normalized = text.rstrip() + "\n"
    heading = f"## {section}"
    start = body.find(heading)
    if start == -1:
        suffix = "" if body.endswith("\n") else "\n"
        return f"{body}{suffix}\n{heading}\n\n{normalized}"

    insert_at = body.find("\n## ", start + len(heading))
    if insert_at == -1:
        insert_at = len(body)
    before = body[:insert_at].rstrip()
    after = body[insert_at:]
    return f"{before}\n\n{normalized}{after}"


def ensure_agent(frontmatter: dict[str, Any], agent_id: str, role: str) -> dict[str, Any]:
    agents = frontmatter.setdefault("agents", {})
    if not isinstance(agents, dict):
        raise ValueError("frontmatter agents field must be a mapping")
    entry = agents.get(agent_id)
    if entry is None:
        entry = agent_entry_template(role)
        agents[agent_id] = entry
    if not isinstance(entry, dict):
        raise ValueError(f"agent entry is not a mapping: {agent_id}")
    if entry.get("role") != role:
        raise ValueError(f"agent {agent_id} has role {entry.get('role')}, not {role}")
    return entry


def string_list(frontmatter: dict[str, Any], field: str) -> list[str]:
    current = frontmatter.setdefault(field, [])
    if current is None:
        current = []
        frontmatter[field] = current
    if not isinstance(current, list):
        raise ValueError(f"{field} must be a list")
    return [str(item) for item in current]


def add_unique(frontmatter: dict[str, Any], field: str, value: str) -> None:
    values = string_list(frontmatter, field)
    if value not in values:
        values.append(value)
    frontmatter[field] = values


def set_agent_state(entry: dict[str, Any], status: str) -> None:
    entry["status"] = status
    entry["last_seen"] = utc_now()


def set_phase(frontmatter: dict[str, Any], phase: str) -> None:
    frontmatter["phase"] = phase
    frontmatter["phase_updated_at"] = utc_now()


def phase(frontmatter: dict[str, Any]) -> str:
    return str(frontmatter.get("phase") or "")


def require_phase(frontmatter: dict[str, Any], operation: str, allowed: set[str]) -> None:
    current = phase(frontmatter)
    if current not in allowed:
        expected = ", ".join(sorted(allowed))
        raise ValueError(f"{operation} requires phase in {{{expected}}}; current phase is {current}")


def review_is_approved(frontmatter: dict[str, Any], target: str) -> bool:
    counter, reviewed_field, verdict_field, _section = REVIEW_FIELDS[target]
    current_counter = frontmatter.get(counter)
    required_reviewers = string_list(frontmatter, "required_reviewers")
    if not required_reviewers:
        return False
    agents = frontmatter.get("agents") or {}
    if not isinstance(agents, dict):
        raise ValueError("frontmatter agents field must be a mapping")
    for reviewer in required_reviewers:
        entry = agents.get(reviewer)
        if not isinstance(entry, dict):
            return False
        if entry.get(reviewed_field) != current_counter or entry.get(verdict_field) != "APPROVED":
            return False
    return True


def review_has_changes_requested(frontmatter: dict[str, Any], target: str) -> bool:
    counter, reviewed_field, verdict_field, _section = REVIEW_FIELDS[target]
    current_counter = frontmatter.get(counter)
    required_reviewers = string_list(frontmatter, "required_reviewers")
    if not required_reviewers:
        return False
    agents = frontmatter.get("agents") or {}
    if not isinstance(agents, dict):
        raise ValueError("frontmatter agents field must be a mapping")
    saw_changes_requested = False
    for reviewer in required_reviewers:
        entry = agents.get(reviewer)
        if not isinstance(entry, dict):
            return False
        if entry.get(reviewed_field) != current_counter:
            return False
        verdict = entry.get(verdict_field)
        if verdict == "CHANGES_REQUESTED":
            saw_changes_requested = True
        elif verdict not in {"APPROVED", "COMMENT"}:
            return False
    return saw_changes_requested


def review_is_complete(frontmatter: dict[str, Any], target: str) -> bool:
    counter, reviewed_field, verdict_field, _section = REVIEW_FIELDS[target]
    current_counter = frontmatter.get(counter)
    required_reviewers = string_list(frontmatter, "required_reviewers")
    if not required_reviewers:
        return False
    agents = frontmatter.get("agents") or {}
    if not isinstance(agents, dict):
        raise ValueError("frontmatter agents field must be a mapping")
    for reviewer in required_reviewers:
        entry = agents.get(reviewer)
        if not isinstance(entry, dict):
            return False
        if entry.get(reviewed_field) != current_counter:
            return False
        if entry.get(verdict_field) not in {"APPROVED", "CHANGES_REQUESTED", "COMMENT"}:
            return False
    return True


def require_review_approved(frontmatter: dict[str, Any], target: str, operation: str) -> None:
    if not review_is_approved(frontmatter, target):
        raise ValueError(f"{operation} requires approved current {target} review")


def require_submit_allowed(frontmatter: dict[str, Any], target: str) -> None:
    if target == "plan" and bool(frontmatter.get("rca_required")):
        require_phase(frontmatter, "submit-plan", PLAN_WITH_RCA_SUBMIT_PHASES)
        require_review_approved(frontmatter, "analysis", "submit-plan")
        return
    if target == "red-test":
        require_phase(frontmatter, "submit-red-test", SUBMIT_PHASES[target])
        require_review_approved(frontmatter, "plan", "submit-red-test")
        return
    require_phase(frontmatter, f"submit-{target}", SUBMIT_PHASES[target])


def require_active_review_target(frontmatter: dict[str, Any], target: str) -> None:
    current = phase(frontmatter)
    if current not in REVIEW_PHASES[target]:
        raise ValueError(f"phase {current} is not reviewable for {target} verdict")


def read_text_file(path: Path) -> str:
    return path.read_text(encoding="utf-8").rstrip() + "\n"


def mutate_existing(path: Path, operation: str, mutator: WorkflowMutator, timeout: float) -> dict[str, Any]:
    lock_path = Path(f"{path}.lock")
    owner_path = Path(f"{path}.lock.owner.json")
    if not path.parent.exists():
        raise FileNotFoundError(f"parent directory does not exist: {path.parent}")

    with lock_path.open("a+b") as lock_file:
        acquire_flock(lock_file, timeout)
        write_owner_metadata(owner_path, operation)
        current_data = path.read_bytes()
        frontmatter, body = split_document(current_data)
        new_frontmatter, new_body = mutator(frontmatter, body)
        new_data = render_document(new_frontmatter, new_body)
        split_document(new_data)
        atomic_replace(path, new_data)

    return summarize(path, None, None) | {
        "old_sha256": sha256_bytes(current_data),
        "new_sha256": sha256_bytes(new_data),
        "operation": operation,
    }


def create_blackboard(args: argparse.Namespace) -> dict[str, Any]:
    path = args.path
    if not path.parent.exists():
        raise FileNotFoundError(f"parent directory does not exist: {path.parent}")
    if path.exists():
        raise FileExistsError(f"blackboard already exists: {path}")

    goal = read_text_file(args.goal_file) if args.goal_file else (args.goal or "").rstrip() + "\n"
    frontmatter: dict[str, Any] = {
        "phase": "DRAFT",
        "yolo": bool(args.yolo),
        "work_type": args.work_type,
        "rca_required": bool(args.rca_required),
        "red_test_required": bool(args.red_test_required),
        "required_reviewers": [],
        "plan_revision": 0,
        "analysis_revision": 0,
        "red_test_round": 0,
        "code_review_round": 0,
        "phase_updated_at": utc_now(),
        "repo_root": None,
        "base_branch": None,
        "base_sha": None,
        "topic_branch": None,
        "commit_sha": None,
        "merged_at": None,
        "merged_into": None,
        "worktree": None,
        "worktree_path": None,
        "worktree_removed": False,
        "agents": {"doer": agent_entry_template("doer")},
    }
    body = (
        "# Adversarial Pairing Blackboard\n\n"
        "## Goal\n\n"
        f"{goal}\n"
        "## Evidence\n\n"
        "## Plan Revisions\n\n"
        "## Plan Reviews\n\n"
        "## Implementation Notes\n\n"
        "## Code Review Rounds\n\n"
        "## Validation\n\n"
        "## Decisions\n"
    )
    data = render_document(frontmatter, body)
    lock_path = Path(f"{path}.lock")
    owner_path = Path(f"{path}.lock.owner.json")
    with lock_path.open("a+b") as lock_file:
        acquire_flock(lock_file, args.timeout)
        write_owner_metadata(owner_path, "create")
        if path.exists():
            raise FileExistsError(f"blackboard already exists: {path}")
        atomic_replace(path, data)
    return summarize(path, None, None) | {"old_sha256": "", "new_sha256": sha256_bytes(data), "operation": "create"}


def op_register_reviewer(args: argparse.Namespace) -> dict[str, Any]:
    def mutator(frontmatter: dict[str, Any], body: str) -> tuple[dict[str, Any], str]:
        entry = ensure_agent(frontmatter, args.agent_id, "reviewer")
        set_agent_state(entry, "IDLE")
        add_unique(frontmatter, "required_reviewers", args.agent_id)
        return frontmatter, body

    return mutate_existing(args.path, "register-reviewer", mutator, args.timeout)


def op_claim_review(args: argparse.Namespace) -> dict[str, Any]:
    claim_phases = {
        "ANALYSIS_SUBMITTED": "REVIEWING_ANALYSIS",
        "PLANNING_SUBMITTED": "REVIEWING_PLAN",
        "RED_TEST_SUBMITTED": "REVIEWING_RED_TEST",
        "CODE_SUBMITTED": "REVIEWING_CODE",
    }

    def mutator(frontmatter: dict[str, Any], body: str) -> tuple[dict[str, Any], str]:
        phase = str(frontmatter.get("phase") or "")
        if phase in claim_phases:
            set_phase(frontmatter, claim_phases[phase])
        elif phase not in set(claim_phases.values()) | {"FOLLOWUP_REVIEW"}:
            raise ValueError(f"phase is not reviewable: {phase}")
        entry = ensure_agent(frontmatter, args.agent_id, "reviewer")
        set_agent_state(entry, "REVIEWING")
        return frontmatter, body

    return mutate_existing(args.path, "claim-review", mutator, args.timeout)


def op_submit_artifact(args: argparse.Namespace) -> dict[str, Any]:
    counter, phase, section = ARTIFACT_PHASES[args.target]
    note = read_text_file(args.body_file)

    def mutator(frontmatter: dict[str, Any], body: str) -> tuple[dict[str, Any], str]:
        require_submit_allowed(frontmatter, args.target)
        frontmatter[counter] = int(frontmatter.get(counter) or 0) + 1
        set_phase(frontmatter, phase)
        doer = ensure_agent(frontmatter, "doer", "doer")
        set_agent_state(doer, "WAITING")
        return frontmatter, append_to_section(body, section, note)

    return mutate_existing(args.path, f"submit-{args.target}", mutator, args.timeout)


def op_submit_followup_review(args: argparse.Namespace) -> dict[str, Any]:
    note = read_text_file(args.body_file)

    def mutator(frontmatter: dict[str, Any], body: str) -> tuple[dict[str, Any], str]:
        require_phase(frontmatter, "submit-followup-review", FOLLOWUP_REVIEW_PHASES)
        if not review_is_complete(frontmatter, "code"):
            raise ValueError("submit-followup-review requires completed current code review")
        frontmatter["code_review_round"] = int(frontmatter.get("code_review_round") or 0) + 1
        set_phase(frontmatter, "FOLLOWUP_REVIEW")
        doer = ensure_agent(frontmatter, "doer", "doer")
        set_agent_state(doer, "WAITING")
        return frontmatter, append_to_section(body, "Code Review Rounds", note)

    return mutate_existing(args.path, "submit-followup-review", mutator, args.timeout)


def apply_verdict(args: argparse.Namespace, operation: str) -> dict[str, Any]:
    counter, reviewed_field, verdict_field, section = REVIEW_FIELDS[args.target]
    note = read_text_file(args.note_file)

    def mutator(frontmatter: dict[str, Any], body: str) -> tuple[dict[str, Any], str]:
        require_active_review_target(frontmatter, args.target)
        current_counter = frontmatter.get(counter)
        if int(args.counter) != int(current_counter or 0):
            raise ValueError(f"{counter} is {current_counter}, not {args.counter}")
        if args.agent_id not in string_list(frontmatter, "required_reviewers"):
            raise ValueError(f"reviewer {args.agent_id} is not required for this gate")
        entry = ensure_agent(frontmatter, args.agent_id, "reviewer")
        entry[reviewed_field] = int(args.counter)
        entry[verdict_field] = args.verdict
        status = {"APPROVED": "APPROVED", "CHANGES_REQUESTED": "CHANGES_REQUESTED", "COMMENT": "WAITING"}[args.verdict]
        set_agent_state(entry, status)
        if review_has_changes_requested(frontmatter, args.target):
            set_phase(frontmatter, CHANGES_REQUESTED_PHASES[args.target])
        elif args.target in APPROVED_PHASES and review_is_approved(frontmatter, args.target):
            set_phase(frontmatter, APPROVED_PHASES[args.target])
        return frontmatter, append_to_section(body, section, note)

    return mutate_existing(args.path, operation, mutator, args.timeout)


def op_submit_verdict(args: argparse.Namespace) -> dict[str, Any]:
    return apply_verdict(args, "submit-verdict")


def op_request_changes(args: argparse.Namespace) -> dict[str, Any]:
    args.verdict = "CHANGES_REQUESTED"
    return apply_verdict(args, "request-changes")


def op_enter_coding(args: argparse.Namespace) -> dict[str, Any]:
    worktree = str(args.worktree.resolve())

    def mutator(frontmatter: dict[str, Any], body: str) -> tuple[dict[str, Any], str]:
        if bool(frontmatter.get("red_test_required")):
            require_phase(frontmatter, "enter-coding", {"RED_TEST_APPROVED"})
            require_review_approved(frontmatter, "red-test", "enter-coding")
        else:
            require_phase(frontmatter, "enter-coding", {"PLAN_APPROVED"})
            require_review_approved(frontmatter, "plan", "enter-coding")
        set_phase(frontmatter, "CODING")
        frontmatter["repo_root"] = str(args.repo_root.resolve()) if args.repo_root else frontmatter.get("repo_root")
        frontmatter["base_branch"] = args.base_branch or frontmatter.get("base_branch")
        frontmatter["base_sha"] = args.base_sha or frontmatter.get("base_sha")
        frontmatter["topic_branch"] = args.topic_branch or frontmatter.get("topic_branch")
        frontmatter["worktree"] = worktree
        frontmatter["worktree_path"] = worktree
        frontmatter["worktree_removed"] = False
        doer = ensure_agent(frontmatter, "doer", "doer")
        set_agent_state(doer, "WORKING")
        return frontmatter, body

    return mutate_existing(args.path, "enter-coding", mutator, args.timeout)


def op_ready_to_commit(args: argparse.Namespace) -> dict[str, Any]:
    note = read_text_file(args.note_file) if args.note_file else None

    def mutator(frontmatter: dict[str, Any], body: str) -> tuple[dict[str, Any], str]:
        require_phase(frontmatter, "ready-to-commit", READY_TO_COMMIT_PHASES)
        require_review_approved(frontmatter, "code", "ready-to-commit")
        set_phase(frontmatter, "READY_TO_COMMIT")
        doer = ensure_agent(frontmatter, "doer", "doer")
        set_agent_state(doer, "WORKING")
        if note:
            body = append_to_section(body, "Decisions", note)
        return frontmatter, body

    return mutate_existing(args.path, "ready-to-commit", mutator, args.timeout)


def op_mark_committed(args: argparse.Namespace) -> dict[str, Any]:
    note = read_text_file(args.note_file) if args.note_file else None

    def mutator(frontmatter: dict[str, Any], body: str) -> tuple[dict[str, Any], str]:
        require_phase(frontmatter, "mark-committed", {"READY_TO_COMMIT"})
        set_phase(frontmatter, "COMMITTED")
        frontmatter["commit_sha"] = args.commit_sha
        if args.topic_branch:
            frontmatter["topic_branch"] = args.topic_branch
        doer = ensure_agent(frontmatter, "doer", "doer")
        set_agent_state(doer, "WORKING")
        if note:
            body = append_to_section(body, "Decisions", note)
        return frontmatter, body

    return mutate_existing(args.path, "mark-committed", mutator, args.timeout)


def op_mark_merged(args: argparse.Namespace) -> dict[str, Any]:
    note = read_text_file(args.note_file) if args.note_file else None

    def mutator(frontmatter: dict[str, Any], body: str) -> tuple[dict[str, Any], str]:
        require_phase(frontmatter, "mark-merged", {"COMMITTED"})
        if not frontmatter.get("commit_sha"):
            raise ValueError("mark-merged requires commit_sha to be recorded first")
        set_phase(frontmatter, "MERGED")
        frontmatter["merged_at"] = args.merged_at or utc_now()
        frontmatter["merged_into"] = args.merged_into
        doer = ensure_agent(frontmatter, "doer", "doer")
        set_agent_state(doer, "WORKING")
        if note:
            body = append_to_section(body, "Decisions", note)
        return frontmatter, body

    return mutate_existing(args.path, "mark-merged", mutator, args.timeout)


def op_mark_cleaned_up(args: argparse.Namespace) -> dict[str, Any]:
    note = read_text_file(args.note_file) if args.note_file else None

    def mutator(frontmatter: dict[str, Any], body: str) -> tuple[dict[str, Any], str]:
        require_phase(frontmatter, "mark-cleaned-up", {"MERGED"})
        set_phase(frontmatter, "CLEANED_UP")
        if args.worktree_path:
            frontmatter["worktree_path"] = str(args.worktree_path.resolve())
        elif frontmatter.get("worktree") and not frontmatter.get("worktree_path"):
            frontmatter["worktree_path"] = frontmatter.get("worktree")
        frontmatter["worktree"] = None
        frontmatter["worktree_removed"] = True
        doer = ensure_agent(frontmatter, "doer", "doer")
        set_agent_state(doer, "APPROVED")
        if note:
            body = append_to_section(body, "Decisions", note)
        return frontmatter, body

    return mutate_existing(args.path, "mark-cleaned-up", mutator, args.timeout)


def op_block_or_stop(args: argparse.Namespace) -> dict[str, Any]:
    note = read_text_file(args.note_file)
    phase = "BLOCKED" if args.command == "block" else "STOPPED"
    status = "BLOCKED" if phase == "BLOCKED" else "STOPPED"

    def mutator(frontmatter: dict[str, Any], body: str) -> tuple[dict[str, Any], str]:
        set_phase(frontmatter, phase)
        if args.agent_id:
            role = args.role or ("doer" if args.agent_id == "doer" else "reviewer")
            entry = ensure_agent(frontmatter, args.agent_id, role)
            set_agent_state(entry, status)
        return frontmatter, append_to_section(body, "Decisions", note)

    return mutate_existing(args.path, args.command, mutator, args.timeout)


def add_common(parser: argparse.ArgumentParser) -> None:
    parser.add_argument("--path", required=True, type=Path, help="Markdown blackboard path")
    parser.add_argument("--timeout", type=float, default=10.0, help="Seconds to wait for the lock")


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)

    create = subparsers.add_parser("create")
    add_common(create)
    create.add_argument("--goal")
    create.add_argument("--goal-file", type=Path)
    create.add_argument("--yolo", action="store_true")
    create.add_argument("--work-type", default="feature")
    create.add_argument("--rca-required", action="store_true")
    create.add_argument("--red-test-required", action="store_true")
    create.set_defaults(func=create_blackboard)

    register = subparsers.add_parser("register-reviewer")
    add_common(register)
    register.add_argument("--agent-id", required=True)
    register.set_defaults(func=op_register_reviewer)

    claim = subparsers.add_parser("claim-review")
    add_common(claim)
    claim.add_argument("--agent-id", required=True)
    claim.set_defaults(func=op_claim_review)

    artifact = subparsers.add_parser("submit-artifact")
    add_common(artifact)
    artifact.add_argument("--target", choices=sorted(ARTIFACT_PHASES), required=True)
    artifact.add_argument("--body-file", type=Path, required=True)
    artifact.set_defaults(func=op_submit_artifact)

    followup = subparsers.add_parser("submit-followup-review")
    add_common(followup)
    followup.add_argument("--body-file", type=Path, required=True)
    followup.set_defaults(func=op_submit_followup_review)

    verdict = subparsers.add_parser("submit-verdict")
    add_common(verdict)
    verdict.add_argument("--agent-id", required=True)
    verdict.add_argument("--target", choices=sorted(REVIEW_FIELDS), required=True)
    verdict.add_argument("--counter", required=True)
    verdict.add_argument("--verdict", choices=["APPROVED", "CHANGES_REQUESTED", "COMMENT"], required=True)
    verdict.add_argument("--note-file", type=Path, required=True)
    verdict.set_defaults(func=op_submit_verdict)

    changes = subparsers.add_parser("request-changes")
    add_common(changes)
    changes.add_argument("--agent-id", required=True)
    changes.add_argument("--target", choices=sorted(REVIEW_FIELDS), required=True)
    changes.add_argument("--counter", required=True)
    changes.add_argument("--note-file", type=Path, required=True)
    changes.set_defaults(func=op_request_changes)

    coding = subparsers.add_parser("enter-coding")
    add_common(coding)
    coding.add_argument("--worktree", type=Path, required=True)
    coding.add_argument("--repo-root", type=Path)
    coding.add_argument("--base-branch")
    coding.add_argument("--base-sha")
    coding.add_argument("--topic-branch")
    coding.set_defaults(func=op_enter_coding)

    ready = subparsers.add_parser("ready-to-commit")
    add_common(ready)
    ready.add_argument("--note-file", type=Path)
    ready.set_defaults(func=op_ready_to_commit)

    committed = subparsers.add_parser("mark-committed")
    add_common(committed)
    committed.add_argument("--commit-sha", required=True)
    committed.add_argument("--topic-branch")
    committed.add_argument("--note-file", type=Path)
    committed.set_defaults(func=op_mark_committed)

    merged = subparsers.add_parser("mark-merged")
    add_common(merged)
    merged.add_argument("--merged-into", required=True)
    merged.add_argument("--merged-at")
    merged.add_argument("--note-file", type=Path)
    merged.set_defaults(func=op_mark_merged)

    cleaned = subparsers.add_parser("mark-cleaned-up")
    add_common(cleaned)
    cleaned.add_argument("--worktree-path", type=Path)
    cleaned.add_argument("--note-file", type=Path)
    cleaned.set_defaults(func=op_mark_cleaned_up)

    for name in ("block", "stop"):
        sub = subparsers.add_parser(name)
        add_common(sub)
        sub.add_argument("--agent-id")
        sub.add_argument("--role", choices=["doer", "reviewer"])
        sub.add_argument("--note-file", type=Path, required=True)
        sub.set_defaults(func=op_block_or_stop)

    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        result = args.func(args)
    except Exception as err:
        print(f"error: {err}", file=sys.stderr)
        return 1
    print(json.dumps(result, sort_keys=True, default=str))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
