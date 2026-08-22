package brandrender

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/liza-mas/liza/internal/brand"
)

var rawDefaultBrandRE = regexp.MustCompile(`(?i)(^|[^A-Za-z])liza($|[^A-Za-z0-9])|liza-mas/liza`)

func TestRenderBytesRejectsUnknownAndStrayMacros(t *testing.T) {
	values := brand.RuntimeValues()
	if _, err := RenderBytes([]byte("hello §BRAND_UNKNOWN§"), values); err == nil {
		t.Fatal("RenderBytes accepted unknown macro")
	}
	if _, err := RenderBytes([]byte("hello §"), values); err == nil {
		t.Fatal("RenderBytes accepted stray delimiter")
	}
}

func TestRenderBytesUsesBrandValues(t *testing.T) {
	values := brand.RuntimeValues()
	values.NameTitle = "Acme Agent"
	got, err := RenderBytes([]byte("Product: §BRAND_NAME_TITLE§"), values)
	if err != nil {
		t.Fatalf("RenderBytes: %v", err)
	}
	if string(got) != "Product: Acme Agent" {
		t.Fatalf("rendered = %q", got)
	}
}

func TestValidateRenderedFileParsesJSONAndTOML(t *testing.T) {
	if err := ValidateRenderedFile("settings.json", []byte(`{"name":"ok"}`)); err != nil {
		t.Fatalf("valid JSON rejected: %v", err)
	}
	if err := ValidateRenderedFile("config.toml", []byte("name = \"ok\"\n")); err != nil {
		t.Fatalf("valid TOML rejected: %v", err)
	}
	if err := ValidateRenderedFile("config.toml", []byte("name = \"unterminated\n")); err == nil {
		t.Fatal("invalid TOML accepted")
	}
}

func TestRenderPathAppliesGeneratedNameMap(t *testing.T) {
	values := brand.RuntimeValues()
	values.NameLower = "acme-agent"
	values.BinaryName = "acme-cli"
	values.ProjectDirName = ".acme-agent"
	got := RenderPath("check-liza-input-readiness/SKILL.md/liza-logs/tools/liza-session-analyzer.html/scripts/liza-index.sh/liza-operator/.liza-hooks/pre-commit", values)
	if got != "check-acme-agent-input-readiness/SKILL.md/acme-cli-logs/tools/acme-cli-session-analyzer.html/scripts/acme-cli-index.sh/acme-agent-operator/.acme-agent-hooks/pre-commit" {
		t.Fatalf("RenderPath = %q", got)
	}
}

func TestRenderPathUsesDerivedDefaults(t *testing.T) {
	values := brand.Values{
		NameLower: "acme-agent",
		NameUpper: "ACME_AGENT",
		NameTitle: "Acme Agent",
		Repo:      "acme/agent",
	}
	got := RenderPath("liza-index/.liza-hooks/pre-commit", values)
	if got != "acme-agent-index/.acme-agent-hooks/pre-commit" {
		t.Fatalf("RenderPath = %q, want derived binary and project dir names", got)
	}
}

func TestExpectedEmbeddedFilesRendersMacros(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, "contracts"))
	mkdirAll(t, filepath.Join(root, "skills", "liza-logs"))
	mkdirAll(t, filepath.Join(root, "support-docs"))
	writeFile(t, filepath.Join(root, "contracts", "CORE.md"), "# §BRAND_NAME_TITLE§\n")
	writeFile(t, filepath.Join(root, "skills", "liza-logs", "SKILL.md"), "name: §BRAND_BINARY_NAME§-logs\n")
	writeFile(t, filepath.Join(root, "support-docs", "USAGE.md"), "run §BRAND_BINARY_NAME§\n")
	writeFile(t, filepath.Join(root, ".bash-policy.yaml"), strings.Join([]string{
		"rules:",
		"- kind: permission-family",
		"  identity: Bash(liza:*)",
		"  status: resolved",
		"",
	}, "\n"))

	values := brand.RuntimeValues()
	values.NameLower = "acme-agent"
	values.NameTitle = "Acme Agent"
	values.BinaryName = "acme-cli"

	files, err := ExpectedEmbeddedFiles(root, values)
	if err != nil {
		t.Fatalf("ExpectedEmbeddedFiles: %v", err)
	}
	var sawRenamedSkill bool
	var sawBrandedBashPolicy bool
	for _, file := range files {
		if strings.Contains(file.RelPath, "acme-cli-logs") {
			sawRenamedSkill = true
		}
		if file.RelPath == "bash-policy.yaml" {
			rendered := string(file.Content)
			if !strings.Contains(rendered, "Bash(acme-cli:*)") {
				t.Fatalf("bash-policy.yaml missing branded binary permission:\n%s", rendered)
			}
			if strings.Contains(rendered, "Bash(liza:*)") {
				t.Fatalf("bash-policy.yaml kept default binary permission:\n%s", rendered)
			}
			sawBrandedBashPolicy = true
		}
		if strings.Contains(string(file.Content), "§") || strings.Contains(string(file.Content), "BRAND_") {
			t.Fatalf("unrendered macro in %s: %s", file.RelPath, file.Content)
		}
	}
	if !sawRenamedSkill {
		t.Fatalf("expected generated skill path rename, got %+v", files)
	}
	if !sawBrandedBashPolicy {
		t.Fatalf("expected generated bash-policy.yaml, got %+v", files)
	}
}

