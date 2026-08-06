package commands

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/providers"
)

func TestSetupCommand_ProviderFromCatalog(t *testing.T) {
	useTestProviderCatalog(t)
	lizaDir := t.TempDir()
	homeDir := t.TempDir()

	if err := SetupCommand(SetupParams{
		TargetDir:   lizaDir,
		HomeDir:     homeDir,
		ProviderIDs: []string{"qwen"},
	}); err != nil {
		t.Fatalf("SetupCommand() error = %v", err)
	}

	linkPath := filepath.Join(homeDir, ".qwen", "skills", "code-review")
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("qwen skill link missing: %v", err)
	}
	if want := filepath.Join(lizaDir, "skills", "code-review"); target != want {
		t.Fatalf("qwen skill link = %q, want %q", target, want)
	}
}

func TestInitPairingCommand_ProviderFromCatalog(t *testing.T) {
	useTestProviderCatalog(t)
	gitDir := setupGitRepo(t)
	defer os.RemoveAll(gitDir)
	fakeHome := setupGlobalLiza(t)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	if err := os.Chdir(gitDir); err != nil {
		t.Fatal(err)
	}

	if err := InitPairingCommand(InitPairingParams{Agents: []string{"qwen"}}); err != nil {
		t.Fatalf("InitPairingCommand() error = %v", err)
	}

	target, err := os.Readlink(filepath.Join(gitDir, "QWEN.md"))
	if err != nil {
		t.Fatalf("QWEN.md not a symlink: %v", err)
	}
	if want := filepath.Join(fakeHome, ".liza", "CORE.md"); target != want {
		t.Fatalf("QWEN.md = %q, want %q", target, want)
	}
}

func TestInitPairingCommand_ProviderFromCatalogCreatesNestedContractParent(t *testing.T) {
	useTestProviderCatalog(t)
	gitDir := setupGitRepo(t)
	defer os.RemoveAll(gitDir)
	fakeHome := setupGlobalLiza(t)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	if err := os.Chdir(gitDir); err != nil {
		t.Fatal(err)
	}

	if err := InitPairingCommand(InitPairingParams{Agents: []string{"devin"}}); err != nil {
		t.Fatalf("InitPairingCommand() error = %v", err)
	}

	linkPath := filepath.Join(gitDir, ".windsurf", "rules", "liza.md")
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf(".windsurf/rules/liza.md not a symlink: %v", err)
	}
	if want := filepath.Join(fakeHome, ".liza", "CORE.md"); target != want {
		t.Fatalf(".windsurf/rules/liza.md = %q, want %q", target, want)
	}
}

func TestCachedLegacyClaudeCatalogBackfillsPreferGlobal(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	cachePath, metaPath := providers.CachePaths(homeDir)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		t.Fatalf("create provider cache directory: %v", err)
	}
	legacyCatalog := `version: 1
providers:
  - id: claude
    display_name: Claude
    backend: cli
    setup:
      contract:
        repo_file: CLAUDE.md
        global_fallback: .claude/CLAUDE.md
    runtime:
      executable: claude
`
	if err := os.WriteFile(cachePath, []byte(legacyCatalog), 0644); err != nil {
		t.Fatalf("write legacy provider cache: %v", err)
	}
	const catalogURL = "https://example.test/provider-catalog.yaml"
	meta, err := json.Marshal(providers.CacheMeta{URL: catalogURL, FetchedAt: time.Now()})
	if err != nil {
		t.Fatalf("marshal provider cache metadata: %v", err)
	}
	if err := os.WriteFile(metaPath, meta, 0644); err != nil {
		t.Fatalf("write provider cache metadata: %v", err)
	}
	t.Setenv(providers.EnvCatalogURL, catalogURL)

	catalog := loadProviderCatalog(homeDir)
	selected, err := resolveCatalogProviders(catalog, []string{"claude"})
	if err != nil {
		t.Fatalf("resolve cached Claude provider: %v", err)
	}
	if len(selected) != 1 || !selected[0].Setup.Contract.PrefersGlobal() {
		t.Fatalf("cached Claude contract = %+v, want embedded prefer_global default", selected)
	}

	projectRoot := t.TempDir()
	contractTarget := filepath.Join(homeDir, ".liza", "CORE.md")
	repoPath := filepath.Join(projectRoot, "CLAUDE.md")
	globalPath := filepath.Join(homeDir, ".claude", "CLAUDE.md")
	if err := os.MkdirAll(filepath.Dir(globalPath), 0755); err != nil {
		t.Fatalf("create global Claude directory: %v", err)
	}
	if err := os.Symlink(contractTarget, repoPath); err != nil {
		t.Fatalf("create repo Claude symlink: %v", err)
	}
	if err := os.Symlink(contractTarget, globalPath); err != nil {
		t.Fatalf("create global Claude symlink: %v", err)
	}

	createContractSymlinksForProviders(projectRoot, contractTarget, selected, "")
	if _, err := os.Lstat(repoPath); !os.IsNotExist(err) {
		t.Fatalf("cached legacy catalog should remove duplicate repo CLAUDE.md; got %v", err)
	}
}

func useTestProviderCatalog(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	if _, err := providers.ParseCatalog([]byte(testProviderCatalogYAML)); err != nil {
		t.Fatalf("test provider catalog YAML invalid: %v", err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(testProviderCatalogYAML))
	}))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on localhost: %v", err)
	}
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
	t.Setenv(providers.EnvCatalogURL, server.URL)
	cat, _ := providers.Load(context.Background(), providers.LoadOptions{URL: server.URL, HomeDir: t.TempDir(), Force: true})
	if _, ok := cat.Resolve("qwen"); !ok {
		t.Fatalf("test provider catalog at %s did not load qwen", server.URL)
	}
}

const testProviderCatalogYAML = `version: 1
providers:
  - id: qwen
    display_name: Qwen
    aliases: [qwen-code]
    backend: cli
    detection:
      binaries: [qwen]
      version_args: [--version]
    setup:
      config_dir: .qwen
      skills_dir: skills
      contract:
        repo_file: QWEN.md
        global_fallback: .qwen/QWEN.md
    runtime:
      provider_key: qwen
      executable: qwen
      prompt_transport: stdin
      run_args: [-p]
      logged_run_args: [-p, --output-format, stream-json]
      contract_key: qwen
  - id: devin
    display_name: Devin
    backend: cli
    detection:
      binaries: [devin]
      version_args: [--version]
    setup:
      config_dir: .config/devin
      skills_dir: skills
      contract:
        repo_file: .windsurf/rules/liza.md
        global_fallback: .config/devin/liza.md
    runtime:
      provider_key: devin
      executable: devin
      prompt_transport: arg
      run_args: [--permission-mode, dangerous, -p, "{{prompt}}"]
      logged_run_args: [--permission-mode, dangerous, -p, "{{prompt}}"]
      contract_key: devin
`
