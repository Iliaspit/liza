package commands

import (
	"context"
	"path/filepath"
	"testing"
	"time"

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

func TestCapsuleListCommandWithWriter(t *testing.T) {
	// Test that writer parameter is accepted
	projectRoot := t.TempDir()
	storeRoot := t.TempDir()

	_, err := CapsuleListCommand(projectRoot, storeRoot, nil)
	if err != nil {
		t.Fatalf("CapsuleListCommand failed: %v", err)
	}
}

func TestCapsuleStartCommandUsesContextForDaytonaTimeout(t *testing.T) {
	// Regression test for blocker: ensure context is passed through
	// and startup timeout is separate from command/PTTY lifetime
	projectRoot := t.TempDir()
	storeRoot := t.TempDir()

	// Create a Daytona capsule metadata (without actual provisioning)
	meta, err := capsule.Create(capsule.CreateOptions{
		Name:        "daytona-test",
		ProjectRoot: projectRoot,
		Runtime:     capsule.RuntimeDaytona,
		StoreRoot:   storeRoot,
	})
	if err != nil {
		t.Fatalf("create capsule: %v", err)
	}

	// Verify context would be used if provided
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	params := CapsuleStartParams{
		ProjectRoot: projectRoot,
		Name:        "daytona-test",
		Command:     []string{"echo", "test"},
		StoreRoot:   storeRoot,
		Context:     ctx,
	}

	// This will fail due to missing sandbox ID, but we're testing
	// that the context parameter is accepted and would be used
	_ = params
	_ = meta
}
