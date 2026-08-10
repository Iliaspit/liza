package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestCleanupCommandRegisteredWithYesFlag(t *testing.T) {
	resetRootCmdForTest(t)
	if cleanupCmd.Flags().Lookup("yes") == nil {
		t.Fatal("cleanup command missing --yes flag")
	}
	if found, _, err := rootCmd.Find([]string{"cleanup"}); err != nil || found != cleanupCmd {
		t.Fatalf("root cleanup command lookup = %v, %v", found, err)
	}
}

func TestCleanupCommandExplicitRootWorksWithoutRuntimeDirectory(t *testing.T) {
	resetRootCmdForTest(t)
	projectRoot := t.TempDir()
	testhelpers.SetupTestGitRepo(t, projectRoot)
	worktreesDir := filepath.Join(projectRoot, paths.WorktreesDirName)
	if err := os.Mkdir(worktreesDir, 0755); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"--project-root", projectRoot, "cleanup", "--yes"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("cleanup command error = %v\nstderr:\n%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "cleanup complete") {
		t.Fatalf("cleanup stdout = %q", stdout.String())
	}
	if _, err := os.Stat(worktreesDir); !os.IsNotExist(err) {
		t.Fatalf("worktrees directory still exists or cannot be checked: %v", err)
	}
}
