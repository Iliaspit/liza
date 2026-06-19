#!/usr/bin/env python3
"""Run a command with an ETA-style progress bar for Gandalf review waits."""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
import time
from pathlib import Path
from typing import TextIO


def progress_line(label: str, elapsed_ms: int, expected_ms: int, width: int) -> str:
    if expected_ms <= 0:
        percent = 0
    else:
        percent = min(99, int((elapsed_ms / expected_ms) * 100))
    filled = int((percent / 100) * width)
    bar = "#" * filled + "-" * (width - filled)
    return f"\r{label} [{bar}] {percent:>3}% {elapsed_ms}ms"


def run_with_progress(args: argparse.Namespace) -> int:
    stdout_handle: TextIO | None = None
    stderr_handle: TextIO | None = None
    try:
        if args.stdout_file:
            stdout_path = Path(args.stdout_file)
            stdout_path.parent.mkdir(parents=True, exist_ok=True)
            stdout_handle = stdout_path.open("w", encoding="utf-8")
        if args.stderr_file:
            stderr_path = Path(args.stderr_file)
            stderr_path.parent.mkdir(parents=True, exist_ok=True)
            stderr_handle = stderr_path.open("w", encoding="utf-8")

        started = time.monotonic()
        process = subprocess.Popen(args.command, stdout=stdout_handle, stderr=stderr_handle)
        while process.poll() is None:
            if not args.no_progress:
                elapsed_ms = int((time.monotonic() - started) * 1000)
                sys.stderr.write(progress_line(args.label, elapsed_ms, args.expected_ms, args.width))
                sys.stderr.flush()
            time.sleep(args.interval_ms / 1000)

        duration_ms = int((time.monotonic() - started) * 1000)
        if not args.no_progress:
            sys.stderr.write(f"\r{args.label} [{'#' * args.width}] 100% {duration_ms}ms\n")
            sys.stderr.flush()

        result = {
            "command": args.command,
            "duration_ms": duration_ms,
            "exit_code": process.returncode,
            "stderr_file": args.stderr_file,
            "stdout_file": args.stdout_file,
        }
        print(json.dumps(result, sort_keys=True))
        return process.returncode or 0
    finally:
        if stdout_handle:
            stdout_handle.close()
        if stderr_handle:
            stderr_handle.close()


def parser() -> argparse.ArgumentParser:
    base = argparse.ArgumentParser(description=__doc__)
    base.add_argument("--label", default="review", help="label shown before the progress bar")
    base.add_argument("--expected-ms", type=int, required=True, help="expected duration before the bar reaches 99%%")
    base.add_argument("--interval-ms", type=int, default=500, help="progress refresh interval")
    base.add_argument("--width", type=int, default=24, help="bar width in terminal cells")
    base.add_argument("--stdout-file", help="write command stdout to this file")
    base.add_argument("--stderr-file", help="write command stderr to this file")
    base.add_argument("--no-progress", action="store_true", help="suppress the terminal progress bar")
    base.add_argument("command", nargs=argparse.REMAINDER, help="command to run, preceded by --")
    return base


def main(argv: list[str] | None = None) -> int:
    args = parser().parse_args(argv)
    if args.command and args.command[0] == "--":
        args.command = args.command[1:]
    if not args.command:
        raise SystemExit("missing command after --")
    return run_with_progress(args)


if __name__ == "__main__":
    raise SystemExit(main())
