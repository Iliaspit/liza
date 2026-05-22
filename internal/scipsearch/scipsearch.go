package scipsearch

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/liza-mas/liza/internal/gitenv"
	"github.com/tailscale/hujson"
)

const EnvEnableScipSearch = "LIZA_ENABLE_SCIP_SEARCH"

const maxFailureDiagnosticBytes = 1024

var supportedLanguages = []string{"go", "typescript", "python"}

var languageIndexers = map[string]string{
	"go":         "scip-go",
	"typescript": "scip-typescript",
	"python":     "scip-python",
}

// CommandRunner runs a fixed executable name with argv entries.
type CommandRunner func(name string, args ...string) (string, error)

// GitFilesFunc returns git-tracked files for a project root.
type GitFilesFunc func(projectRoot string) ([]string, error)

type InitOptions struct {
	ProjectRoot       string
	ExplicitLanguages []string
	EnvValue          string
	CommandRunner     CommandRunner
	GitFiles          GitFilesFunc
}

type InitResult struct {
	Languages   []string
	Diagnostics []string
	Warnings    []string
}

// RuntimePlanOptions configures runtime command planning for one target root.
type RuntimePlanOptions struct {
	TargetRoot          string
	ConfiguredLanguages []string
	GitFiles            GitFilesFunc
}

// RuntimeCommandPlan describes one fixed language-indexer invocation.
type RuntimeCommandPlan struct {
	Language   string
	Name       string
	Args       []string
	Dir        string
	OutputPath string
}

// RuntimeRunner executes one runtime indexer command plan.
type RuntimeRunner func(RuntimeCommandPlan) (string, error)

// TargetKind identifies the lifecycle target being refreshed.
type TargetKind string

const (
	TargetKindProjectRoot  TargetKind = "project-root"
	TargetKindTaskWorktree TargetKind = "task-worktree"
)

// RefreshOptions configures one best-effort runtime index refresh.
type RefreshOptions struct {
	TargetRoot          string
	TargetKind          TargetKind
	ConfiguredLanguages []string
	GitFiles            GitFilesFunc
	Runner              RuntimeRunner
}

// RefreshResult contains successful indexes and isolated language failures from
// one refresh attempt.
type RefreshResult struct {
	Successes []IndexRef
	Failures  []RefreshFailure
}

// IndexRef identifies one prompt-safe generated index file.
type IndexRef struct {
	Language string
	Path     string
}

// RefreshFailure contains bounded diagnostics for one failed language indexer.
type RefreshFailure struct {
	Language   string
	Diagnostic string
}

var (
	runnerMu                    sync.Mutex
	taskWorktreeExcludeConfigMu sync.Mutex
	defaultRunner               CommandRunner = runCommand
)

// SetCommandRunnerForTest replaces the process runner until the returned restore
// function is called. It is intended for package-level init integration tests.
func SetCommandRunnerForTest(runner CommandRunner) func() {
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

func ResolveInitConfig(opts InitOptions) (InitResult, error) {
	runner := opts.CommandRunner
	if runner == nil {
		runner = getDefaultRunner()
	}
	gitFiles := opts.GitFiles
	if gitFiles == nil {
		gitFiles = listGitFiles
	}

	var result InitResult
	if output, err := runner("scip-search", "--help"); err != nil {
		return result, fmt.Errorf("scip-search setup validation failed: scip-search --help failed: %w%s", err, outputSuffix(output))
	}

	if output, err := runner("scip-search", "--version"); err == nil {
		if version := strings.TrimSpace(output); version != "" {
			result.Diagnostics = append(result.Diagnostics, "scip-search --version: "+version)
		}
	}

	selected, err := selectLanguages(opts, gitFiles)
	if err != nil {
		return result, err
	}

	result.Languages, result.Warnings = validateIndexers(selected, runner)
	return result, nil
}

func ParseEnvGate(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true":
		return true
	default:
		return false
	}
}

