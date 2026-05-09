package agent

import (
	"testing"
)

func TestIsSecretKey(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		// Exact matches
		{"API_KEY", true},
		{"SECRET_KEY", true},

		// Suffix patterns
		{"ANTHROPIC_API_KEY", true},
		{"OPENAI_API_KEY", true},
		{"MY_SERVICE_APIKEY", true},
		{"AUTH_TOKEN", true},
		{"REFRESH_TOKEN", true},
		{"APP_SECRET", true},
		{"DB_PASSWORD", true},

		// Prefix: SECRET_
		{"SECRET_SAUCE", true},
		{"SECRET_VALUE", true},

		// Provider-specific
		{"GEMINI_API_KEY", true},
		{"MISTRAL_API_KEY", true},
		{"MOONSHOT_API_KEY", true},
		{"AWS_SECRET_ACCESS_KEY", true},
		{"AWS_SESSION_TOKEN", true},
		{"GITHUB_TOKEN", true},
		{"GH_TOKEN", true},
		{"GITLAB_TOKEN", true},
		{"NPM_TOKEN", true},
		{"DOCKER_PASSWORD", true},

		// Case insensitive
		{"api_key", true},
		{"openai_api_key", true},
		{"secret_key", true},

		// False positives that must NOT match
		{"TOKENIZERS_PARALLELISM", false},
		{"PASSWORD_STORE_DIR", false},
		{"TOKEN_BUCKET_SIZE", false},
		{"SECRETARIAT", false},
		{"HOME", false},
		{"PATH", false},
		{"GOPATH", false},
		{"LIZA_AGENT_ID", false},
		{"USER", false},
		{"SHELL", false},
		{"MY_SECRET_SETTING", false}, // neither _SECRET suffix nor SECRET_ prefix
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			if got := isSecretKey(tc.key); got != tc.want {
				t.Errorf("isSecretKey(%q) = %v, want %v", tc.key, got, tc.want)
			}
		})
	}
}

