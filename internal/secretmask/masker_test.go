package secretmask

import "testing"

func TestMaskerOverlappingValuesReplacedLongestFirst(t *testing.T) {
	m := NewFromEnv([]string{
		"CUSTOM_TOKEN=1234efgh",
		"OPENAI_API_KEY=abcd1234efgh5678",
	})

	input := "full=abcd1234efgh5678 partial=1234efgh"
	want := "full=*** partial=***"
	if got := m.MaskText(input); got != want {
		t.Errorf("MaskText = %q, want %q", got, want)
	}
}

func TestIsSecretKey(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"OPENAI_API_KEY", true},
		{"CUSTOM_TOKEN", true},
		{"SECRET_VALUE", true},
		{"TOKENIZERS_PARALLELISM", false},
		{"PASSWORD_STORE_DIR", false},
		{"HOME", false},
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			if got := IsSecretKey(tc.key); got != tc.want {
				t.Errorf("IsSecretKey(%q) = %v, want %v", tc.key, got, tc.want)
			}
		})
	}
}