// RuntimeEnabled reports whether runtime scip-search behavior is active for the
// current process. It is true only when LIZA_ENABLE_SCIP_SEARCH is truthy and at
// least one configured language from Config.ScipSearch remains available.
func RuntimeEnabled(configuredLanguages []string) bool {
	return ParseEnvGate(os.Getenv(EnvEnableScipSearch)) && len(configuredLanguages) > 0
}

// PlanRuntimeCommands selects detected configured languages for a target root
// and returns fixed command plans without executing indexers or writing files.
func PlanRuntimeCommands(opts RuntimePlanOptions) ([]RuntimeCommandPlan, error) {
	if !RuntimeEnabled(opts.ConfiguredLanguages) {
		return nil, nil
	}

	targetRoot, err := filepath.Abs(opts.TargetRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve scip-search target root: %w", err)
	}

	gitFiles := opts.GitFiles
	if gitFiles == nil {
		gitFiles = listGitFiles
	}
	files, err := gitFiles(targetRoot)
	if err != nil {
		return nil, fmt.Errorf("detect runtime scip-search languages: %w", err)
	}

	return buildRuntimeCommandPlans(targetRoot, filterRuntimeLanguages(opts.ConfiguredLanguages, detectLanguages(files)), files), nil
}

// RefreshIndexes executes selected runtime indexer command plans and reports
// per-language results. Indexer failures are isolated to their language.
func RefreshIndexes(opts RefreshOptions) (RefreshResult, error) {
	plans, err := PlanRuntimeCommands(RuntimePlanOptions{
		TargetRoot:          opts.TargetRoot,
		ConfiguredLanguages: opts.ConfiguredLanguages,
		GitFiles:            opts.GitFiles,
	})
	if err != nil {
		return RefreshResult{}, err
	}
	if len(plans) == 0 {
		return RefreshResult{}, nil
	}

	if opts.TargetKind == TargetKindTaskWorktree {
		if err := ensureTaskWorktreeScipExclude(plans[0].Dir); err != nil {
			return RefreshResult{}, err
		}
	}

	if err := os.MkdirAll(filepath.Dir(plans[0].OutputPath), 0o755); err != nil {
		return RefreshResult{}, fmt.Errorf("create scip-search index directory: %w", err)
	}

	runner := opts.Runner
	if runner == nil {
		runner = runRuntimeCommandPlan
	}

	var result RefreshResult
	for _, plan := range plans {
		if err := removeStaleIndex(plan.OutputPath); err != nil {
			result.Failures = append(result.Failures, RefreshFailure{
				Language:   plan.Language,
				Diagnostic: boundedFailureDiagnostic(err, ""),
			})
			continue
		}
		output, err := runner(plan)
		if err != nil {
			diagnosticErr := err
			if cleanupErr := removeStaleIndex(plan.OutputPath); cleanupErr != nil {
				diagnosticErr = fmt.Errorf("%w; additionally %v", err, cleanupErr)
			}
			result.Failures = append(result.Failures, RefreshFailure{
				Language:   plan.Language,
				Diagnostic: boundedFailureDiagnostic(diagnosticErr, output),
			})
			continue
		}
		if _, err := os.Stat(plan.OutputPath); err != nil {
			result.Failures = append(result.Failures, RefreshFailure{
				Language:   plan.Language,
				Diagnostic: boundedFailureDiagnostic(fmt.Errorf("indexer did not write %s: %w", plan.OutputPath, err), output),
			})
			continue
		}
		result.Successes = append(result.Successes, IndexRef{Language: plan.Language, Path: plan.OutputPath})
	}
	return result, nil
}

// AvailableIndexes returns existing absolute index paths for selected runtime
// languages. Missing files are omitted rather than reported as failures.
func AvailableIndexes(opts RuntimePlanOptions) ([]IndexRef, error) {
	plans, err := PlanRuntimeCommands(opts)
	if err != nil {
		return nil, err
	}

	indexes := make([]IndexRef, 0, len(plans))
	for _, plan := range plans {
		info, err := os.Stat(plan.OutputPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("inspect scip-search index %q: %w", plan.OutputPath, err)
		}
		if info.IsDir() {
			continue
		}
		indexes = append(indexes, IndexRef{Language: plan.Language, Path: plan.OutputPath})
	}
	return indexes, nil
}

