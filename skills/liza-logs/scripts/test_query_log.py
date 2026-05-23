from __future__ import annotations

import importlib.util
import json
import sys
from pathlib import Path
from typing import Any


def load_query_log() -> Any:
    path = Path(__file__).with_name("query-log.py")
    spec = importlib.util.spec_from_file_location("query_log", path)
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


def as_lines(*events: dict[str, Any]) -> str:
    return "\n".join(json.dumps(event) for event in events)


def write_log(tmp_path: Path, *events: dict[str, Any]) -> Path:
    path = tmp_path / "agent.txt"
    path.write_text(as_lines(*events), encoding="utf-8")
    return path


def test_codex_sparse_around_errors_preserves_local_sequence(tmp_path: Path) -> None:
    query_log = load_query_log()
    path = write_log(
        tmp_path,
        {"type": "thread.started", "thread_id": "t"},
        {"type": "turn.started"},
        {
            "type": "item.completed",
            "item": {"type": "agent_message", "status": "completed", "text": "checking task-1"},
        },
        {
            "type": "item.completed",
            "item": {
                "type": "command_execution",
                "status": "completed",
                "command": "rtk rg task-1 specs",
                "aggregated_output": "match\n",
                "exit_code": 0,
            },
        },
        {
            "type": "item.completed",
            "item": {
                "type": "command_execution",
                "status": "completed",
                "command": "rtk go test ./internal/ops",
                "aggregated_output": "boom\n",
                "exit_code": 1,
            },
        },
        {
            "type": "item.completed",
            "item": {"type": "agent_message", "status": "completed", "text": "inspecting failure"},
        },
    )

    events = query_log.parse_events(path)
    windows = query_log.windows_around_errors(events, 2, "task-1")
    rendered = query_log.render_error_windows(path, windows, 200)

    assert len(windows) == 1
    assert "agent_message" in rendered
    assert "rtk rg" in rendered
    assert "rtk go" in rendered
    assert "boom" in rendered


def test_consecutive_errors_are_merged_into_one_cluster(tmp_path: Path) -> None:
    query_log = load_query_log()
    path = write_log(
        tmp_path,
        {"type": "thread.started", "thread_id": "t"},
        {
            "type": "item.completed",
            "item": {
                "type": "command_execution",
                "status": "completed",
                "command": "rtk go test ./first",
                "aggregated_output": "first failed\n",
                "exit_code": 1,
            },
        },
        {
            "type": "item.completed",
            "item": {
                "type": "command_execution",
                "status": "completed",
                "command": "rtk go test ./second",
                "aggregated_output": "second failed\n",
                "exit_code": 1,
            },
        },
        {
            "type": "item.completed",
            "item": {
                "type": "command_execution",
                "status": "completed",
                "command": "rtk go test ./third",
                "aggregated_output": "third passed\n",
                "exit_code": 0,
            },
        },
    )

    events = query_log.parse_events(path)
    windows = query_log.windows_around_errors(events, 0, None)
    rendered = query_log.render_error_windows(path, windows, 200)

    assert len(windows) == 1
    assert [event.index for event in windows[0]] == [1, 2]
    assert "ERROR CLUSTER 1" in rendered
    assert "errors: 1:rtk go, 2:rtk go" in rendered
    assert "third passed" not in rendered


def test_sparse_rtk_rg_exit_one_is_not_an_error(tmp_path: Path) -> None:
    query_log = load_query_log()
    path = write_log(
        tmp_path,
        {"type": "thread.started", "thread_id": "t"},
        {
            "type": "item.completed",
            "item": {
                "type": "command_execution",
                "status": "completed",
                "command": 'rtk rg -n "missing" internal',
                "aggregated_output": "",
                "exit_code": 1,
            },
        },
        {
            "type": "item.completed",
            "item": {
                "type": "command_execution",
                "status": "completed",
                "command": "rtk go test ./internal/ops",
                "aggregated_output": "boom\n",
                "exit_code": 1,
            },
        },
    )

    events = query_log.parse_events(path)
    windows = query_log.windows_around_errors(events, 0, None)

    assert len(windows) == 1
    assert windows[0][0].label == "rtk go"


def test_claude_rich_around_errors_correlates_tool_use_result(tmp_path: Path) -> None:
    query_log = load_query_log()
    path = write_log(
        tmp_path,
        {"type": "system", "session_id": "s", "model": "claude"},
        {
            "type": "assistant",
            "message": {
                "id": "m1",
                "content": [
                    {"type": "text", "text": "checking task-2"},
                    {"type": "tool_use", "id": "toolu_1", "name": "Bash", "input": {"command": "rtk go test ./..."}},
                ],
            },
        },
        {
            "type": "user",
            "message": {
                "content": [
                    {
                        "type": "tool_result",
                        "tool_use_id": "toolu_1",
                        "is_error": True,
                        "content": "failure for task-2\n" + ("x" * 1000),
                    }
                ]
            },
        },
        {
            "type": "assistant",
            "message": {"id": "m2", "content": [{"type": "text", "text": "next diagnostic"}]},
        },
    )

    events = query_log.parse_events(path)
    windows = query_log.windows_around_errors(events, 2, "task-2")
    rendered = query_log.render_error_windows(path, windows, 120)

    assert len(windows) == 1
    assert "checking task-2" in rendered
    assert "Bash" in rendered
    assert "failure for task-2" in rendered
    assert "<trimmed" in rendered
    assert "next diagnostic" in rendered


def test_json_output_is_trimmed(tmp_path: Path) -> None:
    query_log = load_query_log()
    path = write_log(
        tmp_path,
        {"type": "thread.started", "thread_id": "t"},
        {
            "type": "item.completed",
            "item": {
                "type": "command_execution",
                "status": "completed",
                "command": "rtk go test ./internal/ops",
                "aggregated_output": "prefix " + ("middle " * 100) + " suffix",
                "exit_code": 1,
            },
        },
    )

    events = query_log.parse_events(path)
    windows = query_log.windows_around_errors(events, 0, None)
    payload = json.loads(query_log.render_json(path, windows, 80))

    assert payload["windows"][0][0]["is_error"] is True
    assert "<trimmed" in payload["windows"][0][0]["result"]
