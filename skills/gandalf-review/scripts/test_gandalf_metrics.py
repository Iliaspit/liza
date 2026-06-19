import json
import os
import subprocess
import sys
from pathlib import Path

SCRIPT = Path(__file__).with_name("gandalf_metrics.py")
PROGRESS_SCRIPT = Path(__file__).with_name("gandalf_progress.py")
FAST_HOME_SCRIPT = Path(__file__).with_name("gandalf_codex_fast_home.py")
SQUASH_SCRIPT = Path(__file__).with_name("gandalf_squash.py")


def run_metrics(root: Path, *args: str, env: dict[str, str] | None = None) -> dict:
    completed = subprocess.run(
        [sys.executable, str(SCRIPT), "--root", str(root), *args],
        check=True,
        text=True,
        capture_output=True,
        env=env,
    )
    return json.loads(completed.stdout)


def read_jsonl(path: Path) -> list[dict]:
    return [json.loads(line) for line in path.read_text().splitlines() if line.strip()]


def test_records_run_artifacts_and_rebuilds_aggregate(tmp_path: Path) -> None:
    root = tmp_path / "gandalf"
    review = tmp_path / "review.md"
    review.write_text("**Verdict:** Changes requested\n\nFix auth guard.\n")

    started = run_metrics(
        root,
        "start",
        "--run-id",
        "test-run",
        "--repo",
        "omni",
        "--branch",
        "feature/auth",
        "--base-ref",
        "main",
        "--goal",
        "Review auth change",
    )
    assert started["run_id"] == "test-run"

    run_metrics(
        root,
        "record",
        "--run-id",
        "test-run",
        "--kind",
        "primary_review_finished",
        "--iteration",
        "1",
        "--reviewer",
        "primary",
        "--verdict",
        "Changes requested",
        "--duration-kind",
        "review",
        "--duration-ms",
        "1200",
        "--summary",
        "Auth guard missing",
        "--content-file",
        str(review),
        "--artifact-name",
        "primary-review.md",
    )
    run_metrics(
        root,
        "record",
        "--run-id",
        "test-run",
        "--kind",
        "fix_finished",
        "--iteration",
        "1",
        "--duration-kind",
        "fix",
        "--duration-ms",
        "3400",
        "--summary",
        "Added guard and regression test",
        "--commit",
        "abc1234",
    )
    finished = run_metrics(
        root,
        "finish",
        "--run-id",
        "test-run",
        "--final-verdict",
        "APPROVED",
        "--summary",
        "Approved after one fix loop",
    )

    assert finished["iterations"] == 1
    assert finished["review_duration_ms"] == 1200
    assert finished["fix_duration_ms"] == 3400
    assert finished["final_verdict"] == "APPROVED"

    artifact = root / "runs/test-run/artifacts/iteration-1/primary-review.md"
    assert artifact.read_text() == review.read_text()
    assert "Approved after one fix loop" in (root / "runs/test-run/summary.md").read_text()

    aggregate = run_metrics(root, "aggregate")
    assert aggregate["runs"] == 1
    index = read_jsonl(root / "index.jsonl")
    assert index[0]["run_id"] == "test-run"
    assert index[0]["review_duration_ms"] == 1200
    fix_event = next(
        item for item in read_jsonl(root / "runs/test-run/metrics.jsonl") if item["kind"] == "fix_finished"
    )
    assert fix_event["commit"] == "abc1234"
    assert index[0]["artifact_path"].endswith("runs/test-run")
    assert index[0]["recap"] == "Approved after one fix loop"
    assert "Total iterations: 1" in (root / "aggregate.md").read_text()


