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
		if embedded, builtIn := embeddedCatalog.Resolve(provider.ID); builtIn &&
			provider.Setup.Contract.PreferGlobal == nil &&
			embedded.Setup.Contract.PreferGlobal != nil {
			// Copy the value rather than the pointer: sharing it would let a write
			// through the resolved provider mutate the embedded built-in defaults.
			preferGlobal := *embedded.Setup.Contract.PreferGlobal
			provider.Setup.Contract.PreferGlobal = &preferGlobal
		}
		if seen[provider.ID] {
			continue
		}
		seen[provider.ID] = true
		resolved = append(resolved, provider)
	}
	return resolved, nil
}
