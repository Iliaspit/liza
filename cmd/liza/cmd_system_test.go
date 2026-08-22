package main

import (
	"bytes"
	"os"
	"path/filepath"
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

func TestPlanningReviewChurnDocumentationContract(t *testing.T) {
	readRepoFile := func(t *testing.T, path string) string {
		t.Helper()
		content, err := os.ReadFile(filepath.Join("..", "..", path))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return string(content)
	}
	section := func(t *testing.T, content, start, end string) string {
		t.Helper()
		startAt := strings.Index(content, start)
		if startAt < 0 {
			t.Fatalf("missing section start %q", start)
		}
		endAt := strings.Index(content[startAt+len(start):], end)
		if endAt < 0 {
			t.Fatalf("missing section end %q after %q", end, start)
		}
		return content[startAt : startAt+len(start)+endAt]
	}

	protocol := readRepoFile(t, "specs/protocols/circuit-breaker.md")
	buildSpec := readRepoFile(t, "specs/build/1.5 - Circuit Breaker.md")
	hardeningInventory := readRepoFile(t, "docs/liza-hardened-mas.md")
	architecture := readRepoFile(t, "specs/architecture/architectural-issues.md")
	architectureEntry := section(t, architecture,
		"### Circuit Breaker Depends on Participant Reporting",
		"### No Source Type for Pre-Implementation Spec Findings")

	surfaces := []struct {
		name    string
		content string
	}{
		{name: "analyze help", content: analyzeCmd.Long},
		{name: "protocol", content: protocol},
		{name: "functional spec", content: readRepoFile(t, "specs/functional/1.5 - Circuit Breaker.md")},
		{name: "build spec", content: buildSpec},
		{name: "operator recipes", content: readRepoFile(t, "docs/RECIPES.md")},
		{name: "hardening inventory", content: hardeningInventory},
		{name: "architecture blind spot", content: architectureEntry},
	}
	sharedRequirements := []string{
		"anomalies",
		"planning task review evidence",
		"four or more",
		"`MERGED` tasks remain eligible",
		"planning_review_churn",
		"PLANNING_CONVERGENCE_DEGRADED",
	}
	for _, surface := range surfaces {
		t.Run(surface.name, func(t *testing.T) {
			for _, want := range sharedRequirements {
				if !strings.Contains(surface.content, want) {
					t.Errorf("missing %q", want)
				}
			}
		})
	}

	identity := section(t, protocol,
		"## Identity and Constraints",
		"## Input: Anomalies and Planning Task Review Evidence")
	for _, want := range []string{"permissions:", "read:", "anomalies", "planning task review evidence"} {
		if !strings.Contains(identity, want) {
			t.Errorf("Identity and Constraints missing %q", want)
		}
	}
	input := section(t, protocol,
		"## Input: Anomalies and Planning Task Review Evidence",
		"## Anomaly Types")
	for _, want := range []string{"anomalies", "planning task review evidence"} {
		if !strings.Contains(input, want) {
			t.Errorf("input section missing %q", want)
		}
	}
	patterns := section(t, protocol, "## Pattern Detection Rules", "## Severity Classification")
	for _, want := range []string{
		"positive `review_cycles_total`",
		"when `review_cycles_total` is zero",
		"`rejected` and `review_verdict_rejected`",
	} {
		if !strings.Contains(patterns, want) {
			t.Errorf("Pattern Detection Rules missing %q", want)
		}
	}
	watermark := section(t, protocol, "### Acknowledgement Watermark", "## Severity Classification")
	for _, want := range []string{"planning task review evidence", "strictly after the watermark"} {
		if !strings.Contains(watermark, want) {
			t.Errorf("Acknowledgement Watermark missing %q", want)
		}
	}
	severity := section(t, protocol, "## Severity Classification", "## Circuit Breaker Activation")
	for _, want := range []string{
		"`PLANNING_CONVERGENCE_DEGRADED`",
		"code-planning convergence",
		"pause downstream fan-out and inspect rejection evidence before choosing remediation",
	} {
		if !strings.Contains(severity, want) {
			t.Errorf("Severity Classification missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"Input: Anomalies Section",
		"reads the anomalies section",
		"detection normally evaluates durable `state.anomalies`",
	} {
		if strings.Contains(protocol, forbidden) {
			t.Errorf("protocol retains anomaly-only text %q", forbidden)
		}
	}

	for _, forbidden := range []string{"from anomaly signals", "evaluates anomalies", "anomaly patterns"} {
		if strings.Contains(buildSpec, forbidden) {
			t.Errorf("build spec retains anomaly-only text %q", forbidden)
		}
	}
	if strings.Contains(hardeningInventory, "Pattern detection on anomalies") {
		t.Error("hardening inventory retains anomaly-only pattern-detection claim")
	}
	if !strings.Contains(hardeningInventory,
		"| planning_review_churn | four or more planning rejection cycles; `MERGED` tasks remain eligible | PLANNING_CONVERGENCE_DEGRADED |") {
		t.Error("hardening inventory missing the planning_review_churn pattern row")
	}
	for _, want := range []string{"partial independent mitigation", "broader participant-reporting blind spot remains open"} {
		if !strings.Contains(architectureEntry, want) {
			t.Errorf("architecture entry missing %q", want)
		}
	}
}
