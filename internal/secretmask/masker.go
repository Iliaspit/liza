package secretmask

import (
	"os"
	"sort"
	"strings"
)

// Masker replaces known secret values with a redaction marker.
// It applies longest-first replacement to handle overlapping values safely.
type Masker struct {
	secrets []string
}

var secretKeyMatchers = []func(string) bool{
	func(k string) bool { return k == "API_KEY" || k == "SECRET_KEY" },
	func(k string) bool {
		return strings.HasSuffix(k, "_API_KEY") ||
			strings.HasSuffix(k, "_APIKEY") ||
			strings.HasSuffix(k, "_TOKEN") ||
			strings.HasSuffix(k, "_SECRET") ||
			strings.HasSuffix(k, "_PASSWORD")
	},
	func(k string) bool { return strings.HasPrefix(k, "SECRET_") },
	func(k string) bool {
		switch k {
		case "ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GOOGLE_API_KEY",
			"GEMINI_API_KEY", "MISTRAL_API_KEY", "MOONSHOT_API_KEY",
			"AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN",
			"GITHUB_TOKEN", "GH_TOKEN", "GITLAB_TOKEN",
			"NPM_TOKEN", "DOCKER_PASSWORD":
			return true
		}
		return false
	},
}

const (
	redactionMarker = "***"
	minSecretLen    = 8
)

// IsSecretKey reports whether the environment variable name looks sensitive.
func IsSecretKey(key string) bool {
	upper := strings.ToUpper(key)
	for _, match := range secretKeyMatchers {
		if match(upper) {
			return true
		}
	}
	return false
}

// New builds a masker from the current process environment.
func New() *Masker {
	return NewFromEnv(os.Environ())
}

// NewFromEnv builds a masker from an explicit environment slice.
func NewFromEnv(environ []string) *Masker {
	seen := make(map[string]struct{})
	var secrets []string

	for _, entry := range environ {
		k, v, ok := strings.Cut(entry, "=")
		if !ok || v == "" || len(v) < minSecretLen {
			continue
		}
		if !IsSecretKey(k) {
			continue
		}
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		secrets = append(secrets, v)
	}

	sortSecrets(secrets)
	return &Masker{secrets: secrets}
}

// AddEntries extends the masker with additional KEY=VALUE entries.
func (m *Masker) AddEntries(entries []string) {
	seen := make(map[string]struct{}, len(m.secrets))
	for _, s := range m.secrets {
		seen[s] = struct{}{}
	}

	for _, entry := range entries {
		k, v, ok := strings.Cut(entry, "=")
		if !ok || v == "" || len(v) < minSecretLen {
			continue
		}
		if !IsSecretKey(k) {
			continue
		}
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		m.secrets = append(m.secrets, v)
	}

	sortSecrets(m.secrets)
}

// MaskText replaces all occurrences of collected secret values in text.
func (m *Masker) MaskText(text string) string {
	if len(m.secrets) == 0 {
		return text
	}
	result := text
	for _, secret := range m.secrets {
		result = strings.ReplaceAll(result, secret, redactionMarker)
	}
	return result
}

func sortSecrets(secrets []string) {
	sort.Slice(secrets, func(i, j int) bool {
		return len(secrets[i]) > len(secrets[j])
	})
}
