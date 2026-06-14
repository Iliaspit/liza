from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path
from typing import Any

SCRIPT = Path(__file__).with_name("blackboard_op.py")
STATE_SCRIPT = Path(__file__).with_name("blackboard_state.py")


def run_op(*args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [sys.executable, str(SCRIPT), *args],
        check=False,
        text=True,
        capture_output=True,
    )


def run_state(*args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [sys.executable, str(STATE_SCRIPT), *args],
        check=False,
        text=True,
        capture_output=True,
    )


def load_json(result: subprocess.CompletedProcess[str]) -> dict[str, Any]:
    assert result.returncode == 0, result.stderr
    return json.loads(result.stdout)


def assert_error(result: subprocess.CompletedProcess[str], message: str) -> None:
    assert result.returncode == 1
    assert message in result.stderr


def reach_code_approved(tmp_path: Path) -> tuple[Path, Path]:
    blackboard = tmp_path / "board.md"
    worktree = tmp_path / ".worktrees" / "board"
    plan = tmp_path / "plan.md"
    plan_verdict = tmp_path / "plan-verdict.md"
    code = tmp_path / "code.md"
    code_verdict = tmp_path / "code-verdict.md"
    plan.write_text("### Plan Revision 1\n\nDo the thing.\n", encoding="utf-8")
    plan_verdict.write_text("### Plan Review - codex - APPROVED\n\nNo blockers.\n", encoding="utf-8")
    code.write_text("### Code Round 1\n\nStaged diff ready.\n", encoding="utf-8")
    code_verdict.write_text("### Code Review - codex - APPROVED\n\nNo blockers.\n", encoding="utf-8")
    load_json(run_op("create", "--path", str(blackboard), "--goal", "Approved code helper."))
    load_json(run_op("register-reviewer", "--path", str(blackboard), "--agent-id", "codex"))
    load_json(run_op("submit-artifact", "--path", str(blackboard), "--target", "plan", "--body-file", str(plan)))
    load_json(
        run_op(
            "submit-verdict",
            "--path",
            str(blackboard),
            "--agent-id",
            "codex",
            "--target",
            "plan",
            "--counter",
            "1",
            "--verdict",
            "APPROVED",
            "--note-file",
            str(plan_verdict),
        )
    )
    load_json(
        run_op(
            "enter-coding",
            "--path",
            str(blackboard),
            "--worktree",
            str(worktree),
        )
    )
    load_json(run_op("submit-artifact", "--path", str(blackboard), "--target", "code", "--body-file", str(code)))
    load_json(
        run_op(
            "submit-verdict",
            "--path",
            str(blackboard),
            "--agent-id",
            "codex",
            "--target",
            "code",
            "--counter",
            "1",
            "--verdict",
            "APPROVED",
            "--note-file",
            str(code_verdict),
        )
    )
    return blackboard, worktree


