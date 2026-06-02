package pairingindex

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/liza-mas/liza/internal/gitenv"
	"github.com/liza-mas/liza/internal/scipsearch"
	"github.com/liza-mas/liza/internal/stacklit"
)

// ManagedHookMarker identifies hook files owned by Liza's pairing index plumbing.
const ManagedHookMarker = "# LIZA-PAIRING-INDEX-HOOK: managed"

// ManagedIndexScriptMarker identifies liza-index.sh files owned by Liza.
const ManagedIndexScriptMarker = "# LIZA-PAIRING-INDEX-SCRIPT: managed"

const defaultScriptName = "liza-index.sh"
const stacklitArtifactName = "stacklit.json"

var defaultLifecycleHooks = []string{"post-commit", "post-checkout", "post-merge", "post-rewrite"}
var stacklitArtifactExcludeMu sync.Mutex

// DefaultLifecycleHooks returns the Git lifecycle hooks used for pairing index refresh.
func DefaultLifecycleHooks() []string {
	return append([]string(nil), defaultLifecycleHooks...)
}

// InstallHooksOptions configures lifecycle hook installation for one repository.
type InstallHooksOptions struct {
	RepoRoot string
	Hooks    []string
}

// InstallActivationOptions configures the combined pairing index activation hook
// setup for Stacklit and SCIP project-root refresh.
type InstallActivationOptions struct {
	RepoRoot       string
	Hooks          []string
	EnableStacklit bool
	ScipPlans      []scipsearch.RuntimeCommandPlan
}

// InstallActivationResult reports the installed script and lifecycle hooks.
type InstallActivationResult struct {
	HooksDir string
	Script   InstallIndexScriptResult
	Hooks    []HookInstallResult
}

// InstallHooksResult reports the effective hook directory and per-hook actions.
type InstallHooksResult struct {
	HooksDir string
	Hooks    []HookInstallResult
}

// HookInstallResult reports one installed or verified lifecycle hook wrapper.
type HookInstallResult struct {
	Hook   string
	Path   string
	Action HookAction
}

// HookAction describes what InstallLifecycleHooks did to one hook file.
type HookAction string

const (
	// HookActionInstalled means Liza wrote a missing managed hook wrapper.
	HookActionInstalled HookAction = "installed"
	// HookActionVerified means an existing wrapper already matched Liza's content.
	HookActionVerified HookAction = "verified"
	// HookActionUpdated means an existing Liza-managed wrapper was refreshed.
	HookActionUpdated HookAction = "updated"
)

// HookCollision identifies an existing non-Liza hook that must not be overwritten.
type HookCollision struct {
	Hook string
	Path string
}

// InstallIndexScriptOptions configures liza-index.sh installation for one repository.
type InstallIndexScriptOptions struct {
	RepoRoot        string
	DisableStacklit bool
	ScipPlans       []scipsearch.RuntimeCommandPlan
}

// InstallIndexScriptResult reports the generated script location and action.
type InstallIndexScriptResult struct {
	Path   string
	Action HookAction
}

// HookCollisionError reports all non-Liza lifecycle hook collisions found during preflight.
type HookCollisionError struct {
	Collisions []HookCollision
}

func (e *HookCollisionError) Error() string {
	if e == nil || len(e.Collisions) == 0 {
		return "Liza-managed pairing index hook collision"
	}

	parts := make([]string, 0, len(e.Collisions))
	for _, collision := range e.Collisions {
		parts = append(parts, fmt.Sprintf("%s at %s already exists and is not Liza-managed", collision.Hook, collision.Path))
	}
	return "Liza-managed pairing index hook collision: " + strings.Join(parts, "; ")
}

