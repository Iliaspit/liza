package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestAgentHelpListsAllRuntimeRoles(t *testing.T) {
	helpText := agentCmd.Long

	required := []string{
		"coder",
		"code-reviewer",
		"orchestrator",
		"code-planner",
		"code-plan-reviewer",
	}

	for _, role := range required {
		if !strings.Contains(helpText, role) {
			t.Fatalf("agent help missing role %q", role)
		}
	}
}

func TestRepairAgentPoolHelpDescribesClaimEligibleReviewerCapacity(t *testing.T) {
	resetRootCmdForTest(t)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"repair-agent-pool", "--help"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("repair-agent-pool --help failed: %v\n%s", err, out.String())
	}

	help := out.String()
	for _, required := range []string{
		"reviewer work",
		"existing claim filters",
		"prior-approval",
		"provider-diversity",
	} {
		if !strings.Contains(help, required) {
			t.Errorf("repair-agent-pool --help missing %q:\n%s", required, help)
		}
	}
}

func TestContractWarningInitCommandUsesCLIName(t *testing.T) {
	cliName := "cursor-acp"
	contractKey := "codex"

	if got := contractInitCommandForMissingContract(cliName, contractKey); got != "liza init --cursor" {
		t.Fatalf("contractInitCommandForMissingContract(%q, %q) = %q, want liza init --cursor", cliName, contractKey, got)
	}
}

func TestContractInitCommandForProvider(t *testing.T) {
	tests := map[string]string{
		"claude":       "liza init --claude",
		"codex":        "liza init --codex",
		"codex-acp":    "liza init --codex",
		"cursor-acp":   "liza init --cursor",
		"opencode":     "liza init --opencode",
		"opencode-acp": "liza init --opencode",
		"kimi":         "liza init --claude",
		"qwen":         "liza init --provider qwen",
	}

	for cliName, want := range tests {
		if got := contractInitCommandForProvider(cliName); got != want {
			t.Fatalf("contractInitCommandForProvider(%q) = %q, want %q", cliName, got, want)
		}
	}
}
