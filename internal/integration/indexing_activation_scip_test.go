package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liza-mas/liza/internal/commands"
	"github.com/liza-mas/liza/internal/scipsearch"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestIndexingActivationScipPairingInitWritesAggregateCommands(t *testing.T) {
	for _, tc := range []struct {
		name         string
		files        map[string]string
		wantCommand  string
		wantRootArgs []string
	}{
		{
			name: "go",
			files: map[string]string{
				"go.mod":      "module example.com/project\n",
				"cmd/main.go": "package main\nfunc main() {}\n",
			},
			wantCommand: "scip-go index",
			wantRootArgs: []string{
				"index --module-root ${PROJECT}",
				`--output "$tmp_go_scip/go-0.scip"`,
				`scip-search aggregate-index --project-root ${PROJECT} --root . --index "$tmp_go_scip/go-0.scip" --out ${PROJECT}/go.scip`,
			},
		},
		{
			name: "typescript",
			files: map[string]string{
				"web/tsconfig.json": `{"include":["src/**/*.ts"]}` + "\n",
				"web/src/app.ts":    "export const app = 1\n",
			},
			wantCommand: "scip-typescript index",
			wantRootArgs: []string{
				"index --cwd ${PROJECT}/web/src",
				`--output "$tmp_typescript_scip/typescript-0.scip" ${PROJECT}/web`,
				`scip-search aggregate-index --project-root ${PROJECT} --root web/src --index "$tmp_typescript_scip/typescript-0.scip" --out ${PROJECT}/typescript.scip`,
			},
		},
		{
			name: "python",
			files: map[string]string{
				"service/pyproject.toml": "[project]\nname = \"service\"\n",
				"service/src/pkg/app.py": "def main():\n    return 1\n",
			},
			wantCommand: "scip-python index",
			wantRootArgs: []string{
				"index --cwd ${PROJECT}/service",
				`--output "$tmp_python_scip/python-0.scip" --target-only=src`,
				`scip-search aggregate-index --project-root ${PROJECT} --root service --index "$tmp_python_scip/python-0.scip" --out ${PROJECT}/python.scip`,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			projectDir := newScipIndexingActivationProject(t)
			commitScipIndexingActivationFiles(t, projectDir, tc.files, "Add "+tc.name+" source")

			if err := commands.InitPairingCommand(commands.InitPairingParams{
				Agents:         []string{"codex"},
				Stdin:          strings.NewReader(""),
				ContractAction: "global",
			}); err != nil {
				t.Fatalf("InitPairingCommand(): %v", err)
			}

			script := readScipIndexingActivationHookScript(t, projectDir)
			assertIndexingActivationContainsAll(t, script, tc.wantCommand)
			for _, want := range tc.wantRootArgs {
				assertIndexingActivationContainsAll(t, script, expandScipProjectPlaceholder(want, projectDir))
			}
		})
	}
}

func TestIndexingActivationScipLanguageFiltersExcludeOtherDetectedLanguages(t *testing.T) {
	projectDir := newScipIndexingActivationProject(t)
	files := map[string]string{
		"go.mod":                 "module example.com/project\n",
		"cmd/main.go":            "package main\nfunc main() {}\n",
		"web/tsconfig.json":      `{"include":["src/**/*.ts"]}` + "\n",
		"web/src/app.ts":         "export const app = 1\n",
		"service/pyproject.toml": "[project]\nname = \"service\"\n",
		"service/src/pkg/app.py": "def main():\n    return 1\n",
	}
	commitScipIndexingActivationFiles(t, projectDir, files, "Add mixed language source")

	if err := commands.InitPairingCommand(commands.InitPairingParams{
		Agents:         []string{"codex"},
		ScipSearch:     []string{"go", "go"},
		Stdin:          strings.NewReader(""),
		ContractAction: "global",
	}); err != nil {
		t.Fatalf("InitPairingCommand(): %v", err)
	}

	script := readScipIndexingActivationHookScript(t, projectDir)
	assertIndexingActivationContainsAll(t, script,
		"scip-go index --module-root "+projectDir,
		`--output "$tmp_go_scip/go-0.scip"`,
		"scip-search aggregate-index --project-root "+projectDir,
		`--root . --index "$tmp_go_scip/go-0.scip" --out `+filepath.Join(projectDir, "go.scip"),
	)
	assertIndexingActivationContainsNone(t, script, "scip-typescript", "scip-python")
}

