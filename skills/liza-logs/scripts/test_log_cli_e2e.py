from __future__ import annotations

import json
import re
import subprocess
import sys
from pathlib import Path
from typing import Any

import pytest

SCRIPTS_DIR = Path(__file__).parent
ANALYZE_LOG = SCRIPTS_DIR / "analyze-log.py"
QUERY_LOG = SCRIPTS_DIR / "query-log.py"
# The output path is representative; the fixed text and suffix structure come
# from the reported provider result whose raw log is not retained in the repo.
PROVIDER_BACKGROUND_RESULT_WITH_SUFFIX = (
    "Command did not complete within its 180s timeout and was moved to the background "
    "(ID: bdcgot1os). Output is being written to: /tmp/provider-output/bdcgot1os.log"
)
LARGE_RESULT = "UNIQUE-LARGE-RESULT " + ("x" * 5_000)


def _assistant(
    message_id: str,
    content: list[dict[str, Any]],
    *,
    timestamp: str | None = None,
    usage: dict[str, int] | None = None,
) -> dict[str, Any]:
    event: dict[str, Any] = {
        "type": "assistant",
        "message": {
            "id": message_id,
            "usage": (
                usage
                if usage is not None
                else {
                    "input_tokens": 1,
                    "cache_creation_input_tokens": 2,
                    "cache_read_input_tokens": 3,
                    "output_tokens": 4,
                }
            ),
            "content": content,
        },
    }
    if timestamp:
        event["timestamp"] = timestamp
    return event


def _tool_result(
    tool_use_id: str,
    content: str | list[dict[str, str]],
    *,
    timestamp: str | None = None,
) -> dict[str, Any]:
    event: dict[str, Any] = {
        "type": "user",
        "message": {
            "content": [
                {
                    "type": "tool_result",
                    "tool_use_id": tool_use_id,
                    "is_error": False,
                    "content": content,
                }
            ]
        },
    }
    if timestamp:
        event["timestamp"] = timestamp
    return event


def _complete_rich_events() -> list[dict[str, Any]]:
    events: list[dict[str, Any]] = [
        {
            "type": "system",
            "session_id": "complete-rich",
            "provider": "claude",
            "model": "claude-opus-4-1",
            "timestamp": "2026-08-16T12:00:00Z",
        },
        _assistant("outside-before", [{"type": "text", "text": "QUERY-OUTSIDE-BEFORE"}]),
    ]

    for index in range(45):
        tool_use_id = f"toolu_{index}"
        if index == 0:
            tool_name = "Bash"
            tool_input = {"command": "§BRAND_BINARY_NAME§ await-verdict task-111 --agent-id coder-1 --json"}
            result: str | list[dict[str, str]] = PROVIDER_BACKGROUND_RESULT_WITH_SUFFIX
            use_timestamp = "2026-08-16T12:00:00Z"
            result_timestamp = "2026-08-16T12:03:00Z"
        elif index == 1:
            tool_name = "Read"
            tool_input = {"file_path": "specs/large-result.md"}
            result = [{"type": "text", "text": LARGE_RESULT}]
            use_timestamp = None
            result_timestamp = None
        elif index == 2:
            tool_name = "Bash"
            tool_input = {"command": "nohup build >build.log 2>&1 &"}
            result = "started background job"
            use_timestamp = None
            result_timestamp = None
        else:
            tool_name = "Read"
            tool_input = {"file_path": f"specs/{index}.md"}
            text = f"result-{index}"
            result = text if index % 2 == 0 else [{"type": "text", "text": text}]
            use_timestamp = None
            result_timestamp = None

        events.extend(
            [
                _assistant(
                    f"tool-message-{index}",
                    [
                        {
                            "type": "tool_use",
                            "id": tool_use_id,
                            "name": tool_name,
                            "input": tool_input,
                        }
                    ],
                    timestamp=use_timestamp,
                ),
                _tool_result(tool_use_id, result, timestamp=result_timestamp),
            ]
        )
        if index == 0:
            events.append(
                _assistant(
                    "bounded-successor",
                    [{"type": "text", "text": "QUERY-BOUNDED-SUCCESSOR"}],
                )
            )
            events.append(
                _assistant(
                    "outside-after",
                    [{"type": "text", "text": "QUERY-OUTSIDE-AFTER"}],
                )
            )

    events.extend(
        [
            {
                "type": "user",
                "message": {
                    "content": [
                        {
                            "type": "text",
                            "text": "ordinary operator guidance",
                        }
                    ]
                },
            },
            _assistant(
                "meaningful-final",
                [{"type": "text", "text": "MEANINGFUL-FINAL-SUMMARY"}],
            ),
            _assistant("genuinely-empty", []),
            {
                "type": "result",
                "is_error": False,
                "usage": {
                    "input_tokens": 12_345,
                    "cache_creation_input_tokens": 23_456,
                    "cache_read_input_tokens": 34_567,
                    "output_tokens": 107_599,
                },
                "total_cost_usd": 8.0589,
            },
        ]
    )
    return events


