package envgate

import (
	"testing"

	"github.com/liza-mas/liza/internal/brand"
)

func TestValueUsesBrandedEnvBeforeLegacy(t *testing.T) {
	restore := setTestBrandEnvPrefix(t, "ACME_AGENT")
	defer restore()

	t.Setenv("ACME_AGENT_ENABLE_STACKLIT", "true")
	t.Setenv("LIZA_ENABLE_STACKLIT", "false")

	if got := Value("LIZA_ENABLE_STACKLIT"); got != "true" {
		t.Fatalf("Value() = %q, want branded value", got)
	}
	if !TruthyEnv("LIZA_ENABLE_STACKLIT") {
		t.Fatal("TruthyEnv() = false, want true from branded value")
	}
	if got := Value("ACME_AGENT_ENABLE_STACKLIT"); got != "true" {
		t.Fatalf("Value(branded) = %q, want branded value", got)
	}
	if !TruthyEnv("ACME_AGENT_ENABLE_STACKLIT") {
		t.Fatal("TruthyEnv(branded) = false, want true from branded value")
	}
	if got := Value("ENABLE_STACKLIT"); got != "true" {
		t.Fatalf("Value(suffix) = %q, want branded value", got)
	}
}

func TestValueEmptyBrandedEnvSuppressesLegacyAlias(t *testing.T) {
	restore := setTestBrandEnvPrefix(t, "ACME_AGENT")
	defer restore()

	t.Setenv("ACME_AGENT_ENABLE_STACKLIT", "")
	t.Setenv("LIZA_ENABLE_STACKLIT", "true")

	if got := Value("ACME_AGENT_ENABLE_STACKLIT"); got != "" {
		t.Fatalf("Value() = %q, want empty branded value", got)
	}
	if TruthyEnv("ACME_AGENT_ENABLE_STACKLIT") {
		t.Fatal("TruthyEnv() = true, want false from empty branded value")
	}
}

func setTestBrandEnvPrefix(t *testing.T, prefix string) func() {
	t.Helper()
	previous := brand.EnvPrefix
	brand.EnvPrefix = prefix
	return func() {
		brand.EnvPrefix = previous
	}
}
