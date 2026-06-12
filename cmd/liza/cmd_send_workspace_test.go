package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSendWorkspaceForwardsArgsToDelegate(t *testing.T) {
	projectRoot := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "args.log")
	binDir := t.TempDir()

	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + shellEscape(logPath) + "\n"
	bin := filepath.Join(binDir, "liza-send-workspace")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake liza-send-workspace: %v", err)
	}

	t.Setenv("PATH", binDir)
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	resetRootCmdForTest(t)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{
		"send-workspace",
		"--slack-channel",
		"C123",
		"--foo",
		"positional",
	})
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("execute send-workspace failed: %v", err)
	}

	args, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read args log: %v", err)
	}
	got := strings.Fields(strings.TrimSpace(string(args)))
	want := []string{
		"--slack-channel",
		"C123",
		"--foo",
		"positional",
	}
	if len(got) != len(want) {
		t.Fatalf("forwarded args = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSendWorkspaceMissingBinaryGivesGuidance(t *testing.T) {
	projectRoot := t.TempDir()

	t.Setenv("PATH", t.TempDir())
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	resetRootCmdForTest(t)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"send-workspace"})
	err = rootCmd.Execute()
	if err == nil {
		t.Fatal("expected missing-binary error, got nil")
	}
	if !strings.Contains(err.Error(), "missing dependency: liza-send-workspace is not installed") {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(err.Error(), "go install github.com/liza-mas/liza-report/cmd/liza-send-workspace@latest") {
		t.Fatalf("error missing install guidance: %v", err)
	}
}

func shellEscape(path string) string {
	if strings.ContainsAny(path, " '\"\\") {
		return "'" + strings.ReplaceAll(path, "'", "'\"'\"'") + "'"
	}
	return path
}
