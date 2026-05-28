from __future__ import annotations

import fcntl
import hashlib
import json
import subprocess
import sys
from pathlib import Path

SCRIPT = Path(__file__).with_name("blackboard_write.py")


def sha256_text(text: str) -> str:
    return hashlib.sha256(text.encode()).hexdigest()


def run_writer(*args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [sys.executable, str(SCRIPT), *args],
        check=False,
        text=True,
        capture_output=True,
    )


def test_replaces_file_when_expected_hash_matches(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    content = tmp_path / "new.md"
    blackboard.write_text("old\n", encoding="utf-8")
    content.write_text("new\n", encoding="utf-8")

    result = run_writer(
        "--path",
        str(blackboard),
        "--content-file",
        str(content),
        "--operation",
        "test_replace",
        "--expect-sha256",
        sha256_text("old\n"),
    )

    assert result.returncode == 0, result.stderr
    assert blackboard.read_text(encoding="utf-8") == "new\n"
    payload = json.loads(result.stdout)
    assert payload["old_sha256"] == sha256_text("old\n")
    assert payload["new_sha256"] == sha256_text("new\n")
    assert (tmp_path / "board.md.lock").exists()
    assert (tmp_path / "board.md.lock.owner.json").exists()


def test_rejects_stale_expected_hash_without_modifying_file(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    content = tmp_path / "new.md"
    blackboard.write_text("current\n", encoding="utf-8")
    content.write_text("new\n", encoding="utf-8")

    result = run_writer(
        "--path",
        str(blackboard),
        "--content-file",
        str(content),
        "--operation",
        "test_stale",
        "--expect-sha256",
        sha256_text("stale\n"),
    )

    assert result.returncode == 3
    assert "stale blackboard content" in result.stderr
    assert blackboard.read_text(encoding="utf-8") == "current\n"


def test_creates_missing_file_when_allowed(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    content = tmp_path / "new.md"
    content.write_text("created\n", encoding="utf-8")

    result = run_writer(
        "--path",
        str(blackboard),
        "--content-file",
        str(content),
        "--operation",
        "test_create",
        "--create-if-missing",
    )

    assert result.returncode == 0, result.stderr
    assert blackboard.read_text(encoding="utf-8") == "created\n"
    payload = json.loads(result.stdout)
    assert payload["created"] is True
    assert payload["old_sha256"] == ""


def test_create_if_missing_rejects_existing_file_without_modifying_it(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    content = tmp_path / "new.md"
    blackboard.write_text("existing\n", encoding="utf-8")
    content.write_text("new\n", encoding="utf-8")

    result = run_writer(
        "--path",
        str(blackboard),
        "--content-file",
        str(content),
        "--operation",
        "test_create_race",
        "--create-if-missing",
    )

    assert result.returncode == 1
    assert "blackboard already exists" in result.stderr
    assert blackboard.read_text(encoding="utf-8") == "existing\n"


def test_create_if_missing_and_expected_hash_are_mutually_exclusive(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    content = tmp_path / "new.md"
    blackboard.write_text("old\n", encoding="utf-8")
    content.write_text("new\n", encoding="utf-8")

    result = run_writer(
        "--path",
        str(blackboard),
        "--content-file",
        str(content),
        "--operation",
        "test_bad_args",
        "--create-if-missing",
        "--expect-sha256",
        sha256_text("old\n"),
    )

    assert result.returncode == 4
    assert "mutually exclusive" in result.stderr
    assert blackboard.read_text(encoding="utf-8") == "old\n"


def test_times_out_when_lock_is_held(tmp_path: Path) -> None:
    blackboard = tmp_path / "board.md"
    content = tmp_path / "new.md"
    lock_path = tmp_path / "board.md.lock"
    blackboard.write_text("old\n", encoding="utf-8")
    content.write_text("new\n", encoding="utf-8")

    with lock_path.open("a+b") as lock_file:
        fcntl.flock(lock_file.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
        result = run_writer(
            "--path",
            str(blackboard),
            "--content-file",
            str(content),
            "--operation",
            "test_locked",
            "--expect-sha256",
            sha256_text("old\n"),
            "--timeout",
            "0.1",
        )

    assert result.returncode == 2
    assert "lock unavailable" in result.stderr
    assert blackboard.read_text(encoding="utf-8") == "old\n"
