package testhelpers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/liza-mas/liza/internal/embedded"
	"github.com/liza-mas/liza/internal/paths"
)

// SetupPipelineConfig writes the embedded pipeline.yaml into the active project
// runtime directory under tmpDir, creating that directory when needed.
func SetupPipelineConfig(t *testing.T, tmpDir string) {
	t.Helper()

	projectDir := filepath.Join(tmpDir, paths.ProjectDirName())
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project runtime directory %s: %v", paths.ProjectDirName(), err)
	}
	if err := embedded.WritePipelineConfig(projectDir, nil); err != nil {
		t.Fatalf("Failed to write pipeline config: %v", err)
	}
}

// SetupPipelineConfigBytes writes the supplied pipeline config into the active
// project runtime directory. It is useful for exercising frozen legacy topologies.
func SetupPipelineConfigBytes(t *testing.T, tmpDir string, content []byte) {
	t.Helper()

	projectDir := filepath.Join(tmpDir, paths.ProjectDirName())
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("Failed to create project runtime directory %s: %v", paths.ProjectDirName(), err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "pipeline.yaml"), content, 0o644); err != nil {
		t.Fatalf("Failed to write pipeline config: %v", err)
	}
}
