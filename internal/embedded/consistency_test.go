package embedded

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liza-mas/liza/internal/brand"
	"github.com/liza-mas/liza/internal/brandrender"
)

// TestArtifactConsistency verifies that rendered repo master files match their
// embedded copies under internal/embedded/. This catches drift when a master is
// modified without running `make sync-embedded`.
func TestArtifactConsistency(t *testing.T) {
	repoRoot := findRepoRoot(t)
	embeddedDir := filepath.Join(repoRoot, "internal", "embedded")

	expected, err := brandrender.ExpectedEmbeddedFiles(repoRoot, brand.RuntimeValues())
	if err != nil {
		t.Fatalf("building expected embedded files: %v", err)
	}
	if len(expected) == 0 {
		t.Fatal("no rendered embedded files found")
	}
	expectedByPath := make(map[string]brandrender.RenderedFile, len(expected))
	for _, file := range expected {
		if _, exists := expectedByPath[file.RelPath]; exists {
			t.Fatalf("duplicate expected embedded path %s", file.RelPath)
		}
		expectedByPath[file.RelPath] = file
		compareRenderedToEmbedded(t, file, filepath.Join(embeddedDir, filepath.FromSlash(file.RelPath)))
	}
	for relPath := range actualManagedEmbeddedFiles(t, embeddedDir) {
		if _, expected := expectedByPath[relPath]; !expected {
			t.Errorf("STALE: unexpected managed embedded file %s — run `make sync-embedded`", relPath)
		}
	}

	t.Run("docs support stubs resolve", func(t *testing.T) {
		stubs := map[string]string{
			"docs/CONFIGURATION.md":           "support-docs/CONFIGURATION.md",
			"docs/CUSTOMIZING_AGENT_TOOLS.md": "support-docs/CUSTOMIZING_AGENT_TOOLS.md",
			"docs/TROUBLESHOOTING.md":         "support-docs/TROUBLESHOOTING.md",
			"docs/USAGE_MULTI_AGENTS.md":      "support-docs/USAGE_MULTI_AGENTS.md",
			"docs/USAGE_PAIRING.md":           "support-docs/USAGE_PAIRING.md",
			"docs/how-to-produce-a-goal.md":   "support-docs/how-to-produce-a-goal.md",
		}

		for stub, target := range stubs {
			stubPath := filepath.Join(repoRoot, stub)
			targetPath := filepath.Join(repoRoot, target)
			content, err := os.ReadFile(stubPath)
			if err != nil {
				t.Fatalf("reading stub %s: %v", stub, err)
			}
			if _, err := os.Stat(targetPath); err != nil {
				t.Fatalf("stub target missing for %s -> %s: %v", stub, target, err)
			}
			if !strings.Contains(string(content), target) {
				t.Fatalf("stub %s does not point to %s", stub, target)
			}
		}
	})

	t.Run("retarget-dependency cycle diagnostics documentation", func(t *testing.T) {
		docs := []string{
			"support-docs/SUPPORT.md",
			"support-docs/USAGE_MULTI_AGENTS.md",
		}
		required := []string{
			`"ok": false`,
			`"result": null`,
			`"code": "validation"`,
			`"operation": "retarget-dependency"`,
			`"task_id": "A"`,
			`"old_dependency": "old-dependency"`,
			`"new_dependencies": ["B"]`,
			`"phase": "candidate-state-validation"`,
			`"cycle_path": ["A", "B", "C", "A"]`,
			`"diagnostic_action": "retarget_dependency_rejected"`,
			`--json -v`,
			`stdout contains exactly one JSON envelope`,
			`stderr contains only the classified safe message and details`,
			`retarget_dependency_rejected`,
			`candidate dependency, repair request, and task history remain unchanged`,
			"no `retarget-dependency` success activity is recorded",
			"supervisor attributes the failure to `retarget-dependency`",
		}

		for _, doc := range docs {
			content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(doc)))
			if err != nil {
				t.Fatalf("reading %s: %v", doc, err)
			}
			for _, fragment := range required {
				if !strings.Contains(string(content), fragment) {
					t.Errorf("%s missing retarget-dependency diagnostic contract fragment %q", doc, fragment)
				}
			}
		}
	})
}