// InstallActivation installs or verifies the managed liza-index.sh entrypoint
// and lifecycle hooks after preflighting collisions.
func InstallActivation(opts InstallActivationOptions) (InstallActivationResult, error) {
	hooks := opts.Hooks
	if len(hooks) == 0 {
		hooks = DefaultLifecycleHooks()
	}
	hooksDir, err := ResolveEffectiveHooksDir(opts.RepoRoot)
	if err != nil {
		return InstallActivationResult{}, err
	}
	result := InstallActivationResult{HooksDir: hooksDir}
	if err := ensureHooksDir(hooksDir); err != nil {
		return result, err
	}
	if err := rejectHookCollisions(hooksDir, hooks); err != nil {
		return result, err
	}
	if opts.EnableStacklit {
		if err := ensureStacklitArtifactCleanliness(opts.RepoRoot); err != nil {
			return result, err
		}
	}
	if err := ensureScipArtifactCleanliness(opts.RepoRoot, opts.ScipPlans); err != nil {
		return result, err
	}

	scriptPath := filepath.Join(hooksDir, defaultScriptName)
	content, err := renderIndexScript(renderIndexScriptOptions{
		RepoRoot:       opts.RepoRoot,
		EnableStacklit: opts.EnableStacklit,
		ScipPlans:      opts.ScipPlans,
	})
	if err != nil {
		return result, err
	}
	action, err := installManagedIndexScript(scriptPath, content)
	if err != nil {
		return result, err
	}
	result.Script = InstallIndexScriptResult{Path: scriptPath, Action: action}

	for _, hook := range hooks {
		hookPath := filepath.Join(hooksDir, hook)
		action, err := installManagedHook(hookPath, hook)
		if err != nil {
			return result, err
		}
		result.Hooks = append(result.Hooks, HookInstallResult{
			Hook:   hook,
			Path:   hookPath,
			Action: action,
		})
	}
	return result, nil
}

// ResolveEffectiveHooksDir returns the hooks directory Git will use for repoRoot.
func ResolveEffectiveHooksDir(repoRoot string) (string, error) {
	repoRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}

	output, err := gitenv.Output(repoRoot, "rev-parse", "--git-path", "hooks/post-commit")
	if err != nil {
		return "", fmt.Errorf("resolve effective Git hook path: %w%s", err, outputSuffix(string(output)))
	}

	hookPath := strings.TrimSpace(string(output))
	if hookPath == "" {
		return "", fmt.Errorf("resolve effective Git hook path: git returned an empty path")
	}
	if !filepath.IsAbs(hookPath) {
		hookPath = filepath.Join(repoRoot, hookPath)
	}
	return filepath.Clean(filepath.Dir(hookPath)), nil
}

// InstallLifecycleHooks installs or verifies Liza-managed pairing index wrappers.
func InstallLifecycleHooks(opts InstallHooksOptions) (InstallHooksResult, error) {
	hooks := opts.Hooks
	if len(hooks) == 0 {
		hooks = DefaultLifecycleHooks()
	}

	hooksDir, err := ResolveEffectiveHooksDir(opts.RepoRoot)
	if err != nil {
		return InstallHooksResult{}, err
	}
	result := InstallHooksResult{HooksDir: hooksDir}

	if err := ensureHooksDir(hooksDir); err != nil {
		return result, err
	}
	if err := rejectHookCollisions(hooksDir, hooks); err != nil {
		return result, err
	}

	for _, hook := range hooks {
		hookPath := filepath.Join(hooksDir, hook)
		action, err := installManagedHook(hookPath, hook)
		if err != nil {
			return result, err
		}
		result.Hooks = append(result.Hooks, HookInstallResult{
			Hook:   hook,
			Path:   hookPath,
			Action: action,
		})
	}
	return result, nil
}

// RenderIndexScript returns the managed Stacklit liza-index.sh script content for repoRoot.
func RenderIndexScript(repoRoot string) (string, error) {
	return renderIndexScript(renderIndexScriptOptions{RepoRoot: repoRoot, EnableStacklit: true})
}

type renderIndexScriptOptions struct {
	RepoRoot       string
	EnableStacklit bool
	ScipPlans      []scipsearch.RuntimeCommandPlan
}

