package models

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestAgentGenerationLegacyDecode(t *testing.T) {
	var legacy Agent
	if err := yaml.Unmarshal([]byte("role: coder\nstatus: IDLE\nheartbeat: 2026-08-25T00:00:00Z\nterminal: terminal-1\n"), &legacy); err != nil {
		t.Fatalf("decode legacy agent: %v", err)
	}
	if legacy.Generation != "" {
		t.Fatalf("legacy generation = %q, want empty", legacy.Generation)
	}

	generation, err := NewAgentGeneration()
	if err != nil {
		t.Fatalf("mint generation: %v", err)
	}
	encoded, err := yaml.Marshal(Agent{Role: "coder", Status: AgentStatusIdle, Generation: generation})
	if err != nil {
		t.Fatalf("encode current agent: %v", err)
	}
	var decoded Agent
	if err := yaml.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode current agent: %v", err)
	}
	if decoded.Generation != generation {
		t.Fatalf("round-trip generation = %q, want %q", decoded.Generation, generation)
	}
}