def _incomplete_rich_events() -> list[dict[str, Any]]:
    return [
        {
            "type": "system",
            "session_id": "incomplete-rich",
            "provider": "opencode",
            "model": "gpt-5.4",
        },
        _assistant(
            "incomplete-final",
            [{"type": "text", "text": "INCOMPLETE-MEANINGFUL-FINAL"}],
            usage={
                "input_tokens": 10,
                "cache_creation_input_tokens": 20,
                "cache_read_input_tokens": 30,
                "output_tokens": 40,
            },
        ),
        _assistant("incomplete-empty", [], usage={}),
    ]


def _sparse_events() -> list[dict[str, Any]]:
    return [
        {"type": "thread.started", "thread_id": "sparse", "provider": "codex", "model": "gpt-5.4"},
        {"type": "turn.started"},
        {
            "type": "item.completed",
            "item": {"id": "empty-envelope", "type": "agent_message", "status": "completed", "text": ""},
        },
        {
            "type": "item.completed",
            "item": {
                "id": "tool",
                "type": "command_execution",
                "status": "completed",
                "command": "echo initialized",
                "aggregated_output": "initialized\n",
                "exit_code": 0,
            },
        },
        {"type": "turn.completed", "usage": {}},
        {"type": "turn.started"},
        {
            "type": "item.completed",
            "item": {
                "id": "breadcrumb",
                "type": "agent_message",
                "status": "completed",
                "text": "Secret words: Acme / MAS / Empowered / On-rails. Initialization complete.",
            },
        },
        {"type": "turn.completed", "usage": {}},
        {"type": "turn.started"},
        {
            "type": "item.completed",
            "item": {
                "id": "final",
                "type": "agent_message",
                "status": "completed",
                "text": "MEANINGFUL-SPARSE-FINAL",
            },
        },
        {"type": "turn.completed", "usage": {}},
        {"type": "turn.started"},
        {"type": "turn.completed", "usage": {}},
    ]


def _write_ndjson(path: Path, events: list[dict[str, Any]]) -> None:
    path.write_text(
        "\n".join(json.dumps(event) for event in events) + "\n",
        encoding="utf-8",
    )


@pytest.fixture
def rich_logs(tmp_path: Path) -> tuple[Path, Path]:
    complete = tmp_path / "coder-1-20260816-120000.txt"
    incomplete = tmp_path / "reviewer-1-20260816-120001.txt"
    _write_ndjson(complete, _complete_rich_events())
    _write_ndjson(incomplete, _incomplete_rich_events())
    return complete, incomplete


@pytest.fixture
def sparse_log(tmp_path: Path) -> Path:
    sparse = tmp_path / "coder-2-20260816-120002.txt"
    _write_ndjson(sparse, _sparse_events())
    return sparse


