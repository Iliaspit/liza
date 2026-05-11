package secretmask

import (
	"bytes"
	"strings"
	"testing"
)

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

func TestStreamingWriterMasksSecretSplitAcrossWrites(t *testing.T) {
	m := NewFromEnv([]string{
		"ANTHROPIC_API_KEY=sk-ant-secret-key-value",
	})
	var buf bytes.Buffer
	w := m.NewStreamingWriter(&buf)

	if _, err := w.Write([]byte("before sk-ant-sec")); err != nil {
		t.Fatalf("first Write error: %v", err)
	}
	if got := buf.String(); got != "before " {
		t.Fatalf("partial secret should be held, got %q", got)
	}

	if _, err := w.Write([]byte("ret-key-value after")); err != nil {
		t.Fatalf("second Write error: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush error: %v", err)
	}

	got := buf.String()
	if strings.Contains(got, "sk-ant-secret-key-value") {
		t.Fatalf("streamed output leaked secret: %q", got)
	}
	want := "before *** after"
	if got != want {
		t.Fatalf("streamed output = %q, want %q", got, want)
	}
}

func TestStreamingWriterPreservesMaskTextSemanticsForOverlappingSecrets(t *testing.T) {
	m := NewFromEnv([]string{
		"CUSTOM_TOKEN=1234efgh",
		"OPENAI_API_KEY=abcd1234efgh5678",
	})
	var buf bytes.Buffer
	w := m.NewStreamingWriter(&buf)

	for _, chunk := range []string{"full=abcd", "1234efgh", "5678 partial=1234", "efgh"} {
		if _, err := w.Write([]byte(chunk)); err != nil {
			t.Fatalf("Write(%q) error: %v", chunk, err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush error: %v", err)
	}

	got := buf.String()
	want := "full=*** partial=***"
	if got != want {
		t.Fatalf("streamed output = %q, want %q", got, want)
	}
}

func TestStreamingWriterMasksCompleteSecretBeforeHoldingOverlappingPrefix(t *testing.T) {
	m := NewFromEnv([]string{
		"FIRST_TOKEN=AABBCCXX",
		"SECOND_TOKEN=CCXXDDEE",
	})
	var buf bytes.Buffer
	w := m.NewStreamingWriter(&buf)

	if _, err := w.Write([]byte("AABBCCXXDD")); err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush error: %v", err)
	}

	got := buf.String()
	if strings.Contains(got, "AABBCCXX") {
		t.Fatalf("streamed output leaked complete overlapping secret: %q", got)
	}
	want := "***DD"
	if got != want {
		t.Fatalf("streamed output = %q, want %q", got, want)
	}
}

func TestStreamingWriterWithoutSecretsWritesImmediately(t *testing.T) {
	m := NewFromEnv([]string{"HOME=/home/user"})
	var buf bytes.Buffer
	w := m.NewStreamingWriter(&buf)

	if _, err := w.Write([]byte("visible output")); err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if got := buf.String(); got != "visible output" {
		t.Fatalf("output should stream immediately without secrets, got %q", got)
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
