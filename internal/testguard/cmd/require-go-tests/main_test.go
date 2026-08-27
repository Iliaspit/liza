package main

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestRequireGoTestsRejectsZeroMatches(t *testing.T) {
	t.Parallel()

	t.Run("no matching events", func(t *testing.T) {
		var called bool
		exitCode, _, stderr := invokeValidator(t, "./first,./second", "TestAbsent", func(stdout, _ io.Writer, name string, args ...string) error {
			called = true
			assertGoTestCommand(t, name, args, "^(?:TestAbsent)$", "./first", "./second")
			fmt.Fprint(stdout, jsonEvent("start", "example/first", ""))
			fmt.Fprint(stdout, jsonEvent("pass", "example/first", ""))
			return nil
		})

		if !called {
			t.Fatal("go test runner was not called")
		}
		if exitCode == 0 {
			t.Fatal("validator accepted a zero-match test run")
		}
		if !strings.Contains(stderr, "TestAbsent") {
			t.Fatalf("stderr does not name the missing test: %q", stderr)
		}
	})

	t.Run("package failure", func(t *testing.T) {
		exitCode, _, stderr := invokeValidator(t, "./broken", "TestBroken", func(stdout, _ io.Writer, name string, args ...string) error {
			assertGoTestCommand(t, name, args, "^(?:TestBroken)$", "./broken")
			fmt.Fprint(stdout, jsonEvent("fail", "example/broken", ""))
			return errors.New("exit status 1")
		})

		if exitCode == 0 {
			t.Fatal("validator swallowed a package failure")
		}
		if !strings.Contains(stderr, "go test failed") {
			t.Fatalf("stderr does not report the package failure: %q", stderr)
		}
	})

	t.Run("explicit nonempty lists required", func(t *testing.T) {
		for _, testCase := range []struct {
			name     string
			packages string
			tests    string
		}{
			{name: "packages", tests: "TestNamed"},
			{name: "tests", packages: "./pkg"},
			{name: "package item", packages: "./pkg,", tests: "TestNamed"},
			{name: "test item", packages: "./pkg", tests: "TestNamed,"},
			{name: "subtest name", packages: "./pkg", tests: "TestNamed/child"},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				called := false
				exitCode, _, _ := invokeValidator(t, testCase.packages, testCase.tests, func(io.Writer, io.Writer, string, ...string) error {
					called = true
					return nil
				})
				if exitCode == 0 {
					t.Fatal("validator accepted an incomplete explicit list")
				}
				if called {
					t.Fatal("go test runner was called for invalid input")
				}
			})
		}
	})
}

func TestRequireGoTestsRejectsMissingNamedTest(t *testing.T) {
	t.Parallel()

	t.Run("partial match", func(t *testing.T) {
		exitCode, _, stderr := invokeValidator(t, "./pkg", "TestPresent,TestMissing", func(stdout, _ io.Writer, name string, args ...string) error {
			assertGoTestCommand(t, name, args, "^(?:TestPresent|TestMissing)$", "./pkg")
			fmt.Fprint(stdout, jsonEvent("run", "example/pkg", "TestPresent"))
			fmt.Fprint(stdout, jsonEvent("pass", "example/pkg", "TestPresent"))
			return nil
		})

		if exitCode == 0 {
			t.Fatal("validator accepted a partial match set")
		}
		if !strings.Contains(stderr, "TestMissing") {
			t.Fatalf("stderr does not name the missing test: %q", stderr)
		}
	})

	for _, testCase := range []struct {
		name       string
		actions    []string
		wantCounts string
	}{
		{name: "pass without run", actions: []string{"pass"}, wantCounts: "observed 0 run and 1 terminal pass"},
		{name: "run without terminal pass", actions: []string{"run", "fail"}, wantCounts: "observed 1 run and 0 terminal pass"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			exitCode, _, stderr := invokeValidator(t, "./pkg", "TestEvidencePair", func(stdout, _ io.Writer, name string, args ...string) error {
				assertGoTestCommand(t, name, args, "^(?:TestEvidencePair)$", "./pkg")
				for _, action := range testCase.actions {
					fmt.Fprint(stdout, jsonEvent(action, "example/pkg", "TestEvidencePair"))
				}
				return nil
			})

			if exitCode == 0 {
				t.Fatalf("validator accepted incomplete event evidence: %v", testCase.actions)
			}
			if !strings.Contains(stderr, testCase.wantCounts) {
				t.Fatalf("stderr does not report the incomplete event evidence: %q", stderr)
			}
		})
	}

	t.Run("test failure", func(t *testing.T) {
		exitCode, _, stderr := invokeValidator(t, "./pkg", "TestFails", func(stdout, _ io.Writer, name string, args ...string) error {
			assertGoTestCommand(t, name, args, "^(?:TestFails)$", "./pkg")
			fmt.Fprint(stdout, jsonEvent("run", "example/pkg", "TestFails"))
			fmt.Fprint(stdout, jsonEvent("fail", "example/pkg", "TestFails"))
			return errors.New("exit status 1")
		})

		if exitCode == 0 {
			t.Fatal("validator swallowed a test failure")
		}
		if !strings.Contains(stderr, "go test failed") || !strings.Contains(stderr, "TestFails") {
			t.Fatalf("stderr does not report the failed test: %q", stderr)
		}
	})
}

func TestRequireGoTestsAcceptsAllNamedPasses(t *testing.T) {
	t.Parallel()

	exitCode, _, stderr := invokeValidator(t, "./first, ./second", "TestAlpha, TestBeta", func(stdout, _ io.Writer, name string, args ...string) error {
		assertGoTestCommand(t, name, args, "^(?:TestAlpha|TestBeta)$", "./first", "./second")
		fmt.Fprint(stdout, jsonEvent("run", "example/first", "TestAlpha"))
		fmt.Fprint(stdout, jsonEvent("run", "example/first", "TestAlpha/child"))
		fmt.Fprint(stdout, jsonEvent("pass", "example/first", "TestAlpha/child"))
		fmt.Fprint(stdout, jsonEvent("pass", "example/first", "TestAlpha"))
		fmt.Fprint(stdout, jsonEvent("run", "example/second", "TestBeta"))
		fmt.Fprint(stdout, jsonEvent("pass", "example/second", "TestBeta"))
		return nil
	})

	if exitCode != 0 {
		t.Fatalf("validator rejected complete named passes with exit %d: %s", exitCode, stderr)
	}
}

func invokeValidator(
	t *testing.T,
	packages string,
	tests string,
	runner func(io.Writer, io.Writer, string, ...string) error,
) (int, string, string) {
	t.Helper()

	var stdout strings.Builder
	var stderr strings.Builder
	exitCode := runValidator(
		[]string{"--packages", packages, "--tests", tests},
		&stdout,
		&stderr,
		runner,
	)
	return exitCode, stdout.String(), stderr.String()
}

func assertGoTestCommand(t *testing.T, name string, args []string, testPattern string, packages ...string) {
	t.Helper()

	if name != "go" {
		t.Fatalf("command = %q, want go", name)
	}
	wantArgs := []string{"test", "-json", "-count=1", "-run", testPattern}
	wantArgs = append(wantArgs, packages...)
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("go args = %#v, want %#v", args, wantArgs)
	}
}

func jsonEvent(action, pkg, test string) string {
	if test == "" {
		return fmt.Sprintf("{\"Action\":%q,\"Package\":%q}\n", action, pkg)
	}
	return fmt.Sprintf("{\"Action\":%q,\"Package\":%q,\"Test\":%q}\n", action, pkg, test)
}