func renderIndexScript(opts renderIndexScriptOptions) (string, error) {
	var stacklitPlan stacklit.RuntimeCommandPlan
	if opts.EnableStacklit {
		plan, err := stacklit.PlanRuntimeCommand(opts.RepoRoot)
		if err != nil {
			return "", err
		}
		stacklitPlan = plan
	} else {
		repoRoot, err := filepath.Abs(opts.RepoRoot)
		if err != nil {
			return "", fmt.Errorf("resolve repository root: %w", err)
		}
		stacklitPlan.Dir = repoRoot
	}
	body := strings.Builder{}
	body.WriteString("#!/bin/sh\n")
	body.WriteString(ManagedIndexScriptMarker)
	body.WriteString("\nset -eu\n")
	body.WriteString("repo_root=")
	body.WriteString(shellQuote(stacklitPlan.Dir))
	body.WriteString("\n")
	body.WriteString(`run_ai=0
if [ "${1:-}" = "ai" ]; then
	run_ai=1
	shift
fi
`)
	if opts.EnableStacklit {
		stacklitGenerateCommand := shellCommand(stacklitPlan.Name, stacklitPlan.Args)
		body.WriteString(fmt.Sprintf(`if ! command -v %s >/dev/null 2>&1; then
	echo "liza-index.sh: %s not found; skipping Stacklit refresh" >&2
else
	cd "$repo_root"
	if [ "$run_ai" = "1" ]; then
		%s
		%s ai-summary
	fi
	%s
fi
`, shellWord(stacklitPlan.Name), stacklitPlan.Name, stacklitGenerateCommand, shellWord(stacklitPlan.Name), stacklitGenerateCommand))
	}
	for _, plan := range opts.ScipPlans {
		body.WriteString(renderScipCommand(plan))
	}
	return body.String(), nil
}

// InstallIndexScript installs or verifies the managed liza-index.sh entrypoint.
func InstallIndexScript(opts InstallIndexScriptOptions) (InstallIndexScriptResult, error) {
	if !opts.DisableStacklit {
		if err := ensureStacklitArtifactCleanliness(opts.RepoRoot); err != nil {
			return InstallIndexScriptResult{}, err
		}
	}
	if err := ensureScipArtifactCleanliness(opts.RepoRoot, opts.ScipPlans); err != nil {
		return InstallIndexScriptResult{}, err
	}

	hooksDir, err := ResolveEffectiveHooksDir(opts.RepoRoot)
	if err != nil {
		return InstallIndexScriptResult{}, err
	}
	if err := ensureHooksDir(hooksDir); err != nil {
		return InstallIndexScriptResult{}, err
	}

	scriptPath := filepath.Join(hooksDir, defaultScriptName)
	content, err := renderIndexScript(renderIndexScriptOptions{
		RepoRoot:       opts.RepoRoot,
		EnableStacklit: !opts.DisableStacklit,
		ScipPlans:      opts.ScipPlans,
	})
	if err != nil {
		return InstallIndexScriptResult{}, err
	}
	action, err := installManagedIndexScript(scriptPath, content)
	if err != nil {
		return InstallIndexScriptResult{Path: scriptPath}, err
	}
	return InstallIndexScriptResult{Path: scriptPath, Action: action}, nil
}

func renderScipCommand(plan scipsearch.RuntimeCommandPlan) string {
	command := shellCommand(plan.Name, plan.Args)
	return fmt.Sprintf(`if ! command -v %s >/dev/null 2>&1; then
	echo "liza-index.sh: %s not found; skipping %s SCIP refresh" >&2
else
	cd %s
	%s
fi
`, shellWord(plan.Name), plan.Name, plan.Language, shellQuote(plan.Dir), command)
}

func shellCommand(name string, args []string) string {
	parts := []string{shellWord(name)}
	for _, arg := range args {
		parts = append(parts, shellWord(arg))
	}
	return strings.Join(parts, " ")
}

func shellWord(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\n'\"\\$`;&|<>*?!()[]{}") {
		return value
	}
	return shellQuote(value)
}

