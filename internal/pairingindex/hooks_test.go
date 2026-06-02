package pairingindex

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/liza-mas/liza/internal/scipsearch"
)

func TestResolveEffectiveHooksDirDefault(t *testing.T) {
	repo := initGitRepo(t)

	got, err := ResolveEffectiveHooksDir(repo)
	if err != nil {
		t.Fatalf("ResolveEffectiveHooksDir() error = %v", err)
	}

	want := filepath.Join(repo, ".git", "hooks")
	if got != want {
		t.Fatalf("ResolveEffectiveHooksDir() = %q, want %q", got, want)
	}
}

func TestInstallLifecycleHooksDefaultHooks(t *testing.T) {
	repo := initGitRepo(t)

	result, err := InstallLifecycleHooks(InstallHooksOptions{RepoRoot: repo})
	if err != nil {
		t.Fatalf("InstallLifecycleHooks() error = %v", err)
	}

	wantHooksDir := filepath.Join(repo, ".git", "hooks")
	if result.HooksDir != wantHooksDir {
		t.Fatalf("HooksDir = %q, want %q", result.HooksDir, wantHooksDir)
	}
	assertHookActions(t, result, HookActionInstalled)

	for _, hook := range DefaultLifecycleHooks() {
		hookPath := filepath.Join(wantHooksDir, hook)
		info, err := os.Stat(hookPath)
		if err != nil {
			t.Fatalf("%s missing: %v", hookPath, err)
		}
		if info.Mode()&0111 == 0 {
			t.Fatalf("%s is not executable: mode=%v", hookPath, info.Mode())
		}
		content := readFile(t, hookPath)
		if !strings.Contains(content, ManagedHookMarker) {
			t.Fatalf("%s missing managed marker in:\n%s", hook, content)
		}
		if !strings.Contains(content, "liza-index.sh") {
			t.Fatalf("%s does not invoke liza-index.sh in:\n%s", hook, content)
		}
		if strings.Contains(content, "liza-index.sh ai") {
			t.Fatalf("%s lifecycle wrapper must not request AI summary in:\n%s", hook, content)
		}
	}
}

func TestInstallLifecycleHooksRespectsRelativeCoreHooksPath(t *testing.T) {
	repo := initGitRepo(t)
	runGit(t, repo, "config", "core.hooksPath", ".githooks")

	result, err := InstallLifecycleHooks(InstallHooksOptions{RepoRoot: repo})
	if err != nil {
		t.Fatalf("InstallLifecycleHooks() error = %v", err)
	}

	want := filepath.Join(repo, ".githooks")
	if result.HooksDir != want {
		t.Fatalf("HooksDir = %q, want %q", result.HooksDir, want)
	}
	for _, hook := range DefaultLifecycleHooks() {
		if _, err := os.Stat(filepath.Join(want, hook)); err != nil {
			t.Fatalf("relative hooksPath hook %s missing: %v", hook, err)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, ".git", "hooks", "post-commit")); err == nil {
		t.Fatal("post-commit was installed into inert default .git/hooks")
	} else if !os.IsNotExist(err) {
		t.Fatalf("inspect default hook path: %v", err)
	}
}

func TestInstallLifecycleHooksRespectsAbsoluteCoreHooksPath(t *testing.T) {
	repo := initGitRepo(t)
	hooksDir := filepath.Join(t.TempDir(), "hooks")
	runGit(t, repo, "config", "core.hooksPath", hooksDir)

	result, err := InstallLifecycleHooks(InstallHooksOptions{RepoRoot: repo})
	if err != nil {
		t.Fatalf("InstallLifecycleHooks() error = %v", err)
	}

	if result.HooksDir != hooksDir {
		t.Fatalf("HooksDir = %q, want %q", result.HooksDir, hooksDir)
	}
	for _, hook := range DefaultLifecycleHooks() {
		if _, err := os.Stat(filepath.Join(hooksDir, hook)); err != nil {
			t.Fatalf("absolute hooksPath hook %s missing: %v", hook, err)
		}
	}
}

func TestInstallLifecycleHooksIsIdempotentForManagedHooks(t *testing.T) {
	repo := initGitRepo(t)

	first, err := InstallLifecycleHooks(InstallHooksOptions{RepoRoot: repo})
	if err != nil {
		t.Fatalf("first InstallLifecycleHooks() error = %v", err)
	}
	second, err := InstallLifecycleHooks(InstallHooksOptions{RepoRoot: repo})
	if err != nil {
		t.Fatalf("second InstallLifecycleHooks() error = %v", err)
	}

	assertHookActions(t, first, HookActionInstalled)
	assertHookActions(t, second, HookActionVerified)
	for _, hook := range DefaultLifecycleHooks() {
		hookPath := filepath.Join(first.HooksDir, hook)
		if got := readFile(t, hookPath); got != managedHookContent(hook) {
			t.Fatalf("%s changed unexpectedly:\n%s", hook, got)
		}
	}
}

