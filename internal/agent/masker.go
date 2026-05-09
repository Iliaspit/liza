package agent

import (
	"github.com/liza-mas/liza/internal/secretmask"
)

type SecretMasker = secretmask.Masker

func isSecretKey(key string) bool {
	return secretmask.IsSecretKey(key)
}

// NewSecretMasker builds a masker from the current process environment.
// It snapshots os.Environ() at call time; later env changes are not reflected.
func NewSecretMasker() *SecretMasker {
	return secretmask.New()
}

// newSecretMaskerFromEnv builds a masker from an explicit environ slice (for testing).
func newSecretMaskerFromEnv(environ []string) *SecretMasker {
	return secretmask.NewFromEnv(environ)
}