func selectLanguages(opts InitOptions, gitFiles GitFilesFunc) ([]string, error) {
	if len(opts.ExplicitLanguages) > 0 {
		return canonicalizeExplicit(opts.ExplicitLanguages)
	}
	if !ParseEnvGate(opts.EnvValue) {
		return nil, nil
	}
	files, err := gitFiles(opts.ProjectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to auto-detect scip-search languages with git ls-files: %w", err)
	}
	return detectLanguages(files), nil
}

func canonicalizeExplicit(raw []string) ([]string, error) {
	seen := make(map[string]bool, len(raw))
	for _, language := range raw {
		language = strings.TrimSpace(language)
		if !slices.Contains(supportedLanguages, language) {
			return nil, fmt.Errorf("unsupported scip-search language %q (supported: %s)", language, strings.Join(supportedLanguages, ", "))
		}
		seen[language] = true
	}

	var out []string
	for _, language := range supportedLanguages {
		if seen[language] {
			out = append(out, language)
		}
	}
	return out, nil
}

func detectLanguages(files []string) []string {
	detected := map[string]bool{}
	for _, file := range files {
		base := filepath.Base(file)
		ext := filepath.Ext(file)
		switch {
		case base == "go.mod" || ext == ".go":
			detected["go"] = true
		case base == "tsconfig.json" || ext == ".ts" || ext == ".tsx":
			detected["typescript"] = true
		case base == "pyproject.toml" || ext == ".py":
			detected["python"] = true
		}
	}

	var out []string
	for _, language := range supportedLanguages {
		if detected[language] {
			out = append(out, language)
		}
	}
	return out
}

func filterRuntimeLanguages(configuredLanguages, detectedLanguages []string) []string {
	configured := make(map[string]bool, len(configuredLanguages))
	for _, language := range configuredLanguages {
		configured[language] = true
	}
	detected := make(map[string]bool, len(detectedLanguages))
	for _, language := range detectedLanguages {
		detected[language] = true
	}

	var out []string
	for _, language := range supportedLanguages {
		if configured[language] && detected[language] {
			out = append(out, language)
		}
	}
	return out
}

func buildRuntimeCommandPlans(targetRoot string, languages []string, files []string) []RuntimeCommandPlan {
	plans := make([]RuntimeCommandPlan, 0, len(languages))
	for _, language := range languages {
		outputPath := filepath.Join(targetRoot, ".liza", "scip", language+".scip")
		plan, ok := runtimeCommandPlan(targetRoot, language, outputPath, files)
		if ok {
			plans = append(plans, plan)
		}
	}
	return plans
}

func runtimeCommandPlan(targetRoot, language, outputPath string, files []string) (RuntimeCommandPlan, bool) {
	plan := RuntimeCommandPlan{
		Language:   language,
		Name:       languageIndexers[language],
		Dir:        targetRoot,
		OutputPath: outputPath,
	}
	switch language {
	case "go":
		plan.Args = []string{"index", "--module-root", targetRoot, "--skip-tests", "--output", outputPath}
	case "typescript":
		cwd := targetRoot
		projectRoot := targetRoot
		if inputs, ok := inferTypeScriptCommandInputs(targetRoot, files); ok {
			cwd = inputs.Cwd
			projectRoot = inputs.ProjectRoot
		}
		plan.Args = []string{"index", "--cwd", cwd, "--output", outputPath, projectRoot}
	case "python":
		inputs, ok := inferPythonCommandInputs(targetRoot, files)
		if !ok {
			return RuntimeCommandPlan{}, false
		}
		plan.Args = []string{"index", "--cwd", inputs.Cwd, "--output", outputPath}
		if inputs.TargetOnly != "" {
			plan.Args = append(plan.Args, "--target-only="+inputs.TargetOnly)
		}
	}
	return plan, true
}

type pythonCommandInputs struct {
	Cwd        string
	TargetOnly string
}

var pythonProjectMarkerPriority = map[string]int{
	"pyproject.toml":   0,
	"setup.cfg":        1,
	"setup.py":         2,
	"requirements.txt": 3,
}

