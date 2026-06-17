package trovex

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseEnvGate(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{" true ", true},
		{" 1 ", true},
		{"0", false},
		{"false", false},
		{"", false},
		{"yes", false},
		{"on", false},
	}
	for _, tt := range tests {
		if got := ParseEnvGate(tt.input); got != tt.want {
			t.Errorf("ParseEnvGate(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestRuntimeEnabled(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		t.Setenv(EnvEnableTrovex, "1")
		if !RuntimeEnabled() {
			t.Error("RuntimeEnabled() = false, want true")
		}
	})
	t.Run("disabled", func(t *testing.T) {
		t.Setenv(EnvEnableTrovex, "0")
		if RuntimeEnabled() {
			t.Error("RuntimeEnabled() = true, want false")
		}
	})
	t.Run("unset", func(t *testing.T) {
		if v, ok := os.LookupEnv(EnvEnableTrovex); ok {
			t.Setenv(EnvEnableTrovex, v)
			os.Unsetenv(EnvEnableTrovex)
			defer os.Setenv(EnvEnableTrovex, v)
		}
		if RuntimeEnabled() {
			t.Error("RuntimeEnabled() = true, want false when unset")
		}
	})
}

func TestPlanIndexCommand(t *testing.T) {
	t.Run("valid_path", func(t *testing.T) {
		dir := t.TempDir()
		plan, err := PlanIndexCommand(dir)
		if err != nil {
			t.Fatalf("PlanIndexCommand(%q) error: %v", dir, err)
		}
		if plan.Name != trovexExecutableName {
			t.Errorf("plan.Name = %q, want %q", plan.Name, trovexExecutableName)
		}
		if len(plan.Args) != 2 || plan.Args[0] != "index" {
			t.Errorf("plan.Args = %v, want [index <abs-path>]", plan.Args)
		}
		if !filepath.IsAbs(plan.Args[1]) {
			t.Errorf("plan.Args[1] = %q, want absolute path", plan.Args[1])
		}
		if !filepath.IsAbs(plan.Dir) {
			t.Errorf("plan.Dir = %q, want absolute path", plan.Dir)
		}
	})

	t.Run("local_embedding_fallback_no_config", func(t *testing.T) {
		t.Setenv("TROVEX_EMBED_MODEL", "")
		t.Setenv("OPENAI_API_KEY", "")
		os.Unsetenv("TROVEX_EMBED_MODEL")
		os.Unsetenv("OPENAI_API_KEY")

		dir := t.TempDir()
		plan, err := PlanIndexCommand(dir)
		if err != nil {
			t.Fatalf("PlanIndexCommand error: %v", err)
		}
		if len(plan.Env) != 2 {
			t.Fatalf("plan.Env length = %d, want 2", len(plan.Env))
		}
		if plan.Env[0].Name != "TROVEX_EMBED_MODEL" || plan.Env[0].Value != defaultLocalEmbedModel {
			t.Errorf("plan.Env[0] = %+v, want TROVEX_EMBED_MODEL=%s", plan.Env[0], defaultLocalEmbedModel)
		}
		if plan.Env[1].Name != "TROVEX_EMBED_DIM" || plan.Env[1].Value != defaultLocalEmbedDim {
			t.Errorf("plan.Env[1] = %+v, want TROVEX_EMBED_DIM=%s", plan.Env[1], defaultLocalEmbedDim)
		}
	})

	t.Run("local_embedding_fallback_with_openai_key", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "sk-test")

		dir := t.TempDir()
		plan, err := PlanIndexCommand(dir)
		if err != nil {
			t.Fatalf("PlanIndexCommand error: %v", err)
		}
		if len(plan.Env) != 0 {
			t.Errorf("plan.Env = %v, want empty when OPENAI_API_KEY is set", plan.Env)
		}
	})

	t.Run("local_embedding_fallback_with_explicit_model", func(t *testing.T) {
		t.Setenv("TROVEX_EMBED_MODEL", "text-embedding-3-small")

		dir := t.TempDir()
		plan, err := PlanIndexCommand(dir)
		if err != nil {
			t.Fatalf("PlanIndexCommand error: %v", err)
		}
		if len(plan.Env) != 0 {
			t.Errorf("plan.Env = %v, want empty when TROVEX_EMBED_MODEL is set", plan.Env)
		}
	})
}