func TestArtifactConsistencyRendersNonDefaultBrand(t *testing.T) {
	repoRoot := t.TempDir()
	mkdirAllConsistency(t, filepath.Join(repoRoot, "contracts"))
	mkdirAllConsistency(t, filepath.Join(repoRoot, "skills", "liza-logs"))
	mkdirAllConsistency(t, filepath.Join(repoRoot, "skills", "liza-operator"))
	mkdirAllConsistency(t, filepath.Join(repoRoot, "support-docs"))
	writeConsistencyFile(t, filepath.Join(repoRoot, "contracts", "CORE.md"), "You are a §BRAND_NAME_TITLE§ agent.\n")
	writeConsistencyFile(t, filepath.Join(repoRoot, "skills", "liza-logs", "SKILL.md"), "name: §BRAND_BINARY_NAME§-logs\n")
	writeConsistencyFile(t, filepath.Join(repoRoot, "skills", "liza-operator", "SKILL.md"), "name: §BRAND_BINARY_NAME§-operator\n")
	writeConsistencyFile(t, filepath.Join(repoRoot, "support-docs", "USAGE.md"), "Run §BRAND_BINARY_NAME§.\n")
	writeConsistencyFile(t, filepath.Join(repoRoot, ".bash-policy.yaml"), "rules: []\n")

	values := brand.ValuesFromEnv(func(key string) string {
		switch key {
		case "BRAND_NAME_LOWER":
			return "acme-agent"
		case "BRAND_BINARY_NAME":
			return "acme-cli"
		case "BRAND_NAME_UPPER", "BRAND_ENV_PREFIX":
			return "ACME_AGENT"
		case "BRAND_NAME_TITLE":
			return "Acme Agent"
		case "BRAND_REPO", "BRAND_RELEASE_REPO", "BRAND_INSTALL_REPO":
			return "acme/agent"
		default:
			return ""
		}
	})

	expected, err := brandrender.ExpectedEmbeddedFiles(repoRoot, values)
	if err != nil {
		t.Fatalf("building expected embedded files: %v", err)
	}
	var sawRenamedSkill bool
	var sawRenamedOperatorSkill bool
	for _, file := range expected {
		if strings.Contains(file.RelPath, "acme-cli-logs") {
			sawRenamedSkill = true
		}
		if strings.Contains(file.RelPath, "acme-agent-operator") {
			sawRenamedOperatorSkill = true
		}
		if strings.Contains(file.RelPath, "liza-operator") {
			t.Fatalf("%s contains raw default operator skill path", file.RelPath)
		}
		if strings.Contains(file.RelPath, "acme-agent-logs") || strings.Contains(string(file.Content), "acme-agent-logs") {
			t.Fatalf("%s contains name-lower logs skill artifact", file.RelPath)
		}
		if strings.Contains(string(file.Content), "§") || strings.Contains(string(file.Content), "BRAND_") {
			t.Fatalf("unrendered macro in %s: %s", file.RelPath, file.Content)
		}
	}
	if !sawRenamedSkill {
		t.Fatalf("expected rendered skill path rename, got %+v", expected)
	}
	if !sawRenamedOperatorSkill {
		t.Fatalf("expected rendered operator skill path rename, got %+v", expected)
	}
}

func compareRenderedToEmbedded(t *testing.T, expected brandrender.RenderedFile, embeddedPath string) {
	t.Helper()

	info, err := os.Lstat(embeddedPath)
	if err != nil {
		t.Errorf("inspecting embedded copy %s: %v", embeddedPath, err)
		return
	}
	if !info.Mode().IsRegular() {
		t.Errorf("DRIFT: embedded copy %s has non-regular mode %v", embeddedPath, info.Mode())
		return
	}
	wantMode := expected.Mode.Perm()
	if wantMode == 0 {
		wantMode = 0o644
	}
	if info.Mode().Perm() != wantMode {
		t.Errorf("DRIFT: embedded copy %s mode = %v, want %v — run `make sync-embedded`",
			embeddedPath, info.Mode().Perm(), wantMode)
	}

	embedded, err := os.ReadFile(embeddedPath)
	if err != nil {
		t.Errorf("reading embedded copy %s: %v", embeddedPath, err)
		return
	}

	if string(expected.Content) != string(embedded) {
		t.Errorf("DRIFT: rendered source %s differs from embedded copy %s — run `make sync-embedded`",
			expected.RelPath, embeddedPath)
	}
}

func actualManagedEmbeddedFiles(t *testing.T, embeddedDir string) map[string]bool {
	t.Helper()
	actual := make(map[string]bool)
	for _, managedDir := range brandrender.ManagedEmbeddedDirs() {
		root := filepath.Join(embeddedDir, filepath.FromSlash(managedDir))
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(embeddedDir, path)
			if err != nil {
				return err
			}
			actual[filepath.ToSlash(rel)] = true
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			t.Fatalf("walk managed embedded directory %s: %v", managedDir, err)
		}
	}
	bashPolicy := filepath.Join(embeddedDir, "bash-policy.yaml")
	if _, err := os.Lstat(bashPolicy); err == nil {
		actual["bash-policy.yaml"] = true
	} else if !os.IsNotExist(err) {
		t.Fatalf("inspect embedded bash-policy.yaml: %v", err)
	}
	return actual
}

func mkdirAllConsistency(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeConsistencyFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// findRepoRoot walks up from the working directory to find the directory
// containing go.mod (the repository root).
func findRepoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (no go.mod found in any parent directory)")
		}
		dir = parent
	}
}