func ensureStacklitArtifactCleanliness(repoRoot string) error {
	repoRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return fmt.Errorf("resolve repository root: %w", err)
	}

	tracked, err := stacklitArtifactTracked(repoRoot)
	if err != nil {
		return err
	}
	if tracked {
		return nil
	}

	ignored, err := stacklitArtifactIgnored(repoRoot)
	if err != nil {
		return err
	}
	if ignored {
		return nil
	}

	if _, err := os.Stat(filepath.Join(repoRoot, stacklitArtifactName)); err == nil {
		return fmt.Errorf("%s is untracked and not ignored or privately excluded; commit it, add it to .gitignore, or add it to .git/info/exclude before enabling pairing Stacklit activation", stacklitArtifactName)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect repo-root %s: %w", stacklitArtifactName, err)
	}

	return ensureRepoPrivateExclude(repoRoot, stacklitArtifactName)
}

func ensureScipArtifactCleanliness(repoRoot string, plans []scipsearch.RuntimeCommandPlan) error {
	for _, plan := range plans {
		rel, err := filepath.Rel(repoRoot, plan.OutputPath)
		if err != nil || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
			return fmt.Errorf("scip-search output path %q is outside repository root %q", plan.OutputPath, repoRoot)
		}
		if err := ensureGeneratedArtifactCleanliness(repoRoot, filepath.ToSlash(rel)); err != nil {
			return err
		}
	}
	return nil
}

func ensureGeneratedArtifactCleanliness(repoRoot, artifact string) error {
	tracked, err := artifactTracked(repoRoot, artifact)
	if err != nil {
		return err
	}
	if tracked {
		return nil
	}
	ignored, err := artifactIgnored(repoRoot, artifact)
	if err != nil {
		return err
	}
	if ignored {
		return nil
	}
	if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(artifact))); err == nil {
		return fmt.Errorf("%s is untracked and not ignored or privately excluded; commit it, add it to .gitignore, or add it to .git/info/exclude before enabling pairing index activation", artifact)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect repo-root %s: %w", artifact, err)
	}
	return ensureRepoPrivateExclude(repoRoot, artifact)
}

func stacklitArtifactTracked(repoRoot string) (bool, error) {
	return artifactTracked(repoRoot, stacklitArtifactName)
}

func artifactTracked(repoRoot, artifact string) (bool, error) {
	output, err := gitenv.CombinedOutput(repoRoot, "ls-files", "--error-unmatch", artifact)
	if err == nil {
		return true, nil
	}
	if gitUnmatchedPath(err, output) {
		return false, nil
	}
	return false, fmt.Errorf("inspect repo-root %s tracking: %w%s", artifact, err, outputSuffix(string(output)))
}

func stacklitArtifactIgnored(repoRoot string) (bool, error) {
	return artifactIgnored(repoRoot, stacklitArtifactName)
}

func artifactIgnored(repoRoot, artifact string) (bool, error) {
	output, err := gitenv.CombinedOutput(repoRoot, "check-ignore", artifact)
	if err == nil {
		return true, nil
	}
	if gitExitCode(err, 1) {
		return false, nil
	}
	return false, fmt.Errorf("inspect repo-root %s ignore state: %w%s", artifact, err, outputSuffix(string(output)))
}

func ensureRepoPrivateExclude(repoRoot, entry string) error {
	stacklitArtifactExcludeMu.Lock()
	defer stacklitArtifactExcludeMu.Unlock()

	gitDir, err := resolveGitDir(repoRoot)
	if err != nil {
		return err
	}
	excludePath := filepath.Join(gitDir, "info", "exclude")
	return appendPrivateExcludeEntry(excludePath, entry)
}

func resolveGitDir(repoRoot string) (string, error) {
	output, err := gitenv.Output(repoRoot, "rev-parse", "--git-dir")
	if err != nil {
		return "", fmt.Errorf("resolve repository gitdir: %w%s", err, outputSuffix(string(output)))
	}
	gitDir := strings.TrimSpace(string(output))
	if gitDir == "" {
		return "", fmt.Errorf("resolve repository gitdir: git returned an empty path")
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(repoRoot, gitDir)
	}
	return filepath.Clean(gitDir), nil
}