func inferPythonCommandInputs(targetRoot string, files []string) (pythonCommandInputs, bool) {
	pythonFiles := pythonTrackedFiles(targetRoot, files)
	if len(pythonFiles) == 0 {
		return pythonCommandInputs{}, false
	}

	roots := pythonProjectRootCandidates(targetRoot, files)
	for _, root := range roots {
		eligible := eligiblePythonFilesForRoot(root, roots, pythonFiles)
		if len(eligible) == 0 {
			continue
		}
		inputs := pythonCommandInputs{Cwd: root}
		if pythonFilesAllUnderSrc(root, eligible) {
			inputs.TargetOnly = "src"
		}
		return inputs, true
	}

	return pythonCommandInputs{Cwd: targetRoot}, true
}

func pythonTrackedFiles(targetRoot string, files []string) []string {
	var out []string
	for _, file := range files {
		if filepath.Ext(file) != ".py" {
			continue
		}
		path, ok := cleanTargetRelativePath(targetRoot, file)
		if ok {
			out = append(out, path)
		}
	}
	sort.Strings(out)
	return out
}

func pythonProjectRootCandidates(targetRoot string, files []string) []string {
	seen := map[string]int{}
	for _, file := range files {
		base := filepath.Base(file)
		priority, ok := pythonProjectMarkerPriority[base]
		if !ok {
			continue
		}
		path, ok := cleanTargetRelativePath(targetRoot, file)
		if !ok {
			continue
		}
		root := filepath.Dir(path)
		if previous, exists := seen[root]; !exists || priority < previous {
			seen[root] = priority
		}
	}

	roots := make([]string, 0, len(seen))
	for root := range seen {
		roots = append(roots, root)
	}
	sort.Slice(roots, func(i, j int) bool {
		iDepth := pathDepthWithin(targetRoot, roots[i])
		jDepth := pathDepthWithin(targetRoot, roots[j])
		if iDepth != jDepth {
			return iDepth < jDepth
		}
		return roots[i] < roots[j]
	})
	return roots
}

func eligiblePythonFilesForRoot(root string, candidateRoots, pythonFiles []string) []string {
	descendantRoots := make([]string, 0)
	for _, candidateRoot := range candidateRoots {
		if candidateRoot != root && pathWithin(root, candidateRoot) {
			descendantRoots = append(descendantRoots, candidateRoot)
		}
	}

	var eligible []string
	for _, file := range pythonFiles {
		if !pathWithin(root, file) {
			continue
		}
		if pathWithinAny(descendantRoots, file) {
			continue
		}
		eligible = append(eligible, file)
	}
	return eligible
}

func pythonFilesAllUnderSrc(root string, files []string) bool {
	srcRoot := filepath.Join(root, "src")
	for _, file := range files {
		if !pathWithin(srcRoot, file) {
			return false
		}
	}
	return len(files) > 0
}

func pathWithinAny(roots []string, path string) bool {
	for _, root := range roots {
		if pathWithin(root, path) {
			return true
		}
	}
	return false
}

func cleanTargetRelativePath(targetRoot, file string) (string, bool) {
	file = filepath.Clean(filepath.FromSlash(file))
	if file == "." || filepath.IsAbs(file) || strings.HasPrefix(file, ".."+string(filepath.Separator)) || file == ".." {
		return "", false
	}
	path := filepath.Join(targetRoot, file)
	if !pathWithin(targetRoot, path) {
		return "", false
	}
	return filepath.Clean(path), true
}

func pathDepthWithin(root, path string) int {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil || rel == "." {
		return 0
	}
	return strings.Count(filepath.ToSlash(rel), "/") + 1
}

type typeScriptCommandInputs struct {
	Cwd         string
	ProjectRoot string
}

type tsconfigFile struct {
	Files      []string            `json:"files"`
	Include    []string            `json:"include"`
	References []tsconfigReference `json:"references"`
}

type tsconfigReference struct {
	Path string `json:"path"`
}

type typeScriptSourceCandidate struct {
	configPath  string
	sourceRoot  string
	projectRoot string
}

