package embedded

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBlockedMetadataReconciliationDocumentationContract(t *testing.T) {
	t.Parallel()

	repoRoot := findRepoRoot(t)
	surfaces := []string{
		"support-docs/USAGE_MULTI_AGENTS.md",
		"internal/embedded/support-docs/USAGE_MULTI_AGENTS.md",
		"specs/implementation/tooling.md",
	}
	required := []string{
		"After a partial repair",
		"assess-blocked <task-id> --reason",
		`--question "<current question>"`,
		"one to three",
		"clears the previous",
		"After a full repair",
		"unblock-task <task-id> --reason",
		"clears canonical blocker metadata",
		"claimability still depends on direct dependencies",
		"valid pending dependencies keep it dependency-held and unclaimable",
		"until every direct dependency is `MERGED`",
		`--note "<assessment>"`,
		"note-only",
		"history-only",
		"command-style replacement is all-or-nothing",
		"--repair-operation",
		"--repair-target",
		"--repair-command",
		"--repair-evidence",
		"--repair-validation",
		"command-free declarative `apply-dependency-repair` replacement",
		"dependency_updates",
		"--repair-request-file",
		"cannot be combined with individual",
		"mutually exclusive",
		"get <task-id> --json",
		"canonical current view",
	}
	forbidden := []string{
		"guarded transition restores claimability",
		"unblock-task` to restore claimability",
		"unblock-task` remains the only path back to claimability",
	}

	for _, surface := range surfaces {
		t.Run(surface, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(surface)))
			if err != nil {
				t.Fatalf("read documentation surface: %v", err)
			}
			text := string(content)
			for _, marker := range required {
				if !strings.Contains(text, marker) {
					t.Errorf("missing blocked-metadata reconciliation marker %q", marker)
				}
			}
			for _, marker := range forbidden {
				if strings.Contains(text, marker) {
					t.Errorf("unconditional blocked-repair claimability marker %q", marker)
				}
			}
		})
	}
}