func TestInstallLifecycleHooksRefreshesStaleManagedHook(t *testing.T) {
	repo := initGitRepo(t)
	hooksDir := filepath.Join(repo, ".git", "hooks")
	hookPath := filepath.Join(hooksDir, "post-merge")
	staleContent := ManagedHookMarker + "\n# stale wrapper from an older Liza release\n"
	if err := os.WriteFile(hookPath, []byte(staleContent), 0644); err != nil {
		t.Fatalf("write stale managed hook: %v", err)
	}

	result, err := InstallLifecycleHooks(InstallHooksOptions{RepoRoot: repo})
	if err != nil {
		t.Fatalf("InstallLifecycleHooks() error = %v", err)
	}

	index := slices.IndexFunc(result.Hooks, func(got HookInstallResult) bool {
		return got.Hook == "post-merge"
	})
	if index == -1 {
		t.Fatalf("missing post-merge result: %#v", result.Hooks)
	}
	if result.Hooks[index].Action != HookActionUpdated {
		t.Fatalf("post-merge action = %q, want %q", result.Hooks[index].Action, HookActionUpdated)
	}
	if got := readFile(t, hookPath); got != managedHookContent("post-merge") {
		t.Fatalf("post-merge content was not refreshed:\n%s", got)
	}
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("stat refreshed hook: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Fatalf("refreshed hook is not executable: mode=%v", info.Mode())
	}
}

func TestManagedHookWrapperInvokesLocalIndexScriptWithoutAIArgument(t *testing.T) {
	repo := initGitRepo(t)
	result, err := InstallLifecycleHooks(InstallHooksOptions{
		RepoRoot: repo,
		Hooks:    []string{"post-rewrite"},
	})
	if err != nil {
		t.Fatalf("InstallLifecycleHooks() error = %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "args.log")
	scriptPath := filepath.Join(result.HooksDir, "liza-index.sh")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$LIZA_TEST_HOOK_LOG\"\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write liza-index.sh fixture: %v", err)
	}

	cmd := exec.Command(filepath.Join(result.HooksDir, "post-rewrite"), "rebase", "amend")
	cmd.Env = append(os.Environ(), "LIZA_TEST_HOOK_LOG="+logPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("post-rewrite hook failed: %v\n%s", err, output)
	}

	args := readFile(t, logPath)
	if args != "rebase\namend\n" {
		t.Fatalf("liza-index.sh args = %q, want lifecycle args without ai", args)
	}
	if strings.Contains(args, "ai") {
		t.Fatalf("automatic lifecycle wrapper must not request AI summary: %q", args)
	}
}

func TestRenderIndexScriptUsesStacklitGenerateJSONWithoutAutomaticAISummary(t *testing.T) {
	repo := initGitRepo(t)

	script, err := RenderIndexScript(repo)
	if err != nil {
		t.Fatalf("RenderIndexScript() error = %v", err)
	}

	if !strings.Contains(script, ManagedIndexScriptMarker) {
		t.Fatalf("script missing managed marker:\n%s", script)
	}
	if !strings.Contains(script, "stacklit generate-json -o stacklit.json") {
		t.Fatalf("script missing no-AI Stacklit generation command:\n%s", script)
	}
	if !strings.Contains(script, "stacklit ai-summary") {
		t.Fatalf("script missing manual AI-summary command:\n%s", script)
	}
	if strings.Contains(script, "stacklit generate-json -o stacklit.json --ai") {
		t.Fatalf("automatic Stacklit generation command includes AI flag:\n%s", script)
	}
}

func TestInstallActivationWritesScipCommandsWithoutStacklit(t *testing.T) {
	repo := initGitRepo(t)
	outputPath := filepath.Join(repo, "go.scip")

	result, err := InstallActivation(InstallActivationOptions{
		RepoRoot: repo,
		Hooks:    []string{"post-commit"},
		ScipPlans: []scipsearch.RuntimeCommandPlan{{
			Language:   "go",
			Name:       "scip-go",
			Args:       []string{"index", "--module-root", repo, "--output", outputPath},
			Dir:        repo,
			OutputPath: outputPath,
		}},
	})
	if err != nil {
		t.Fatalf("InstallActivation() error = %v", err)
	}

	script := readFile(t, result.Script.Path)
	if strings.Contains(script, "stacklit generate-json") {
		t.Fatalf("script contains Stacklit command despite Stacklit being disabled:\n%s", script)
	}
	for _, want := range []string{"scip-go index --module-root", repo, "--output", outputPath} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
	hook := readFile(t, filepath.Join(result.HooksDir, "post-commit"))
	if !strings.Contains(hook, ManagedHookMarker) || !strings.Contains(hook, "liza-index.sh") {
		t.Fatalf("post-commit hook missing managed wrapper:\n%s", hook)
	}
	if got := runGitOutput(t, repo, "check-ignore", "go.scip"); got != "go.scip" {
		t.Fatalf("git check-ignore go.scip = %q, want private exclude", got)
	}
}