func inferTypeScriptCommandInputs(targetRoot string, files []string) (typeScriptCommandInputs, bool) {
	rootConfigs := typeScriptRootConfigCandidates(files)
	for _, relConfig := range rootConfigs {
		rootConfig := filepath.Join(targetRoot, filepath.FromSlash(relConfig))
		rootConfig = filepath.Clean(rootConfig)
		if !pathWithin(targetRoot, rootConfig) {
			continue
		}

		projectRoot := filepath.Dir(rootConfig)
		sourceCandidates := typeScriptSourceCandidatesForRoot(targetRoot, rootConfig, projectRoot)
		if len(sourceCandidates) == 0 {
			continue
		}
		sort.Slice(sourceCandidates, func(i, j int) bool {
			if sourceCandidates[i].configPath != sourceCandidates[j].configPath {
				return sourceCandidates[i].configPath < sourceCandidates[j].configPath
			}
			return sourceCandidates[i].sourceRoot < sourceCandidates[j].sourceRoot
		})
		candidate := sourceCandidates[0]
		return typeScriptCommandInputs{Cwd: candidate.sourceRoot, ProjectRoot: candidate.projectRoot}, true
	}
	return typeScriptCommandInputs{}, false
}

func typeScriptRootConfigCandidates(files []string) []string {
	candidates := make([]string, 0)
	for _, file := range files {
		file = filepath.ToSlash(filepath.Clean(file))
		if file == "." || strings.HasPrefix(file, "../") || filepath.IsAbs(file) {
			continue
		}
		if filepath.Base(file) == "tsconfig.json" {
			candidates = append(candidates, file)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		iDepth := strings.Count(candidates[i], "/")
		jDepth := strings.Count(candidates[j], "/")
		if iDepth != jDepth {
			return iDepth < jDepth
		}
		return candidates[i] < candidates[j]
	})
	return candidates
}

func typeScriptSourceCandidatesForRoot(targetRoot, rootConfig, projectRoot string) []typeScriptSourceCandidate {
	leafConfigs := collectTypeScriptLeafConfigs(targetRoot, rootConfig, map[string]bool{})
	candidates := make([]typeScriptSourceCandidate, 0, len(leafConfigs))
	for _, leafConfig := range leafConfigs {
		config, err := readTSConfig(leafConfig)
		if err != nil {
			continue
		}
		for _, sourceRoot := range typeScriptSourceRootsForConfig(filepath.Dir(leafConfig), config) {
			if !pathWithin(targetRoot, sourceRoot) {
				continue
			}
			candidates = append(candidates, typeScriptSourceCandidate{
				configPath:  filepath.Clean(leafConfig),
				sourceRoot:  filepath.Clean(sourceRoot),
				projectRoot: projectRoot,
			})
		}
	}
	return candidates
}

func collectTypeScriptLeafConfigs(targetRoot, configPath string, visited map[string]bool) []string {
	configPath = filepath.Clean(configPath)
	if visited[configPath] || !pathWithin(targetRoot, configPath) {
		return nil
	}
	visited[configPath] = true

	config, err := readTSConfig(configPath)
	if err != nil {
		return nil
	}
	if len(config.References) == 0 {
		return []string{configPath}
	}

	refs := append([]tsconfigReference(nil), config.References...)
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].Path < refs[j].Path
	})

	var leaves []string
	configDir := filepath.Dir(configPath)
	for _, ref := range refs {
		refConfig, ok := resolveTypeScriptReferenceConfig(targetRoot, configDir, ref.Path)
		if !ok {
			continue
		}
		leaves = append(leaves, collectTypeScriptLeafConfigs(targetRoot, refConfig, visited)...)
	}
	return leaves
}

func readTSConfig(path string) (tsconfigFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return tsconfigFile{}, err
	}
	standard, err := hujson.Standardize(content)
	if err != nil {
		return tsconfigFile{}, err
	}
	var config tsconfigFile
	if err := json.Unmarshal(standard, &config); err != nil {
		return tsconfigFile{}, err
	}
	return config, nil
}

