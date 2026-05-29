package embedded

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSessionContextHook_EmitsSessionStartContextForIndexedRepo(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	hookPath := writeSessionContextHook(t)
	projectRoot := t.TempDir()
	writePairingProjectDocs(t, projectRoot)
	writeIndexedRepoMarkers(t, projectRoot)

	output := runHook(t, hookPath, sessionStartPayload(t, projectRoot), 0)
	var got struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("session context output is not JSON: %v\n%s", err, output)
	}
	if got.HookSpecificOutput.HookEventName != "SessionStart" {
		t.Fatalf("hookEventName = %q, want SessionStart", got.HookSpecificOutput.HookEventName)
	}

	context := got.HookSpecificOutput.AdditionalContext
	for _, want := range []string{
		"Liza session initialization is mandatory",
		"~/.liza/PAIRING_MODE.md",
		"~/.liza/AGENT_TOOLS.md",
		"REPOSITORY.md",
		"docs/USAGE.md",
		"~/.liza/COLLABORATION_CONTINUITY.md",
		"Liza repository indexes detected",
		"stacklit derive --ai-summary -i '" + filepath.Join(projectRoot, "stacklit.json") + "'",
		"Go index: " + filepath.Join(projectRoot, "go.scip"),
		"Python index: " + filepath.Join(projectRoot, "python.scip"),
		"scip-search symbols --index <index-path> --name Foo --name Bar",
		"scip-search references --index <index-path> --symbol '<exact-foo>' --symbol '<exact-bar>' --location-only",
		"(except python): scip-search implementations --index <index-path> --symbol '<exact-symbol>'",
		"do not reflect uncommitted changes",
	} {
		if !strings.Contains(context, want) {
			t.Fatalf("session context missing %q, got:\n%s", want, context)
		}
	}
	for _, needle := range []string{
		"scip-search symbols --index",
		"scip-search references --index",
		"scip-search implementations --index",
	} {
		if got := strings.Count(context, needle); got != 1 {
			t.Fatalf("session context should include one compact %q example, got %d in:\n%s", needle, got, context)
		}
	}
	for _, unwanted := range []string{
		"scip-search symbols --index '" + filepath.Join(projectRoot, "go.scip") + "'",
		"scip-search symbols --index '" + filepath.Join(projectRoot, "python.scip") + "'",
	} {
		if strings.Contains(context, unwanted) {
			t.Fatalf("session context should not repeat path-specific SCIP commands, found %q in:\n%s", unwanted, context)
		}
	}
}

func TestSessionContextHook_IncludesBoundedStacklitSummaryWhenAvailable(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	if runtime.GOOS == "windows" {
		t.Skip("shell script test")
	}

	hookPath := writeSessionContextHook(t)
	projectRoot := t.TempDir()
	writePairingProjectDocs(t, projectRoot)
	writeIndexedRepoMarkers(t, projectRoot)
	binDir := t.TempDir()
	stacklitPath := filepath.Join(binDir, "stacklit")
	longOutput := strings.Repeat("summary ", 600)
	if err := os.WriteFile(stacklitPath, []byte("#!/bin/sh\nprintf '%s' '"+longOutput+"'\n"), 0755); err != nil {
		t.Fatalf("write fake stacklit: %v", err)
	}

	cmd := exec.Command("bash", hookPath)
	cmd.Stdin = strings.NewReader(sessionStartPayload(t, projectRoot))
	cmd.Env = append(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hook exited non-zero: %v\n%s", err, output)
	}
	context := sessionStartAdditionalContext(t, string(output))
	if !strings.Contains(context, "Stacklit summary:") ||
		!strings.Contains(context, "summary summary") {
		t.Fatalf("startup context should include stacklit summary, got:\n%s", context)
	}
	if strings.Count(context, "summary ") > 450 {
		t.Fatalf("stacklit summary should be bounded, got %d repetitions", strings.Count(context, "summary "))
	}
}

func TestSessionContextHook_StillEmitsContextWhenStacklitFails(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	if runtime.GOOS == "windows" {
		t.Skip("shell script test")
	}

	hookPath := writeSessionContextHook(t)
	projectRoot := t.TempDir()
	writePairingProjectDocs(t, projectRoot)
	writeIndexedRepoMarkers(t, projectRoot)
	binDir := t.TempDir()
	stacklitPath := filepath.Join(binDir, "stacklit")
	if err := os.WriteFile(stacklitPath, []byte("#!/bin/sh\necho boom >&2\nexit 42\n"), 0755); err != nil {
		t.Fatalf("write failing stacklit: %v", err)
	}

	cmd := exec.Command("bash", hookPath)
	cmd.Stdin = strings.NewReader(sessionStartPayload(t, projectRoot))
	cmd.Env = append(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hook should not fail when stacklit fails: %v\n%s", err, output)
	}
	context := sessionStartAdditionalContext(t, string(output))
	if !strings.Contains(context, "Liza session initialization is mandatory") ||
		!strings.Contains(context, "stacklit derive --ai-summary") {
		t.Fatalf("startup context should remain useful after stacklit failure, got:\n%s", context)
	}
	if strings.Contains(context, "Stacklit summary:") || strings.Contains(context, "boom") {
		t.Fatalf("failing stacklit output should not be surfaced, got:\n%s", context)
	}
}