func TestInstallIndexScriptWritesExecutableManagedScript(t *testing.T) {
	repo := initGitRepo(t)

	result, err := InstallIndexScript(InstallIndexScriptOptions{RepoRoot: repo})
	if err != nil {
		t.Fatalf("InstallIndexScript() error = %v", err)
	}

	wantPath := filepath.Join(repo, ".git", "hooks", "liza-index.sh")
	if result.Path != wantPath {
		t.Fatalf("script path = %q, want %q", result.Path, wantPath)
	}
	if result.Action != HookActionInstalled {
		t.Fatalf("script action = %q, want %q", result.Action, HookActionInstalled)
	}
	info, err := os.Stat(wantPath)
	if err != nil {
		t.Fatalf("installed script missing: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Fatalf("installed script is not executable: mode=%v", info.Mode())
	}
	if got := readFile(t, wantPath); !strings.Contains(got, ManagedIndexScriptMarker) {
		t.Fatalf("installed script missing managed marker:\n%s", got)
	}
}

func TestInstalledIndexScriptRefreshesStacklitJSONWithoutAIByDefault(t *testing.T) {
	repo := initGitRepo(t)
	result, err := InstallIndexScript(InstallIndexScriptOptions{RepoRoot: repo})
	if err != nil {
		t.Fatalf("InstallIndexScript() error = %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "stacklit.log")
	pathDir := writeFakeStacklit(t)

	cmd := exec.Command(result.Path)
	cmd.Env = append(os.Environ(),
		"PATH="+pathDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"LIZA_TEST_STACKLIT_LOG="+logPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("liza-index.sh failed: %v\n%s", err, output)
	}

	if got := readFile(t, logPath); got != "generate-json -o stacklit.json\n" {
		t.Fatalf("stacklit calls = %q, want no-AI generate-json only", got)
	}
	if got := readFile(t, filepath.Join(repo, "stacklit.json")); got != "generated index\n" {
		t.Fatalf("stacklit.json = %q, want generated index", got)
	}
	if got := runGitOutput(t, repo, "status", "--porcelain"); got != "" {
		t.Fatalf("git status --porcelain = %q, want clean generated Stacklit artifact", got)
	}
	if got := runGitOutput(t, repo, "check-ignore", "stacklit.json"); got != "stacklit.json" {
		t.Fatalf("git check-ignore stacklit.json = %q, want private exclude", got)
	}
}

func TestInstallIndexScriptAllowsTrackedStacklitJSON(t *testing.T) {
	repo := initGitRepo(t)
	writeFile(t, filepath.Join(repo, "stacklit.json"), "tracked index\n", 0644)
	commitPath(t, repo, "stacklit.json", "Add tracked Stacklit index")

	result, err := InstallIndexScript(InstallIndexScriptOptions{RepoRoot: repo})
	if err != nil {
		t.Fatalf("InstallIndexScript() error = %v", err)
	}
	runInstalledIndexScript(t, result.Path)

	status := runGitOutput(t, repo, "status", "--porcelain")
	if strings.Contains(status, "?? stacklit.json") {
		t.Fatalf("git status --porcelain = %q, want tracked Stacklit index, not accidental untracked file", status)
	}
	if got := readFile(t, filepath.Join(repo, "stacklit.json")); got != "generated index\n" {
		t.Fatalf("stacklit.json = %q, want generated index", got)
	}
}

func TestInstallIndexScriptAllowsIgnoredStacklitJSON(t *testing.T) {
	repo := initGitRepo(t)
	writeFile(t, filepath.Join(repo, ".gitignore"), "stacklit.json\n", 0644)
	commitPath(t, repo, ".gitignore", "Ignore Stacklit index")

	result, err := InstallIndexScript(InstallIndexScriptOptions{RepoRoot: repo})
	if err != nil {
		t.Fatalf("InstallIndexScript() error = %v", err)
	}
	runInstalledIndexScript(t, result.Path)

	if got := runGitOutput(t, repo, "status", "--porcelain"); got != "" {
		t.Fatalf("git status --porcelain = %q, want ignored generated Stacklit artifact", got)
	}
}

func TestInstallIndexScriptAllowsPrivatelyExcludedStacklitJSON(t *testing.T) {
	repo := initGitRepo(t)
	writeFile(t, filepath.Join(repo, ".git", "info", "exclude"), "stacklit.json\n", 0644)

	result, err := InstallIndexScript(InstallIndexScriptOptions{RepoRoot: repo})
	if err != nil {
		t.Fatalf("InstallIndexScript() error = %v", err)
	}
	runInstalledIndexScript(t, result.Path)

	if got := runGitOutput(t, repo, "status", "--porcelain"); got != "" {
		t.Fatalf("git status --porcelain = %q, want privately excluded generated Stacklit artifact", got)
	}
}

func TestInstallIndexScriptRejectsUnsafeUntrackedStacklitJSON(t *testing.T) {
	repo := initGitRepo(t)
	writeFile(t, filepath.Join(repo, "stacklit.json"), "unsafe untracked index\n", 0644)

	_, err := InstallIndexScript(InstallIndexScriptOptions{RepoRoot: repo})
	if err == nil {
		t.Fatal("InstallIndexScript() error = nil, want unsafe stacklit.json diagnostic")
	}
	if !strings.Contains(err.Error(), "stacklit.json is untracked") {
		t.Fatalf("error = %v, want unsafe untracked stacklit.json diagnostic", err)
	}
	if _, statErr := os.Stat(filepath.Join(repo, ".git", "hooks", "liza-index.sh")); statErr == nil {
		t.Fatal("liza-index.sh installed despite unsafe untracked stacklit.json")
	} else if !os.IsNotExist(statErr) {
		t.Fatalf("inspect liza-index.sh: %v", statErr)
	}
}

func TestInstalledIndexScriptManualAIArgumentRunsAISummary(t *testing.T) {
	repo := initGitRepo(t)
	result, err := InstallIndexScript(InstallIndexScriptOptions{RepoRoot: repo})
	if err != nil {
		t.Fatalf("InstallIndexScript() error = %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "stacklit.log")
	pathDir := writeFakeStacklit(t)

	cmd := exec.Command(result.Path, "ai")
	cmd.Env = append(os.Environ(),
		"PATH="+pathDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"LIZA_TEST_STACKLIT_LOG="+logPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("liza-index.sh ai failed: %v\n%s", err, output)
	}

	want := "generate-json -o stacklit.json\nai-summary\ngenerate-json -o stacklit.json\n"
	if got := readFile(t, logPath); got != want {
		t.Fatalf("stacklit calls = %q, want %q", got, want)
	}
}

func TestManagedLifecycleHookInvokesInstalledIndexScriptWithoutAI(t *testing.T) {
	repo := initGitRepo(t)
	hookResult, err := InstallLifecycleHooks(InstallHooksOptions{
		RepoRoot: repo,
		Hooks:    []string{"post-commit"},
	})
	if err != nil {
		t.Fatalf("InstallLifecycleHooks() error = %v", err)
	}
	if _, err := InstallIndexScript(InstallIndexScriptOptions{RepoRoot: repo}); err != nil {
		t.Fatalf("InstallIndexScript() error = %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "stacklit.log")
	pathDir := writeFakeStacklit(t)

	cmd := exec.Command(filepath.Join(hookResult.HooksDir, "post-commit"))
	cmd.Env = append(os.Environ(),
		"PATH="+pathDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"LIZA_TEST_STACKLIT_LOG="+logPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("post-commit hook failed: %v\n%s", err, output)
	}

	if got := readFile(t, logPath); got != "generate-json -o stacklit.json\n" {
		t.Fatalf("lifecycle stacklit calls = %q, want no-AI generate-json only", got)
	}
}

func TestInstallLifecycleHooksCreatesMissingHooksDirectory(t *testing.T) {
	repo := initGitRepo(t)
	hooksDir := filepath.Join(repo, ".githooks", "nested")
	runGit(t, repo, "config", "core.hooksPath", ".githooks/nested")

	result, err := InstallLifecycleHooks(InstallHooksOptions{RepoRoot: repo})
	if err != nil {
		t.Fatalf("InstallLifecycleHooks() error = %v", err)
	}

	if result.HooksDir != hooksDir {
		t.Fatalf("HooksDir = %q, want %q", result.HooksDir, hooksDir)
	}
	for _, hook := range DefaultLifecycleHooks() {
		if _, err := os.Stat(filepath.Join(hooksDir, hook)); err != nil {
			t.Fatalf("hook %s missing from created hooks directory: %v", hook, err)
		}
	}
}

func TestInstallLifecycleHooksRejectsUnsafeHooksPathFile(t *testing.T) {
	repo := initGitRepo(t)
	hooksFile := filepath.Join(repo, ".git", "hooks-file")
	if err := os.WriteFile(hooksFile, []byte("not a directory\n"), 0644); err != nil {
		t.Fatalf("write hooks file: %v", err)
	}
	runGit(t, repo, "config", "core.hooksPath", hooksFile)

	_, err := InstallLifecycleHooks(InstallHooksOptions{RepoRoot: repo})
	if err == nil {
		t.Fatal("InstallLifecycleHooks() error = nil, want unsafe hooks path error")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("error = %v, want not-a-directory diagnostic", err)
	}
}

func TestInstallLifecycleHooksReportsExistingHookCollision(t *testing.T) {
	repo := initGitRepo(t)
	hooksDir := filepath.Join(repo, ".git", "hooks")
	collidingHook := filepath.Join(hooksDir, "post-commit")
	if err := os.WriteFile(collidingHook, []byte("#!/bin/sh\necho user hook\n"), 0755); err != nil {
		t.Fatalf("write colliding hook: %v", err)
	}

	_, err := InstallLifecycleHooks(InstallHooksOptions{RepoRoot: repo})
	if err == nil {
		t.Fatal("InstallLifecycleHooks() error = nil, want collision")
	}
	var collision *HookCollisionError
	if !errors.As(err, &collision) {
		t.Fatalf("error type = %T, want *HookCollisionError: %v", err, err)
	}
	if len(collision.Collisions) != 1 {
		t.Fatalf("collision count = %d, want 1", len(collision.Collisions))
	}
	if collision.Collisions[0].Hook != "post-commit" || collision.Collisions[0].Path != collidingHook {
		t.Fatalf("collision = %#v, want post-commit at %s", collision.Collisions[0], collidingHook)
	}
	if !strings.Contains(err.Error(), "post-commit") || !strings.Contains(err.Error(), "not Liza-managed") {
		t.Fatalf("error = %v, want explicit hook collision diagnostic", err)
	}
	if _, err := os.Stat(filepath.Join(hooksDir, "post-checkout")); err == nil {
		t.Fatal("preflight should not install other hooks after detecting a collision")
	} else if !os.IsNotExist(err) {
		t.Fatalf("inspect post-checkout: %v", err)
	}
}

func assertHookActions(t *testing.T, result InstallHooksResult, action HookAction) {
	t.Helper()
	lifecycleHooks := DefaultLifecycleHooks()
	if len(result.Hooks) != len(lifecycleHooks) {
		t.Fatalf("hook results = %d, want %d", len(result.Hooks), len(lifecycleHooks))
	}
	for _, hook := range lifecycleHooks {
		index := slices.IndexFunc(result.Hooks, func(got HookInstallResult) bool {
			return got.Hook == hook
		})
		if index == -1 {
			t.Fatalf("missing result for hook %s: %#v", hook, result.Hooks)
		}
		if result.Hooks[index].Action != action {
			t.Fatalf("%s action = %q, want %q", hook, result.Hooks[index].Action, action)
		}
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	runGit(t, repo, "init")
	return repo
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s failed: %v\n%s", strings.Join(args, " "), dir, err, output)
	}
}

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s failed: %v\n%s", strings.Join(args, " "), dir, err, output)
	}
	return strings.TrimSpace(string(output))
}

func commitPath(t *testing.T, repo, path, message string) {
	t.Helper()

	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Liza Test")
	runGit(t, repo, "add", path)
	runGit(t, repo, "commit", "-m", message)
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return string(data)
}

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func runInstalledIndexScript(t *testing.T, scriptPath string) {
	t.Helper()

	logPath := filepath.Join(t.TempDir(), "stacklit.log")
	pathDir := writeFakeStacklit(t)
	cmd := exec.Command(scriptPath)
	cmd.Env = append(os.Environ(),
		"PATH="+pathDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"LIZA_TEST_STACKLIT_LOG="+logPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("liza-index.sh failed: %v\n%s", err, output)
	}
}

func writeFakeStacklit(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "stacklit")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$LIZA_TEST_STACKLIT_LOG"
if [ "$1" = "generate-json" ]; then
	printf '%s\n' "generated index" > "$PWD/stacklit.json"
fi
if [ "$1" = "ai-summary" ] && [ ! -f "$PWD/stacklit.json" ]; then
	echo "stacklit generate-json must run before ai-summary" >&2
	exit 7
fi
	`
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write fake stacklit: %v", err)
	}
	return dir
}