func TestIndexingActivationScipLanguageFilterAggregatesFilteredRoots(t *testing.T) {
	projectDir := newScipIndexingActivationProject(t)
	files := map[string]string{
		"services/api/go.mod":     "module example.com/api\n",
		"services/api/main.go":    "package main\nfunc main() {}\n",
		"services/worker/go.mod":  "module example.com/worker\n",
		"services/worker/main.go": "package main\nfunc main() {}\n",
		"web/tsconfig.json":       `{"include":["src/**/*.ts"]}` + "\n",
		"web/src/app.ts":          "export const app = 1\n",
	}
	commitScipIndexingActivationFiles(t, projectDir, files, "Add ambiguous Go roots")

	err := commands.InitPairingCommand(commands.InitPairingParams{
		Agents:         []string{"codex"},
		ScipSearch:     []string{"go", "go"},
		Stdin:          strings.NewReader(""),
		ContractAction: "global",
	})
	if err != nil {
		t.Fatalf("InitPairingCommand(): %v", err)
	}
	script := readScipIndexingActivationHookScript(t, projectDir)
	assertIndexingActivationContainsAll(t, script,
		"scip-go index --module-root "+filepath.Join(projectDir, "services", "api"),
		"scip-go index --module-root "+filepath.Join(projectDir, "services", "worker"),
		"scip-search aggregate-index --project-root "+projectDir,
		"--root services/api --index",
		"--root services/worker --index",
		"--out "+filepath.Join(projectDir, "go.scip"),
	)
	assertIndexingActivationContainsNone(t, script, "scip-typescript")
}

func TestIndexingActivationScipMonorepoRootsAggregate(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
		wants []string
	}{
		{
			name: "typescript",
			files: map[string]string{
				"tsconfig.json":            `{"references":[{"path":"apps/web"},{"path":"apps/admin"}]}` + "\n",
				"apps/web/tsconfig.json":   `{"include":["src/**/*.ts"]}` + "\n",
				"apps/web/src/app.ts":      "export const web = 1\n",
				"apps/admin/tsconfig.json": `{"include":["src/**/*.ts"]}` + "\n",
				"apps/admin/src/app.ts":    "export const admin = 1\n",
			},
			wants: []string{
				"scip-typescript index --cwd ${PROJECT}/apps/admin/src",
				"scip-typescript index --cwd ${PROJECT}/apps/web/src",
				"scip-search aggregate-index --project-root ${PROJECT}",
				"--root apps/admin/src --index",
				"--root apps/web/src --index",
				"--out ${PROJECT}/typescript.scip",
			},
		},
		{
			name: "python",
			files: map[string]string{
				"apps/api/pyproject.toml":    "[project]\nname = \"api\"\n",
				"apps/api/app.py":            "def api():\n    return 1\n",
				"apps/worker/pyproject.toml": "[project]\nname = \"worker\"\n",
				"apps/worker/worker.py":      "def worker():\n    return 1\n",
			},
			wants: []string{
				"scip-python index --cwd ${PROJECT}/apps/api",
				"scip-python index --cwd ${PROJECT}/apps/worker",
				"scip-search aggregate-index --project-root ${PROJECT}",
				"--root apps/api --index",
				"--root apps/worker --index",
				"--out ${PROJECT}/python.scip",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			projectDir := newScipIndexingActivationProject(t)
			commitScipIndexingActivationFiles(t, projectDir, tc.files, "Add ambiguous "+tc.name+" roots")

			err := commands.InitPairingCommand(commands.InitPairingParams{
				Agents:         []string{"codex"},
				ScipSearch:     []string{tc.name},
				Stdin:          strings.NewReader(""),
				ContractAction: "global",
			})
			if err != nil {
				t.Fatalf("InitPairingCommand(): %v", err)
			}
			script := readScipIndexingActivationHookScript(t, projectDir)
			for _, want := range tc.wants {
				want = expandScipProjectPlaceholder(want, projectDir)
				assertIndexingActivationContainsAll(t, script, want)
			}
		})
	}
}

func newScipIndexingActivationProject(t *testing.T) string {
	t.Helper()

	t.Setenv(scipsearch.EnvEnableScipSearch, "true")
	projectDir := newIndexingActivationProject(t)
	resolved, err := filepath.EvalSymlinks(projectDir)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", projectDir, err)
	}
	return resolved
}

func commitScipIndexingActivationFiles(t *testing.T, projectDir string, files map[string]string, message string) {
	t.Helper()

	paths := make([]string, 0, len(files))
	for rel, content := range files {
		writeIndexingActivationFile(t, filepath.Join(projectDir, rel), content)
		paths = append(paths, rel)
	}
	testhelpers.MustGit(t, projectDir, append([]string{"add"}, paths...)...)
	testhelpers.MustGit(t, projectDir, "commit", "-m", message)
}

func readScipIndexingActivationHookScript(t *testing.T, projectDir string) string {
	t.Helper()

	path := filepath.Join(projectDir, ".git", "hooks", "liza-index.sh")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", path, err)
	}
	return string(content)
}

func expandScipProjectPlaceholder(value, projectDir string) string {
	return strings.ReplaceAll(value, "${PROJECT}", projectDir)
}
