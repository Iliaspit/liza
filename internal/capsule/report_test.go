package capsule

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateReportExcludesGeneratedReportsSecretsAndSymlinks(t *testing.T) {
	root := t.TempDir()
	projectLiza := filepath.Join(root, "project-liza")
	reports := filepath.Join(root, "reports")
	if err := os.MkdirAll(filepath.Join(projectLiza, "reports"), 0o755); err != nil {
		t.Fatalf("mkdir nested reports: %v", err)
	}
	if err := os.MkdirAll(reports, 0o755); err != nil {
		t.Fatalf("mkdir reports: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectLiza, "state.yaml"), []byte("ok: true\n"), 0o644); err != nil {
		t.Fatalf("write state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectLiza, ".env"), []byte("SECRET=leak\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectLiza, "reports", "old.zip"), []byte("old report"), 0o644); err != nil {
		t.Fatalf("write old report: %v", err)
	}
	outsideSecret := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outsideSecret, []byte("outside secret"), 0o600); err != nil {
		t.Fatalf("write outside secret: %v", err)
	}
	if err := os.Symlink(outsideSecret, filepath.Join(projectLiza, "linked-state.txt")); err != nil {
		t.Fatalf("symlink outside secret: %v", err)
	}

	reportPath, err := CreateReport(&CapsuleMetadata{
		Name: "report-filter",
		Env:  map[string]string{"SLACK_BOT_TOKEN": "secret", "VISIBLE": "ok"},
		Paths: CapsulePaths{
			ProjectLiza: projectLiza,
			Reports:     reports,
		},
	}, ReportOptions{
		Now: func() time.Time { return time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("CreateReport: %v", err)
	}

	entries := reportZipEntries(t, reportPath)
	for _, unwanted := range []string{
		"project-liza/.env",
		"project-liza/reports/old.zip",
		"project-liza/linked-state.txt",
	} {
		if entries[unwanted] {
			t.Fatalf("report included %s", unwanted)
		}
	}
	if !entries["project-liza/state.yaml"] {
		t.Fatal("report missing project-liza/state.yaml")
	}
}

func reportZipEntries(t *testing.T, reportPath string) map[string]bool {
	t.Helper()
	zr, err := zip.OpenReader(reportPath)
	if err != nil {
		t.Fatalf("open report zip: %v", err)
	}
	defer zr.Close()

	entries := map[string]bool{}
	for _, file := range zr.File {
		entries[file.Name] = true
	}
	return entries
}
