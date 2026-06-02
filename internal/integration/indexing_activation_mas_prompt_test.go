package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liza-mas/liza/internal/agent"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/scipsearch"
	"github.com/liza-mas/liza/internal/semble"
	"github.com/liza-mas/liza/internal/stacklit"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestIndexingActivationMASPromptsRenderEnabledMetadataFromRoleTargetRoots(t *testing.T) {
	projectRoot := t.TempDir()
	configureIndexingActivationSembleReady(t)
	enableOptionalIndexingForTest(t)

	pairingStacklitIndex := filepath.Join(projectRoot, "stacklit.json")
	pairingScipIndex := filepath.Join(projectRoot, ".liza", "scip", "go.scip")
	writeIndexingActivationFile(t, pairingStacklitIndex, `{"project":{"name":"pairing-root"}}`)
	writeIndexingActivationFile(t, pairingScipIndex, "pairing root go index")
	writeIndexingActivationFile(t, filepath.Join(projectRoot, ".sembleignore"), semble.DefaultIgnorePayload())

	for _, tt := range []struct {
		name           string
		role           string
		agentID        string
		taskID         string
		worktreeRel    string
		targetRoot     string
		forbiddenPaths []string
	}{
		{
			name:        "task prompt",
			role:        "coder",
			agentID:     "coder-1",
			taskID:      "task-1",
			worktreeRel: ".worktrees/task-1",
			targetRoot:  filepath.Join(projectRoot, ".worktrees", "task-1"),
			forbiddenPaths: []string{
				pairingStacklitIndex,
				pairingScipIndex,
			},
		},
		{
			name:        "reviewer prompt",
			role:        "code-reviewer",
			agentID:     "code-reviewer-1",
			taskID:      "task-1",
			worktreeRel: ".worktrees/reviewer-task-1",
			targetRoot:  filepath.Join(projectRoot, ".worktrees", "reviewer-task-1"),
			forbiddenPaths: []string{
				pairingStacklitIndex,
				pairingScipIndex,
				filepath.Join(projectRoot, ".worktrees", "task-1"),
			},
		},
		{
			name:       "orchestrator prompt",
			role:       "orchestrator",
			agentID:    "orchestrator-1",
			taskID:     "",
			targetRoot: projectRoot,
			forbiddenPaths: []string{
				filepath.Join(projectRoot, ".worktrees", "task-1"),
				filepath.Join(projectRoot, ".worktrees", "reviewer-task-1"),
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			prepareOptionalIndexTargetRoot(t, tt.targetRoot)
			if tt.worktreeRel != "" && tt.worktreeRel != ".worktrees/task-1" {
				writeIndexingActivationFile(t, filepath.Join(projectRoot, ".worktrees", "task-1", "stacklit.json"), `{"project":{"name":"other-worktree"}}`)
			}

			prompt := buildIndexingActivationMASPrompt(t, projectRoot, tt.role, tt.agentID, tt.taskID, tt.worktreeRel)
			stacklitIndex := filepath.Join(tt.targetRoot, "stacklit.json")
			scipIndex := filepath.Join(tt.targetRoot, ".liza", "scip", "go.scip")

			assertIndexingActivationContainsAll(t, prompt,
				"=== STACKLIT INDEX ===",
				"stacklit derive --ai-summary -i '"+stacklitIndex+"'",
				"=== SCIP-SEARCH INDEXES ===",
				"scip-search symbols --index '"+scipIndex+"' --name Foo --name Bar",
				"=== SEMBLE SEARCH ===",
				`HF_HUB_OFFLINE=1 semble search "where is review submission validated?" `+shellQuoteForIndexingActivationTest(tt.targetRoot),
			)
			assertIndexingActivationContainsNone(t, prompt, tt.forbiddenPaths...)
		})
	}
}

func TestIndexingActivationMASPromptsOmitDisabledSectionsDespiteStaleArtifacts(t *testing.T) {
	for _, tt := range indexingActivationMASPromptRoleCases() {
		t.Run(tt.name, func(t *testing.T) {
			disableOptionalIndexingForTest(t)
			projectRoot := t.TempDir()
			targetRoot := indexingActivationMASTargetRoot(projectRoot, tt.worktreeRel)
			prepareOptionalIndexTargetRoot(t, targetRoot)

			prompt := buildIndexingActivationMASPrompt(t, projectRoot, tt.role, tt.agentID, tt.taskID, tt.worktreeRel)

			assertIndexingActivationContainsNone(t, prompt, append(optionalIndexCommandBlocks(),
				"=== STACKLIT INDEX ===",
				"=== SCIP-SEARCH INDEXES ===",
				"=== SEMBLE SEARCH ===",
				filepath.Join(targetRoot, "stacklit.json"),
				filepath.Join(targetRoot, ".liza", "scip", "go.scip"),
			)...)
		})
	}
}

