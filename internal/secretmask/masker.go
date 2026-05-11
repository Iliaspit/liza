package secretmask

import (
	"io"
	"os"
	"sort"
	"strings"
)

// Masker replaces known secret values with a redaction marker.
// It applies longest-first replacement to handle overlapping values safely.
type Masker struct {
	secrets []string
}

// StreamingWriter masks secret values while forwarding output incrementally.
// It holds only a suffix that could still become a secret in a future write.
type StreamingWriter struct {
	dst     io.Writer
	masker  *Masker
	pending string
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
	return &Masker{secrets: collectSecretValues(nil, environ)}
}

// AddEntries extends the masker with additional KEY=VALUE entries.
func (m *Masker) AddEntries(entries []string) {
	m.secrets = collectSecretValues(m.secrets, entries)
}

func collectSecretValues(existing []string, entries []string) []string {
	seen := make(map[string]struct{}, len(existing))
	secrets := make([]string, 0, len(existing)+len(entries))

	for _, secret := range existing {
		if _, dup := seen[secret]; dup {
			continue
		}
		seen[secret] = struct{}{}
		secrets = append(secrets, secret)
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
		secrets = append(secrets, v)
	}

	sortSecrets(secrets)
	return secrets
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

// NewStreamingWriter returns a writer that applies MaskText semantics across
// write boundaries before forwarding content to dst.
func (m *Masker) NewStreamingWriter(dst io.Writer) *StreamingWriter {
	return &StreamingWriter{dst: dst, masker: m}
}

func (w *StreamingWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if w.masker == nil || len(w.masker.secrets) == 0 {
		if _, err := w.dst.Write(p); err != nil {
			return 0, err
		}
		return len(p), nil
	}

	w.pending += string(p)
	if err := w.flushSafePrefix(); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Flush writes any held suffix. Callers should invoke it when the stream ends.
func (w *StreamingWriter) Flush() error {
	if w.pending == "" {
		return nil
	}
	masked := w.masker.MaskText(w.pending)
	w.pending = ""
	_, err := w.dst.Write([]byte(masked))
	return err
}

func (w *StreamingWriter) flushSafePrefix() error {
	holdLen := w.masker.longestSecretPrefixSuffix(w.pending)
	flushLen := len(w.pending) - holdLen
	flushLen = w.masker.adjustFlushLenForSecretOverlap(w.pending, flushLen)
	if flushLen <= 0 {
		return nil
	}

	masked := w.masker.MaskText(w.pending[:flushLen])
	if _, err := w.dst.Write([]byte(masked)); err != nil {
		return err
	}
	w.pending = w.pending[flushLen:]
	return nil
}

func (m *Masker) longestSecretPrefixSuffix(text string) int {
	holdLen := 0
	for _, secret := range m.secrets {
		maxPrefix := min(len(text), len(secret)-1)
		for prefixLen := maxPrefix; prefixLen > holdLen; prefixLen-- {
			if strings.HasSuffix(text, secret[:prefixLen]) {
				holdLen = prefixLen
				break
			}
		}
	}
	return holdLen
}

func (m *Masker) adjustFlushLenForSecretOverlap(text string, flushLen int) int {
	for {
		adjusted := flushLen
		for _, secret := range m.secrets {
			searchStart := 0
			for {
				idx := strings.Index(text[searchStart:], secret)
				if idx < 0 {
					break
				}
				start := searchStart + idx
				end := start + len(secret)
				if start < adjusted && adjusted < end {
					adjusted = start
				}
				searchStart = start + 1
			}
		}
		if adjusted == flushLen {
			return flushLen
		}
		flushLen = adjusted
	}
}

func sortSecrets(secrets []string) {
	sort.Slice(secrets, func(i, j int) bool {
		return len(secrets[i]) > len(secrets[j])
	})
}
