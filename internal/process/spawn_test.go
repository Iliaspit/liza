package process

import (
	"os"
	"testing"
)

func TestBuildSpawnCommand_PassesExpectedArgs(t *testing.T) {
	cmd, devNull, err := buildSpawnCommand("/tmp/project", "coder", "codex", "--agent-id", "coder-9")
	if err != nil {
		t.Fatalf("buildSpawnCommand() error = %v", err)
	}
	defer devNull.Close()

	wantArgs := []string{"liza", "agent", "coder", "--cli", "codex", "--agent-id", "coder-9"}
	if len(cmd.Args) != len(wantArgs) {
		t.Fatalf("len(args) = %d, want %d (%v)", len(cmd.Args), len(wantArgs), cmd.Args)
	}
	for i, want := range wantArgs {
		if cmd.Args[i] != want {
			t.Fatalf("args[%d] = %q, want %q (all args: %v)", i, cmd.Args[i], want, cmd.Args)
		}
	}

	if cmd.Dir != "/tmp/project" {
		t.Fatalf("Dir = %q, want %q", cmd.Dir, "/tmp/project")
	}
}

func TestBuildSpawnCommand_BindsAllStdioToDevNull(t *testing.T) {
	cmd, devNull, err := buildSpawnCommand("/tmp/project", "coder", "codex")
	if err != nil {
		t.Fatalf("buildSpawnCommand() error = %v", err)
	}
	defer devNull.Close()

	stdinFile, ok := cmd.Stdin.(*os.File)
	if !ok {
		t.Fatalf("Stdin type = %T, want *os.File", cmd.Stdin)
	}
	stdoutFile, ok := cmd.Stdout.(*os.File)
	if !ok {
		t.Fatalf("Stdout type = %T, want *os.File", cmd.Stdout)
	}
	stderrFile, ok := cmd.Stderr.(*os.File)
	if !ok {
		t.Fatalf("Stderr type = %T, want *os.File", cmd.Stderr)
	}

	if stdinFile.Name() != os.DevNull {
		t.Fatalf("stdin file = %q, want %q", stdinFile.Name(), os.DevNull)
	}
	if stdoutFile.Name() != os.DevNull {
		t.Fatalf("stdout file = %q, want %q", stdoutFile.Name(), os.DevNull)
	}
	if stderrFile.Name() != os.DevNull {
		t.Fatalf("stderr file = %q, want %q", stderrFile.Name(), os.DevNull)
	}
}
