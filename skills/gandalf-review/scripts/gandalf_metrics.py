#!/usr/bin/env python3
"""Archive and aggregate gandalf-review review metrics."""

from __future__ import annotations

import argparse
import datetime as dt
import fcntl
import hashlib
import json
import os
import re
import shlex
import shutil
import subprocess
import sys
import tempfile
from collections.abc import Iterator
from contextlib import contextmanager
from pathlib import Path
from typing import Any

APP_DIR = "gandalf-review"
INDEX_FILE = "index.jsonl"
AGGREGATE_FILE = "aggregate.md"


def utc_now() -> str:
    return dt.datetime.now(dt.UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def default_root() -> Path:
    return Path.home() / ".liza" / APP_DIR


def slug(value: str, fallback: str = "run") -> str:
    normalized = re.sub(r"[^a-zA-Z0-9._-]+", "-", value.strip()).strip("-._")
    return normalized[:80] or fallback


def safe_run_id(value: str) -> str:
    return slug(value, "run")


def json_line(data: dict[str, Any]) -> str:
    return json.dumps(data, sort_keys=True, separators=(",", ":")) + "\n"


def write_text_atomic(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, tmp_name = tempfile.mkstemp(prefix=f".{path.name}.", suffix=".tmp", dir=path.parent)
    tmp_path = Path(tmp_name)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            handle.write(text)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(tmp_path, path)
    except Exception:
        tmp_path.unlink(missing_ok=True)
        raise


@contextmanager
def locked_archive(root: Path) -> Iterator[None]:
    root.mkdir(parents=True, exist_ok=True)
    lock_path = root / ".gandalf_metrics.lock"
    with lock_path.open("w", encoding="utf-8") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        try:
            yield
        finally:
            fcntl.flock(lock, fcntl.LOCK_UN)


def read_json(path: Path) -> dict[str, Any]:
    data = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(data, dict):
        raise TypeError(f"{path.name} must contain a JSON object")
    return data


def append_jsonl(path: Path, data: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("a", encoding="utf-8") as handle:
        handle.write(json_line(data))


def read_jsonl(path: Path) -> list[dict[str, Any]]:
    if not path.exists():
        return []
    events: list[dict[str, Any]] = []
    for line_number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        if line.strip():
            event = json.loads(line)
            if not isinstance(event, dict):
                raise TypeError(f"{path.name}:{line_number} must contain a JSON object")
            events.append(event)
    return events


def run_dir(root: Path, run_id: str) -> Path:
    return root / "runs" / safe_run_id(run_id)


def metrics_path(root: Path, run_id: str) -> Path:
    return run_dir(root, run_id) / "metrics.jsonl"


def metadata_path(root: Path, run_id: str) -> Path:
    return run_dir(root, run_id) / "metadata.json"


def summary_path(root: Path, run_id: str) -> Path:
    return run_dir(root, run_id) / "summary.md"


def make_run_id(repo: str, branch: str, started_at: str) -> str:
    digest = hashlib.sha256(f"{repo}\0{branch}\0{started_at}".encode()).hexdigest()[:8]
    stamp = started_at.replace("-", "").replace(":", "").replace("Z", "")
    return f"{stamp}-{slug(repo)}-{slug(branch)}-{digest}"


def copy_artifact(root: Path, run_id: str, args: argparse.Namespace) -> str | None:
    if not args.content_file:
        return None
    source = Path(args.content_file)
    if not source.is_file():
        raise SystemExit(f"content file does not exist: {source}")

    iteration = f"iteration-{args.iteration}" if args.iteration is not None else "run"
    artifact_name = args.artifact_name or f"{args.kind}.md"
    destination = run_dir(root, run_id) / "artifacts" / iteration / slug(artifact_name, "artifact.md")
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(source, destination)
    return str(destination)


def write_summary(root: Path, run_id: str) -> dict[str, Any]:
    run_id = safe_run_id(run_id)
    metadata = read_json(metadata_path(root, run_id))
    events = read_jsonl(metrics_path(root, run_id))
    entry = summarize_run(metadata, events, root, run_id)

    lines = [
        f"# Gandalf Review: {run_id}",
        "",
        f"- Repo: {entry.get('repo', '')}",
        f"- Branch: {entry.get('branch', '')}",
        f"- Base ref: {entry.get('base_ref', '')}",
        f"- Final verdict: {entry.get('final_verdict', 'IN_PROGRESS')}",
        f"- Iterations: {entry.get('iterations', 0)}",
        f"- Total duration: {entry.get('total_duration_ms', 0)} ms",
        f"- Review time: {entry.get('review_duration_ms', 0)} ms",
        f"- Fix time: {entry.get('fix_duration_ms', 0)} ms",
        f"- Validation time: {entry.get('validation_duration_ms', 0)} ms",
        "",
        "## Recap",
        "",
        entry.get("recap") or "No recap recorded.",
        "",
        "## Timeline",
        "",
    ]
    for event in events:
        detail = event.get("summary") or event.get("verdict") or event.get("duration_kind") or ""
        iteration = event.get("iteration")
        suffix = f" iteration={iteration}" if iteration is not None else ""
        lines.append(f"- {event['timestamp']} `{event['kind']}`{suffix} {detail}".rstrip())
    lines.append("")

    write_text_atomic(summary_path(root, run_id), "\n".join(lines))
    return entry


def summarize_run(metadata: dict[str, Any], events: list[dict[str, Any]], root: Path, run_id: str) -> dict[str, Any]:
    run_id = safe_run_id(run_id)
    started_at = metadata.get("started_at")
    finished = next((event for event in reversed(events) if event["kind"] in {"run_finished", "run_blocked"}), None)
    ended_at = finished.get("timestamp") if finished else None
    iterations = 0
    review_duration_ms = 0
    fix_duration_ms = 0
    validation_duration_ms = 0

    for event in events:
        if isinstance(event.get("iteration"), int):
            iterations = max(iterations, event["iteration"])
        if not isinstance(event.get("duration_ms"), int):
            continue
        duration_kind = event.get("duration_kind")
        if duration_kind == "review":
            review_duration_ms += event["duration_ms"]
        elif duration_kind == "fix":
            fix_duration_ms += event["duration_ms"]
        elif duration_kind == "validation":
            validation_duration_ms += event["duration_ms"]

    total_duration_ms = None
    if started_at is not None and not isinstance(started_at, str):
        raise TypeError("started_at must be an ISO timestamp string")
    if ended_at is not None and not isinstance(ended_at, str):
        raise TypeError("ended_at must be an ISO timestamp string")
    if started_at and ended_at:
        total_duration_ms = int(
            (
                dt.datetime.fromisoformat(ended_at.replace("Z", "+00:00"))
                - dt.datetime.fromisoformat(started_at.replace("Z", "+00:00"))
            ).total_seconds()
            * 1000
        )

    return {
        "run_id": run_id,
        "repo": metadata.get("repo"),
        "branch": metadata.get("branch"),
        "base_ref": metadata.get("base_ref"),
        "goal": metadata.get("goal"),
        "started_at": started_at,
        "ended_at": ended_at,
        "total_duration_ms": total_duration_ms,
        "iterations": iterations,
        "final_verdict": finished.get("final_verdict") if finished else "IN_PROGRESS",
        "blocker": finished.get("blocker") if finished else None,
        "review_duration_ms": review_duration_ms,
        "fix_duration_ms": fix_duration_ms,
        "validation_duration_ms": validation_duration_ms,
        "recap": finished.get("summary") if finished else metadata.get("goal"),
        "artifact_path": str(run_dir(root, run_id)),
        "summary_path": str(summary_path(root, run_id)),
    }


def corrupt_run_entry(run_id: str, child: Path, error: Exception) -> dict[str, Any]:
    return {
        "run_id": safe_run_id(run_id),
        "repo": None,
        "branch": None,
        "base_ref": None,
        "goal": None,
        "started_at": None,
        "ended_at": None,
        "total_duration_ms": None,
        "iterations": 0,
        "final_verdict": "CORRUPT",
        "blocker": f"cannot read run metrics: {error}",
        "review_duration_ms": 0,
        "fix_duration_ms": 0,
        "validation_duration_ms": 0,
        "recap": "Run metrics are corrupt; inspect artifact directory.",
        "artifact_path": str(child),
        "summary_path": str(child / "summary.md"),
    }


def write_aggregate(root: Path) -> list[dict[str, Any]]:
    entries: list[dict[str, Any]] = []
    runs_root = root / "runs"
    if runs_root.exists():
        for child in sorted(runs_root.iterdir()):
            if not child.is_dir():
                continue
            metadata_file = child / "metadata.json"
            metrics_file = child / "metrics.jsonl"
            if not metadata_file.exists() or not metrics_file.exists():
                continue
            run_id = safe_run_id(child.name)
            try:
                metadata = read_json(metadata_file)
                events = read_jsonl(metrics_file)
                entry = summarize_run(metadata, events, root, run_id)
                summary_path(root, run_id).parent.mkdir(parents=True, exist_ok=True)
                write_summary(root, run_id)
            except (OSError, json.JSONDecodeError, KeyError, TypeError, ValueError) as error:
                entries.append(corrupt_run_entry(run_id, child, error))
                continue
            entries.append(entry)

    root.mkdir(parents=True, exist_ok=True)
    write_text_atomic(root / INDEX_FILE, "".join(json_line(entry) for entry in entries))

    completed = [entry for entry in entries if entry["final_verdict"] in {"APPROVED", "BLOCKED", "STOPPED"}]
    approved = [entry for entry in completed if entry["final_verdict"] == "APPROVED"]
    blocked = [entry for entry in completed if entry["final_verdict"] == "BLOCKED"]
    corrupt = [entry for entry in entries if entry["final_verdict"] == "CORRUPT"]
    total_review = sum(entry["review_duration_ms"] for entry in completed)
    total_fix = sum(entry["fix_duration_ms"] for entry in completed)
    total_iterations = sum(entry["iterations"] for entry in completed)
    aggregate = [
        "# Gandalf Review Aggregate",
        "",
        f"- Runs: {len(entries)}",
        f"- Completed/blocked runs: {len(completed)}",
        f"- Approved runs: {len(approved)}",
        f"- Blocked runs: {len(blocked)}",
        f"- Corrupt runs: {len(corrupt)}",
        f"- Total iterations: {total_iterations}",
        f"- Total review time: {total_review} ms",
        f"- Total fix time: {total_fix} ms",
        "",
        "## Runs",
        "",
    ]
    for entry in entries:
        aggregate.append(
            f"- `{entry['run_id']}` {entry['final_verdict']} "
            f"iterations={entry['iterations']} review_ms={entry['review_duration_ms']} "
            f"fix_ms={entry['fix_duration_ms']}"
        )
    aggregate.append("")
    write_text_atomic(root / AGGREGATE_FILE, "\n".join(aggregate))
    return entries


def exporter_failure_message(error: Exception) -> str:
    if isinstance(error, subprocess.CalledProcessError):
        return f"exit {error.returncode}"
    if isinstance(error, subprocess.TimeoutExpired):
        return "timed out"
    return str(error)


def export_event(root: Path, run_id: str, event: dict[str, Any], *, no_export: bool) -> None:
    if no_export:
        return
    command = os.environ.get("GANDALF_REVIEW_EXPORT_CMD")
    if not command:
        return

    export_dir = run_dir(root, run_id) / "export"
    export_dir.mkdir(parents=True, exist_ok=True)
    event_path = export_dir / "latest_event.json"
    write_text_atomic(event_path, json.dumps(event, indent=2, sort_keys=True) + "\n")

    env = os.environ.copy()
    env.update(
        {
            "GANDALF_REVIEW_EVENT_PATH": str(event_path),
            "GANDALF_REVIEW_INDEX_PATH": str(root / INDEX_FILE),
            "GANDALF_REVIEW_AGGREGATE_PATH": str(root / AGGREGATE_FILE),
            "GANDALF_REVIEW_RUN_SUMMARY_PATH": str(summary_path(root, run_id)),
            "GANDALF_REVIEW_RUN_DIR": str(run_dir(root, run_id)),
            "GANDALF_REVIEW_RUN_ID": run_id,
        }
    )
    try:
        subprocess.run(shlex.split(command), check=True, env=env, text=True, timeout=60)
    except (FileNotFoundError, OSError, subprocess.CalledProcessError, subprocess.TimeoutExpired) as error:
        print(
            f"warning: GANDALF_REVIEW_EXPORT_CMD failed: {command!r}: {exporter_failure_message(error)}",
            file=sys.stderr,
        )


def command_start(args: argparse.Namespace) -> None:
    root = Path(args.root).expanduser()
    started_at = utc_now()
    run_id = safe_run_id(args.run_id) if args.run_id else make_run_id(args.repo, args.branch, started_at)
    directory = run_dir(root, run_id)

    metadata = {
        "run_id": run_id,
        "repo": args.repo,
        "branch": args.branch,
        "base_ref": args.base_ref,
        "goal": args.goal,
        "started_at": started_at,
    }
    event = {"kind": "run_started", "timestamp": started_at, "run_id": run_id, "summary": args.goal}
    with locked_archive(root):
        try:
            directory.mkdir(parents=True, exist_ok=False)
        except FileExistsError:
            raise SystemExit(f"run id already exists: {run_id}") from None
        write_text_atomic(metadata_path(root, run_id), json.dumps(metadata, indent=2, sort_keys=True) + "\n")
        append_jsonl(metrics_path(root, run_id), event)
        write_summary(root, run_id)
        write_aggregate(root)
    export_event(root, run_id, event, no_export=args.no_export)
    print(json.dumps({"run_id": run_id, "run_dir": str(directory)}, sort_keys=True))


def command_record(args: argparse.Namespace) -> None:
    root = Path(args.root).expanduser()
    run_id = safe_run_id(args.run_id)
    event = {
        "kind": args.kind,
        "timestamp": utc_now(),
        "run_id": run_id,
    }
    for field in ("iteration", "reviewer", "verdict", "duration_kind", "duration_ms", "summary", "commit"):
        value = getattr(args, field)
        if value is not None:
            event[field] = value
    with locked_archive(root):
        if not metadata_path(root, run_id).exists():
            raise SystemExit(f"unknown run id: {run_id}")
        artifact_path = copy_artifact(root, run_id, args)
        if artifact_path:
            event["artifact_path"] = artifact_path
        append_jsonl(metrics_path(root, run_id), event)
        write_summary(root, run_id)
        write_aggregate(root)
    export_event(root, run_id, event, no_export=args.no_export)
    print(json.dumps(event, sort_keys=True))


def command_finish(args: argparse.Namespace) -> None:
    root = Path(args.root).expanduser()
    run_id = safe_run_id(args.run_id)
    kind = "run_blocked" if args.final_verdict == "BLOCKED" else "run_finished"
    event = {
        "kind": kind,
        "timestamp": utc_now(),
        "run_id": run_id,
        "final_verdict": args.final_verdict,
        "summary": args.summary,
    }
    if args.blocker:
        event["blocker"] = args.blocker
    with locked_archive(root):
        if not metadata_path(root, run_id).exists():
            raise SystemExit(f"unknown run id: {run_id}")
        append_jsonl(metrics_path(root, run_id), event)
        entry = write_summary(root, run_id)
        write_aggregate(root)
    export_event(root, run_id, event, no_export=args.no_export)
    print(json.dumps(entry, sort_keys=True))


def command_aggregate(args: argparse.Namespace) -> None:
    root = Path(args.root).expanduser()
    with locked_archive(root):
        entries = write_aggregate(root)
    print(json.dumps({"runs": len(entries), "index": str(root / INDEX_FILE)}, sort_keys=True))


def parser() -> argparse.ArgumentParser:
    base = argparse.ArgumentParser(description=__doc__)
    base.add_argument("--root", default=str(default_root()), help="metrics root directory")
    subcommands = base.add_subparsers(dest="command", required=True)

    start = subcommands.add_parser("start", help="start a new review loop run")
    start.add_argument("--run-id")
    start.add_argument("--repo", required=True)
    start.add_argument("--branch", required=True)
    start.add_argument("--base-ref", required=True)
    start.add_argument("--goal", required=True)
    start.add_argument("--no-export", action="store_true")
    start.set_defaults(func=command_start)

    record = subcommands.add_parser("record", help="record one event or artifact")
    record.add_argument("--run-id", required=True)
    record.add_argument("--kind", required=True)
    record.add_argument("--iteration", type=int)
    record.add_argument("--reviewer", choices=["primary", "adversarial", "implementer"])
    record.add_argument("--verdict")
    record.add_argument("--duration-kind", choices=["review", "fix", "validation", "other"])
    record.add_argument("--duration-ms", type=int)
    record.add_argument("--summary")
    record.add_argument("--commit")
    record.add_argument("--content-file")
    record.add_argument("--artifact-name")
    record.add_argument("--no-export", action="store_true")
    record.set_defaults(func=command_record)

    finish = subcommands.add_parser("finish", help="finish or block a run")
    finish.add_argument("--run-id", required=True)
    finish.add_argument("--final-verdict", choices=["APPROVED", "BLOCKED", "STOPPED"], required=True)
    finish.add_argument("--summary", required=True)
    finish.add_argument("--blocker")
    finish.add_argument("--no-export", action="store_true")
    finish.set_defaults(func=command_finish)

    aggregate = subcommands.add_parser("aggregate", help="rebuild the global aggregate")
    aggregate.set_defaults(func=command_aggregate)
    return base


def main(argv: list[str] | None = None) -> int:
    args = parser().parse_args(argv)
    args.func(args)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
