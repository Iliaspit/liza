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

	output := runSessionContextHook(t, hookPath, sessionStartPayload(t, projectRoot), nil, 0)
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

func TestSessionContextHook_AvoidsBash4CaseConversion(t *testing.T) {
	for _, pattern := range []string{",,}", "^^}"} {
		if strings.Contains(string(sessionContextHookContent), pattern) {
			t.Fatalf("session-context.sh uses Bash 4-only case conversion %q", pattern)
		}
	}
}

func TestSessionContextHook_InstructsAgentsToRunStacklitSummaryWhenAvailable(t *testing.T) {
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

	output := runSessionContextHook(t, hookPath, sessionStartPayload(t, projectRoot), nil, 0)
	context := sessionStartAdditionalContext(t, output)
	want := "Run `stacklit derive --ai-summary -i '" + filepath.Join(projectRoot, "stacklit.json") + "'` at the end of the session initialization."
	if !strings.Contains(context, want) {
		t.Fatalf("startup context should instruct agent to run stacklit summary, missing %q in:\n%s", want, context)
	}
	if strings.Contains(context, "Stacklit summary:") {
		t.Fatalf("startup context should not inline stacklit summary, got:\n%s", context)
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

	output := runSessionContextHook(t, hookPath, sessionStartPayload(t, projectRoot), []string{
		"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
	}, 0)
	context := sessionStartAdditionalContext(t, output)
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

	output := runSessionContextHook(t, hookPath, sessionStartPayload(t, projectRoot), []string{
		"LIZA_AGENT_ID=coder-1",
	}, 0)
	context := sessionStartAdditionalContext(t, output)
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

	output := runSessionContextHook(t, hookPath, sessionStartPayload(t, projectRoot), nil, 0)
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

	output := runSessionContextHook(t, hookPath, sessionStartPayload(t, projectRoot), nil, 0)
	context := sessionStartAdditionalContext(t, output)
	if !strings.Contains(context, "Liza session initialization is mandatory") {
		t.Fatalf("startup context should include initialization reminder, got:\n%s", context)
	}
	if strings.Contains(context, "Liza repository indexes detected") {
		t.Fatalf("startup context should omit index paths without indexes, got:\n%s", context)
	}
}

func TestSessionContextHook_EmitsStacklitBlockOnlyWhenStacklitArtifactExists(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	hookPath := writeSessionContextHook(t)
	projectRoot := t.TempDir()
	writeLizaIndexHook(t, projectRoot)
	if err := os.WriteFile(filepath.Join(projectRoot, "stacklit.json"), []byte("{}\n"), 0644); err != nil {
		t.Fatalf("write stacklit index: %v", err)
	}

	output := runSessionContextHook(t, hookPath, sessionStartPayload(t, projectRoot), nil, 0)
	context := sessionStartAdditionalContext(t, output)
	for _, want := range []string{
		"Liza repository indexes detected",
		"Stacklit index: " + filepath.Join(projectRoot, "stacklit.json"),
		"stacklit derive --ai-summary -i '" + filepath.Join(projectRoot, "stacklit.json") + "'",
	} {
		if !strings.Contains(context, want) {
			t.Fatalf("startup context missing %q, got:\n%s", want, context)
		}
	}
	for _, notWant := range []string{
		"SCIP indexes:",
		"scip-search symbols --index",
		"Semble semantic search is available",
	} {
		if strings.Contains(context, notWant) {
			t.Fatalf("startup context should omit unavailable optional block %q, got:\n%s", notWant, context)
		}
	}
}

func TestSessionContextHook_EmitsScipBlockOnlyWhenScipArtifactExists(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	hookPath := writeSessionContextHook(t)
	projectRoot := t.TempDir()
	writeLizaIndexHook(t, projectRoot)
	if err := os.WriteFile(filepath.Join(projectRoot, "go.scip"), []byte("index\n"), 0644); err != nil {
		t.Fatalf("write Go SCIP index: %v", err)
	}

	output := runSessionContextHook(t, hookPath, sessionStartPayload(t, projectRoot), nil, 0)
	context := sessionStartAdditionalContext(t, output)
	for _, want := range []string{
		"Liza repository indexes detected",
		"SCIP indexes:",
		"Go index: " + filepath.Join(projectRoot, "go.scip"),
		"scip-search symbols --index <index-path> --name Foo --name Bar",
	} {
		if !strings.Contains(context, want) {
			t.Fatalf("startup context missing %q, got:\n%s", want, context)
		}
	}
	for _, notWant := range []string{
		"Stacklit index:",
		"stacklit derive --ai-summary",
		"Semble semantic search is available",
	} {
		if strings.Contains(context, notWant) {
			t.Fatalf("startup context should omit unavailable optional block %q, got:\n%s", notWant, context)
		}
	}
}

func TestSessionContextHook_FiltersMissingPairingProjectDocs(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	hookPath := writeSessionContextHook(t)
	projectRoot := t.TempDir()

	output := runSessionContextHook(t, hookPath, sessionStartPayload(t, projectRoot), nil, 0)
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

	output := runSessionContextHook(t, hookPath, sessionStartPayload(t, projectRoot), []string{
		"LIZA_AGENT_ID=coder-1",
	}, 0)
	context := sessionStartAdditionalContext(t, output)
	if !strings.Contains(context, "Liza session initialization is mandatory") {
		t.Fatalf("startup context should include initialization reminder for Liza agents, got:\n%s", context)
	}
	if strings.Contains(context, "Liza repository indexes detected") {
		t.Fatalf("startup context should not include repo-root indexes for Liza agent sessions, got:\n%s", context)
	}
}

func TestSessionContextHook_EmitsSembleWhenEnabledSafeAndOfflineReady(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	if runtime.GOOS == "windows" {
		t.Skip("shell script test")
	}

	hookPath := writeSessionContextHook(t)
	projectRoot := t.TempDir()
	writeRootSembleIgnore(t, projectRoot)
	binDir := writeFakeSembleTools(t, true)

	output := runSessionContextHook(t, hookPath, sessionStartPayload(t, projectRoot), []string{
		"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"LIZA_ENABLE_SEMBLE=true",
	}, 0)
	context := sessionStartAdditionalContext(t, output)
	for _, want := range []string{
		"Semble semantic search is available for this repo root: " + projectRoot,
		"env HF_HUB_OFFLINE=1 semble search \"where is review submission validated?\" '" + projectRoot + "'",
		"env HF_HUB_OFFLINE=1 semble search \"where is task superseding specified?\" '" + projectRoot + "' --content docs",
		"Use --content with one of: code, docs, config, all; code is the default.",
		"Semble returns candidate chunks, not proof",
		"Do not use rg for broad-scope or common-word conceptual queries.",
	} {
		if !strings.Contains(context, want) {
			t.Fatalf("session context missing %q, got:\n%s", want, context)
		}
	}
	if strings.Contains(context, "Liza repository indexes detected") {
		t.Fatalf("Semble-only context should not claim Stacklit/SCIP indexes, got:\n%s", context)
	}
}

func TestSessionContextHook_OmitsSembleWithoutRootIgnore(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	if runtime.GOOS == "windows" {
		t.Skip("shell script test")
	}

	hookPath := writeSessionContextHook(t)
	projectRoot := t.TempDir()
	binDir := writeFakeSembleTools(t, true)

	output := runSessionContextHook(t, hookPath, sessionStartPayload(t, projectRoot), []string{
		"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"LIZA_ENABLE_SEMBLE=true",
	}, 0)
	context := sessionStartAdditionalContext(t, output)
	if strings.Contains(context, "Semble semantic search is available") {
		t.Fatalf("startup context should omit Semble without root .sembleignore, got:\n%s", context)
	}
}

func TestSessionContextHook_OmitsSembleWithIncompleteRootIgnore(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	if runtime.GOOS == "windows" {
		t.Skip("shell script test")
	}

	hookPath := writeSessionContextHook(t)
	projectRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectRoot, ".sembleignore"), []byte(".liza/\n.worktrees/\n"), 0644); err != nil {
		t.Fatalf("write incomplete root .sembleignore: %v", err)
	}
	binDir := writeFakeSembleTools(t, true)

	output := runSessionContextHook(t, hookPath, sessionStartPayload(t, projectRoot), []string{
		"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"LIZA_ENABLE_SEMBLE=true",
	}, 0)
	context := sessionStartAdditionalContext(t, output)
	if strings.Contains(context, "Semble semantic search is available") {
		t.Fatalf("startup context should omit Semble with incomplete root .sembleignore, got:\n%s", context)
	}
}