func TestSessionContextHook_UsesMultiAgentModeForLizaAgentSessions(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	hookPath := writeSessionContextHook(t)
	projectRoot := t.TempDir()

	cmd := exec.Command("bash", hookPath)
	cmd.Stdin = strings.NewReader(sessionStartPayload(t, projectRoot))
	cmd.Env = append(os.Environ(), "LIZA_AGENT_ID=coder-1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hook exited non-zero: %v\n%s", err, output)
	}
	context := sessionStartAdditionalContext(t, string(output))
	if !strings.Contains(context, "~/.liza/MULTI_AGENT_MODE.md") {
		t.Fatalf("startup context should name MULTI_AGENT_MODE for Liza agents, got:\n%s", context)
	}
	if strings.Contains(context, "~/.liza/PAIRING_MODE.md") ||
		strings.Contains(context, "REPOSITORY.md") ||
		strings.Contains(context, "docs/USAGE.md") {
		t.Fatalf("Liza agent context should not include Pairing-only docs, got:\n%s", context)
	}
}

func TestSessionContextHook_EmitsInitContextWithoutLizaIndexHook(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	hookPath := writeSessionContextHook(t)
	projectRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectRoot, "stacklit.json"), []byte("{}\n"), 0644); err != nil {
		t.Fatalf("write stacklit index: %v", err)
	}

	output := runHook(t, hookPath, sessionStartPayload(t, projectRoot), 0)
	context := sessionStartAdditionalContext(t, output)
	if !strings.Contains(context, "Liza session initialization is mandatory") {
		t.Fatalf("startup context should include initialization reminder, got:\n%s", context)
	}
	if strings.Contains(context, "Liza repository indexes detected") {
		t.Fatalf("startup context should omit index paths without liza-index hook, got:\n%s", context)
	}
}

func TestSessionContextHook_EmitsInitContextWithoutIndexes(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	hookPath := writeSessionContextHook(t)
	projectRoot := t.TempDir()
	hooksDir := filepath.Join(projectRoot, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("create git hooks dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "post-commit"), []byte("#!/bin/sh\nliza-index\n"), 0755); err != nil {
		t.Fatalf("write post-commit hook: %v", err)
	}

	output := runHook(t, hookPath, sessionStartPayload(t, projectRoot), 0)
	context := sessionStartAdditionalContext(t, output)
	if !strings.Contains(context, "Liza session initialization is mandatory") {
		t.Fatalf("startup context should include initialization reminder, got:\n%s", context)
	}
	if strings.Contains(context, "Liza repository indexes detected") {
		t.Fatalf("startup context should omit index paths without indexes, got:\n%s", context)
	}
}

func TestSessionContextHook_FiltersMissingPairingProjectDocs(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	hookPath := writeSessionContextHook(t)
	projectRoot := t.TempDir()

	output := runHook(t, hookPath, sessionStartPayload(t, projectRoot), 0)
	context := sessionStartAdditionalContext(t, output)
	if !strings.Contains(context, "~/.liza/PAIRING_MODE.md") ||
		!strings.Contains(context, "~/.liza/AGENT_TOOLS.md") ||
		!strings.Contains(context, "~/.liza/COLLABORATION_CONTINUITY.md") {
		t.Fatalf("startup context missing required Pairing globals, got:\n%s", context)
	}
	for _, notWant := range []string{"REPOSITORY.md", "docs/USAGE.md", "GUARDRAILS.md"} {
		if strings.Contains(context, notWant) {
			t.Fatalf("startup context should filter missing %q, got:\n%s", notWant, context)
		}
	}
}

func TestSessionContextHook_SuppressesRepoIndexesForLizaAgentSessions(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	hookPath := writeSessionContextHook(t)
	projectRoot := t.TempDir()
	writeIndexedRepoMarkers(t, projectRoot)

	cmd := exec.Command("bash", hookPath)
	cmd.Stdin = strings.NewReader(sessionStartPayload(t, projectRoot))
	cmd.Env = append(os.Environ(), "LIZA_AGENT_ID=coder-1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hook exited non-zero: %v\n%s", err, output)
	}
	context := sessionStartAdditionalContext(t, string(output))
	if !strings.Contains(context, "Liza session initialization is mandatory") {
		t.Fatalf("startup context should include initialization reminder for Liza agents, got:\n%s", context)
	}
	if strings.Contains(context, "Liza repository indexes detected") {
		t.Fatalf("startup context should not include repo-root indexes for Liza agent sessions, got:\n%s", context)
	}
}

func writeSessionContextHook(t *testing.T) string {
	t.Helper()
	hookPath := filepath.Join(t.TempDir(), "session-context.sh")
	if err := os.WriteFile(hookPath, sessionContextHookContent, 0755); err != nil {
		t.Fatalf("write hook: %v", err)
	}
	return hookPath
}

func writePairingProjectDocs(t *testing.T, projectRoot string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(projectRoot, "REPOSITORY.md"), []byte("# repo\n"), 0644); err != nil {
		t.Fatalf("write REPOSITORY.md: %v", err)
	}
	docsDir := filepath.Join(projectRoot, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatalf("create docs dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "USAGE.md"), []byte("# usage\n"), 0644); err != nil {
		t.Fatalf("write docs/USAGE.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "GUARDRAILS.md"), []byte("# guardrails\n"), 0644); err != nil {
		t.Fatalf("write GUARDRAILS.md: %v", err)
	}
}

func sessionStartPayload(t *testing.T, cwd string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"hook_event_name": "SessionStart",
		"cwd":             cwd,
	})
	if err != nil {
		t.Fatalf("marshal session start payload: %v", err)
	}
	return string(payload)
}

func sessionStartAdditionalContext(t *testing.T, output string) string {
	t.Helper()
	var got struct {
		HookSpecificOutput struct {
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("session context output is not JSON: %v\n%s", err, output)
	}
	return got.HookSpecificOutput.AdditionalContext
}
