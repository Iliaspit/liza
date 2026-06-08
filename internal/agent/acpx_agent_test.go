package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestACPXAgentRunUsesPersistentCodexSession(t *testing.T) {
	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "acpx.log")
	writeFakeACPX(t, filepath.Join(binDir, "acpx"), logPath)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var events []LLMAgentEvent
	sink := LLMAgentEventFunc(func(_ context.Context, event LLMAgentEvent) {
		events = append(events, event)
	})

	agent := NewACPXAgent("")
	req := LLMAgentRunRequest{
		BackendName: "codex-acp",
		AgentID:     "coder-1",
		TaskID:      "task-acp",
		Prompt:      "implement the requested change",
		ProjectRoot: t.TempDir(),
		EventSink:   sink,
	}

	first, err := agent.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	if first.ExitCode != 0 {
		t.Fatalf("first ExitCode = %d, want 0", first.ExitCode)
	}
	if first.Output != "done from acpx" {
		t.Fatalf("first Output = %q, want fake acpx message", first.Output)
	}
	if first.SessionID != "liza-coder-1" {
		t.Fatalf("first SessionID = %q, want liza-coder-1", first.SessionID)
	}
	if first.WarmUsage {
		t.Fatal("first WarmUsage = true, want false")
	}
	wantUsage := LLMAgentUsage{InputTokens: 123, OutputTokens: 7, CachedReadTokens: 42}
	if first.Usage != wantUsage {
		t.Fatalf("first Usage = %+v, want %+v", first.Usage, wantUsage)
	}

	second, err := agent.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("second Run() error = %v", err)
	}
	if !second.WarmUsage {
		t.Fatal("second WarmUsage = false, want true")
	}

	log := readTextForTest(t, logPath)
	for _, want := range []string{
		"ENV_LIZA_AGENT_ID:coder-1",
		"ARGS:--cwd " + req.ProjectRoot + " codex sessions ensure --name liza-coder-1",
		"ARGS:--cwd " + req.ProjectRoot + " --format json --approve-all codex prompt -s liza-coder-1 --file -",
		"STDIN:implement the requested change",
	} {
		if !strings.Contains(log, want) {
			t.Fatalf("fake acpx log missing %q:\n%s", want, log)
		}
	}

	if !hasLLMAgentEvent(events, LLMAgentEventStarted) {
		t.Fatal("missing started event")
	}
	if !allLLMAgentEventsHaveTask(events, "task-acp") {
		t.Fatalf("events = %#v, want task-acp attribution", events)
	}
	if !hasLLMAgentEvent(events, LLMAgentEventMessage) {
		t.Fatal("missing message event")
	}
	if !hasLLMAgentEvent(events, LLMAgentEventUsage) {
		t.Fatal("missing usage event")
	}
	if !hasLLMAgentEvent(events, LLMAgentEventCompleted) {
		t.Fatal("missing completed event")
	}
}

func TestACPXAgentMasksReturnedOutputAndEvents(t *testing.T) {
	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "acpx.log")
	writeFakeACPX(t, filepath.Join(binDir, "acpx"), logPath)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ANTHROPIC_API_KEY", "sk-acpx-secret")

	var events []LLMAgentEvent
	sink := LLMAgentEventFunc(func(_ context.Context, event LLMAgentEvent) {
		events = append(events, event)
	})

	result, err := NewACPXAgent("").Run(context.Background(), LLMAgentRunRequest{
		BackendName: "codex-acp",
		AgentID:     "coder-1",
		TaskID:      "task-acp",
		Prompt:      "emit secret",
		ProjectRoot: t.TempDir(),
		EventSink:   sink,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.Contains(result.Output, "sk-acpx-secret") {
		t.Fatalf("Output leaked secret: %q", result.Output)
	}
	if !strings.Contains(result.Output, "***") {
		t.Fatalf("Output = %q, want masked secret placeholder", result.Output)
	}
	for _, event := range events {
		if strings.Contains(event.Message, "sk-acpx-secret") {
			t.Fatalf("event leaked secret: %#v", event)
		}
	}
}

func TestACPXAgentDetectsPersistedWarmSession(t *testing.T) {
	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "acpx.log")
	writeFakeACPX(t, filepath.Join(binDir, "acpx"), logPath)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_ACPX_SESSION_EXISTS", "1")

	result, err := NewACPXAgent("").Run(context.Background(), LLMAgentRunRequest{
		BackendName: "codex-acp",
		AgentID:     "coder-1",
		TaskID:      "task-acp",
		Prompt:      "implement the requested change",
		ProjectRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.WarmUsage {
		t.Fatal("WarmUsage = false, want true for existing ACPX session")
	}
}

func writeFakeACPX(t *testing.T, path, logPath string) {
	t.Helper()
	script := `#!/bin/sh
printf 'ARGS:%s\n' "$*" >> "` + logPath + `"
printf 'ENV_LIZA_AGENT_ID:%s\n' "$LIZA_AGENT_ID" >> "` + logPath + `"
case "$*" in
  *" sessions show "*)
    if [ "$FAKE_ACPX_SESSION_EXISTS" = "1" ]; then
      exit 0
    fi
    exit 2
    ;;
  *" sessions ensure "*)
    exit 0
    ;;
  *" prompt "*)
    prompt="$(cat)"
    printf 'STDIN:%s\n' "$prompt" >> "` + logPath + `"
    if [ "$prompt" = "emit secret" ]; then
      printf '%s\n' '{"params":{"update":{"sessionUpdate":"agent_message_chunk","content":{"text":"secret sk-acpx-secret"}}}}'
    else
      printf '%s\n' '{"params":{"update":{"sessionUpdate":"agent_message_chunk","content":{"text":"done from acpx"}}}}'
    fi
    printf '%s\n' '{"result":{"usage":{"inputTokens":123,"outputTokens":7,"cachedReadTokens":42}}}'
    exit 0
    ;;
  *)
    exit 2
    ;;
esac
`
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write fake acpx: %v", err)
	}
}

func readTextForTest(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func hasLLMAgentEvent(events []LLMAgentEvent, kind LLMAgentEventKind) bool {
	for _, event := range events {
		if event.Kind == kind {
			return true
		}
	}
	return false
}

func allLLMAgentEventsHaveTask(events []LLMAgentEvent, taskID string) bool {
	for _, event := range events {
		if event.TaskID != taskID {
			return false
		}
	}
	return true
}
