package process

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liza-mas/liza/internal/agent"
	"github.com/liza-mas/liza/internal/testhelpers"
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

func TestBuildSpawnCommand_AddsGoalIDFromState(t *testing.T) {
	projectRoot := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, projectRoot)
	state := testhelpers.CreateValidState()
	state.Goal.ID = "goal-xyz"
	testhelpers.WriteInitialState(t, statePath, state)

	cmd, devNull, err := buildSpawnCommand(projectRoot, "coder", "codex")
	if err != nil {
		t.Fatalf("buildSpawnCommand() error = %v", err)
	}
	defer devNull.Close()

	wantArgs := []string{"liza", "agent", "coder", "--cli", "codex", "--goal-id", "goal-xyz"}
	if strings.Join(cmd.Args, " ") != strings.Join(wantArgs, " ") {
		t.Fatalf("args = %v, want %v", cmd.Args, wantArgs)
	}
}

func TestBuildSpawnCommand_DoesNotOverrideExplicitGoalID(t *testing.T) {
	projectRoot := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, projectRoot)
	state := testhelpers.CreateValidState()
	state.Goal.ID = "goal-xyz"
	testhelpers.WriteInitialState(t, statePath, state)

	cmd, devNull, err := buildSpawnCommand(projectRoot, "coder", "codex", "--goal-id", "manual-goal")
	if err != nil {
		t.Fatalf("buildSpawnCommand() error = %v", err)
	}
	defer devNull.Close()

	wantArgs := []string{"liza", "agent", "coder", "--cli", "codex", "--goal-id", "manual-goal"}
	if strings.Join(cmd.Args, " ") != strings.Join(wantArgs, " ") {
		t.Fatalf("args = %v, want %v", cmd.Args, wantArgs)
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

func TestSpawnAgent_QuotaSignalBlocksSpawnAndAlerts(t *testing.T) {
	projectRoot := t.TempDir()
	lizaDir := filepath.Join(projectRoot, ".liza")
	if err := os.MkdirAll(lizaDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := agent.WriteQuotaSignal(projectRoot, "codex", "quota hit"); err != nil {
		t.Fatal(err)
	}

	cmd, err := SpawnAgent(projectRoot, "coder", "codex")
	if err == nil {
		t.Fatal("SpawnAgent error = nil, want quota refusal")
	}
	if cmd != nil {
		t.Fatalf("SpawnAgent command = %#v, want nil", cmd)
	}
	if !strings.Contains(err.Error(), "provider quota exhausted for codex") {
		t.Fatalf("SpawnAgent error = %q, want quota refusal", err)
	}

	alertsPath := filepath.Join(lizaDir, "alerts.log")
	data, readErr := os.ReadFile(alertsPath)
	if readErr != nil {
		t.Fatalf("failed to read alerts log: %v", readErr)
	}
	alerts := string(data)
	if !strings.Contains(alerts, "PROVIDER QUOTA SPAWN BLOCKED") {
		t.Fatalf("alerts log missing spawn-blocked alert:\n%s", alerts)
	}
	if !strings.Contains(alerts, "codex: refused to spawn coder while quota signal is set") {
		t.Fatalf("alerts log missing spawn-blocked details:\n%s", alerts)
	}
	if !strings.Contains(alerts, "delete the flag file or run liza pause then liza resume") {
		t.Fatalf("alerts log missing recovery hint:\n%s", alerts)
	}
}

func TestSpawnAgent_ProviderUnavailableSignalBlocksSpawnAndAlerts(t *testing.T) {
	projectRoot := t.TempDir()
	lizaDir := filepath.Join(projectRoot, ".liza")
	if err := os.MkdirAll(lizaDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := agent.WriteProviderUnavailableSignal(projectRoot, "codex", "session access denied"); err != nil {
		t.Fatal(err)
	}

	cmd, err := SpawnAgent(projectRoot, "coder", "codex")
	if err == nil {
		t.Fatal("SpawnAgent error = nil, want provider-unavailable refusal")
	}
	if cmd != nil {
		t.Fatalf("SpawnAgent command = %#v, want nil", cmd)
	}
	if !strings.Contains(err.Error(), "provider unavailable for codex") {
		t.Fatalf("SpawnAgent error = %q, want provider-unavailable refusal", err)
	}

	alertsPath := filepath.Join(lizaDir, "alerts.log")
	data, readErr := os.ReadFile(alertsPath)
	if readErr != nil {
		t.Fatalf("failed to read alerts log: %v", readErr)
	}
	alerts := string(data)
	if !strings.Contains(alerts, "PROVIDER UNAVAILABLE SPAWN BLOCKED") {
		t.Fatalf("alerts log missing spawn-blocked alert:\n%s", alerts)
	}
	if !strings.Contains(alerts, "codex: refused to spawn coder while provider-unavailable signal is set") {
		t.Fatalf("alerts log missing spawn-blocked details:\n%s", alerts)
	}
	if !strings.Contains(alerts, "repair the provider, then delete the flag file or run liza pause then liza resume") {
		t.Fatalf("alerts log missing recovery hint:\n%s", alerts)
	}
}
