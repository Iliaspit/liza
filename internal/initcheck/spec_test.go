package initcheck

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureSpecCommittedClean(t *testing.T) {
	repo := setupRepo(t)
	writeFile(t, repo, "specs/vision.md", "# Vision\n")
	git(t, repo, "add", "specs/vision.md")
	git(t, repo, "commit", "-m", "Add spec")

	got, err := EnsureSpecCommittedClean(repo, filepath.Join(repo, "specs", "vision.md"))
	if err != nil {
		t.Fatalf("EnsureSpecCommittedClean() error = %v", err)
	}
	if got != "specs/vision.md" {
		t.Fatalf("EnsureSpecCommittedClean() rel = %q, want specs/vision.md", got)
	}
}

func TestEnsureSpecCommittedCleanRejectsPendingSpec(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, repo string)
	}{
		{
			name: "untracked",
			setup: func(t *testing.T, repo string) {
				writeFile(t, repo, "specs/vision.md", "# Vision\n")
			},
		},
		{
			name: "staged new",
			setup: func(t *testing.T, repo string) {
				writeFile(t, repo, "specs/vision.md", "# Vision\n")
				git(t, repo, "add", "specs/vision.md")
			},
		},
		{
			name: "staged modification",
			setup: func(t *testing.T, repo string) {
				writeFile(t, repo, "specs/vision.md", "# Vision\n")
				git(t, repo, "add", "specs/vision.md")
				git(t, repo, "commit", "-m", "Add spec")
				writeFile(t, repo, "specs/vision.md", "# Changed\n")
				git(t, repo, "add", "specs/vision.md")
			},
		},
		{
			name: "unstaged modification",
			setup: func(t *testing.T, repo string) {
				writeFile(t, repo, "specs/vision.md", "# Vision\n")
				git(t, repo, "add", "specs/vision.md")
				git(t, repo, "commit", "-m", "Add spec")
				writeFile(t, repo, "specs/vision.md", "# Changed\n")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := setupRepo(t)
			tt.setup(t, repo)

			_, err := EnsureSpecCommittedClean(repo, "specs/vision.md")
			if err == nil {
				t.Fatal("EnsureSpecCommittedClean() succeeded, want error")
			}
			if !strings.Contains(err.Error(), "commit") {
				t.Fatalf("EnsureSpecCommittedClean() error = %v, want commit precondition", err)
			}
		})
	}
}

func TestEnsureSpecCommittedCleanRejectsOutsideRepo(t *testing.T) {
	repo := setupRepo(t)
	outside := filepath.Join(t.TempDir(), "vision.md")
	if err := os.WriteFile(outside, []byte("# Vision\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := EnsureSpecCommittedClean(repo, outside)
	if err == nil {
		t.Fatal("EnsureSpecCommittedClean() succeeded for outside-repo spec")
	}
	if !strings.Contains(err.Error(), "inside the git repository") {
		t.Fatalf("EnsureSpecCommittedClean() error = %v, want outside repo error", err)
	}
}

func TestEnsureSpecCommittedCleanRejectsSymlinkOutsideRepo(t *testing.T) {
	repo := setupRepo(t)
	outside := filepath.Join(t.TempDir(), "vision.md")
	if err := os.WriteFile(outside, []byte("# Vision\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(repo, "specs"), 0755); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(repo, "specs", "vision.md")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	git(t, repo, "add", "specs/vision.md")
	git(t, repo, "commit", "-m", "Add symlink spec")

	_, err := EnsureSpecCommittedClean(repo, "specs/vision.md")
	if err == nil {
		t.Fatal("EnsureSpecCommittedClean() succeeded for outside-repo symlink")
	}
	if !strings.Contains(err.Error(), "inside the git repository") {
		t.Fatalf("EnsureSpecCommittedClean() error = %v, want outside repo error", err)
	}
}

func setupRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	git(t, repo, "init", "-b", "main")
	git(t, repo, "config", "user.email", "test@example.com")
	git(t, repo, "config", "user.name", "Test User")
	writeFile(t, repo, "README.md", "# Test\n")
	git(t, repo, "add", "README.md")
	git(t, repo, "commit", "-m", "Initial commit")
	return repo
}

func writeFile(t *testing.T, repo, rel, content string) {
	t.Helper()

	path := filepath.Join(repo, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func git(t *testing.T, repo string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}