def test_export_command_receives_generic_paths(tmp_path: Path) -> None:
    root = tmp_path / "gandalf"
    export_log = tmp_path / "export.log"
    exporter = tmp_path / "exporter.py"
    exporter.write_text(
        "import os, pathlib\n"
        "pathlib.Path(os.environ['EXPORT_LOG']).write_text('\\n'.join([\n"
        "os.environ['GANDALF_REVIEW_EVENT_PATH'],\n"
        "os.environ['GANDALF_REVIEW_INDEX_PATH'],\n"
        "os.environ['GANDALF_REVIEW_AGGREGATE_PATH'],\n"
        "os.environ['GANDALF_REVIEW_RUN_SUMMARY_PATH'],\n"
        "os.environ['GANDALF_REVIEW_RUN_ID'],\n"
        "]))\n"
    )

    env = os.environ.copy()
    env["EXPORT_LOG"] = str(export_log)
    env["GANDALF_REVIEW_EXPORT_CMD"] = f"{sys.executable} {exporter}"

    run_metrics(
        root,
        "start",
        "--run-id",
        "export-run",
        "--repo",
        "omni",
        "--branch",
        "feature/export",
        "--base-ref",
        "main",
        "--goal",
        "Check export",
        env=env,
    )

    exported = export_log.read_text().splitlines()
    assert exported[0].endswith("latest_event.json")
    assert exported[1].endswith("index.jsonl")
    assert exported[2].endswith("aggregate.md")
    assert exported[3].endswith("summary.md")
    assert exported[4] == "export-run"


def test_export_failure_warns_without_failing_core_archive(tmp_path: Path) -> None:
    root = tmp_path / "gandalf"
    env = os.environ.copy()
    env["GANDALF_REVIEW_EXPORT_CMD"] = f'{sys.executable} -c "import sys; sys.exit(7)"'

    completed = subprocess.run(
        [
            sys.executable,
            str(SCRIPT),
            "--root",
            str(root),
            "start",
            "--run-id",
            "export-fails",
            "--repo",
            "liza",
            "--branch",
            "feature/export",
            "--base-ref",
            "main",
            "--goal",
            "Check export warning",
        ],
        check=True,
        text=True,
        capture_output=True,
        env=env,
    )

    assert json.loads(completed.stdout)["run_id"] == "export-fails"
    assert "warning: GANDALF_REVIEW_EXPORT_CMD failed" in completed.stderr
    assert (root / "runs/export-fails/metrics.jsonl").exists()
    assert (root / "index.jsonl").exists()


def test_duplicate_run_id_exits_without_traceback(tmp_path: Path) -> None:
    root = tmp_path / "gandalf"
    run_metrics(
        root,
        "start",
        "--run-id",
        "duplicate",
        "--repo",
        "liza",
        "--branch",
        "feature/duplicate",
        "--base-ref",
        "main",
        "--goal",
        "First run",
    )

    completed = subprocess.run(
        [
            sys.executable,
            str(SCRIPT),
            "--root",
            str(root),
            "start",
            "--run-id",
            "duplicate",
            "--repo",
            "liza",
            "--branch",
            "feature/duplicate",
            "--base-ref",
            "main",
            "--goal",
            "Second run",
        ],
        check=False,
        text=True,
        capture_output=True,
    )

    assert completed.returncode != 0
    assert completed.stdout == ""
    assert "run id already exists: duplicate" in completed.stderr
    assert "Traceback" not in completed.stderr


def test_start_recovers_from_malformed_cached_index(tmp_path: Path) -> None:
    root = tmp_path / "gandalf"
    root.mkdir()
    (root / "index.jsonl").write_text("{}\n")

    started = run_metrics(
        root,
        "start",
        "--run-id",
        "healthy",
        "--repo",
        "liza",
        "--branch",
        "feature/index",
        "--base-ref",
        "main",
        "--goal",
        "Recover aggregate cache",
    )

    assert started["run_id"] == "healthy"
    index = read_jsonl(root / "index.jsonl")
    assert [entry["run_id"] for entry in index] == ["healthy"]
    assert "healthy" in (root / "aggregate.md").read_text()