def _run_cli(script: Path, *args: str | Path) -> str:
    result = subprocess.run(
        [sys.executable, str(script), *(str(arg) for arg in args)],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, result.stderr
    assert result.stderr == ""
    return result.stdout


def _section(rendered: str, heading: str, next_heading: str) -> str:
    start = rendered.index(heading)
    return rendered[start : rendered.index(next_heading, start + len(heading))]


def test_analyze_complete_rich_session_renders_corrected_cli_facts(
    rich_logs: tuple[Path, Path],
) -> None:
    complete, _ = rich_logs

    rendered = _run_cli(ANALYZE_LOG, complete)
    token_summary = _section(rendered, "TOKEN SUMMARY", "CONTENT BREAKDOWN")
    content_breakdown = _section(
        rendered,
        "CONTENT BREAKDOWN",
        "TOP 10 ITEMS BY SIZE",
    )
    top_items = _section(rendered, "TOP 10 ITEMS BY SIZE", "TOOL USAGE")
    empty_turns = _section(rendered, "EMPTY TURNS", "SECRET WORDS")
    secret_words = _section(rendered, "SECRET WORDS", "PER-TURN CONTEXT GROWTH")
    friction = _section(rendered, "OPERATIONAL FRICTION", "TOKEN SUMMARY")

    assert "Total Input:          70.4K" in token_summary
    assert "fresh: 12.3K, cache_create: 23.5K, cache_read: 34.6K" in token_summary
    assert "Output:              107.6K" in token_summary
    assert "Usage Source:    terminal" in token_summary
    assert "$8.0589" in rendered

    tool_result_rows = re.findall(r"^\s*tool_result\s+45\s+5,573\s+", content_breakdown, re.MULTILINE)
    assert len(tool_result_rows) == 1
    assert re.search(r"^\s*text\s+5\s+", content_breakdown, re.MULTILINE)
    assert top_items.count("UNIQUE-LARGE-RESULT") == 1
    assert "ordinary operator guidance" not in content_breakdown

    assert "Events: 1" in friction
    assert friction.count("provider foreground timeout/backgrounding") == 1
    assert "Bash" in friction
    assert "await-verdict task-111" in friction
    assert "180.0s" in friction
    assert PROVIDER_BACKGROUND_RESULT_WITH_SUFFIX[:120] in friction
    assert complete.name in friction
    assert "PERMISSION & POLICY FRICTION" not in rendered

    assert "        50        45         4         1     2.00%" in empty_turns
    assert "genuinely-empty" in empty_turns
    assert "MEANINGFUL-FINAL-SUMMARY" not in empty_turns
    assert "Applicability: not required" in secret_words
    assert "Status: missing" not in secret_words


def test_analyze_incomplete_rich_session_labels_partial_and_missing_breadcrumbs(
    rich_logs: tuple[Path, Path],
) -> None:
    _, incomplete = rich_logs

    rendered = _run_cli(ANALYZE_LOG, incomplete)
    token_summary = _section(rendered, "TOKEN SUMMARY", "CONTENT BREAKDOWN")
    empty_turns = _section(rendered, "EMPTY TURNS", "SECRET WORDS")
    secret_words = _section(rendered, "SECRET WORDS", "PER-TURN CONTEXT GROWTH")

    assert re.search(r"Total Input:\s+60\b", token_summary)
    assert "fresh: 10, cache_create: 20, cache_read: 30" in token_summary
    assert re.search(r"Output:\s+40\b", token_summary)
    assert "Usage Source:    envelope-partial" in token_summary
    assert "$8.0589" not in rendered
    assert "         2         0         1         1    50.00%" in empty_turns
    assert "incomplete-empty" in empty_turns
    assert "INCOMPLETE-MEANINGFUL-FINAL" not in empty_turns
    assert "Applicability: required" in secret_words
    assert "Status: missing" in secret_words


def test_analyze_sparse_session_renders_outer_turns_and_initial_breadcrumb(
    sparse_log: Path,
) -> None:
    rendered = _run_cli(ANALYZE_LOG, sparse_log)
    empty_turns = _section(rendered, "EMPTY TURNS", "SECRET WORDS")
    secret_words = _section(rendered, "SECRET WORDS", "TURN TIMELINE")

    assert "Unit basis: API turns" in empty_turns
    assert "Text-only" in empty_turns
    assert "         4         1         2         1    25.00%" in empty_turns
    assert "MEANINGFUL-SPARSE-FINAL" not in empty_turns
    assert "Applicability: required" in secret_words
    assert "Found: Acme, MAS, Empowered, On-rails" in secret_words
    assert "Found: Acme, MAS, Empowered, On-rails, Initialization" not in secret_words
    assert "Status: missing" not in secret_words


def test_query_operational_friction_cli_returns_one_bounded_text_and_json_window(
    rich_logs: tuple[Path, Path],
) -> None:
    complete, _ = rich_logs
    arguments: tuple[str | Path, ...] = (
        complete,
        "--around-operational-friction",
        "1",
        "--task",
        "task-111",
        "--max-field",
        "200",
    )

    rendered = _run_cli(QUERY_LOG, *arguments)

    assert rendered.count("OPERATIONAL FRICTION CLUSTER") == 1
    assert f"log: {complete}" in rendered
    assert "-1 event" in rendered
    assert "+0 event" in rendered
    assert "+1 event" in rendered
    assert "Bash" in rendered
    assert "await-verdict task-111" in rendered
    assert PROVIDER_BACKGROUND_RESULT_WITH_SUFFIX in rendered
    assert "QUERY-BOUNDED-SUCCESSOR" in rendered
    assert "QUERY-OUTSIDE-BEFORE" not in rendered
    assert "QUERY-OUTSIDE-AFTER" not in rendered
    assert "nohup build" not in rendered
    assert "ERROR" not in rendered

    payload = json.loads(_run_cli(QUERY_LOG, *arguments, "--json"))

    assert payload["log"] == str(complete)
    assert len(payload["windows"]) == 1
    assert len(payload["windows"][0]) == 3
    predecessor, center, successor = payload["windows"][0]
    assert predecessor["kind"] == "tool_request"
    assert predecessor["label"] == "Bash"
    assert "await-verdict task-111" in predecessor["detail"]
    assert center["kind"] == "tool_result"
    assert center["operational_friction"] is True
    assert center["is_error"] is False
    assert center["result"] == PROVIDER_BACKGROUND_RESULT_WITH_SUFFIX
    assert successor["result"] == "QUERY-BOUNDED-SUCCESSOR"


def test_cli_ignores_successful_output_that_quotes_provider_wording(tmp_path: Path) -> None:
    path = tmp_path / "coder-1-20260816-120002.txt"
    _write_ndjson(
        path,
        [
            {"type": "system", "session_id": "quoted", "model": "claude-opus-4-1"},
            _assistant(
                "tool-message",
                [
                    {
                        "type": "tool_use",
                        "id": "toolu_quoted",
                        "name": "Bash",
                        "input": {"command": "gh issue view 111"},
                    }
                ],
            ),
            _tool_result(
                "toolu_quoted",
                (
                    f"Issue evidence quotes: {PROVIDER_BACKGROUND_RESULT_WITH_SUFFIX}. "
                    "This successful output is not a provider control message."
                ),
            ),
        ],
    )

    analyzed = _run_cli(ANALYZE_LOG, path)
    queried = _run_cli(QUERY_LOG, path, "--around-operational-friction", "1")

    assert "OPERATIONAL FRICTION" not in analyzed
    assert queried == ""


def test_summary_by_role_aggregates_mixed_usage_provenance_and_friction(
    rich_logs: tuple[Path, Path],
) -> None:
    complete, incomplete = rich_logs

    rendered = _run_cli(
        ANALYZE_LOG,
        "--summary-by-role",
        complete,
        incomplete,
    )

    assert "Logs:          2" in rendered
    assert "Usage Sources: envelope-partial:1, terminal:1" in rendered
    coder_row = next(line for line in rendered.splitlines() if line.strip().startswith("coder-1 "))
    reviewer_row = next(line for line in rendered.splitlines() if line.strip().startswith("reviewer-1 "))
    assert coder_row.split()[-1] == "0"
    assert reviewer_row.split()[-1] == "1"

    friction = rendered[rendered.index("OPERATIONAL FRICTION") :]
    assert "Events: 1 (1 categories)" in friction
    assert re.search(r"provider foreground timeout/backgrounding\s+1", friction)
    assert re.search(r"^\s*coder-1\s+1$", friction, re.MULTILINE)
    assert complete.name in friction
    assert "Bash" in friction
    assert "await-verdict task-111" in friction
    assert "Errors:        0" in rendered
