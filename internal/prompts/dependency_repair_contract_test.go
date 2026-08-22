package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDependencyRepairContract_AllDeclaredSurfaces(t *testing.T) {
	t.Parallel()

	surfaces := []struct {
		path     string
		required []string
	}{
		{
			path: "specs/architecture/ADR/0115-declarative-atomic-dependency-repairs.md",
			required: []string{
				"--repair-request-file",
				"apply-dependency-repair",
				"retarget-dependency",
				"one direct edge",
			},
		},
		{
			path: "specs/architecture/ADR/README.md",
			required: []string{
				"0115-declarative-atomic-dependency-repairs.md",
				"--repair-request-file",
				"apply-dependency-repair",
			},
		},
		{
			path: "INVARIANTS.md",
			required: []string{
				"--repair-request-file",
				"apply-dependency-repair",
				"retarget-dependency",
				"one direct edge",
			},
		},
		{
			path: "specs/architecture/blackboard-schema.md",
			required: []string{
				"--repair-request-file",
				"apply-dependency-repair",
				"dependency_updates",
			},
		},
		{
			path: "support-docs/SUPPORT.md",
			required: []string{
				"--repair-request-file",
				"apply-dependency-repair",
				"retarget-dependency",
				"one direct edge",
			},
		},
		{
			path: "support-docs/USAGE_MULTI_AGENTS.md",
			required: []string{
				"--repair-request-file",
				"apply-dependency-repair",
				"retarget-dependency",
				"one direct edge",
			},
		},
		{
			path: "internal/prompts/templates/wake_blocked_tasks.tmpl",
			required: []string{
				"apply-dependency-repair <blocked-task-id>",
				"retarget-dependency",
				"one direct edge",
			},
		},
		{
			path:     "internal/prompts/templates/blocks/blocking_protocol.tmpl",
			required: []string{"--repair-request-file", "apply-dependency-repair", "command-free"},
		},
		{
			path:     "internal/prompts/templates/blocks/architect_tools.tmpl",
			required: []string{"--repair-request-file", "apply-dependency-repair", "command-free"},
		},
		{
			path:     "internal/prompts/templates/blocks/code_planner_tools.tmpl",
			required: []string{"--repair-request-file", "apply-dependency-repair", "command-free"},
		},
		{
			path:     "internal/prompts/templates/blocks/coder_tools.tmpl",
			required: []string{"--repair-request-file", "apply-dependency-repair", "command-free"},
		},
		{
			path:     "internal/prompts/templates/blocks/epic_planner_tools.tmpl",
			required: []string{"--repair-request-file", "apply-dependency-repair", "command-free"},
		},
		{
			path:     "internal/prompts/templates/blocks/integration_analyst_tools.tmpl",
			required: []string{"--repair-request-file", "apply-dependency-repair", "command-free"},
		},
		{
			path:     "internal/prompts/templates/blocks/us_writer_tools.tmpl",
			required: []string{"--repair-request-file", "apply-dependency-repair", "command-free"},
		},
	}

	for _, surface := range surfaces {
		t.Run(surface.path, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", surface.path))
			if err != nil {
				t.Fatalf("read declared dependency-repair surface: %v", err)
			}
			text := string(content)
			for _, marker := range surface.required {
				if !strings.Contains(text, marker) {
					t.Errorf("missing dependency-repair contract marker %q", marker)
				}
			}

			for _, line := range strings.Split(strings.ToLower(text), "\n") {
				for _, forbidden := range []string{
					"run multiple retarget-dependency commands",
					"run a sequence of retarget-dependency commands",
					"one retarget-dependency command per update",
					"encode multi-command dependency repairs",
				} {
					if strings.Contains(line, forbidden) &&
						!strings.Contains(line, "do not") &&
						!strings.Contains(line, "must not") &&
						!strings.Contains(line, "never") {
						t.Errorf("contains forbidden imperative dependency-repair guidance %q", forbidden)
					}
				}
			}
		})
	}
}

func TestDependencyRepairContract_SummarySurfacesDistinguishRequestShapes(t *testing.T) {
	t.Parallel()

	surfaces := []struct {
		path          string
		summaryPrefix string
	}{
		{path: "INVARIANTS.md", summaryPrefix: "| BLOCKED |"},
		{
			path:          "specs/architecture/blackboard-schema.md",
			summaryPrefix: `- "BLOCKED task repair_request, when present`,
		},
		{path: "support-docs/SUPPORT.md", summaryPrefix: "- `repair_request` —"},
	}

	for _, surface := range surfaces {
		t.Run(surface.path, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", surface.path))
			if err != nil {
				t.Fatalf("read dependency-repair summary surface: %v", err)
			}

			var summary string
			for _, line := range strings.Split(string(content), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, surface.summaryPrefix) {
					summary = line
					break
				}
			}
			if summary == "" {
				t.Fatalf("missing dependency-repair summary with prefix %q", surface.summaryPrefix)
			}

			for _, marker := range []string{
				"`command` for command-based non-dependency requests",
				"`dependency_updates` for `apply-dependency-repair`",
			} {
				if !strings.Contains(summary, marker) {
					t.Errorf("summary does not distinguish repair request shapes; missing %q", marker)
				}
			}
		})
	}
}

func TestDependencyRepairWakeContract_ResumesFromConsumedRequest(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "internal/prompts/templates/wake_blocked_tasks.tmpl"))
	if err != nil {
		t.Fatalf("read blocked-task wake guidance: %v", err)
	}
	text := string(content)
	for _, marker := range []string{
		"dependency_repair_receipt",
		"affected_task_ids",
		"declared validation",
	} {
		if !strings.Contains(text, marker) {
			t.Errorf("wake guidance missing consumed-request recovery marker %q", marker)
		}
	}
}
