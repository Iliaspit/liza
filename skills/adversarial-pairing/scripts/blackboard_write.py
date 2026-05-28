#!/usr/bin/env python3
"""Locked compare-and-swap writer for adversarial-pairing Markdown blackboards."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import socket
import sys
import tempfile
import time
from pathlib import Path
from typing import BinaryIO

try:
    import fcntl
except ImportError:  # pragma: no cover - adversarial-pairing is Unix-like only today.
    fcntl = None  # type: ignore[assignment]


EXIT_LOCK_TIMEOUT = 2
EXIT_STALE_CONTENT = 3
EXIT_USAGE = 4


def sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def read_bytes_if_exists(path: Path) -> bytes | None:
    try:
        return path.read_bytes()
    except FileNotFoundError:
        return None


def acquire_flock(lock_file: BinaryIO, timeout: float) -> None:
    if fcntl is None:
        raise RuntimeError("fcntl.flock is unavailable on this platform")

    deadline = time.monotonic() + timeout
    while True:
        try:
            fcntl.flock(lock_file.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
            return
        except BlockingIOError:
            if time.monotonic() >= deadline:
                raise TimeoutError(f"lock unavailable after {timeout:g}s")
            time.sleep(0.05)


def write_owner_metadata(owner_path: Path, operation: str) -> None:
    metadata = {
        "version": 1,
        "pid": os.getpid(),
        "hostname": socket.gethostname(),
        "operation": operation,
        "acquired_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
    }
    try:
        owner_path.write_text(json.dumps(metadata, indent=2) + "\n", encoding="utf-8")
    except OSError as err:
        print(f"warning: failed to write lock owner metadata {owner_path}: {err}", file=sys.stderr)
        return


def atomic_replace(path: Path, data: bytes) -> None:
    directory = path.parent
    with tempfile.NamedTemporaryFile(prefix=f".{path.name}.tmp-", dir=directory, delete=False) as tmp:
        tmp.write(data)
        tmp.flush()
        os.fsync(tmp.fileno())
        tmp_path = Path(tmp.name)

    try:
        os.replace(tmp_path, path)
    except Exception:
        tmp_path.unlink(missing_ok=True)
        raise


def write_blackboard(
    path: Path,
    content_file: Path,
    expect_sha256: str | None,
    create_if_missing: bool,
    operation: str,
    timeout: float,
) -> dict[str, str | bool]:
    lock_path = Path(f"{path}.lock")
    owner_path = Path(f"{path}.lock.owner.json")

    if not path.parent.exists():
        raise FileNotFoundError(f"parent directory does not exist: {path.parent}")

    with lock_path.open("a+b") as lock_file:
        acquire_flock(lock_file, timeout)
        write_owner_metadata(owner_path, operation)
        new_data = content_file.read_bytes()

        current_data = read_bytes_if_exists(path)
        if current_data is None:
            if not create_if_missing:
                raise FileNotFoundError(f"blackboard does not exist: {path}")
            old_hash = ""
        else:
            old_hash = sha256_bytes(current_data)
            if expect_sha256 is None and create_if_missing:
                raise FileExistsError(f"blackboard already exists: {path}")

        if expect_sha256 is not None and old_hash != expect_sha256:
            raise ValueError(f"stale blackboard content: expected {expect_sha256}, found {old_hash}")

        atomic_replace(path, new_data)
        return {
            "path": str(path),
            "created": current_data is None,
            "old_sha256": old_hash,
            "new_sha256": sha256_bytes(new_data),
            "operation": operation,
        }


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--path", required=True, type=Path, help="Markdown blackboard path to write")
    parser.add_argument(
        "--content-file",
        required=True,
        type=Path,
        help="File containing the complete new blackboard content",
    )
    parser.add_argument("--operation", required=True, help="Short operation name for lock owner diagnostics")
    parser.add_argument("--expect-sha256", help="Expected current blackboard SHA-256 before writing")
    parser.add_argument("--create-if-missing", action="store_true", help="Allow creating --path when it does not exist")
    parser.add_argument("--timeout", type=float, default=10.0, help="Seconds to wait for the lock")
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    if args.expect_sha256 is not None and args.create_if_missing:
        print("error: --create-if-missing and --expect-sha256 are mutually exclusive", file=sys.stderr)
        return EXIT_USAGE
    if args.expect_sha256 is None and not args.create_if_missing:
        print("error: --expect-sha256 is required unless --create-if-missing is set", file=sys.stderr)
        return EXIT_USAGE

    try:
        result = write_blackboard(
            path=args.path,
            content_file=args.content_file,
            expect_sha256=args.expect_sha256,
            create_if_missing=args.create_if_missing,
            operation=args.operation,
            timeout=args.timeout,
        )
    except TimeoutError as err:
        print(f"error: {err}", file=sys.stderr)
        return EXIT_LOCK_TIMEOUT
    except ValueError as err:
        print(f"error: {err}", file=sys.stderr)
        return EXIT_STALE_CONTENT
    except Exception as err:
        print(f"error: {err}", file=sys.stderr)
        return 1

    print(json.dumps(result, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