def test_concurrent_records_keep_metrics_and_aggregate_consistent(tmp_path: Path) -> None:
    root = tmp_path / "gandalf"
    run_metrics(
        root,
        "start",
        "--run-id",
        "concurrent",
        "--repo",
        "liza",
        "--branch",
        "feature/concurrent",
        "--base-ref",
        "main",
        "--goal",
        "Check concurrent records",
    )

    commands = [
        [
            sys.executable,
            str(SCRIPT),
            "--root",
            str(root),
            "record",
            "--run-id",
            "concurrent",
            "--kind",
            "fix_finished",
            "--iteration",
            str(iteration),
            "--duration-kind",
            "fix",
            "--duration-ms",
            "10",
            "--summary",
            f"fix {iteration}",
        ]
        for iteration in range(1, 9)
    ]

    processes = [
        subprocess.Popen(command, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE) for command in commands
    ]
    results = [process.communicate(timeout=10) + (process.returncode,) for process in processes]

    for stdout, stderr, returncode in results:
        assert returncode == 0, stderr
        assert json.loads(stdout)["kind"] == "fix_finished"

    events = read_jsonl(root / "runs/concurrent/metrics.jsonl")
    fix_events = [event for event in events if event["kind"] == "fix_finished"]
    assert len(fix_events) == 8
    assert {event["iteration"] for event in fix_events} == set(range(1, 9))

    index = read_jsonl(root / "index.jsonl")
    assert index[0]["run_id"] == "concurrent"
    assert index[0]["iterations"] == 8
    assert index[0]["fix_duration_ms"] == 80


def test_record_updates_current_run_without_rewriting_historical_summaries(tmp_path: Path) -> None:
    root = tmp_path / "gandalf"
    run_metrics(
        root,
        "start",
        "--run-id",
        "historical",
        "--repo",
        "liza",
        "--branch",
        "main",
        "--base-ref",
        "main",
        "--goal",
        "Historical run",
    )
    run_metrics(
        root,
        "start",
        "--run-id",
        "active",
        "--repo",
        "liza",
        "--branch",
        "feature/perf",
        "--base-ref",
        "main",
        "--goal",
        "Active run",
    )
    historical_summary = root / "runs/historical/summary.md"
    historical_summary.write_text("sentinel historical summary\n")

    run_metrics(
        root,
        "record",
        "--run-id",
        "active",
        "--kind",
        "fix_finished",
        "--iteration",
        "1",
        "--duration-kind",
        "fix",
        "--duration-ms",
        "25",
        "--summary",
        "incremental fix",
    )

    assert historical_summary.read_text() == "sentinel historical summary\n"
    index = {entry["run_id"]: entry for entry in read_jsonl(root / "index.jsonl")}
    assert index["historical"]["final_verdict"] == "IN_PROGRESS"
    assert index["active"]["iterations"] == 1
    assert index["active"]["fix_duration_ms"] == 25
    aggregate = (root / "aggregate.md").read_text()
    assert "`historical` IN_PROGRESS" in aggregate
    assert "`active` IN_PROGRESS iterations=1 review_ms=0 fix_ms=25" in aggregate


def test_custom_run_id_is_confined_to_runs_root(tmp_path: Path) -> None:
    root = tmp_path / "gandalf"

    started = run_metrics(
        root,
        "start",
        "--run-id",
        "../escape/../../bad",
        "--repo",
        "omni",
        "--branch",
        "feature/auth",
        "--base-ref",
        "main",
        "--goal",
        "Review auth change",
    )

    assert started["run_id"] == "escape-..-..-bad"
    assert (root / "runs" / "escape-..-..-bad" / "metadata.json").exists()
    assert not (root.parent / "bad").exists()

    recorded = run_metrics(
        root,
        "record",
        "--run-id",
        "../escape/../../bad",
        "--kind",
        "fix_finished",
        "--iteration",
        "1",
        "--summary",
        "safe lookup",
    )

    assert recorded["run_id"] == "escape-..-..-bad"