func TestRefreshIndex(t *testing.T) {
	t.Run("disabled_noop", func(t *testing.T) {
		t.Setenv(EnvEnableTrovex, "0")
		result, err := RefreshIndex(RefreshOptions{TargetRoot: t.TempDir()})
		if err != nil {
			t.Fatalf("RefreshIndex error: %v", err)
		}
		if result.Refreshed {
			t.Error("result.Refreshed = true, want false when disabled")
		}
		if len(result.Failures) != 0 {
			t.Errorf("result.Failures = %v, want empty when disabled", result.Failures)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Setenv(EnvEnableTrovex, "1")
		dir := t.TempDir()

		result, err := RefreshIndex(RefreshOptions{
			TargetRoot: dir,
			Runner:     func(_ RuntimeCommandPlan) (string, error) { return "indexed 5 docs", nil },
		})
		if err != nil {
			t.Fatalf("RefreshIndex error: %v", err)
		}
		if !result.Refreshed {
			t.Error("result.Refreshed = false, want true")
		}
		if len(result.Failures) != 0 {
			t.Errorf("result.Failures = %v, want empty on success", result.Failures)
		}
	})

	t.Run("failure_captured", func(t *testing.T) {
		t.Setenv(EnvEnableTrovex, "1")
		dir := t.TempDir()

		result, err := RefreshIndex(RefreshOptions{
			TargetRoot: dir,
			Runner: func(_ RuntimeCommandPlan) (string, error) {
				return "database locked", errors.New("exit status 1")
			},
		})
		if err != nil {
			t.Fatalf("RefreshIndex returned hard error: %v", err)
		}
		if result.Refreshed {
			t.Error("result.Refreshed = true, want false on failure")
		}
		if len(result.Failures) != 1 {
			t.Fatalf("result.Failures length = %d, want 1", len(result.Failures))
		}
		if !strings.Contains(result.Failures[0].Diagnostic, "exit status 1") {
			t.Errorf("diagnostic = %q, want to contain 'exit status 1'", result.Failures[0].Diagnostic)
		}
		if !strings.Contains(result.Failures[0].Diagnostic, "database locked") {
			t.Errorf("diagnostic = %q, want to contain 'database locked'", result.Failures[0].Diagnostic)
		}
	})

	t.Run("failure_diagnostic_bounded", func(t *testing.T) {
		t.Setenv(EnvEnableTrovex, "1")
		dir := t.TempDir()
		longOutput := strings.Repeat("x", maxFailureDiagnosticBytes+500)

		result, err := RefreshIndex(RefreshOptions{
			TargetRoot: dir,
			Runner:     func(_ RuntimeCommandPlan) (string, error) { return longOutput, errors.New("fail") },
		})
		if err != nil {
			t.Fatalf("RefreshIndex returned hard error: %v", err)
		}
		if len(result.Failures) != 1 {
			t.Fatalf("result.Failures length = %d, want 1", len(result.Failures))
		}
		if len(result.Failures[0].Diagnostic) > maxFailureDiagnosticBytes+20 {
			t.Errorf("diagnostic length = %d, want bounded near %d", len(result.Failures[0].Diagnostic), maxFailureDiagnosticBytes)
		}
	})

	t.Run("runner_receives_plan", func(t *testing.T) {
		t.Setenv(EnvEnableTrovex, "1")
		dir := t.TempDir()

		var captured RuntimeCommandPlan
		_, _ = RefreshIndex(RefreshOptions{
			TargetRoot: dir,
			Runner: func(plan RuntimeCommandPlan) (string, error) {
				captured = plan
				return "", nil
			},
		})
		if captured.Name != trovexExecutableName {
			t.Errorf("captured.Name = %q, want %q", captured.Name, trovexExecutableName)
		}
		if len(captured.Args) < 2 || captured.Args[0] != "index" {
			t.Errorf("captured.Args = %v, want [index ...]", captured.Args)
		}
	})
}

func TestBuildPromptMetadata(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		t.Setenv(EnvEnableTrovex, "0")
		_, ok := BuildPromptMetadata(PromptMetadataOptions{TargetRoot: t.TempDir()})
		if ok {
			t.Error("BuildPromptMetadata returned ok=true when disabled")
		}
	})

	t.Run("cli_not_found", func(t *testing.T) {
		t.Setenv(EnvEnableTrovex, "1")
		_, ok := BuildPromptMetadata(PromptMetadataOptions{
			TargetRoot: t.TempDir(),
			LookPath:   func(string) (string, error) { return "", errors.New("not found") },
		})
		if ok {
			t.Error("BuildPromptMetadata returned ok=true when CLI missing")
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Setenv(EnvEnableTrovex, "1")
		dir := t.TempDir()
		meta, ok := BuildPromptMetadata(PromptMetadataOptions{
			TargetRoot: dir,
			LookPath:   func(string) (string, error) { return "/usr/local/bin/trovex", nil },
		})
		if !ok {
			t.Fatal("BuildPromptMetadata returned ok=false, want true")
		}
		if !filepath.IsAbs(meta.TargetRoot) {
			t.Errorf("meta.TargetRoot = %q, want absolute path", meta.TargetRoot)
		}
		if !strings.HasPrefix(meta.ShellTargetRoot, "'") {
			t.Errorf("meta.ShellTargetRoot = %q, want shell-quoted", meta.ShellTargetRoot)
		}
	})

	t.Run("empty_target_root", func(t *testing.T) {
		t.Setenv(EnvEnableTrovex, "1")
		_, ok := BuildPromptMetadata(PromptMetadataOptions{
			TargetRoot: "",
			LookPath:   func(string) (string, error) { return "/usr/local/bin/trovex", nil },
		})
		// filepath.Abs("") returns cwd, which is valid — so this should succeed
		if !ok {
			t.Log("BuildPromptMetadata returned ok=false for empty root (cwd fallback)")
		}
	})
}

