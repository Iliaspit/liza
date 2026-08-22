package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCrossTransitionDependencyUsageContract(t *testing.T) {
	t.Parallel()

	guides := []string{
		"support-docs/USAGE_MULTI_AGENTS.md",
		"internal/embedded/support-docs/USAGE_MULTI_AGENTS.md",
	}
	requiredClaims := map[string]string{
		"same-transition inheritance":        "inherit dependency children only when both tasks execute the same transition name",
		"different-transition boundary":      "Different transition names do not propagate child ordering",
		"explicit cross-transition ordering": "For cross-transition child ordering, use explicit `output[].task_depends_on` entries containing existing concrete task IDs",
	}

	for _, guide := range guides {
		t.Run(guide, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", guide))
			if err != nil {
				t.Fatalf("read dependency usage guide: %v", err)
			}

			text := strings.Join(strings.Fields(string(content)), " ")
			for name, marker := range requiredClaims {
				t.Run(name, func(t *testing.T) {
					if !strings.Contains(text, marker) {
						t.Errorf("missing cross-transition dependency contract marker %q", marker)
					}
				})
			}
		})
	}
}