def test_aggregate_survives_corrupt_historical_run(tmp_path: Path) -> None:
    root = tmp_path / "gandalf"
    corrupt = root / "runs" / "corrupt-run"
    corrupt.mkdir(parents=True)
    (corrupt / "metadata.json").write_text('{"run_id": "corrupt-run"}')
    (corrupt / "metrics.jsonl").write_text("{not json}\n")

    run_metrics(
        root,
        "start",
        "--run-id",
        "healthy-run",
        "--repo",
        "omni",
        "--branch",
        "feature/auth",
        "--base-ref",
        "main",
        "--goal",
        "Review auth change",
    )

    index = read_jsonl(root / "index.jsonl")
    corrupt_entry = next(item for item in index if item["run_id"] == "corrupt-run")
    healthy_entry = next(item for item in index if item["run_id"] == "healthy-run")
    assert corrupt_entry["final_verdict"] == "CORRUPT"
    assert "cannot read run metrics" in corrupt_entry["blocker"]
    assert healthy_entry["final_verdict"] == "IN_PROGRESS"
    aggregate = (root / "aggregate.md").read_text()
    assert "Completed/blocked runs: 0" in aggregate
    assert "Corrupt runs: 1" in aggregate


def test_aggregate_survives_structurally_corrupt_historical_run(tmp_path: Path) -> None:
    root = tmp_path / "gandalf"
    corrupt = root / "runs" / "structural-corrupt-run"
    corrupt.mkdir(parents=True)
    (corrupt / "metadata.json").write_text('{"run_id": "structural-corrupt-run", "started_at": "not-a-date"}')
    (corrupt / "metrics.jsonl").write_text(
        '{"kind": "run_finished", "timestamp": "2026-06-19T00:00:00Z", '
        '"final_verdict": "APPROVED", "summary": "bad historical data"}\n'
    )

    run_metrics(
        root,
        "start",
        "--run-id",
        "healthy-run",
        "--repo",
        "omni",
        "--branch",
        "feature/auth",
        "--base-ref",
        "main",
        "--goal",
        "Review auth change",
    )

    index = read_jsonl(root / "index.jsonl")
    corrupt_entry = next(item for item in index if item["run_id"] == "structural-corrupt-run")
    healthy_entry = next(item for item in index if item["run_id"] == "healthy-run")
    assert corrupt_entry["final_verdict"] == "CORRUPT"
    assert "Invalid isoformat string" in corrupt_entry["blocker"]
    assert healthy_entry["final_verdict"] == "IN_PROGRESS"


def test_aggregate_survives_wrong_json_shapes_in_historical_run(tmp_path: Path) -> None:
    root = tmp_path / "gandalf"
    bad_metadata = root / "runs" / "bad-metadata-shape"
    bad_metadata.mkdir(parents=True)
    (bad_metadata / "metadata.json").write_text("[]")
    (bad_metadata / "metrics.jsonl").write_text('{"kind": "run_started", "timestamp": "2026-06-19T00:00:00Z"}\n')

    bad_event = root / "runs" / "bad-event-shape"
    bad_event.mkdir(parents=True)
    (bad_event / "metadata.json").write_text('{"run_id": "bad-event-shape"}')
    (bad_event / "metrics.jsonl").write_text("[]\n")

    run_metrics(
        root,
        "start",
        "--run-id",
        "healthy-run",
        "--repo",
        "omni",
        "--branch",
        "feature/auth",
        "--base-ref",
        "main",
        "--goal",
        "Review auth change",
    )

    index = read_jsonl(root / "index.jsonl")
    corrupt_entries = {
        item["run_id"]: item for item in index if item["run_id"] in {"bad-metadata-shape", "bad-event-shape"}
    }
    assert corrupt_entries["bad-metadata-shape"]["final_verdict"] == "CORRUPT"
    assert "metadata.json must contain a JSON object" in corrupt_entries["bad-metadata-shape"]["blocker"]
    assert corrupt_entries["bad-event-shape"]["final_verdict"] == "CORRUPT"
    assert "metrics.jsonl:1 must contain a JSON object" in corrupt_entries["bad-event-shape"]["blocker"]


