package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liza-mas/liza/internal/providers"
)

func TestInitPairingCommand_GlobalFirstDeduplicatesUnownedRepoFallback(t *testing.T) {
	gitDir := setupGitRepo(t)
	defer os.RemoveAll(gitDir)
	fakeHome := setupGlobalLiza(t)
	contractTarget := filepath.Join(fakeHome, ".liza", "CORE.md")
	globalPath := filepath.Join(fakeHome, ".codex", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(globalPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(globalPath, []byte("user-owned\n"), 0644); err != nil {
		t.Fatal(err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })
	if err := os.Chdir(gitDir); err != nil {
		t.Fatal(err)
	}
	if err := InitPairingCommand(InitPairingParams{Agents: []string{"codex"}}); err != nil {
		t.Fatalf("first InitPairingCommand() error = %v", err)
	}
	repoPath := filepath.Join(gitDir, "AGENTS.md")
	if target, err := os.Readlink(repoPath); err != nil || target != contractTarget {
		t.Fatalf("repo fallback target = %q, err = %v; want %q", target, err, contractTarget)
	}

	if err := os.Remove(globalPath); err != nil {
		t.Fatal(err)
	}
	if err := InitPairingCommand(InitPairingParams{Agents: []string{"codex"}}); err != nil {
		t.Fatalf("second InitPairingCommand() error = %v", err)
	}
	if _, err := os.Lstat(repoPath); !os.IsNotExist(err) {
		t.Fatalf("repo fallback should be removed after global activation; got %v", err)
	}
	if target, err := os.Readlink(globalPath); err != nil || target != contractTarget {
		t.Fatalf("global target = %q, err = %v; want %q", target, err, contractTarget)
	}
}

func TestBootstrapLegacyRepoContractActivationsPreservesAmbiguousManagedLink(t *testing.T) {
	projectRoot := setupGitRepo(t)
	defer os.RemoveAll(projectRoot)
	contractTarget := filepath.Join(t.TempDir(), "CORE.md")
	if err := os.Symlink(contractTarget, filepath.Join(projectRoot, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	state := repoContractActivationState{
		Version:       repoContractActivationStateVersion,
		ProviderPaths: make(map[string]string),
	}
	bootstrapLegacyRepoContractActivations(&state, projectRoot, contractTarget, providers.EmbeddedCatalog())

	found := false
	for _, path := range state.ProviderPaths {
		if path == "AGENTS.md" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("legacy activation state = %+v, want repo-only owner for AGENTS.md", state.ProviderPaths)
	}
}

func TestRepoContractActivationStatePrunesMissingManagedLinks(t *testing.T) {
	state := repoContractActivationState{
		Version:       repoContractActivationStateVersion,
		ProviderPaths: map[string]string{"cursor": "AGENTS.md"},
	}
	state.pruneMissingLinks(t.TempDir(), filepath.Join(t.TempDir(), "CORE.md"))
	if len(state.ProviderPaths) != 0 {
		t.Fatalf("stale activation state = %+v, want empty", state.ProviderPaths)
	}
}

func TestRecordSelectedProvidersRecordsLocalFallbackActivation(t *testing.T) {
	projectRoot := t.TempDir()
	contractTarget := filepath.Join(t.TempDir(), "CORE.md")
	if err := os.Symlink(contractTarget, filepath.Join(projectRoot, "CUSTOM.local.md")); err != nil {
		t.Fatal(err)
	}
	state := repoContractActivationState{
		Version:       repoContractActivationStateVersion,
		ProviderPaths: make(map[string]string),
	}
	provider := providers.Provider{
		ID: "custom",
		Setup: providers.Setup{Contract: providers.ContractLinks{
			RepoFile:      "CUSTOM.md",
			LocalFallback: "CUSTOM.local.md",
		}},
	}

	state.recordSelectedProviders(projectRoot, contractTarget, []providers.Provider{provider})

	if got := state.ProviderPaths["custom"]; got != "CUSTOM.local.md" {
		t.Fatalf("local fallback activation state = %+v, want custom owner at CUSTOM.local.md", state.ProviderPaths)
	}
}

func TestReadRepoContractActivationStateRejectsMalformedData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "provider-activations.json")
	if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	_, _, err := readRepoContractActivationState(path)
	if err == nil || !strings.Contains(err.Error(), "decode provider contract activation state") {
		t.Fatalf("readRepoContractActivationState() error = %v, want decode diagnostic", err)
	}
}
