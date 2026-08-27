package main

import (
	"strings"
	"testing"

	"github.com/liza-mas/liza/internal/brand"
	"github.com/spf13/cobra"
)

func TestRequireAgentAuthority(t *testing.T) {
	brandedID := brand.EnvName("AGENT_ID")
	brandedGeneration := brand.EnvName("AGENT_GENERATION")
	legacyID := brand.LegacyEnvName("AGENT_ID")
	legacyGeneration := brand.LegacyEnvName("AGENT_GENERATION")
	for _, name := range []string{brandedID, brandedGeneration, legacyID, legacyGeneration} {
		t.Setenv(name, "")
	}

	cmd := &cobra.Command{Use: "test"}
	addAgentIDFlag(cmd)
	if err := cmd.Flags().Set("agent-id", "coder-1"); err != nil {
		t.Fatalf("set agent-id flag: %v", err)
	}
	t.Setenv(brandedGeneration, "generation-b")

	authority, err := requireAgentAuthority(cmd)
	if err != nil {
		t.Fatalf("requireAgentAuthority: %v", err)
	}
	if authority.ID != "coder-1" || authority.Generation != "generation-b" {
		t.Fatalf("authority = %#v, want coder-1/generation-b", authority)
	}

	t.Setenv(brandedGeneration, "")
	if _, err := requireAgentAuthority(cmd); err == nil || !strings.Contains(err.Error(), brandedGeneration) {
		t.Fatalf("missing generation error = %v, want %s diagnostic", err, brandedGeneration)
	}

	legacyCmd := &cobra.Command{Use: "legacy"}
	addAgentIDFlag(legacyCmd)
	t.Setenv(legacyID, "coder-2")
	t.Setenv(legacyGeneration, "generation-legacy")
	authority, err = requireAgentAuthority(legacyCmd)
	if err != nil {
		t.Fatalf("legacy requireAgentAuthority: %v", err)
	}
	if authority.ID != "coder-2" || authority.Generation != "generation-legacy" {
		t.Fatalf("legacy authority = %#v, want coder-2/generation-legacy", authority)
	}
}