func appendPrivateExcludeEntry(excludePath, entry string) error {
	if err := os.MkdirAll(filepath.Dir(excludePath), 0755); err != nil {
		return fmt.Errorf("create private exclude directory: %w", err)
	}

	content, err := os.ReadFile(excludePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read private exclude: %w", err)
	}

	for _, line := range strings.Split(string(content), "\n") {
		if strings.TrimSpace(line) == entry {
			return nil
		}
	}

	next := append([]byte(nil), content...)
	if len(next) > 0 && next[len(next)-1] != '\n' {
		next = append(next, '\n')
	}
	next = append(next, entry...)
	next = append(next, '\n')

	if err := os.WriteFile(excludePath, next, 0644); err != nil {
		return fmt.Errorf("write private exclude: %w", err)
	}
	return nil
}

func ensureHooksDir(hooksDir string) error {
	info, err := os.Stat(hooksDir)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("effective Git hooks path %q is not a directory", hooksDir)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("inspect effective Git hooks directory %q: %w", hooksDir, err)
	}
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("create effective Git hooks directory %q: %w", hooksDir, err)
	}
	return nil
}

func installManagedIndexScript(scriptPath, want string) (HookAction, error) {
	current, err := os.ReadFile(scriptPath)
	if os.IsNotExist(err) {
		if err := os.WriteFile(scriptPath, []byte(want), 0755); err != nil {
			return "", fmt.Errorf("install %s: %w", defaultScriptName, err)
		}
		return HookActionInstalled, nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s: %w", defaultScriptName, err)
	}
	if !strings.Contains(string(current), ManagedIndexScriptMarker) {
		return "", fmt.Errorf("%s at %s already exists and is not Liza-managed", defaultScriptName, scriptPath)
	}
	if string(current) == want {
		if err := os.Chmod(scriptPath, 0755); err != nil {
			return "", fmt.Errorf("chmod %s: %w", defaultScriptName, err)
		}
		return HookActionVerified, nil
	}
	if err := os.WriteFile(scriptPath, []byte(want), 0755); err != nil {
		return "", fmt.Errorf("update %s: %w", defaultScriptName, err)
	}
	if err := os.Chmod(scriptPath, 0755); err != nil {
		return "", fmt.Errorf("chmod %s: %w", defaultScriptName, err)
	}
	return HookActionUpdated, nil
}

func rejectHookCollisions(hooksDir string, hooks []string) error {
	var collisions []HookCollision
	for _, hook := range hooks {
		hookPath := filepath.Join(hooksDir, hook)
		content, err := os.ReadFile(hookPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil || !strings.Contains(string(content), ManagedHookMarker) {
			collisions = append(collisions, HookCollision{Hook: hook, Path: hookPath})
		}
	}
	if len(collisions) > 0 {
		return &HookCollisionError{Collisions: collisions}
	}
	return nil
}

func installManagedHook(hookPath, hook string) (HookAction, error) {
	want := managedHookContent(hook)
	current, err := os.ReadFile(hookPath)
	if os.IsNotExist(err) {
		if err := os.WriteFile(hookPath, []byte(want), 0755); err != nil {
			return "", fmt.Errorf("install %s hook wrapper: %w", hook, err)
		}
		return HookActionInstalled, nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s hook wrapper: %w", hook, err)
	}
	if string(current) == want {
		if err := os.Chmod(hookPath, 0755); err != nil {
			return "", fmt.Errorf("chmod %s hook wrapper: %w", hook, err)
		}
		return HookActionVerified, nil
	}
	if err := os.WriteFile(hookPath, []byte(want), 0755); err != nil {
		return "", fmt.Errorf("update %s hook wrapper: %w", hook, err)
	}
	if err := os.Chmod(hookPath, 0755); err != nil {
		return "", fmt.Errorf("chmod %s hook wrapper: %w", hook, err)
	}
	return HookActionUpdated, nil
}

func managedHookContent(hook string) string {
	return fmt.Sprintf(`#!/bin/sh
%s
# Hook: %s
hook_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd) || exit 0
script="$hook_dir/%s"
if [ -x "$script" ]; then
	"$script" "$@"
fi
exit 0
		`, ManagedHookMarker, hook, defaultScriptName)
}

func gitUnmatchedPath(err error, output []byte) bool {
	return gitExitCode(err, 1) && strings.Contains(string(output), "did not match any file")
}

func gitExitCode(err error, code int) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == code
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func outputSuffix(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	return ": " + output
}