func resolveTypeScriptReferenceConfig(targetRoot, configDir, refPath string) (string, bool) {
	refPath = strings.TrimSpace(refPath)
	if refPath == "" {
		return "", false
	}

	base := filepath.Clean(filepath.Join(configDir, filepath.FromSlash(refPath)))
	candidates := typeScriptReferenceConfigCandidates(base)
	for _, candidate := range candidates {
		if !pathWithin(targetRoot, candidate) {
			continue
		}
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return filepath.Clean(candidate), true
		}
	}
	return "", false
}

func typeScriptReferenceConfigCandidates(base string) []string {
	if filepath.Ext(base) == ".json" {
		return []string{base}
	}
	return dedupePaths([]string{
		base + ".json",
		filepath.Join(base, "tsconfig.json"),
		base,
	})
}

func dedupePaths(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.Clean(path)
		if seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	return out
}

func typeScriptSourceRootsForConfig(configDir string, config tsconfigFile) []string {
	var roots []string
	for _, include := range config.Include {
		if root, ok := typeScriptIncludeSourceRoot(configDir, include); ok {
			roots = append(roots, root)
		}
	}
	for _, file := range config.Files {
		if root, ok := typeScriptFileSourceRoot(configDir, file); ok {
			roots = append(roots, root)
		}
	}
	return dedupePaths(roots)
}

func typeScriptIncludeSourceRoot(configDir, include string) (string, bool) {
	include = strings.TrimSpace(include)
	if include == "" {
		return "", false
	}
	prefix := staticGlobDirectoryPrefix(filepath.ToSlash(include))
	root, ok := resolveTypeScriptSourceRoot(configDir, prefix)
	if !ok {
		return "", false
	}
	if info, err := os.Stat(root); err == nil && !info.IsDir() {
		return filepath.Dir(root), true
	}
	return root, true
}

func typeScriptFileSourceRoot(configDir, file string) (string, bool) {
	file = strings.TrimSpace(file)
	if file == "" {
		return "", false
	}
	file = filepath.ToSlash(file)
	if hasGlobMeta(file) {
		return "", false
	}
	return resolveTypeScriptSourceRoot(configDir, filepath.Dir(file))
}

func staticGlobDirectoryPrefix(pattern string) string {
	pattern = strings.Trim(pattern, "/")
	if pattern == "" || pattern == "." {
		return "."
	}

	parts := strings.Split(pattern, "/")
	var prefix []string
	for _, part := range parts {
		if hasGlobMeta(part) {
			break
		}
		prefix = append(prefix, part)
	}
	if len(prefix) == 0 {
		return "."
	}
	return strings.Join(prefix, "/")
}

func hasGlobMeta(value string) bool {
	return strings.ContainsAny(value, "*?[")
}

func resolveTypeScriptSourceRoot(configDir, sourceRoot string) (string, bool) {
	sourceRoot = strings.TrimSpace(sourceRoot)
	if sourceRoot == "" {
		return "", false
	}
	if filepath.IsAbs(sourceRoot) {
		return filepath.Clean(sourceRoot), true
	}
	return filepath.Clean(filepath.Join(configDir, filepath.FromSlash(sourceRoot))), true
}

func pathWithin(root, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." && !filepath.IsAbs(rel))
}

func validateIndexers(languages []string, runner CommandRunner) ([]string, []string) {
	var validated []string
	var warnings []string
	for _, language := range languages {
		indexer := languageIndexers[language]
		if output, err := runner(indexer, "--help"); err != nil {
			warnings = append(warnings, fmt.Sprintf("dropping scip-search language %q: %s --help failed: %v%s", language, indexer, err, outputSuffix(output)))
			continue
		}
		validated = append(validated, language)
	}
	return validated, warnings
}

func getDefaultRunner() CommandRunner {
	runnerMu.Lock()
	defer runnerMu.Unlock()
	return defaultRunner
}

func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func runRuntimeCommandPlan(plan RuntimeCommandPlan) (string, error) {
	cmd := exec.Command(plan.Name, plan.Args...)
	cmd.Dir = plan.Dir
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func removeStaleIndex(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale scip-search index %q: %w", path, err)
	}
	return nil
}