func TestSecretMaskerMaskText(t *testing.T) {
	t.Run("no secrets collected", func(t *testing.T) {
		m := newSecretMaskerFromEnv([]string{"HOME=/home/user", "PATH=/usr/bin"})
		input := "some log output with HOME=/home/user"
		if got := m.MaskText(input); got != input {
			t.Errorf("MaskText should not alter text when no secrets collected, got %q", got)
		}
	})

	t.Run("single secret replaced", func(t *testing.T) {
		m := newSecretMaskerFromEnv([]string{
			"OPENAI_API_KEY=sk-abc123xyz789",
			"HOME=/home/user",
		})
		input := "Error: auth failed with key sk-abc123xyz789 for model gpt-4"
		want := "Error: auth failed with key *** for model gpt-4"
		if got := m.MaskText(input); got != want {
			t.Errorf("MaskText =\n  %q\nwant\n  %q", got, want)
		}
	})

	t.Run("multiple secrets replaced", func(t *testing.T) {
		m := newSecretMaskerFromEnv([]string{
			"ANTHROPIC_API_KEY=sk-ant-long-secret-value",
			"GITHUB_TOKEN=ghp_1234567890abcdef",
		})
		input := "Using sk-ant-long-secret-value to call API, push with ghp_1234567890abcdef"
		want := "Using *** to call API, push with ***"
		if got := m.MaskText(input); got != want {
			t.Errorf("MaskText =\n  %q\nwant\n  %q", got, want)
		}
	})

	t.Run("overlapping values replaced longest first", func(t *testing.T) {
		// OPENAI_API_KEY value contains the shorter CUSTOM_TOKEN value as substring.
		m := newSecretMaskerFromEnv([]string{
			"OPENAI_API_KEY=abcd1234efgh5678",
			"CUSTOM_TOKEN=1234efgh",
		})
		input := "key=abcd1234efgh5678 and partial=1234efgh"
		want := "key=*** and partial=***"
		if got := m.MaskText(input); got != want {
			t.Errorf("MaskText =\n  %q\nwant\n  %q", got, want)
		}
	})

	t.Run("short values ignored", func(t *testing.T) {
		m := newSecretMaskerFromEnv([]string{
			"AUTH_TOKEN=short", // 5 chars < minSecretLen
		})
		input := "the value short should not be masked"
		if got := m.MaskText(input); got != input {
			t.Errorf("short secret value should not be masked, got %q", got)
		}
	})

	t.Run("empty env", func(t *testing.T) {
		m := newSecretMaskerFromEnv(nil)
		input := "nothing to mask"
		if got := m.MaskText(input); got != input {
			t.Errorf("nil env should produce no masking, got %q", got)
		}
	})

	t.Run("duplicate values deduplicated", func(t *testing.T) {
		m := newSecretMaskerFromEnv([]string{
			"OPENAI_API_KEY=same-secret-value-here",
			"BACKUP_API_KEY=same-secret-value-here",
		})
		// Should still work — just one replacement pass
		input := "key=same-secret-value-here"
		want := "key=***"
		if got := m.MaskText(input); got != want {
			t.Errorf("MaskText =\n  %q\nwant\n  %q", got, want)
		}
	})

	t.Run("false positive env vars not masked", func(t *testing.T) {
		m := newSecretMaskerFromEnv([]string{
			"TOKENIZERS_PARALLELISM=true-long-value-here",
			"PASSWORD_STORE_DIR=/home/user/.password-store-extended",
		})
		input := "TOKENIZERS_PARALLELISM=true-long-value-here and PASSWORD_STORE_DIR=/home/user/.password-store-extended"
		if got := m.MaskText(input); got != input {
			t.Errorf("non-secret env vars should not be masked, got %q", got)
		}
	})

	t.Run("multiple occurrences all replaced", func(t *testing.T) {
		m := newSecretMaskerFromEnv([]string{
			"ANTHROPIC_API_KEY=sk-ant-secret123",
		})
		input := "first=sk-ant-secret123 second=sk-ant-secret123"
		want := "first=*** second=***"
		if got := m.MaskText(input); got != want {
			t.Errorf("MaskText =\n  %q\nwant\n  %q", got, want)
		}
	})
}

func TestSecretMaskerAddEntries(t *testing.T) {
	t.Run("extends masker with new secrets", func(t *testing.T) {
		m := newSecretMaskerFromEnv([]string{
			"ANTHROPIC_API_KEY=sk-ant-original-key",
		})
		m.AddEntries([]string{
			"GITHUB_TOKEN=ghp_added-later-token",
		})
		input := "keys: sk-ant-original-key and ghp_added-later-token"
		want := "keys: *** and ***"
		if got := m.MaskText(input); got != want {
			t.Errorf("MaskText =\n  %q\nwant\n  %q", got, want)
		}
	})

	t.Run("skips non-secret entries", func(t *testing.T) {
		m := newSecretMaskerFromEnv(nil)
		m.AddEntries([]string{
			"CLAUDE_CODE_DISABLE_1M_CONTEXT=1",
			"HOME=/home/user",
		})
		input := "setting CLAUDE_CODE_DISABLE_1M_CONTEXT=1"
		if got := m.MaskText(input); got != input {
			t.Errorf("non-secret env should not be masked, got %q", got)
		}
	})

	t.Run("deduplicates with existing secrets", func(t *testing.T) {
		m := newSecretMaskerFromEnv([]string{
			"OPENAI_API_KEY=sk-same-secret-value",
		})
		m.AddEntries([]string{
			"BACKUP_API_KEY=sk-same-secret-value",
		})
		input := "first=sk-same-secret-value second=sk-same-secret-value"
		want := "first=*** second=***"
		if got := m.MaskText(input); got != want {
			t.Errorf("MaskText =\n  %q\nwant\n  %q", got, want)
		}
	})
}
