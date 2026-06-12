from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

import pytest

SCRIPT = Path(__file__).with_name("blackboard_state.py")


def run_state(*args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [sys.executable, str(SCRIPT), *args],
        check=False,
        text=True,
        capture_output=True,
    )


def write_board(path: Path, frontmatter: str) -> None:
    path.write_text(
        f"""---
{frontmatter.strip()}
---

# Adversarial Pairing Blackboard

## Goal
""",
        encoding="utf-8",
    )


def minimal_frontmatter(extra: str = "") -> str:
    return f"""
phase: DRAFT
yolo: false
work_type: feature
rca_required: false
red_test_required: false
required_reviewers: []
plan_revision: 0
analysis_revision: 0
red_test_round: 0
code_review_round: 0
phase_updated_at: "2026-06-12T12:00:00Z"
worktree: null
agents:
  doer:
    role: doer
    status: DRAFT
    last_seen: null
    reviewed_analysis_revision: null
    analysis_verdict: null
    reviewed_plan_revision: null
    plan_verdict: null
    reviewed_red_test_round: null
    red_test_verdict: null
    reviewed_code_round: null
    code_verdict: null
{extra}
"""


def load_json(result: subprocess.CompletedProcess[str]) -> dict[str, object]:
    assert result.returncode == 0, result.stderr
    return json.loads(result.stdout)


def test_missing_reviewer_is_told_to_register_and_become_required(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    write_board(blackboard, minimal_frontmatter())

    result = run_state(
        "--path",
        str(blackboard),
        "--role-or-reviewer-id",
        "reviewer-codex",
        "--json",
    )

    payload = load_json(result)
    agent = payload["agent"]
    assert isinstance(agent, dict)
    assert agent["needs_registration"] is True
    assert agent["needs_required_registration"] is False
    assert payload["next"] == {
        "actor": "reviewer",
        "action": "self-register under agents.<id> and add the same id to required_reviewers in one locked write",
        "handoff_to": ["codex"],
    }


def test_registered_but_unrequired_reviewer_is_told_to_join_required_reviewers(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    write_board(
        blackboard,
        minimal_frontmatter(
            """
  codex:
    role: reviewer
    status: IDLE
    last_seen: "2026-06-12T12:00:30Z"
    reviewed_analysis_revision: null
    analysis_verdict: null
    reviewed_plan_revision: null
    plan_verdict: null
    reviewed_red_test_round: null
    red_test_verdict: null
    reviewed_code_round: null
    code_verdict: null
"""
        ),
    )

    result = run_state(
        "--path",
        str(blackboard),
        "--role-or-reviewer-id",
        "reviewer-codex",
        "--json",
    )

    payload = load_json(result)
    agent = payload["agent"]
    assert isinstance(agent, dict)
    assert agent["needs_registration"] is False
    assert agent["needs_required_registration"] is True
    assert payload["unrequired_registered_reviewers"] == ["codex"]
    assert payload["next"] == {
        "actor": "reviewer",
        "action": "add this reviewer id to required_reviewers in one locked write with status/last_seen update",
        "handoff_to": ["codex"],
    }


def test_late_required_reviewer_blocks_current_plan_review_until_reviewed(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    write_board(
        blackboard,
        minimal_frontmatter(
            """
  claude:
    role: reviewer
    status: APPROVED
    last_seen: "2026-06-12T12:00:30Z"
    reviewed_analysis_revision: null
    analysis_verdict: null
    reviewed_plan_revision: 1
    plan_verdict: APPROVED
    reviewed_red_test_round: null
    red_test_verdict: null
    reviewed_code_round: null
    code_verdict: null
  codex:
    role: reviewer
    status: IDLE
    last_seen: "2026-06-12T12:00:31Z"
    reviewed_analysis_revision: null
    analysis_verdict: null
    reviewed_plan_revision: null
    plan_verdict: null
    reviewed_red_test_round: null
    red_test_verdict: null
    reviewed_code_round: null
    code_verdict: null
""",
        )
        .replace("phase: DRAFT", "phase: PLANNING_SUBMITTED")
        .replace("required_reviewers: []", "required_reviewers: [claude, codex]")
        .replace("plan_revision: 0", "plan_revision: 1"),
    )

    result = run_state(
        "--path",
        str(blackboard),
        "--role-or-reviewer-id",
        "doer",
        "--json",
    )

    payload = load_json(result)
    review = payload["review"]
    assert isinstance(review, dict)
    assert review["target"] == "plan"
    assert review["complete"] is False
    assert review["pending_reviewers"] == ["codex"]
    assert review["approved_reviewers"] == ["claude"]
    assert payload["next"] == {
        "actor": "reviewer",
        "action": "review current plan artifact",
        "handoff_to": ["codex"],
    }


def test_code_review_reports_recorded_worktree_and_diff_scope(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    worktree = tmp_path / ".worktrees" / "board"
    write_board(
        blackboard,
        minimal_frontmatter(
            """
  codex:
    role: reviewer
    status: IDLE
    last_seen: "2026-06-12T12:00:31Z"
    reviewed_analysis_revision: null
    analysis_verdict: null
    reviewed_plan_revision: null
    plan_verdict: null
    reviewed_red_test_round: null
    red_test_verdict: null
    reviewed_code_round: null
    code_verdict: null
"""
        )
        .replace("phase: DRAFT", "phase: CODE_SUBMITTED")
        .replace("required_reviewers: []", "required_reviewers: [codex]")
        .replace("code_review_round: 0", "code_review_round: 2")
        .replace("worktree: null", f'worktree: "{worktree}"'),
    )

    result = run_state(
        "--path",
        str(blackboard),
        "--role-or-reviewer-id",
        "reviewer-codex",
        "--json",
    )

    payload = load_json(result)
    review = payload["review"]
    assert isinstance(review, dict)
    assert payload["worktree"] == str(worktree)
    assert review["target"] == "code"
    assert review["diff_scope"] == "staged"
    assert payload["next"] == {
        "actor": "reviewer",
        "action": "review current code artifact",
        "handoff_to": ["codex"],
    }


def test_fallback_parser_handles_multiline_required_reviewers() -> None:
    from blackboard_state import parse_frontmatter_fallback

    parsed = parse_frontmatter_fallback(
        """
phase: PLANNING_SUBMITTED
required_reviewers:
  - claude
  - codex
agents:
  claude:
    role: reviewer
    status: APPROVED
"""
    )

    assert parsed["required_reviewers"] == ["claude", "codex"]
    assert parsed["agents"]["claude"]["role"] == "reviewer"


def test_fallback_parser_rejects_mapping_valued_list_items() -> None:
    from blackboard_state import parse_frontmatter_fallback

    with pytest.raises(ValueError, match="mapping-valued list items require PyYAML"):
        parse_frontmatter_fallback(
            """
required_reviewers:
  - id: codex
"""
        )


def test_relative_worktree_is_rejected(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    write_board(
        blackboard,
        minimal_frontmatter().replace("worktree: null", 'worktree: ".worktrees/board"'),
    )

    result = run_state(
        "--path",
        str(blackboard),
        "--role-or-reviewer-id",
        "doer",
        "--json",
    )

    assert result.returncode == 1
    assert "worktree must be an absolute path" in result.stderr
