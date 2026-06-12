package commands

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/liza-mas/liza/internal/capsule"
)

func TestCapsuleReportCommandCreatesReportWithoutSlackConfig(t *testing.T) {
	projectRoot := t.TempDir()
	storeRoot := t.TempDir()
	paths := capsule.BuildPaths(storeRoot, projectRoot, "report-only")

	if _, err := capsule.Create(capsule.CreateOptions{
		Name:        "report-only",
		ProjectRoot: projectRoot,
		StoreRoot:   storeRoot,
	}); err != nil {
		t.Fatalf("create capsule: %v", err)
	}

	reportPath, err := CapsuleReportCommand(context.Background(), CapsuleReportParams{
		ProjectRoot: projectRoot,
		Name:        "report-only",
		StoreRoot:   storeRoot,
	})
	if err != nil {
		t.Fatalf("CapsuleReportCommand: %v", err)
	}
	if filepath.Dir(reportPath) != paths.Reports {
		t.Fatalf("report path = %q, want %q", reportPath, paths.Reports)
	}
}
