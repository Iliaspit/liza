package procscan

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestFindZombieAgents_DetectsScopedUnregisteredSupervisor(t *testing.T) {
	projectRoot := t.TempDir()
	procRoot := t.TempDir()
	writeProc(t, procRoot, 1234, projectRoot, []string{"liza", "agent", "coder", "--cli", "codex", "--goal-id", "goal-1"})

	zombies, err := FindZombieAgents(ZombieScanOptions{
		ProjectRoot: projectRoot,
		GoalID:      "goal-1",
		ProcRoot:    procRoot,
	})
	if err != nil {
		t.Fatalf("FindZombieAgents() error = %v", err)
	}
	if len(zombies) != 1 {
		t.Fatalf("zombie count = %d, want 1: %+v", len(zombies), zombies)
	}
	got := zombies[0]
	if got.PID != 1234 || got.Role != "coder" || got.CLI != "codex" || got.GoalID != "goal-1" {
		t.Fatalf("zombie = %+v, want pid/role/cli/goal populated", got)
	}
	if got.Reason != "not_registered_in_state" {
		t.Fatalf("reason = %q, want not_registered_in_state", got.Reason)
	}
}

func TestFindZombieAgents_SkipsRegisteredPID(t *testing.T) {
	projectRoot := t.TempDir()
	procRoot := t.TempDir()
	writeProc(t, procRoot, 1234, projectRoot, []string{"liza", "agent", "coder", "--cli", "codex"})

	zombies, err := FindZombieAgents(ZombieScanOptions{
		ProjectRoot:    projectRoot,
		RegisteredPIDs: map[int]bool{1234: true},
		ProcRoot:       procRoot,
	})
	if err != nil {
		t.Fatalf("FindZombieAgents() error = %v", err)
	}
	if len(zombies) != 0 {
		t.Fatalf("zombie count = %d, want 0: %+v", len(zombies), zombies)
	}
}

func TestFindZombieAgents_SkipsOtherProjectWithoutGoalMatch(t *testing.T) {
	projectRoot := t.TempDir()
	otherRoot := t.TempDir()
	procRoot := t.TempDir()
	writeProc(t, procRoot, 1234, otherRoot, []string{"liza", "agent", "coder", "--cli", "codex", "--goal-id", "other-goal"})

	zombies, err := FindZombieAgents(ZombieScanOptions{
		ProjectRoot: projectRoot,
		GoalID:      "goal-1",
		ProcRoot:    procRoot,
	})
	if err != nil {
		t.Fatalf("FindZombieAgents() error = %v", err)
	}
	if len(zombies) != 0 {
		t.Fatalf("zombie count = %d, want 0: %+v", len(zombies), zombies)
	}
}

func TestFindZombieAgents_SkipsOtherProjectWithSameGoalWhenCWDReadable(t *testing.T) {
	projectRoot := t.TempDir()
	otherRoot := t.TempDir()
	procRoot := t.TempDir()
	writeProc(t, procRoot, 1234, otherRoot, []string{"liza", "agent", "coder", "--cli", "codex", "--goal-id", "goal-1"})

	zombies, err := FindZombieAgents(ZombieScanOptions{
		ProjectRoot: projectRoot,
		GoalID:      "goal-1",
		ProcRoot:    procRoot,
	})
	if err != nil {
		t.Fatalf("FindZombieAgents() error = %v", err)
	}
	if len(zombies) != 0 {
		t.Fatalf("zombie count = %d, want 0: %+v", len(zombies), zombies)
	}
}

func TestFindZombieAgents_SkipsGoalMatchWhenProjectRootSetAndCWDUnreadable(t *testing.T) {
	projectRoot := t.TempDir()
	procRoot := t.TempDir()
	writeProcWithoutCWD(t, procRoot, 1234, []string{"liza", "agent", "coder", "--cli", "codex", "--goal-id", "goal-1"})

	zombies, err := FindZombieAgents(ZombieScanOptions{
		ProjectRoot: projectRoot,
		GoalID:      "goal-1",
		ProcRoot:    procRoot,
	})
	if err != nil {
		t.Fatalf("FindZombieAgents() error = %v", err)
	}
	if len(zombies) != 0 {
		t.Fatalf("zombie count = %d, want 0: %+v", len(zombies), zombies)
	}
}

func TestFindZombieAgents_LegacyCWDMatchWithoutGoalID(t *testing.T) {
	projectRoot := t.TempDir()
	procRoot := t.TempDir()
	writeProc(t, procRoot, 1234, projectRoot, []string{"liza", "agent", "code-reviewer", "--cli=codex"})

	zombies, err := FindZombieAgents(ZombieScanOptions{
		ProjectRoot: projectRoot,
		GoalID:      "goal-1",
		ProcRoot:    procRoot,
	})
	if err != nil {
		t.Fatalf("FindZombieAgents() error = %v", err)
	}
	if len(zombies) != 1 {
		t.Fatalf("zombie count = %d, want 1: %+v", len(zombies), zombies)
	}
	if zombies[0].GoalID != "" || zombies[0].CLI != "codex" {
		t.Fatalf("zombie = %+v, want legacy goal empty and cli parsed", zombies[0])
	}
}

func TestFindZombieAgents_ProcfsUnavailable(t *testing.T) {
	_, err := FindZombieAgents(ZombieScanOptions{ProcRoot: filepath.Join(t.TempDir(), "missing")})
	if !errors.Is(err, ErrProcessScanUnavailable) {
		t.Fatalf("FindZombieAgents() error = %v, want ErrProcessScanUnavailable", err)
	}
}

func TestIsLizaAgentArgv(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want bool
	}{
		{name: "liza agent", argv: []string{"/usr/bin/liza", "agent", "coder"}, want: true},
		{name: "too short", argv: []string{"liza"}, want: false},
		{name: "other liza command", argv: []string{"liza", "status"}, want: false},
		{name: "provider cli", argv: []string{"codex", "exec"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsLizaAgentArgv(tt.argv); got != tt.want {
				t.Fatalf("IsLizaAgentArgv(%v) = %v, want %v", tt.argv, got, tt.want)
			}
		})
	}
}

func writeProc(t *testing.T, procRoot string, pid int, cwd string, argv []string) {
	t.Helper()
	procDir := writeProcCmdline(t, procRoot, pid, argv)
	if err := os.Symlink(cwd, filepath.Join(procDir, "cwd")); err != nil {
		t.Fatal(err)
	}
}

func writeProcWithoutCWD(t *testing.T, procRoot string, pid int, argv []string) {
	t.Helper()
	writeProcCmdline(t, procRoot, pid, argv)
}

func writeProcCmdline(t *testing.T, procRoot string, pid int, argv []string) string {
	t.Helper()
	procDir := filepath.Join(procRoot, strconv.Itoa(pid))
	if err := os.MkdirAll(procDir, 0755); err != nil {
		t.Fatal(err)
	}
	cmdline := ""
	for _, arg := range argv {
		cmdline += arg + "\x00"
	}
	if err := os.WriteFile(filepath.Join(procDir, "cmdline"), []byte(cmdline), 0644); err != nil {
		t.Fatal(err)
	}
	return procDir
}
