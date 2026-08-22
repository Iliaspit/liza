package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSemanticDependencyDirectionContract_AllDeclaredSurfaces(t *testing.T) {
	t.Parallel()

	surfaces := []struct {
		path     string
		required []string
	}{
		{
			path: "specs/architecture/blackboard-schema.md",
			required: []string{
				"free-form planning annotations, not authoritative machine relationships",
				"Structural dependency validation checks task IDs, pipeline direction, terminal-state legality, and cycles, but it does not infer provider/consumer direction",
			},
		},
		{
			path: "support-docs/USAGE_MULTI_AGENTS.md",
			required: []string{
				"During master decomposition",
				"provider-before-consumer ordering: a consumer may depend on its provider",
				"Before `retarget-dependency`, the orchestrator re-reads the affected tasks and their planning/decomposition context and applies the same provider-before-consumer ordering",
			},
		},
		{
			path: "support-docs/SUPPORT.md",
			required: []string{
				"Before any dependency repair, perform semantic verification",
				"planning/decomposition context",
				"explicit exception rationale",
			},
		},
	}

	for _, surface := range surfaces {
		t.Run(surface.path, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", surface.path))
			if err != nil {
				t.Fatalf("read declared semantic dependency-direction surface: %v", err)
			}

			text := strings.Join(strings.Fields(string(content)), " ")
			for _, marker := range surface.required {
				if !strings.Contains(text, marker) {
					t.Errorf("missing semantic dependency-direction contract marker %q", marker)
				}
			}
		})
	}
}