func TestSessionContextHook_OmitsSembleWhenOfflineValidationFails(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	if runtime.GOOS == "windows" {
		t.Skip("shell script test")
	}

	hookPath := writeSessionContextHook(t)
	projectRoot := t.TempDir()
	writeRootSembleIgnore(t, projectRoot)
	binDir := writeFakeSembleTools(t, false)

	output := runSessionContextHook(t, hookPath, sessionStartPayload(t, projectRoot), []string{
		"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"LIZA_ENABLE_SEMBLE=true",
	}, 0)
	context := sessionStartAdditionalContext(t, output)
	if strings.Contains(context, "Semble semantic search is available") {
		t.Fatalf("startup context should omit Semble when offline validation fails, got:\n%s", context)
	}
}

func TestSessionContextHook_SuppressesSembleForLizaAgentSessions(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	if runtime.GOOS == "windows" {
		t.Skip("shell script test")
	}

	hookPath := writeSessionContextHook(t)
	projectRoot := t.TempDir()
	writeRootSembleIgnore(t, projectRoot)
	binDir := writeFakeSembleTools(t, true)

	output := runSessionContextHook(t, hookPath, sessionStartPayload(t, projectRoot), []string{
		"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"LIZA_ENABLE_SEMBLE=true",
		"LIZA_AGENT_ID=coder-1",
	}, 0)
	context := sessionStartAdditionalContext(t, output)
	if strings.Contains(context, "Semble semantic search is available") {
		t.Fatalf("Liza agent context should not include Pairing Semble guidance, got:\n%s", context)
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

func writeLizaIndexHook(t *testing.T, projectRoot string) {
	t.Helper()

	hooksDir := filepath.Join(projectRoot, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("create git hooks dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "post-commit"), []byte("#!/bin/sh\nliza-index\n"), 0755); err != nil {
		t.Fatalf("write post-commit hook: %v", err)
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

func runSessionContextHook(t *testing.T, hookPath, payload string, extraEnv []string, wantCode int) string {
	t.Helper()
	cmd := exec.Command("bash", hookPath)
	cmd.Stdin = strings.NewReader(payload)
	cmd.Env = sessionContextHookEnv(extraEnv)
	output, err := cmd.CombinedOutput()
	if wantCode == 0 {
		if err != nil {
			t.Fatalf("hook exited non-zero: %v\n%s", err, output)
		}
		return string(output)
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("hook exit = %v, want code %d\n%s", err, wantCode, output)
	}
	if exitErr.ExitCode() != wantCode {
		t.Fatalf("hook exit code = %d, want %d\n%s", exitErr.ExitCode(), wantCode, output)
	}
	return string(output)
}

func sessionContextHookEnv(extraEnv []string) []string {
	env := make([]string, 0, len(os.Environ())+len(extraEnv))
	for _, item := range os.Environ() {
		switch {
		case strings.HasPrefix(item, "CLAUDE_PROJECT_DIR="):
			continue
		case strings.HasPrefix(item, "LIZA_AGENT_ID="):
			continue
		case strings.HasPrefix(item, "LIZA_ENABLE_SEMBLE="):
			continue
		}
		env = append(env, item)
	}
	return append(env, extraEnv...)
}

func writeRootSembleIgnore(t *testing.T, projectRoot string) {
	t.Helper()
	content := strings.Join([]string{
		".liza/",
		".worktrees/",
		"stacklit.json",
		"*.scip",
		".env",
		".env.*",
		"*.env",
		"credentials.*",
		"secrets.*",
		"*secret*.*",
		"*.pem",
		"*.key",
		"*.p12",
		"*.pfx",
		"*.jks",
		"*_rsa",
		"*_dsa",
		"*_ecdsa",
		"*_ed25519",
		"*.keystore",
		"*.truststore",
		"config/secrets/",
		"**/secrets/",
		"serviceAccountKey.json",
		"*-credentials.json",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(projectRoot, ".sembleignore"), []byte(content), 0644); err != nil {
		t.Fatalf("write root .sembleignore: %v", err)
	}
}

func writeFakeSembleTools(t *testing.T, validationSucceeds bool) string {
	t.Helper()
	binDir := t.TempDir()
	timeoutPath := filepath.Join(binDir, "timeout")
	if err := os.WriteFile(timeoutPath, []byte("#!/bin/sh\nshift\nexec \"$@\"\n"), 0755); err != nil {
		t.Fatalf("write fake timeout: %v", err)
	}
	exitCode := "0"
	if !validationSucceeds {
		exitCode = "42"
	}
	sembleScript := "#!/bin/sh\n" +
		"test \"${HF_HUB_OFFLINE:-}\" = \"1\" || exit 17\n" +
		"test \"$1\" = \"search\" || exit 18\n" +
		"exit " + exitCode + "\n"
	if err := os.WriteFile(filepath.Join(binDir, "semble"), []byte(sembleScript), 0755); err != nil {
		t.Fatalf("write fake semble: %v", err)
	}
	return binDir
}
