package trovex

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

const EnvEnableTrovex = "LIZA_ENABLE_TROVEX"

const DefaultServePort = 8765

const (
	trovexExecutableName   = "trovex"
	defaultLocalEmbedModel = "BAAI/bge-small-en-v1.5"
	defaultLocalEmbedDim   = "384"
)

const maxFailureDiagnosticBytes = 1024

// ExecutableLookup resolves an executable name to the path that would be run.
type ExecutableLookup func(name string) (string, error)

// RuntimeCommandPlan describes a fixed Trovex command without executing it.
type RuntimeCommandPlan struct {
	Name string
	Args []string
	Dir  string
	Env  []EnvVar
}

// EnvVar is one environment override for a Trovex subprocess.
type EnvVar struct {
	Name  string
	Value string
}

// RuntimeRunner executes one Trovex runtime command plan.
type RuntimeRunner func(RuntimeCommandPlan) (string, error)

// RefreshOptions configures one best-effort runtime Trovex index refresh.
type RefreshOptions struct {
	TargetRoot string
	Runner     RuntimeRunner
}

// RefreshResult contains the refresh outcome and isolated failure diagnostics.
type RefreshResult struct {
	Refreshed bool
	Failures  []RefreshFailure
}

// RefreshFailure contains bounded diagnostics for a failed Trovex indexing.
type RefreshFailure struct {
	Diagnostic string
}

// PromptMetadataOptions configures prompt-safe Trovex metadata construction.
type PromptMetadataOptions struct {
	TargetRoot string
	LookPath   ExecutableLookup
}

// PromptMetadata is safe for prompt rendering; it intentionally excludes
// diagnostics, command output, and executable paths.
type PromptMetadata struct {
	TargetRoot      string
	ShellTargetRoot string
}

var (
	runnerMu      sync.Mutex
	defaultRunner RuntimeRunner = runRuntimeCommandPlan
)

// SetRuntimeRunnerForTest replaces the process runner until the returned
// restore function is called.
func SetRuntimeRunnerForTest(runner RuntimeRunner) func() {
	runnerMu.Lock()
	previous := defaultRunner
	defaultRunner = runner
	runnerMu.Unlock()

	return func() {
		runnerMu.Lock()
		defaultRunner = previous
		runnerMu.Unlock()
	}
}

// ParseEnvGate reports whether a supplied Trovex activation value is enabled.
func ParseEnvGate(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true":
		return true
	default:
		return false
	}
}

// RuntimeEnabled reports whether Trovex behavior is active for this process.
func RuntimeEnabled() bool {
	return ParseEnvGate(os.Getenv(EnvEnableTrovex))
}

// PlanIndexCommand returns the fixed Trovex index command for a target root
// without executing it. When the operator has not configured an explicit
// embedding model or OpenAI API key, local fastembed environment overrides are
// included so indexing works offline.
func PlanIndexCommand(targetRoot string) (RuntimeCommandPlan, error) {
	targetRoot, err := filepath.Abs(targetRoot)
	if err != nil {
		return RuntimeCommandPlan{}, fmt.Errorf("resolve trovex target root: %w", err)
	}
	return RuntimeCommandPlan{
		Name: trovexExecutableName,
		Args: []string{"index", targetRoot},
		Dir:  targetRoot,
		Env:  localEmbeddingFallbackEnv(),
	}, nil
}

// RefreshIndex runs trovex index for one target root. When disabled it returns
// an empty result. On execution failure it captures bounded diagnostics without
// propagating the error.
func RefreshIndex(opts RefreshOptions) (RefreshResult, error) {
	if !RuntimeEnabled() {
		return RefreshResult{}, nil
	}

	plan, err := PlanIndexCommand(opts.TargetRoot)
	if err != nil {
		return RefreshResult{}, err
	}

	runner := opts.Runner
	if runner == nil {
		runner = getDefaultRunner()
	}

	output, err := runner(plan)
	if err != nil {
		return RefreshResult{
			Failures: []RefreshFailure{{Diagnostic: boundedFailureDiagnostic(err, output)}},
		}, nil
	}
	return RefreshResult{Refreshed: true}, nil
}

// BuildPromptMetadata returns prompt-safe Trovex context when activation and
// CLI availability both pass.
func BuildPromptMetadata(opts PromptMetadataOptions) (PromptMetadata, bool) {
	if !RuntimeEnabled() {
		return PromptMetadata{}, false
	}

	targetRoot, err := filepath.Abs(opts.TargetRoot)
	if err != nil {
		return PromptMetadata{}, false
	}

	lookup := opts.LookPath
	if lookup == nil {
		lookup = exec.LookPath
	}
	if _, err := lookup(trovexExecutableName); err != nil {
		return PromptMetadata{}, false
	}

	return PromptMetadata{
		TargetRoot:      targetRoot,
		ShellTargetRoot: shellQuote(targetRoot),
	}, true
}

// PlanServeCommand returns the fixed Trovex MCP server command for the given
// port without executing it.
func PlanServeCommand(port int) RuntimeCommandPlan {
	if port <= 0 {
		port = DefaultServePort
	}
	return RuntimeCommandPlan{
		Name: trovexExecutableName,
		Args: []string{"serve", "--port", fmt.Sprintf("%d", port)},
	}
}

// MCPEndpointURL returns the MCP endpoint URL for a locally running Trovex
// server at the given port.
func MCPEndpointURL(port int) string {
	if port <= 0 {
		port = DefaultServePort
	}
	return fmt.Sprintf("http://localhost:%d/mcp", port)
}

// HealthCheckURL returns the health check URL for a locally running Trovex
// server at the given port.
func HealthCheckURL(port int) string {
	if port <= 0 {
		port = DefaultServePort
	}
	return fmt.Sprintf("http://localhost:%d/healthz", port)
}

func localEmbeddingFallbackEnv() []EnvVar {
	if os.Getenv("TROVEX_EMBED_MODEL") != "" || os.Getenv("OPENAI_API_KEY") != "" {
		return nil
	}
	return []EnvVar{
		{Name: "TROVEX_EMBED_MODEL", Value: defaultLocalEmbedModel},
		{Name: "TROVEX_EMBED_DIM", Value: defaultLocalEmbedDim},
	}
}

func getDefaultRunner() RuntimeRunner {
	runnerMu.Lock()
	defer runnerMu.Unlock()
	return defaultRunner
}

func runRuntimeCommandPlan(plan RuntimeCommandPlan) (string, error) {
	cmd := exec.Command(plan.Name, plan.Args...)
	cmd.Dir = plan.Dir
	if len(plan.Env) > 0 {
		cmd.Env = append(os.Environ(), envVars(plan.Env)...)
	}
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func envVars(vars []EnvVar) []string {
	values := make([]string, 0, len(vars))
	for _, env := range vars {
		values = append(values, env.Name+"="+env.Value)
	}
	return values
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func boundedFailureDiagnostic(err error, output string) string {
	diagnostic := strings.TrimSpace(err.Error())
	output = strings.TrimSpace(output)
	if output != "" {
		diagnostic += ": " + output
	}
	if len(diagnostic) <= maxFailureDiagnosticBytes {
		return diagnostic
	}
	return diagnostic[:maxFailureDiagnosticBytes] + "...(truncated)"
}
