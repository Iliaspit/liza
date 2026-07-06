package envgate

import (
	"fmt"
	"os"
	"strings"

	"github.com/liza-mas/liza/internal/brand"
)

// Truthy reports whether a user-facing environment gate is enabled.
func Truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true":
		return true
	default:
		return false
	}
}

// Value returns the value for a user-facing env var, honoring the branded
// namespace first and the legacy LIZA_* name as a compatibility alias. The input
// may be a branded env name, a legacy LIZA_* env name, or a bare suffix.
func Value(name string) string {
	return Lookup(name).Value
}

// Lookup returns detailed alias resolution for a user-facing env var.
func Lookup(name string) brand.EnvLookup {
	suffix := envSuffix(name)
	values := brand.RuntimeValues()
	brandedName := values.EnvName(suffix)
	legacyName := brand.LegacyEnvName(suffix)
	// Feature gates are presence-aware so an explicitly empty branded env value
	// disables the gate instead of falling through to a legacy LIZA_* alias.
	if branded, ok := os.LookupEnv(brandedName); ok {
		out := brand.EnvLookup{Value: strings.TrimSpace(branded), Source: brandedName}
		if legacy, ok := os.LookupEnv(legacyName); ok && legacyName != brandedName {
			legacy = strings.TrimSpace(legacy)
			if legacy != "" && legacy != out.Value {
				out.Warning = fmt.Sprintf("%s and %s are both set; using %s", brandedName, legacyName, brandedName)
			}
		}
		return out
	}
	if legacy, ok := os.LookupEnv(legacyName); ok {
		return brand.EnvLookup{Value: strings.TrimSpace(legacy), Source: legacyName}
	}
	return brand.EnvLookup{}
}

// TruthyEnv reports whether a user-facing env var is enabled through the
// branded namespace or legacy compatibility alias.
func TruthyEnv(name string) bool {
	return Truthy(Value(name))
}

func envSuffix(name string) string {
	name = strings.TrimSpace(name)
	values := brand.RuntimeValues()
	for _, prefix := range []string{values.EnvPrefix + "_", brand.LegacyEnvName("")} {
		if strings.HasPrefix(name, prefix) {
			return strings.TrimPrefix(name, prefix)
		}
	}
	return strings.TrimPrefix(name, "_")
}
