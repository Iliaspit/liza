package bashpolicycli

import (
	"errors"
	"strings"
	"testing"
)

type fakeCommandRunner struct {
	commands []Command
	output   CommandOutput
	err      error
}

func (f *fakeCommandRunner) Run(command Command) (CommandOutput, error) {
	f.commands = append(f.commands, command)
	return f.output, f.err
}

func TestInitHooksDisabledSkipsLookup(t *testing.T) {
	t.Setenv(EnvEnableBashPolicy, "")

	var lookups int
	got := InitHooks(InitHooksOptions{
		ProjectRoot: t.TempDir(),
		Provider:    ProviderClaude,
		LookPath: func(name string) (string, error) {
			lookups++
			return "/bin/" + name, nil
		},
		Runner: &fakeCommandRunner{},
	})

	if got.Status != StatusDisabled {
		t.Fatalf("status = %s, want disabled", got.Status)
	}
	if lookups != 0 {
		t.Fatalf("lookups = %d, want zero", lookups)
	}
}

func TestInitHooksMissingExecutable(t *testing.T) {
	t.Setenv(EnvEnableBashPolicy, " true ")

	got := InitHooks(InitHooksOptions{
		ProjectRoot: t.TempDir(),
		Provider:    ProviderClaude,
		LookPath: func(string) (string, error) {
			return "", errors.New("not found")
		},
		Runner: &fakeCommandRunner{},
	})

	if got.Status != StatusMissing {
		t.Fatalf("status = %s, want missing", got.Status)
	}
}

func TestInitHooksRunsProviderAwareInit(t *testing.T) {
	t.Setenv(EnvEnableBashPolicy, "1")
	stdin := strings.NewReader("input\n")
	projectRoot := t.TempDir()
	runner := &fakeCommandRunner{
		output: CommandOutput{Stdout: "installed\n", Stderr: "diagnostic\n"},
	}
	var stdout strings.Builder
	var stderr strings.Builder

	got := InitHooks(InitHooksOptions{
		ProjectRoot: projectRoot,
		Provider:    ProviderAll,
		Stdin:       stdin,
		Stdout:      &stdout,
		Stderr:      &stderr,
		LookPath: func(name string) (string, error) {
			return "/custom/bin/" + name, nil
		},
		Runner: runner,
	})

	if got.Status != StatusInstalled {
		t.Fatalf("status = %s, want installed", got.Status)
	}
	if len(runner.commands) != 1 {
		t.Fatalf("commands = %d, want 1", len(runner.commands))
	}
	command := runner.commands[0]
	if command.Path != "/custom/bin/bash-policy" {
		t.Fatalf("path = %q", command.Path)
	}
	wantArgs := strings.Join([]string{"init", "--provider", "all", "--policy-artifact-root", projectRoot}, "\x00")
	if strings.Join(command.Args, "\x00") != wantArgs {
		t.Fatalf("args = %v", command.Args)
	}
	if command.Dir != projectRoot {
		t.Fatalf("dir = %q, want %q", command.Dir, projectRoot)
	}
	if command.Stdin != stdin {
		t.Fatalf("stdin was not preserved")
	}
	if stdout.String() != "installed\n" || stderr.String() != "diagnostic\n" {
		t.Fatalf("stdout/stderr = %q/%q", stdout.String(), stderr.String())
	}
}

func TestInitHooksFailureDiagnosticIncludesCapturedOutput(t *testing.T) {
	t.Setenv(EnvEnableBashPolicy, "true")
	runner := &fakeCommandRunner{
		output: CommandOutput{Stdout: "partial stdout", Stderr: "policy failed"},
		err:    errors.New("exit status 2"),
	}

	got := InitHooks(InitHooksOptions{
		ProjectRoot: t.TempDir(),
		Provider:    ProviderCodex,
		LookPath: func(name string) (string, error) {
			return "/bin/" + name, nil
		},
		Runner: runner,
	})

	if got.Status != StatusFailed {
		t.Fatalf("status = %s, want failed", got.Status)
	}
	diagnostic := got.Diagnostic()
	for _, want := range []string{"exit status 2", "policy failed", "partial stdout"} {
		if !strings.Contains(diagnostic, want) {
			t.Fatalf("diagnostic %q missing %q", diagnostic, want)
		}
	}
}
