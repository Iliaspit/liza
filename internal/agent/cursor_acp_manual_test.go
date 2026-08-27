//go:build manual

package agent

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCursorACPRealSmoke(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, err := NewACPXAgent("").Run(ctx, LLMAgentRunRequest{
		BackendName: "cursor-acp",
		AgentID:     "cursor-smoke",
		TaskID:      "cursor-acp-real-smoke",
		ProjectRoot: ".",
		Prompt:      `This is a smoke test through Liza's ACPX Cursor backend. Do not modify files or run tools. Reply exactly: CURSOR_ACP_REAL_OK`,
		LaunchGate:  immediateLaunchGate,
	})
	if err != nil {
		t.Fatalf("Cursor ACP smoke failed: %v\nOutput:\n%s", err, result.Output)
	}
	if result.ExitCode != 0 {
		if qe := DetectQuotaExhaustion(result.Output, "cursor-acp"); qe != nil {
			t.Skipf("Cursor ACP reached the account but Cursor refused the turn: %s", qe.Message)
		}
		t.Fatalf("ExitCode = %d, want 0\nOutput:\n%q", result.ExitCode, result.Output)
	}
	if !strings.Contains(result.Output, "CURSOR_ACP_REAL_OK") {
		t.Fatalf("Output = %q, want CURSOR_ACP_REAL_OK", result.Output)
	}
}
