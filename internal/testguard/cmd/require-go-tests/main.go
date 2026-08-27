package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type commandRunner func(stdout, stderr io.Writer, name string, args ...string) error

type testEvent struct {
	Action string
	Test   string
}

type eventCount struct {
	run  int
	pass int
}

func main() {
	os.Exit(runValidator(os.Args[1:], os.Stdout, os.Stderr, runCommand))
}

func runValidator(args []string, stdout, stderr io.Writer, runner commandRunner) int {
	flags := flag.NewFlagSet("require-go-tests", flag.ContinueOnError)
	flags.SetOutput(stderr)
	packageList := flags.String("packages", "", "comma-separated Go package list")
	testList := flags.String("tests", "", "comma-separated top-level Go test list")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected positional arguments: %s\n", strings.Join(flags.Args(), " "))
		return 2
	}

	packages, err := parseExplicitList("packages", *packageList)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	tests, err := parseExplicitList("tests", *testList)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err := requireTopLevelTests(tests); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	testPattern := exactTestPattern(tests)
	commandArgs := []string{"test", "-json", "-count=1", "-run", testPattern}
	commandArgs = append(commandArgs, packages...)

	var testOutput bytes.Buffer
	commandErr := runner(&testOutput, stderr, "go", commandArgs...)
	counts, decodeErr := countRequestedEvents(&testOutput, tests)

	failed := false
	if decodeErr != nil {
		fmt.Fprintf(stderr, "invalid go test -json output: %v\n", decodeErr)
		failed = true
	}
	if commandErr != nil {
		fmt.Fprintf(stderr, "go test failed: %v\n", commandErr)
		failed = true
	}
	for _, testName := range tests {
		count := counts[testName]
		if count.run != 1 || count.pass != 1 {
			fmt.Fprintf(
				stderr,
				"test %s: observed %d run and %d terminal pass events; want exactly 1 of each\n",
				testName,
				count.run,
				count.pass,
			)
			failed = true
		}
	}
	if failed {
		return 1
	}

	fmt.Fprintf(stdout, "verified %d named Go tests across %d packages\n", len(tests), len(packages))
	return 0
}

func parseExplicitList(name, value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("--%s requires a nonempty comma-separated list", name)
	}

	items := strings.Split(value, ",")
	seen := make(map[string]struct{}, len(items))
	for index, item := range items {
		items[index] = strings.TrimSpace(item)
		if items[index] == "" {
			return nil, fmt.Errorf("--%s contains an empty list item", name)
		}
		if _, exists := seen[items[index]]; exists {
			return nil, fmt.Errorf("--%s contains duplicate item %q", name, items[index])
		}
		seen[items[index]] = struct{}{}
	}
	return items, nil
}

func requireTopLevelTests(tests []string) error {
	for _, testName := range tests {
		if strings.Contains(testName, "/") {
			return fmt.Errorf("--tests item %q is a subtest; top-level test names are required", testName)
		}
	}
	return nil
}

func exactTestPattern(tests []string) string {
	quoted := make([]string, len(tests))
	for index, testName := range tests {
		quoted[index] = regexp.QuoteMeta(testName)
	}
	return "^(?:" + strings.Join(quoted, "|") + ")$"
}

func countRequestedEvents(reader io.Reader, tests []string) (map[string]eventCount, error) {
	counts := make(map[string]eventCount, len(tests))
	requested := make(map[string]struct{}, len(tests))
	for _, testName := range tests {
		requested[testName] = struct{}{}
	}

	decoder := json.NewDecoder(reader)
	for {
		var event testEvent
		if err := decoder.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				return counts, nil
			}
			return counts, err
		}
		if _, wanted := requested[event.Test]; !wanted {
			continue
		}

		count := counts[event.Test]
		switch event.Action {
		case "run":
			count.run++
		case "pass":
			count.pass++
		}
		counts[event.Test] = count
	}
}

func runCommand(stdout, stderr io.Writer, name string, args ...string) error {
	command := exec.Command(name, args...)
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}