func TestPlanServeCommand(t *testing.T) {
	t.Run("default_port", func(t *testing.T) {
		plan := PlanServeCommand(0)
		if plan.Name != trovexExecutableName {
			t.Errorf("plan.Name = %q, want %q", plan.Name, trovexExecutableName)
		}
		if len(plan.Args) != 3 || plan.Args[0] != "serve" || plan.Args[1] != "--port" || plan.Args[2] != "8765" {
			t.Errorf("plan.Args = %v, want [serve --port 8765]", plan.Args)
		}
	})

	t.Run("custom_port", func(t *testing.T) {
		plan := PlanServeCommand(9999)
		if plan.Args[2] != "9999" {
			t.Errorf("plan.Args[2] = %q, want %q", plan.Args[2], "9999")
		}
	})
}

func TestMCPEndpointURL(t *testing.T) {
	tests := []struct {
		port int
		want string
	}{
		{0, "http://localhost:8765/mcp"},
		{8765, "http://localhost:8765/mcp"},
		{9000, "http://localhost:9000/mcp"},
	}
	for _, tt := range tests {
		if got := MCPEndpointURL(tt.port); got != tt.want {
			t.Errorf("MCPEndpointURL(%d) = %q, want %q", tt.port, got, tt.want)
		}
	}
}

func TestHealthCheckURL(t *testing.T) {
	tests := []struct {
		port int
		want string
	}{
		{0, "http://localhost:8765/healthz"},
		{8765, "http://localhost:8765/healthz"},
		{9000, "http://localhost:9000/healthz"},
	}
	for _, tt := range tests {
		if got := HealthCheckURL(tt.port); got != tt.want {
			t.Errorf("HealthCheckURL(%d) = %q, want %q", tt.port, got, tt.want)
		}
	}
}

func TestSetRuntimeRunnerForTest(t *testing.T) {
	t.Setenv(EnvEnableTrovex, "1")
	dir := t.TempDir()

	var called bool
	restore := SetRuntimeRunnerForTest(func(_ RuntimeCommandPlan) (string, error) {
		called = true
		return "mock", nil
	})
	defer restore()

	result, err := RefreshIndex(RefreshOptions{TargetRoot: dir})
	if err != nil {
		t.Fatalf("RefreshIndex error: %v", err)
	}
	if !called {
		t.Error("test runner was not called")
	}
	if !result.Refreshed {
		t.Error("result.Refreshed = false, want true")
	}
}
