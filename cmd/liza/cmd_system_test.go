package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestTuiCmd_HeadlessFlag(t *testing.T) {
	flag := tuiCmd.Flags().Lookup("headless")
	if flag == nil {
		t.Fatal("tuiCmd missing --headless flag")
	}
	if flag.DefValue != "false" {
		t.Errorf("--headless default = %q, want %q", flag.DefValue, "false")
	}
	if flag.Usage == "" {
		t.Error("--headless flag has no usage text")
	}
}

func TestTuiCmd_IntervalFlag(t *testing.T) {
	flag := tuiCmd.Flags().Lookup("interval")
	if flag == nil {
		t.Fatal("tuiCmd missing --interval flag (needed for headless backward compatibility)")
	}
}

func TestTuiCmd_ShortDescription(t *testing.T) {
	want := "Interactive TUI dashboard for monitoring Liza"
	if tuiCmd.Short != want {
		t.Errorf("tuiCmd.Short = %q, want %q", tuiCmd.Short, want)
	}
}

func TestTuiCmd_FallsBackToHeadlessWithoutTTY(t *testing.T) {
	// Replace stdin with a pipe to simulate non-interactive (CI/cron).
	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = origStdin
		r.Close()
		w.Close()
	})

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(oldDir) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	resetRootCmdForTest(t)

	var stderr bytes.Buffer
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"tui"})

	err = rootCmd.Execute()

	// The command will error downstream (no git repo for project root),
	// but it must NOT fail with a TTY-related error.
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "tty") {
		t.Fatalf("got TTY error despite auto-fallback: %v", err)
	}

	// Verify the fallback notice was emitted to stderr.
	if !strings.Contains(stderr.String(), "falling back to headless mode") {
		t.Errorf("stderr = %q, want fallback notice", stderr.String())
	}
}

func TestTuiCmd_ExplicitHeadlessSkipsFallback(t *testing.T) {
	// Replace stdin with a pipe to simulate non-interactive,
	// but --headless is already set so no fallback message expected.
	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = origStdin
		r.Close()
		w.Close()
	})

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(oldDir) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	resetRootCmdForTest(t)

	var stderr bytes.Buffer
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"tui", "--headless"})

	_ = rootCmd.Execute()

	if strings.Contains(stderr.String(), "falling back to headless mode") {
		t.Errorf("fallback message should not appear when --headless is explicit, stderr = %q", stderr.String())
	}
}

func TestAnalyzeHelpListsProviderAuditDegradation(t *testing.T) {
	helpText := analyzeCmd.Long

	expected := []string{
		"provider_audit_degradation",
		"2+ agents or 3+ hits for same provider",
		"OBSERVABILITY_DEGRADED",
	}
	for _, want := range expected {
		if !strings.Contains(helpText, want) {
			t.Fatalf("analyze help missing %q\nHelp:\n%s", want, helpText)
		}
	}
}