def test_aggregate_survives_wrong_timestamp_type_in_historical_run(tmp_path: Path) -> None:
    root = tmp_path / "gandalf"
    corrupt = root / "runs" / "bad-timestamp-type"
    corrupt.mkdir(parents=True)
    (corrupt / "metadata.json").write_text('{"run_id": "bad-timestamp-type", "started_at": 1700000000}')
    (corrupt / "metrics.jsonl").write_text(
        '{"kind": "run_finished", "timestamp": "2026-06-19T00:00:00Z", '
        '"final_verdict": "APPROVED", "summary": "bad historical data"}\n'
    )

    run_metrics(
        root,
        "start",
        "--run-id",
        "healthy-run",
        "--repo",
        "omni",
        "--branch",
        "feature/auth",
        "--base-ref",
        "main",
        "--goal",
        "Review auth change",
    )

    index = read_jsonl(root / "index.jsonl")
    corrupt_entry = next(item for item in index if item["run_id"] == "bad-timestamp-type")
    healthy_entry = next(item for item in index if item["run_id"] == "healthy-run")
    assert corrupt_entry["final_verdict"] == "CORRUPT"
    assert "started_at must be an ISO timestamp string" in corrupt_entry["blocker"]
    assert healthy_entry["final_verdict"] == "IN_PROGRESS"


def test_progress_wrapper_records_duration_and_stdout_file(tmp_path: Path) -> None:
    output = tmp_path / "review.md"
    completed = subprocess.run(
        [
            sys.executable,
            str(PROGRESS_SCRIPT),
            "--label",
            "primary",
            "--expected-ms",
            "1000",
            "--no-progress",
            "--stdout-file",
            str(output),
            "--",
            sys.executable,
            "-c",
            "print('approved')",
        ],
        check=True,
        text=True,
        capture_output=True,
    )

    result = json.loads(completed.stdout)
    assert result["exit_code"] == 0
    assert result["stdout_file"] == str(output)
    assert result["duration_ms"] >= 0
    assert output.read_text().strip() == "approved"


def test_progress_wrapper_caps_waiting_bar_at_99_percent(tmp_path: Path) -> None:
    completed = subprocess.run(
        [
            sys.executable,
            str(PROGRESS_SCRIPT),
            "--label",
            "primary",
            "--expected-ms",
            "1",
            "--interval-ms",
            "5",
            "--width",
            "10",
            "--stdout-file",
            str(tmp_path / "out.txt"),
            "--",
            sys.executable,
            "-c",
            "import time; time.sleep(0.03)",
        ],
        check=True,
        text=True,
        capture_output=True,
    )

    assert " 99%" in completed.stderr
    assert "100%" in completed.stderr


def test_codex_fast_home_writes_task_local_fast_config(tmp_path: Path) -> None:
    auth_source = tmp_path / "auth.json"
    auth_source.write_text("{}")
    codex_home = tmp_path / "codex-home"

    completed = subprocess.run(
        [
            sys.executable,
            str(FAST_HOME_SCRIPT),
            "--output-dir",
            str(codex_home),
            "--auth-source",
            str(auth_source),
        ],
        check=True,
        text=True,
        capture_output=True,
    )

    result = json.loads(completed.stdout)
    config = (codex_home / "config.toml").read_text()
    assert result["env"]["CODEX_HOME"] == str(codex_home)
    assert result["reasoning_effort"] == "minimal"
    assert 'model_reasoning_effort = "minimal"' in config
    assert 'approval_policy = "never"' in config
    assert (codex_home / "auth.json").is_symlink()
    assert (codex_home / "auth.json").resolve() == auth_source


def git(repo: Path, *args: str) -> str:
    completed = subprocess.run(["git", "-C", str(repo), *args], check=True, text=True, capture_output=True)
    return completed.stdout.strip()


def commit_file(repo: Path, name: str, content: str, message: str) -> str:
    path = repo / name
    path.write_text(content)
    git(repo, "add", name)
    git(repo, "commit", "-m", message)
    return git(repo, "rev-parse", "HEAD")