func TestExpectedEmbeddedFilesUseNonDefaultBrand(t *testing.T) {
	files, err := ExpectedEmbeddedFiles(findRepoRoot(t), nonDefaultBrandValues())
	if err != nil {
		t.Fatalf("ExpectedEmbeddedFiles: %v", err)
	}

	var sawBinaryLogsSkill bool
	var sawNameLowerOperatorSkill bool
	for _, file := range files {
		rendered := string(file.Content)
		if strings.Contains(file.RelPath, "acme-cli-logs") {
			sawBinaryLogsSkill = true
		}
		if strings.Contains(file.RelPath, "acme-agent-operator") {
			sawNameLowerOperatorSkill = true
		}
		if strings.Contains(file.RelPath, "liza-operator") {
			t.Fatalf("%s contains raw default operator skill path", file.RelPath)
		}
		if strings.Contains(file.RelPath, "acme-agent-logs") || strings.Contains(rendered, "acme-agent-logs") {
			t.Fatalf("%s contains name-lower logs skill artifact", file.RelPath)
		}
		if match := rawDefaultBrandRE.FindString(rendered); match != "" {
			t.Fatalf("%s contains raw default brand token %q", file.RelPath, match)
		}
	}
	if !sawBinaryLogsSkill {
		t.Fatalf("expected generated logs skill path to use binary name")
	}
	if !sawNameLowerOperatorSkill {
		t.Fatalf("expected generated operator skill path to use name-lower")
	}
}

func TestRenderedRepairAgentPoolDocsDescribeClaimEligibleReviewerCapacity(t *testing.T) {
	files, err := ExpectedEmbeddedFiles(findRepoRoot(t), nonDefaultBrandValues())
	if err != nil {
		t.Fatalf("ExpectedEmbeddedFiles: %v", err)
	}

	renderedByPath := make(map[string][]byte, len(files))
	for _, file := range files {
		renderedByPath[file.RelPath] = file.Content
	}

	statement := "For reviewer work, capacity requires a live usable agent that can pass the existing claim filters for the task, including prior-approval and configured provider-diversity eligibility."
	for _, relPath := range []string{
		"support-docs/USAGE_MULTI_AGENTS.md",
		"support-docs/CONFIGURATION.md",
		"support-docs/TROUBLESHOOTING.md",
		"skills/acme-agent-operator/SKILL.md",
	} {
		content, ok := renderedByPath[relPath]
		if !ok {
			t.Errorf("rendered artifacts missing %s", relPath)
			continue
		}
		if !strings.Contains(string(content), statement) {
			t.Errorf("%s missing reviewer-capacity statement %q", relPath, statement)
		}
		if match := rawDefaultBrandRE.Find(content); match != nil {
			t.Errorf("%s contains raw default brand token %q", relPath, match)
		}
	}
}

func TestExpectedEmbeddedFilesDocumentDependencyHeldUnblock(t *testing.T) {
	root := findRepoRoot(t)
	surfacePaths := []string{
		"INVARIANTS.md",
		"support-docs/SUPPORT.md",
		"support-docs/USAGE_MULTI_AGENTS.md",
		"internal/embedded/support-docs/SUPPORT.md",
		"internal/embedded/support-docs/USAGE_MULTI_AGENTS.md",
		"specs/architecture/state-machines.md",
		"specs/architecture/ADR/0077-dependency-edge-canonicalization.md",
		"specs/architecture/ADR/0080-claimable-rebase-unblock.md",
		"specs/protocols/worktree-management.md",
	}
	requiredStatements := []string{
		"valid pending dependencies",
		"role-pair initial status",
		"dependency-held",
		"--assign-to",
		"remains rejected while any dependency is unmet",
		"captured integration SHA",
		"completion lock",
		"equality check and assignment",
		"cooperating integration movement",
		"without holding the integration mutation lock across",
	}
	for _, relPath := range surfacePaths {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relPath)))
		if err != nil {
			t.Fatalf("read %s: %v", relPath, err)
		}
		for _, required := range requiredStatements {
			if !strings.Contains(string(content), required) {
				t.Errorf("%s missing dependency-held unblock statement %q", relPath, required)
			}
		}
	}

	files, err := ExpectedEmbeddedFiles(root, nonDefaultBrandValues())
	if err != nil {
		t.Fatalf("ExpectedEmbeddedFiles: %v", err)
	}
	for _, file := range files {
		if !strings.HasPrefix(file.RelPath, "support-docs/") {
			continue
		}
		if match := rawDefaultBrandRE.Find(file.Content); match != nil {
			t.Errorf("%s contains raw default brand token %q", file.RelPath, match)
		}
	}
}

func nonDefaultBrandValues() brand.Values {
	return brand.ValuesFromEnv(func(key string) string {
		switch key {
		case "BRAND_NAME_LOWER", "BRAND_ARCHIVE_PREFIX", "BRAND_MISTRAL_PROMPT_ID":
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
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root")
		}
		dir = parent
	}
}
