package embedded

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIndexContextHook_EmitsSessionStartContextForIndexedRepo(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	hookPath := writeIndexContextHook(t)
	projectRoot := t.TempDir()
	writeIndexedRepoMarkers(t, projectRoot)

	output := runHook(t, hookPath, sessionStartPayload(t, projectRoot), 0)
	var got struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("index context output is not JSON: %v\n%s", err, output)
	}
	if got.HookSpecificOutput.HookEventName != "SessionStart" {
		t.Fatalf("hookEventName = %q, want SessionStart", got.HookSpecificOutput.HookEventName)
	}

	context := got.HookSpecificOutput.AdditionalContext
	for _, want := range []string{
		"Liza repository indexes detected",
		"stacklit derive --ai-summary -i '" + filepath.Join(projectRoot, "stacklit.json") + "'",
		"scip-search symbols --index '" + filepath.Join(projectRoot, "go.scip") + "'",
		"scip-search references --index '" + filepath.Join(projectRoot, "go.scip") + "'",
		"scip-search implementations --index '" + filepath.Join(projectRoot, "go.scip") + "'",
		"scip-search symbols --index '" + filepath.Join(projectRoot, "python.scip") + "'",
		"do not reflect uncommitted changes",
	} {
		if !strings.Contains(context, want) {
			t.Fatalf("index context missing %q, got:\n%s", want, context)
		}
	}
	if strings.Contains(context, "scip-search implementations --index '"+filepath.Join(projectRoot, "python.scip")+"'") {
		t.Fatalf("python context should not include unsupported implementations command, got:\n%s", context)
	}
}

func TestIndexContextHook_EmitsNothingWithoutLizaIndexHook(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	hookPath := writeIndexContextHook(t)
	projectRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectRoot, "stacklit.json"), []byte("{}\n"), 0644); err != nil {
		t.Fatalf("write stacklit index: %v", err)
	}

	output := runHook(t, hookPath, sessionStartPayload(t, projectRoot), 0)
	if output != "" {
		t.Fatalf("index context should be empty without liza-index hook, got:\n%s", output)
	}
}

func TestIndexContextHook_EmitsNothingWithoutIndexes(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	hookPath := writeIndexContextHook(t)
	projectRoot := t.TempDir()
	hooksDir := filepath.Join(projectRoot, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("create git hooks dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "post-commit"), []byte("#!/bin/sh\nliza-index\n"), 0755); err != nil {
		t.Fatalf("write post-commit hook: %v", err)
	}

	output := runHook(t, hookPath, sessionStartPayload(t, projectRoot), 0)
	if output != "" {
		t.Fatalf("index context should be empty without index files, got:\n%s", output)
	}
}

func TestIndexContextHook_SkipsLizaAgentSessions(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	hookPath := writeIndexContextHook(t)
	projectRoot := t.TempDir()
	writeIndexedRepoMarkers(t, projectRoot)

	cmd := exec.Command("bash", hookPath)
	cmd.Stdin = strings.NewReader(sessionStartPayload(t, projectRoot))
	cmd.Env = append(os.Environ(), "LIZA_AGENT_ID=coder-1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hook exited non-zero: %v\n%s", err, output)
	}
	if string(output) != "" {
		t.Fatalf("index context should be empty for Liza agent sessions, got:\n%s", string(output))
	}
}

func writeIndexContextHook(t *testing.T) string {
	t.Helper()
	hookPath := filepath.Join(t.TempDir(), "index-context.sh")
	if err := os.WriteFile(hookPath, indexContextHookContent, 0755); err != nil {
		t.Fatalf("write hook: %v", err)
	}
	return hookPath
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