func TestIndexingActivationMASPromptsOmitFailedOptionalToolOnly(t *testing.T) {
	for _, tt := range indexingActivationMASPromptRoleCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(stacklit.EnvEnableStacklit, "true")
			t.Setenv(scipsearch.EnvEnableScipSearch, "true")
			t.Setenv(semble.EnvEnableSemble, "false")
			projectRoot := t.TempDir()
			targetRoot := indexingActivationMASTargetRoot(projectRoot, tt.worktreeRel)
			writeIndexingActivationFile(t, filepath.Join(targetRoot, "stacklit.json"), `{"project":{"name":"ready"}}`)

			prompt := buildIndexingActivationMASPrompt(t, projectRoot, tt.role, tt.agentID, tt.taskID, tt.worktreeRel)

			assertIndexingActivationContainsAll(t, prompt,
				"=== STACKLIT INDEX ===",
				"stacklit derive --ai-summary -i '"+filepath.Join(targetRoot, "stacklit.json")+"'",
			)
			assertIndexingActivationContainsNone(t, prompt,
				"=== SCIP-SEARCH INDEXES ===",
				"scip-search symbols --index",
				"=== SEMBLE SEARCH ===",
				"semble search",
			)
		})
	}
}

type indexingActivationMASPromptRoleCase struct {
	name        string
	role        string
	agentID     string
	taskID      string
	worktreeRel string
}

func indexingActivationMASPromptRoleCases() []indexingActivationMASPromptRoleCase {
	return []indexingActivationMASPromptRoleCase{
		{
			name:        "task prompt",
			role:        "coder",
			agentID:     "coder-1",
			taskID:      "task-1",
			worktreeRel: ".worktrees/task-1",
		},
		{
			name:        "reviewer prompt",
			role:        "code-reviewer",
			agentID:     "code-reviewer-1",
			taskID:      "task-1",
			worktreeRel: ".worktrees/reviewer-task-1",
		},
		{
			name:    "orchestrator prompt",
			role:    "orchestrator",
			agentID: "orchestrator-1",
			taskID:  "",
		},
	}
}

func indexingActivationMASTargetRoot(projectRoot, worktreeRel string) string {
	if worktreeRel == "" {
		return projectRoot
	}
	return filepath.Join(projectRoot, worktreeRel)
}

func enableOptionalIndexingForTest(t *testing.T) {
	t.Helper()

	t.Setenv(stacklit.EnvEnableStacklit, "true")
	t.Setenv(scipsearch.EnvEnableScipSearch, "true")
	t.Setenv(semble.EnvEnableSemble, "true")
}

func configureIndexingActivationSembleReady(t *testing.T) {
	t.Helper()

	binDir := t.TempDir()
	semblePath := filepath.Join(binDir, "semble")
	if err := os.WriteFile(semblePath, []byte("#!/bin/sh\nprintf '%s\\n' '[{\"path\":\"prewarm.py\"}]'\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(fake semble): %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func prepareOptionalIndexTargetRoot(t *testing.T, targetRoot string) {
	t.Helper()

	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", targetRoot, err)
	}
	testhelpers.SetupTestGitRepo(t, targetRoot)
	writeIndexingActivationFile(t, filepath.Join(targetRoot, "go.mod"), "module example.com/indexing\n")
	testhelpers.MustGit(t, targetRoot, "add", "go.mod")
	testhelpers.MustGit(t, targetRoot, "commit", "-m", "Add go module")
	writeIndexingActivationFile(t, filepath.Join(targetRoot, "stacklit.json"), `{"project":{"name":"target"}}`)
	writeIndexingActivationFile(t, filepath.Join(targetRoot, ".liza", "scip", "go.scip"), "target go index")
	writeIndexingActivationFile(t, filepath.Join(targetRoot, ".sembleignore"), semble.DefaultIgnorePayload())
}

func buildIndexingActivationMASPrompt(t *testing.T, projectRoot, role, agentID, taskID, worktreeRel string) string {
	t.Helper()

	testhelpers.SetupPipelineConfig(t, projectRoot)
	state := &models.State{
		Goal: models.Goal{
			Description: "Indexing activation",
			SpecRef:     "specs/goals/20260602-indexing-activation.md",
		},
		Config: models.Config{
			IntegrationBranch: "main",
			ScipSearch:        []string{"go"},
		},
	}
	if taskID != "" {
		state.Tasks = []models.Task{
			{
				ID:          taskID,
				Description: "MAS optional-index prompt coverage",
				Status:      models.TaskStatusImplementing,
				DoneWhen:    "Optional index prompt sections follow target metadata",
				Scope:       "Integration",
				SpecRef:     "specs/goals/20260602-indexing-activation.md",
				RolePair:    "coding-pair",
				Worktree:    &worktreeRel,
			},
		}
	}

	strategy, err := agent.NewRoleStrategy(role, embeddedPipelineResolver(t))
	if err != nil {
		t.Fatalf("NewRoleStrategy(%q): %v", role, err)
	}
	prompt, err := strategy.BuildPrompt(state, agent.SupervisorConfig{
		Role:        role,
		AgentID:     agentID,
		ProjectRoot: projectRoot,
		SpecsDir:    filepath.Join(projectRoot, "specs"),
		StatePath:   filepath.Join(projectRoot, ".liza", "state.yaml"),
	}, taskID)
	if err != nil {
		t.Fatalf("BuildPrompt(%s): %v", role, err)
	}
	return prompt
}

func shellQuoteForIndexingActivationTest(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