def test_create_starts_without_required_reviewers(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"

    result = run_op(
        "create",
        "--path",
        str(blackboard),
        "--goal",
        "Tighten adversarial pairing.",
        "--yolo",
    )

    payload = load_json(result)
    assert payload["phase"] == "DRAFT"
    assert payload["required_reviewers"] == []
    assert payload["missing_required_agent_records"] == []


def test_register_reviewer_updates_only_reviewer_state_and_required_list(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    load_json(
        run_op(
            "create",
            "--path",
            str(blackboard),
            "--goal",
            "Review startup race.",
        )
    )

    result = run_op(
        "register-reviewer",
        "--path",
        str(blackboard),
        "--agent-id",
        "codex",
    )

    payload = load_json(result)
    assert payload["registered_reviewers"] == ["codex"]
    assert payload["required_reviewers"] == ["codex"]
    agent = load_json(
        run_state(
            "--path",
            str(blackboard),
            "--role-or-reviewer-id",
            "reviewer-codex",
            "--json",
        )
    )["agent"]
    assert isinstance(agent, dict)
    assert agent["needs_registration"] is False
    assert agent["needs_required_registration"] is False


def test_submit_artifact_and_verdict_are_scoped_semantic_updates(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    plan = tmp_path / "plan.md"
    verdict = tmp_path / "verdict.md"
    plan.write_text("### Plan Revision 1\n\nDo the thing.\n", encoding="utf-8")
    verdict.write_text("### Plan Review - codex - APPROVED\n\nNo blockers.\n", encoding="utf-8")
    load_json(
        run_op(
            "create",
            "--path",
            str(blackboard),
            "--goal",
            "Exercise helper.",
        )
    )
    load_json(
        run_op(
            "register-reviewer",
            "--path",
            str(blackboard),
            "--agent-id",
            "codex",
        )
    )

    submitted = load_json(
        run_op(
            "submit-artifact",
            "--path",
            str(blackboard),
            "--target",
            "plan",
            "--body-file",
            str(plan),
        )
    )
    assert submitted["phase"] == "PLANNING_SUBMITTED"
    assert submitted["counters"]["plan_revision"] == 1

    reviewed = load_json(
        run_op(
            "submit-verdict",
            "--path",
            str(blackboard),
            "--agent-id",
            "codex",
            "--target",
            "plan",
            "--counter",
            "1",
            "--verdict",
            "APPROVED",
            "--note-file",
            str(verdict),
        )
    )

    assert reviewed["phase"] == "PLAN_APPROVED"
    assert reviewed["review"] is None
    text = blackboard.read_text(encoding="utf-8")
    assert "### Plan Revision 1" in text
    assert "### Plan Review - codex - APPROVED" in text


def test_late_reviewer_registration_blocks_current_review_gate(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    plan = tmp_path / "plan.md"
    verdict = tmp_path / "verdict.md"
    plan.write_text("### Plan Revision 1\n\nDo the thing.\n", encoding="utf-8")
    verdict.write_text("### Plan Review - claude - APPROVED\n\nNo blockers.\n", encoding="utf-8")
    load_json(run_op("create", "--path", str(blackboard), "--goal", "Exercise late registration."))
    load_json(run_op("register-reviewer", "--path", str(blackboard), "--agent-id", "claude"))
    load_json(run_op("register-reviewer", "--path", str(blackboard), "--agent-id", "gemini"))
    load_json(run_op("submit-artifact", "--path", str(blackboard), "--target", "plan", "--body-file", str(plan)))

    first_review = load_json(
        run_op(
            "submit-verdict",
            "--path",
            str(blackboard),
            "--agent-id",
            "claude",
            "--target",
            "plan",
            "--counter",
            "1",
            "--verdict",
            "APPROVED",
            "--note-file",
            str(verdict),
        )
    )
    assert first_review["review"]["complete"] is False

    late_registration = load_json(
        run_op(
            "register-reviewer",
            "--path",
            str(blackboard),
            "--agent-id",
            "codex",
        )
    )

    review = late_registration["review"]
    assert isinstance(review, dict)
    assert late_registration["required_reviewers"] == ["claude", "gemini", "codex"]
    assert review["complete"] is False
    assert review["approved_reviewers"] == ["claude"]
    assert review["pending_reviewers"] == ["gemini", "codex"]


def test_request_changes_is_explicit_verdict_operation(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    plan = tmp_path / "plan.md"
    note = tmp_path / "changes.md"
    plan.write_text("### Plan Revision 1\n\nDo the thing.\n", encoding="utf-8")
    note.write_text("### Plan Review - codex - CHANGES_REQUESTED\n\nTighten scope.\n", encoding="utf-8")
    load_json(run_op("create", "--path", str(blackboard), "--goal", "Exercise request changes."))
    load_json(run_op("register-reviewer", "--path", str(blackboard), "--agent-id", "codex"))
    load_json(run_op("submit-artifact", "--path", str(blackboard), "--target", "plan", "--body-file", str(plan)))

    reviewed = load_json(
        run_op(
            "request-changes",
            "--path",
            str(blackboard),
            "--agent-id",
            "codex",
            "--target",
            "plan",
            "--counter",
            "1",
            "--note-file",
            str(note),
        )
    )

    assert reviewed["phase"] == "PLAN_CHANGES_REQUESTED"
    assert reviewed["review"] is None
    assert "### Plan Review - codex - CHANGES_REQUESTED" in blackboard.read_text(encoding="utf-8")


def test_comment_verdict_uses_allowed_agent_status(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    plan = tmp_path / "plan.md"
    note = tmp_path / "comment.md"
    plan.write_text("### Plan Revision 1\n\nDo the thing.\n", encoding="utf-8")
    note.write_text("### Plan Review - codex - COMMENT\n\nLooks plausible.\n", encoding="utf-8")
    load_json(run_op("create", "--path", str(blackboard), "--goal", "Exercise comment."))
    load_json(run_op("register-reviewer", "--path", str(blackboard), "--agent-id", "codex"))
    load_json(run_op("submit-artifact", "--path", str(blackboard), "--target", "plan", "--body-file", str(plan)))

    reviewed = load_json(
        run_op(
            "submit-verdict",
            "--path",
            str(blackboard),
            "--agent-id",
            "codex",
            "--target",
            "plan",
            "--counter",
            "1",
            "--verdict",
            "COMMENT",
            "--note-file",
            str(note),
        )
    )

    review = reviewed["review"]
    assert isinstance(review, dict)
    assert review["comment_only"] is True
    agent = load_json(
        run_state(
            "--path",
            str(blackboard),
            "--role-or-reviewer-id",
            "reviewer-codex",
            "--json",
        )
    )["agent"]
    assert isinstance(agent, dict)
    assert agent["status"] == "WAITING"


def test_commit_merge_cleanup_milestones_record_audit_metadata(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    worktree = tmp_path / ".worktrees" / "board"
    plan = tmp_path / "plan.md"
    plan_verdict = tmp_path / "plan-verdict.md"
    code = tmp_path / "code.md"
    code_verdict = tmp_path / "code-verdict.md"
    plan.write_text("### Plan Revision 1\n\nDo the thing.\n", encoding="utf-8")
    plan_verdict.write_text("### Plan Review - codex - APPROVED\n\nNo blockers.\n", encoding="utf-8")
    code.write_text("### Code Round 1\n\nStaged diff ready.\n", encoding="utf-8")
    code_verdict.write_text("### Code Review - codex - APPROVED\n\nNo blockers.\n", encoding="utf-8")
    load_json(run_op("create", "--path", str(blackboard), "--goal", "Merge lifecycle."))
    load_json(run_op("register-reviewer", "--path", str(blackboard), "--agent-id", "codex"))
    load_json(run_op("submit-artifact", "--path", str(blackboard), "--target", "plan", "--body-file", str(plan)))
    load_json(
        run_op(
            "submit-verdict",
            "--path",
            str(blackboard),
            "--agent-id",
            "codex",
            "--target",
            "plan",
            "--counter",
            "1",
            "--verdict",
            "APPROVED",
            "--note-file",
            str(plan_verdict),
        )
    )
    load_json(
        run_op(
            "enter-coding",
            "--path",
            str(blackboard),
            "--worktree",
            str(worktree),
            "--repo-root",
            str(tmp_path),
            "--base-branch",
            "main",
            "--base-sha",
            "abc123",
            "--topic-branch",
            "ap-board",
        )
    )
    load_json(run_op("submit-artifact", "--path", str(blackboard), "--target", "code", "--body-file", str(code)))
    load_json(
        run_op(
            "submit-verdict",
            "--path",
            str(blackboard),
            "--agent-id",
            "codex",
            "--target",
            "code",
            "--counter",
            "1",
            "--verdict",
            "APPROVED",
            "--note-file",
            str(code_verdict),
        )
    )
    ready = load_json(run_op("ready-to-commit", "--path", str(blackboard)))
    assert ready["phase"] == "READY_TO_COMMIT"

    committed = load_json(
        run_op(
            "mark-committed",
            "--path",
            str(blackboard),
            "--commit-sha",
            "deadbeef",
        )
    )
    assert committed["phase"] == "COMMITTED"
    assert committed["audit"]["commit_sha"] == "deadbeef"

    merged = load_json(
        run_op(
            "mark-merged",
            "--path",
            str(blackboard),
            "--merged-into",
            "main",
        )
    )
    assert merged["phase"] == "MERGED"
    assert merged["audit"]["merged_into"] == "main"

    cleaned = load_json(
        run_op(
            "mark-cleaned-up",
            "--path",
            str(blackboard),
            "--worktree-path",
            str(worktree),
        )
    )
    assert cleaned["phase"] == "CLEANED_UP"
    assert cleaned["terminal"] is True
    assert cleaned["worktree"] is None
    assert cleaned["worktree_path"] == str(worktree.resolve())
    assert cleaned["audit"]["worktree_removed"] is True


def test_submit_artifact_rejects_wrong_phase(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    body = tmp_path / "code.md"
    body.write_text("### Code Round 1\n\nNo approved plan yet.\n", encoding="utf-8")
    load_json(run_op("create", "--path", str(blackboard), "--goal", "Reject wrong phase."))

    result = run_op("submit-artifact", "--path", str(blackboard), "--target", "code", "--body-file", str(body))

    assert_error(result, "submit-code requires phase in {CODING}; current phase is DRAFT")


def test_submit_plan_with_rca_rejects_phase_regression_from_coding(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    worktree = tmp_path / ".worktrees" / "board"
    analysis = tmp_path / "analysis.md"
    analysis_verdict = tmp_path / "analysis-verdict.md"
    plan = tmp_path / "plan.md"
    plan_verdict = tmp_path / "plan-verdict.md"
    analysis.write_text("### RCA 1\n\nRoot cause.\n", encoding="utf-8")
    analysis_verdict.write_text("### RCA Review - codex - APPROVED\n\nNo blockers.\n", encoding="utf-8")
    plan.write_text("### Plan Revision 1\n\nDo the thing.\n", encoding="utf-8")
    plan_verdict.write_text("### Plan Review - codex - APPROVED\n\nNo blockers.\n", encoding="utf-8")
    load_json(run_op("create", "--path", str(blackboard), "--goal", "Reject plan regression.", "--rca-required"))
    load_json(run_op("register-reviewer", "--path", str(blackboard), "--agent-id", "codex"))
    load_json(
        run_op("submit-artifact", "--path", str(blackboard), "--target", "analysis", "--body-file", str(analysis))
    )
    load_json(
        run_op(
            "submit-verdict",
            "--path",
            str(blackboard),
            "--agent-id",
            "codex",
            "--target",
            "analysis",
            "--counter",
            "1",
            "--verdict",
            "APPROVED",
            "--note-file",
            str(analysis_verdict),
        )
    )
    load_json(run_op("submit-artifact", "--path", str(blackboard), "--target", "plan", "--body-file", str(plan)))
    load_json(
        run_op(
            "submit-verdict",
            "--path",
            str(blackboard),
            "--agent-id",
            "codex",
            "--target",
            "plan",
            "--counter",
            "1",
            "--verdict",
            "APPROVED",
            "--note-file",
            str(plan_verdict),
        )
    )
    load_json(run_op("enter-coding", "--path", str(blackboard), "--worktree", str(worktree)))

    result = run_op("submit-artifact", "--path", str(blackboard), "--target", "plan", "--body-file", str(plan))

    assert_error(
        result,
        "submit-plan requires phase in {ANALYSIS_APPROVED, PLANNING, PLAN_CHANGES_REQUESTED}; current phase is CODING",
    )


def test_submit_red_test_rejects_phase_regression_from_ready_to_commit(tmp_path: Path) -> None:
    blackboard, _worktree = reach_code_approved(tmp_path)
    red_test = tmp_path / "red-test.md"
    red_test.write_text("### Red Test Round 1\n\nToo late.\n", encoding="utf-8")
    load_json(run_op("ready-to-commit", "--path", str(blackboard)))

    result = run_op(
        "submit-artifact",
        "--path",
        str(blackboard),
        "--target",
        "red-test",
        "--body-file",
        str(red_test),
    )

    assert_error(
        result,
        "submit-red-test requires phase in {PLAN_APPROVED, RED_TESTING, "
        "RED_TEST_CHANGES_REQUESTED}; current phase is READY_TO_COMMIT",
    )


def test_verdict_rejects_counter_mismatch(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    plan = tmp_path / "plan.md"
    verdict = tmp_path / "verdict.md"
    plan.write_text("### Plan Revision 1\n\nDo the thing.\n", encoding="utf-8")
    verdict.write_text("### Plan Review - codex - APPROVED\n\nNo blockers.\n", encoding="utf-8")
    load_json(run_op("create", "--path", str(blackboard), "--goal", "Reject stale verdict."))
    load_json(run_op("register-reviewer", "--path", str(blackboard), "--agent-id", "codex"))
    load_json(run_op("submit-artifact", "--path", str(blackboard), "--target", "plan", "--body-file", str(plan)))

    result = run_op(
        "submit-verdict",
        "--path",
        str(blackboard),
        "--agent-id",
        "codex",
        "--target",
        "plan",
        "--counter",
        "2",
        "--verdict",
        "APPROVED",
        "--note-file",
        str(verdict),
    )

    assert_error(result, "plan_revision is 1, not 2")


def test_verdict_rejects_wrong_active_target(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    plan = tmp_path / "plan.md"
    verdict = tmp_path / "verdict.md"
    plan.write_text("### Plan Revision 1\n\nDo the thing.\n", encoding="utf-8")
    verdict.write_text("### Code Review - codex - APPROVED\n\nWrong target.\n", encoding="utf-8")
    load_json(run_op("create", "--path", str(blackboard), "--goal", "Reject wrong target."))
    load_json(run_op("register-reviewer", "--path", str(blackboard), "--agent-id", "codex"))
    load_json(run_op("submit-artifact", "--path", str(blackboard), "--target", "plan", "--body-file", str(plan)))

    result = run_op(
        "submit-verdict",
        "--path",
        str(blackboard),
        "--agent-id",
        "codex",
        "--target",
        "code",
        "--counter",
        "0",
        "--verdict",
        "APPROVED",
        "--note-file",
        str(verdict),
    )

    assert_error(result, "phase PLANNING_SUBMITTED is not reviewable for code verdict")


def test_verdict_rejects_role_mismatch(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    plan = tmp_path / "plan.md"
    verdict = tmp_path / "verdict.md"
    plan.write_text("### Plan Revision 1\n\nDo the thing.\n", encoding="utf-8")
    verdict.write_text("### Plan Review - doer - APPROVED\n\nWrong role.\n", encoding="utf-8")
    load_json(run_op("create", "--path", str(blackboard), "--goal", "Reject role mismatch."))
    load_json(run_op("submit-artifact", "--path", str(blackboard), "--target", "plan", "--body-file", str(plan)))
    blackboard.write_text(
        blackboard.read_text(encoding="utf-8").replace("required_reviewers: []", "required_reviewers:\n- doer"),
        encoding="utf-8",
    )

    result = run_op(
        "submit-verdict",
        "--path",
        str(blackboard),
        "--agent-id",
        "doer",
        "--target",
        "plan",
        "--counter",
        "1",
        "--verdict",
        "APPROVED",
        "--note-file",
        str(verdict),
    )

    assert_error(result, "agent doer has role doer, not reviewer")


def test_enter_coding_rejects_before_plan_approval(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    load_json(run_op("create", "--path", str(blackboard), "--goal", "Reject early coding."))

    result = run_op("enter-coding", "--path", str(blackboard), "--worktree", str(tmp_path / ".worktrees" / "board"))

    assert_error(result, "enter-coding requires phase in {PLAN_APPROVED}; current phase is DRAFT")


def test_ready_to_commit_rejects_before_code_approval(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    load_json(run_op("create", "--path", str(blackboard), "--goal", "Reject early commit."))

    result = run_op("ready-to-commit", "--path", str(blackboard))

    assert_error(
        result,
        "ready-to-commit requires phase in {CODE_SUBMITTED, FOLLOWUP_REVIEW, REVIEWING_CODE}; current phase is DRAFT",
    )


def test_submit_followup_review_accepts_approved_code_round(tmp_path: Path) -> None:
    blackboard, _worktree = reach_code_approved(tmp_path)
    followup = tmp_path / "followup.md"
    followup.write_text("### Code Round 2\n\nIncorporate non-blocking suggestions.\n", encoding="utf-8")

    result = load_json(
        run_op(
            "submit-followup-review",
            "--path",
            str(blackboard),
            "--body-file",
            str(followup),
        )
    )

    assert result["phase"] == "FOLLOWUP_REVIEW"
    assert result["counters"]["code_review_round"] == 2


def test_enter_coding_and_ready_to_commit_reject_after_commit(tmp_path: Path) -> None:
    blackboard, worktree = reach_code_approved(tmp_path)
    load_json(run_op("ready-to-commit", "--path", str(blackboard)))
    load_json(run_op("mark-committed", "--path", str(blackboard), "--commit-sha", "deadbeef"))

    enter_coding = run_op("enter-coding", "--path", str(blackboard), "--worktree", str(worktree))
    ready_again = run_op("ready-to-commit", "--path", str(blackboard))

    assert_error(enter_coding, "enter-coding requires phase in {PLAN_APPROVED}; current phase is COMMITTED")
    assert_error(
        ready_again,
        "ready-to-commit requires phase in {CODE_SUBMITTED, FOLLOWUP_REVIEW, "
        "REVIEWING_CODE}; current phase is COMMITTED",
    )


def test_merge_and_cleanup_milestones_reject_wrong_order(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    load_json(run_op("create", "--path", str(blackboard), "--goal", "Reject wrong milestone order."))

    merged = run_op("mark-merged", "--path", str(blackboard), "--merged-into", "main")
    cleaned = run_op("mark-cleaned-up", "--path", str(blackboard))

    assert_error(merged, "mark-merged requires phase in {COMMITTED}; current phase is DRAFT")
    assert_error(cleaned, "mark-cleaned-up requires phase in {MERGED}; current phase is DRAFT")
