from __future__ import annotations

import importlib.util
import json
import sys
from pathlib import Path
from typing import Any


def load_analyzer() -> Any:
    path = Path(__file__).with_name("analyze-log.py")
    spec = importlib.util.spec_from_file_location("analyze_log", path)
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


def as_lines(*events: dict[str, Any]) -> list[str]:
    return [json.dumps(event) for event in events]


def test_sparse_message_and_command_in_one_turn_counts_as_tool_turn() -> None:
    analyzer = load_analyzer()

    report = analyzer.parse_sparse(
        as_lines(
            {"type": "thread.started", "thread_id": "t"},
            {"type": "turn.started"},
            {
                "type": "item.completed",
                "item": {"id": "item_1", "type": "agent_message", "status": "completed", "text": "thinking aloud"},
            },
            {
                "type": "item.completed",
                "item": {
                    "id": "item_2",
                    "type": "command_execution",
                    "status": "completed",
                    "command": "echo ok",
                    "aggregated_output": "ok\n",
                    "exit_code": 0,
                },
            },
            {"type": "turn.completed", "usage": {"input_tokens": 1, "cached_input_tokens": 0, "output_tokens": 1}},
        )
    )

    assert report.meta.num_turns == 1
    assert report.turn_units == 1
    assert report.tool_turn_units == 1
    assert report.empty_turns == []


def test_sparse_failed_turn_after_tool_item_does_not_add_empty_turn() -> None:
    analyzer = load_analyzer()

    report = analyzer.parse_sparse(
        as_lines(
            {"type": "thread.started", "thread_id": "t"},
            {"type": "turn.started"},
            {
                "type": "item.completed",
                "item": {
                    "id": "item_1",
                    "type": "command_execution",
                    "status": "failed",
                    "command": "false",
                    "aggregated_output": "",
                    "exit_code": 1,
                },
            },
            {"type": "turn.failed", "error": {"message": "command failed"}},
        )
    )

    assert report.meta.num_turns == 1
    assert report.turn_units == 1
    assert report.tool_turn_units == 1
    assert report.empty_turns == []


def test_sparse_failed_turn_without_completed_items_counts_as_empty_turn() -> None:
    analyzer = load_analyzer()

    report = analyzer.parse_sparse(
        as_lines(
            {"type": "thread.started", "thread_id": "t"},
            {"type": "turn.started"},
            {"type": "turn.failed", "error": {"message": "usage limit"}},
        )
    )

    assert report.meta.num_turns == 1
    assert report.turn_units == 1
    assert report.tool_turn_units == 0
    assert len(report.empty_turns) == 1
    assert report.empty_turns[0].item_type == "turn.failed"
    assert report.empty_turns[0].preview == "usage limit"


def test_sparse_tool_actions_keep_codex_turn_numbers() -> None:
    analyzer = load_analyzer()

    report = analyzer.parse_sparse(
        as_lines(
            {"type": "thread.started", "thread_id": "t"},
            {"type": "turn.started"},
            {
                "type": "item.completed",
                "item": {
                    "id": "item_1",
                    "type": "command_execution",
                    "status": "completed",
                    "command": "echo one",
                    "aggregated_output": "one\n",
                    "exit_code": 0,
                },
            },
            {
                "type": "item.completed",
                "item": {
                    "id": "item_2",
                    "type": "command_execution",
                    "status": "completed",
                    "command": "echo two",
                    "aggregated_output": "two\n",
                    "exit_code": 0,
                },
            },
            {"type": "turn.completed", "usage": {"input_tokens": 2, "cached_input_tokens": 0, "output_tokens": 2}},
        )
    )

    assert report.meta.num_turns == 1
    assert report.turn_units == 2
    assert report.tool_turn_units == 2
    assert [action.turn_num for action in report.actions] == [1, 2]


def test_sparse_single_outer_turn_counts_each_action_item_as_turn() -> None:
    analyzer = load_analyzer()
    events: list[dict[str, Any]] = [
        {"type": "thread.started", "thread_id": "t"},
        {"type": "turn.started"},
        {
            "type": "item.completed",
            "item": {"id": "item_0", "type": "agent_message", "status": "completed", "text": "starting"},
        },
    ]
    events.extend(
        {
            "type": "item.completed",
            "item": {
                "id": f"item_{i}",
                "type": "command_execution",
                "status": "completed",
                "command": f"echo {i}",
                "aggregated_output": f"{i}\n",
                "exit_code": 0,
            },
        }
        for i in range(1, 43)
    )
    events.append(
        {"type": "turn.completed", "usage": {"input_tokens": 1, "cached_input_tokens": 0, "output_tokens": 1}}
    )

    report = analyzer.parse_sparse(as_lines(*events))

    assert report.meta.num_turns == 1
    assert report.turn_units == 42
    assert report.tool_turn_units == 42
    assert report.empty_turns == []
    assert [action.turn_num for action in report.actions[-3:]] == [40, 41, 42]
