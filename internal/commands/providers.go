package commands

import (
	"context"
	"fmt"

	"github.com/liza-mas/liza/internal/providers"
)

func loadProviderCatalog(homeDir string) providers.Catalog {
	cat, _ := providers.Load(context.Background(), providers.LoadOptions{HomeDir: homeDir})
	return cat
}

func resolveCatalogProviders(cat providers.Catalog, ids []string) ([]providers.Provider, error) {
	resolved := make([]providers.Provider, 0, len(ids))
	seen := map[string]bool{}
	embeddedCatalog := providers.EmbeddedCatalog()
	for _, id := range ids {
		provider, ok := cat.Resolve(id)
		if !ok {
			// Backfill embedded built-ins when a stale or partial catalog omits
			// providers that Liza requires for convenience setup paths.
			provider, ok = embeddedCatalog.Resolve(id)
		}
		if !ok {
			return nil, fmt.Errorf("unknown provider: %s", id)
		}
		if embedded, builtIn := embeddedCatalog.Resolve(provider.ID); builtIn && cat.Version < 2 {
			provider.Setup.Contract = backfillLegacyContractPolicy(provider.ID, provider.Setup.Contract, embedded.Setup.Contract)
		}
		if seen[provider.ID] {
			continue
		}
		seen[provider.ID] = true
		resolved = append(resolved, provider)
	}
	return resolved, nil
}

// backfillLegacyContractPolicy reconciles v1 built-ins with v2 placement
// capabilities. A missing embedded global path means the provider is repo-only;
// only known historical defaults are removed so custom v1 paths remain
// authoritative. Providers that still support global activation preserve
// explicit operator paths and preferences because custom paths cannot safely
// inherit policy for another location.
func backfillLegacyContractPolicy(providerID string, contract, defaults providers.ContractLinks) providers.ContractLinks {
	if defaults.GlobalFallback == "" {
		if !isKnownLegacyRepoOnlyGlobalFallback(providerID, contract.GlobalFallback) {
			return contract
		}
		contract.GlobalFallback = ""
		contract.GlobalFallbackEnv = ""
		contract.GlobalFallbackEnvSuffix = ""
		contract.GlobalFallbackEnvExpandHome = false
		contract.PreferGlobal = nil
		if contract.RepoFile == "" {
			contract.RepoFile = defaults.RepoFile
		}
		return contract
	}
	if contract.GlobalFallback == "" || contract.GlobalFallback != defaults.GlobalFallback {
		return contract
	}
	if contract.GlobalFallbackEnv == "" && contract.GlobalFallbackEnvSuffix == "" {
		contract.GlobalFallbackEnv = defaults.GlobalFallbackEnv
		contract.GlobalFallbackEnvSuffix = defaults.GlobalFallbackEnvSuffix
		contract.GlobalFallbackEnvExpandHome = defaults.GlobalFallbackEnvExpandHome
	}
	if contract.PreferGlobal == nil && defaults.PreferGlobal != nil {
		preferGlobal := *defaults.PreferGlobal
		contract.PreferGlobal = &preferGlobal
	}
	return contract
}

func isKnownLegacyRepoOnlyGlobalFallback(providerID, globalFallback string) bool {
	// Remove when provider catalog schema v1 support is dropped.
	switch providerID {
	case "cursor":
		return globalFallback == ".cursor/AGENTS.md"
	case "cursor-acp":
		return globalFallback == ".cursor/AGENTS.md" || globalFallback == ".codex/AGENTS.md"
	case "kimi":
		return globalFallback == ".claude/CLAUDE.md"
	case "devin", "devin-acp":
		return globalFallback == ".config/devin/liza.md"
	default:
		return false
	}
}