def test_squash_helper_collapses_iteration_commits(tmp_path: Path) -> None:
    repo = tmp_path / "repo"
    repo.mkdir()
    git(repo, "init", "-b", "main")
    git(repo, "config", "user.email", "gandalf@example.test")
    git(repo, "config", "user.name", "Gandalf Test")
    commit_file(repo, "README.md", "base\n", "chore: base")
    git(repo, "checkout", "-b", "feature/gandalf")
    commit_file(repo, "one.txt", "one\n", "fix(gandalf): iteration 1 repair")
    commit_file(repo, "two.txt", "two\n", "fix(gandalf): iteration 2 repair")

    completed = subprocess.run(
        [
            sys.executable,
            str(SQUASH_SCRIPT),
            "--repo",
            str(repo),
            "--base-ref",
            "main",
            "--message",
            "feat(gandalf): final approval loop",
            "--body",
            "Why: collapse reviewed repair commits.\n\nWhat: preserve the approved final tree.",
        ],
        check=True,
        text=True,
        capture_output=True,
    )

    result = json.loads(completed.stdout)
    assert result["commit_count"] == 2
    assert result["no_op"] is False
    assert git(repo, "rev-list", "--count", "main..HEAD") == "1"
    assert git(repo, "log", "-1", "--pretty=%s") == "feat(gandalf): final approval loop"
    assert "Why: collapse reviewed repair commits." in git(repo, "log", "-1", "--pretty=%B")
    assert (repo / "one.txt").read_text() == "one\n"
    assert (repo / "two.txt").read_text() == "two\n"


def test_squash_helper_accepts_message_file(tmp_path: Path) -> None:
    repo = tmp_path / "repo"
    repo.mkdir()
    git(repo, "init", "-b", "main")
    git(repo, "config", "user.email", "gandalf@example.test")
    git(repo, "config", "user.name", "Gandalf Test")
    commit_file(repo, "README.md", "base\n", "chore: base")
    git(repo, "checkout", "-b", "feature/gandalf")
    commit_file(repo, "one.txt", "one\n", "fix(gandalf): iteration 1 repair")
    commit_file(repo, "two.txt", "two\n", "fix(gandalf): iteration 2 repair")
    message_file = tmp_path / "commit-message.txt"
    message_file.write_text(
        "feat(gandalf): final approval loop\n\n"
        "Why: preserve reviewability during fixes and ship one clean PR commit.\n\n"
        "What: squash iteration commits after both reviewers approve.\n"
    )

    completed = subprocess.run(
        [
            sys.executable,
            str(SQUASH_SCRIPT),
            "--repo",
            str(repo),
            "--base-ref",
            "main",
            "--message-file",
            str(message_file),
        ],
        check=True,
        text=True,
        capture_output=True,
    )

    result = json.loads(completed.stdout)
    assert result["commit_count"] == 2
    assert git(repo, "rev-list", "--count", "main..HEAD") == "1"
    assert git(repo, "log", "-1", "--pretty=%B") == message_file.read_text().strip()


def test_squash_helper_restores_head_when_commit_fails(tmp_path: Path) -> None:
    repo = tmp_path / "repo"
    repo.mkdir()
    git(repo, "init", "-b", "main")
    git(repo, "config", "user.email", "gandalf@example.test")
    git(repo, "config", "user.name", "Gandalf Test")
    commit_file(repo, "README.md", "base\n", "chore: base")
    git(repo, "checkout", "-b", "feature/gandalf")
    commit_file(repo, "one.txt", "one\n", "fix(gandalf): iteration 1 repair")
    commit_file(repo, "two.txt", "two\n", "fix(gandalf): iteration 2 repair")
    head_before = git(repo, "rev-parse", "HEAD")
    hook = repo / ".git/hooks/pre-commit"
    hook.write_text("#!/bin/sh\necho failing hook >&2\nexit 1\n")
    hook.chmod(0o755)

    completed = subprocess.run(
        [
            sys.executable,
            str(SQUASH_SCRIPT),
            "--repo",
            str(repo),
            "--base-ref",
            "main",
            "--message",
            "feat(gandalf): final approval loop",
        ],
        check=False,
        text=True,
        capture_output=True,
    )

    assert completed.returncode != 0
    assert "commit failed; restored HEAD" in completed.stderr
    assert git(repo, "rev-parse", "HEAD") == head_before
    assert git(repo, "rev-list", "--count", "main..HEAD") == "2"
    assert git(repo, "status", "--porcelain") == ""
