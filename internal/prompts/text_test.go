package prompts

import (
	"strings"
	"testing"
)

func TestTruncateText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxRunes int
		want     string
	}{
		{"short", "hello", 100, "hello"},
		{"exact", strings.Repeat("a", 100), 100, strings.Repeat("a", 100)},
		{"long", strings.Repeat("b", 200), 100, strings.Repeat("b", 100) + "..."},
		{"empty", "", 100, ""},
		{"unicode preserved when under limit", "h\u00e9llo w\u00f6rld", 100, "h\u00e9llo w\u00f6rld"},
		{"unicode safe truncation", "caf\u00e9", 3, "caf..."},
		{"zero limit", "hello", 0, "..."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := TruncateText(tc.input, tc.maxRunes)
			if got != tc.want {
				t.Errorf("TruncateText(%q, %d) = %q, want %q", tc.input, tc.maxRunes, got, tc.want)
			}
		})
	}
}
