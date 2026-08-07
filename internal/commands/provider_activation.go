package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/liza-mas/liza/internal/brand"
	gitpkg "github.com/liza-mas/liza/internal/gitenv"
	"github.com/liza-mas/liza/internal/providers"
)

const repoContractActivationStateVersion = 1

type repoContractActivationState struct {
	Version       int               `json:"version"`
	ProviderPaths map[string]string `json:"provider_paths"`
}

func activateProviderContracts(projectRoot, contractTarget string, agents []providers.Provider, catalog providers.Catalog, options contractSymlinkOptions) error {
	// Init is serial per repository. Atomic replacement protects against partial
	// writes; callers must not invoke this read-modify-write concurrently.
	statePath, err := repoContractActivationStatePath(projectRoot)
	if err != nil {
		return err
	}
	state, exists, err := readRepoContractActivationState(statePath)
	if err != nil {
		return err
	}
	if !exists {
		bootstrapLegacyRepoContractActivations(&state, projectRoot, contractTarget, catalog)
	}
	state.pruneMissingLinks(projectRoot, contractTarget)

	options.PreserveRepoPaths = state.preservedPaths(projectRoot)
	createContractSymlinksForProviders(projectRoot, contractTarget, agents, options)

	state.recordSelectedProviders(projectRoot, contractTarget, agents)
	if err := writeRepoContractActivationState(statePath, state); err != nil {
		return fmt.Errorf("persist provider contract activation state: %w", err)
	}
	return nil
}

func repoContractActivationStatePath(projectRoot string) (string, error) {
	output, err := gitpkg.Output(projectRoot, "rev-parse", "--git-dir")
	if err != nil {
		return "", fmt.Errorf("resolve Git directory for provider activation state: %w", err)
	}
	gitDir := strings.TrimSpace(string(output))
	if gitDir == "" {
		return "", fmt.Errorf("resolve Git directory for provider activation state: empty path")
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(projectRoot, gitDir)
	}
	absGitDir, err := filepath.Abs(gitDir)
	if err != nil {
		return "", fmt.Errorf("resolve provider activation state directory: %w", err)
	}
	return filepath.Join(absGitDir, brand.NameLower+"-provider-activations.json"), nil
}

func readRepoContractActivationState(path string) (repoContractActivationState, bool, error) {
	state := repoContractActivationState{
		Version:       repoContractActivationStateVersion,
		ProviderPaths: make(map[string]string),
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return state, false, nil
	}
	if err != nil {
		return state, false, fmt.Errorf("read provider contract activation state: %w", err)
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, true, fmt.Errorf("decode provider contract activation state: %w", err)
	}
	if state.Version != repoContractActivationStateVersion {
		return state, true, fmt.Errorf("unsupported provider contract activation state version %d", state.Version)
	}
	if state.ProviderPaths == nil {
		state.ProviderPaths = make(map[string]string)
	}
	for providerID, repoPath := range state.ProviderPaths {
		if strings.TrimSpace(providerID) == "" {
			return state, true, fmt.Errorf("provider contract activation state contains an empty provider ID")
		}
		cleaned, err := normalizeRepoContractPath(repoPath)
		if err != nil {
			return state, true, fmt.Errorf("provider contract activation state for %s: %w", providerID, err)
		}
		state.ProviderPaths[providerID] = cleaned
	}
	return state, true, nil
}

func writeRepoContractActivationState(path string, state repoContractActivationState) error {
	state.Version = repoContractActivationStateVersion
	if state.ProviderPaths == nil {
		state.ProviderPaths = make(map[string]string)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return nil
}

func normalizeRepoContractPath(path string) (string, error) {
	cleaned := filepath.Clean(path)
	if cleaned == "." || filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid repo-relative contract path %q", path)
	}
	return cleaned, nil
}

func (state *repoContractActivationState) pruneMissingLinks(projectRoot, contractTarget string) {
	for providerID, repoPath := range state.ProviderPaths {
		if !isLizaSymlink(filepath.Join(projectRoot, repoPath), contractTarget) {
			delete(state.ProviderPaths, providerID)
		}
	}
}

func (state repoContractActivationState) preservedPaths(projectRoot string) map[string]bool {
	paths := make(map[string]bool, len(state.ProviderPaths))
	for _, repoPath := range state.ProviderPaths {
		paths[filepath.Join(projectRoot, repoPath)] = true
	}
	return paths
}

func (state *repoContractActivationState) recordSelectedProviders(projectRoot, contractTarget string, agents []providers.Provider) {
	for _, provider := range agents {
		delete(state.ProviderPaths, provider.ID)
		contract := provider.Setup.Contract
		if contract.PrefersGlobal() {
			continue
		}
		// Record the managed path this provider actually owns. Another provider may
		// declare this local fallback as its repo path, so later init must preserve it.
		for _, candidate := range []string{contract.RepoFile, contract.LocalFallback} {
			if candidate == "" {
				continue
			}
			cleaned, err := normalizeRepoContractPath(candidate)
			if err != nil || !isLizaSymlink(filepath.Join(projectRoot, cleaned), contractTarget) {
				continue
			}
			state.ProviderPaths[provider.ID] = cleaned
			break
		}
	}
}

func bootstrapLegacyRepoContractActivations(state *repoContractActivationState, projectRoot, contractTarget string, catalog providers.Catalog) {
	providerIDs := make(map[string]bool)
	for _, provider := range catalog.ProvidersSorted() {
		providerIDs[provider.ID] = true
	}
	for _, provider := range providers.EmbeddedCatalog().ProvidersSorted() {
		providerIDs[provider.ID] = true
	}
	for providerID := range providerIDs {
		resolved, err := resolveCatalogProviders(catalog, []string{providerID})
		if err != nil || len(resolved) != 1 {
			continue
		}
		contract := resolved[0].Setup.Contract
		if contract.RepoFile == "" || contract.PrefersGlobal() {
			continue
		}
		cleaned, err := normalizeRepoContractPath(contract.RepoFile)
		if err != nil || !isLizaSymlink(filepath.Join(projectRoot, cleaned), contractTarget) {
			continue
		}
		state.ProviderPaths[resolved[0].ID] = cleaned
	}
}