func ensureTaskWorktreeScipExclude(targetRoot string) error {
	output, err := gitenv.Output(targetRoot, "rev-parse", "--git-dir")
	if err != nil {
		return fmt.Errorf("resolve task worktree gitdir: %w", err)
	}

	gitDir := strings.TrimSpace(string(output))
	if gitDir == "" {
		return fmt.Errorf("resolve task worktree gitdir: git rev-parse --git-dir returned empty path")
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(targetRoot, gitDir)
	}

	excludePath := filepath.Join(gitDir, "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return fmt.Errorf("create task worktree exclude directory: %w", err)
	}

	content, err := os.ReadFile(excludePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read task worktree exclude: %w", err)
	}
	if hasTaskWorktreeScipExclude(content) {
		return configureTaskWorktreeExclude(targetRoot, excludePath)
	}

	next := slices.Clone(content)
	if len(next) > 0 && next[len(next)-1] != '\n' {
		next = append(next, '\n')
	}
	next = append(next, ".liza/scip/\n"...)
	if err := os.WriteFile(excludePath, next, 0o644); err != nil {
		return fmt.Errorf("write task worktree exclude: %w", err)
	}
	if err := configureTaskWorktreeExclude(targetRoot, excludePath); err != nil {
		return err
	}
	return nil
}

func hasTaskWorktreeScipExclude(content []byte) bool {
	for _, line := range strings.Split(string(content), "\n") {
		if strings.TrimSpace(line) == ".liza/scip/" {
			return true
		}
	}
	return false
}

func configureTaskWorktreeExclude(targetRoot, excludePath string) error {
	taskWorktreeExcludeConfigMu.Lock()
	defer taskWorktreeExcludeConfigMu.Unlock()

	// Linked worktrees do not consult their private info/exclude unless
	// worktree-specific config points core.excludesFile at it.
	worktreeConfigEnabled := false
	output, err := gitenv.Output(targetRoot, "config", "--get", "extensions.worktreeConfig")
	if err == nil {
		worktreeConfigEnabled = strings.EqualFold(strings.TrimSpace(string(output)), "true")
	} else if !gitConfigUnset(err) {
		return fmt.Errorf("inspect task worktree config extension: %w", err)
	}

	if output, err := gitenv.CombinedOutput(targetRoot, "config", "extensions.worktreeConfig", "true"); err != nil {
		return fmt.Errorf("enable task worktree config for scip-search exclude: %w%s", err, outputSuffix(string(output)))
	}
	if !worktreeConfigEnabled {
		log.Printf("INFO: enabled git extensions.worktreeConfig for scip-search task worktree excludes in %s", targetRoot)
	}

	output, err = gitenv.Output(targetRoot, "config", "--worktree", "--get", "core.excludesFile")
	if err == nil {
		current := strings.TrimSpace(string(output))
		if current != "" && filepath.Clean(current) != filepath.Clean(excludePath) {
			return fmt.Errorf("task worktree core.excludesFile already configured as %q", current)
		}
	}
	if err != nil && !gitConfigUnset(err) {
		return fmt.Errorf("inspect task worktree core.excludesFile: %w", err)
	}

	if output, err := gitenv.CombinedOutput(targetRoot, "config", "--worktree", "core.excludesFile", excludePath); err != nil {
		return fmt.Errorf("configure task worktree scip-search exclude: %w%s", err, outputSuffix(string(output)))
	}
	return nil
}

func gitConfigUnset(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 1
}

func listGitFiles(projectRoot string) ([]string, error) {
	output, err := gitenv.Output(projectRoot, "ls-files")
	if err != nil {
		return nil, err
	}

	var files []string
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

func outputSuffix(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	return ": " + output
}

func boundedFailureDiagnostic(err error, output string) string {
	diagnostic := err.Error()
	if output = strings.TrimSpace(output); output != "" {
		diagnostic += ": " + output
	}
	if len(diagnostic) <= maxFailureDiagnosticBytes {
		return diagnostic
	}
	return diagnostic[:maxFailureDiagnosticBytes]
}
