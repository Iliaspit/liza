package capsule

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureProjectLizaSeedCopiesFilteredState(t *testing.T) {
	projectRoot := t.TempDir()
	source := filepath.Join(projectRoot, ".liza")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "state.yaml"), []byte("tasks: []\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "state.yaml.lock"), []byte("lock"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "auth-token"), []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	meta := &CapsuleMetadata{Paths: CapsulePaths{ProjectLiza: filepath.Join(t.TempDir(), "project-liza")}}

	if err := EnsureProjectLizaSeed(projectRoot, meta); err != nil {
		t.Fatalf("EnsureProjectLizaSeed failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(meta.Paths.ProjectLiza, "state.yaml")); err != nil {
		t.Fatalf("state.yaml not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(meta.Paths.ProjectLiza, "state.yaml.lock")); !os.IsNotExist(err) {
		t.Fatalf("lock file copied or stat failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(meta.Paths.ProjectLiza, "auth-token")); !os.IsNotExist(err) {
		t.Fatalf("auth token copied or stat failed: %v", err)
	}
}

func TestEnsureProjectLizaSeedSkipsSensitiveDirectories(t *testing.T) {
	// Regression test for blocker: ensure sensitive directories are skipped
	// before descending in copyFilteredDir
	projectRoot := t.TempDir()
	source := filepath.Join(projectRoot, ".liza")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	
	// Create sensitive directories with files
	secretsDir := filepath.Join(source, "secrets")
	if err := os.MkdirAll(secretsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secretsDir, "config.yaml"), []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	
	authDir := filepath.Join(source, "auth")
	if err := os.MkdirAll(authDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(authDir, "session.json"), []byte("token"), 0644); err != nil {
		t.Fatal(err)
	}
	
	credentialsDir := filepath.Join(source, "credentials")
	if err := os.MkdirAll(credentialsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(credentialsDir, "config.json"), []byte("creds"), 0644); err != nil {
		t.Fatal(err)
	}
	
	// Create a safe file
	if err := os.WriteFile(filepath.Join(source, "state.yaml"), []byte("tasks: []\n"), 0644); err != nil {
		t.Fatal(err)
	}
	
	meta := &CapsuleMetadata{Paths: CapsulePaths{ProjectLiza: filepath.Join(t.TempDir(), "project-liza")}}

	if err := EnsureProjectLizaSeed(projectRoot, meta); err != nil {
		t.Fatalf("EnsureProjectLizaSeed failed: %v", err)
	}
	
	// Safe file should be copied
	if _, err := os.Stat(filepath.Join(meta.Paths.ProjectLiza, "state.yaml")); err != nil {
		t.Fatalf("state.yaml not copied: %v", err)
	}
	
	// Sensitive directory files should not be copied
	if _, err := os.Stat(filepath.Join(meta.Paths.ProjectLiza, "secrets", "config.yaml")); !os.IsNotExist(err) {
		t.Fatalf("secrets/config.yaml was copied")
	}
	if _, err := os.Stat(filepath.Join(meta.Paths.ProjectLiza, "auth", "session.json")); !os.IsNotExist(err) {
		t.Fatalf("auth/session.json was copied")
	}
	if _, err := os.Stat(filepath.Join(meta.Paths.ProjectLiza, "credentials", "config.json")); !os.IsNotExist(err) {
		t.Fatalf("credentials/config.json was copied")
	}
}

func TestBuildProjectLizaArchiveExcludesLocksAndSecrets(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "state.yaml"), []byte("tasks: []\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "state.yaml.lock"), []byte("lock"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "secret.env"), []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}

	archive, err := BuildProjectLizaArchive(root)
	if err != nil {
		t.Fatalf("BuildProjectLizaArchive failed: %v", err)
	}
	entries := tarEntries(t, archive)
	if !entries[".liza/state.yaml"] {
		t.Fatalf("archive entries = %#v, want .liza/state.yaml", entries)
	}
	if entries[".liza/state.yaml.lock"] || entries[".liza/secret.env"] {
		t.Fatalf("archive included excluded entries: %#v", entries)
	}
}

func TestBuildProjectLizaArchiveExcludesSensitiveDirectories(t *testing.T) {
	// Regression test for blocker: ensure sensitive directories are skipped
	// before descending, not just by filename
	root := t.TempDir()
	
	// Create sensitive directories with files that should be excluded
	secretsDir := filepath.Join(root, "secrets")
	if err := os.MkdirAll(secretsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secretsDir, "config.yaml"), []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	
	authDir := filepath.Join(root, "auth")
	if err := os.MkdirAll(authDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(authDir, "session.json"), []byte("token"), 0644); err != nil {
		t.Fatal(err)
	}
	
	credentialsDir := filepath.Join(root, "credentials")
	if err := os.MkdirAll(credentialsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(credentialsDir, "config.json"), []byte("creds"), 0644); err != nil {
		t.Fatal(err)
	}
	
	// Create a safe file
	if err := os.WriteFile(filepath.Join(root, "state.yaml"), []byte("tasks: []\n"), 0644); err != nil {
		t.Fatal(err)
	}

	archive, err := BuildProjectLizaArchive(root)
	if err != nil {
		t.Fatalf("BuildProjectLizaArchive failed: %v", err)
	}
	entries := tarEntries(t, archive)
	
	// Safe file should be included
	if !entries[".liza/state.yaml"] {
		t.Fatalf("archive entries = %#v, want .liza/state.yaml", entries)
	}
	
	// Sensitive directory files should be excluded
	if entries[".liza/secrets/config.yaml"] {
		t.Fatalf("archive included .liza/secrets/config.yaml")
	}
	if entries[".liza/auth/session.json"] {
		t.Fatalf("archive included .liza/auth/session.json")
	}
	if entries[".liza/credentials/config.json"] {
		t.Fatalf("archive included .liza/credentials/config.json")
	}
}

func tarEntries(t *testing.T, data []byte) map[string]bool {
	t.Helper()
	reader, err := gzip.NewReader(bytesReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	tarReader := tar.NewReader(reader)
	entries := map[string]bool{}
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		entries[header.Name] = true
	}
	return entries
}

func bytesReader(data []byte) *bytes.Reader {
	return bytes.NewReader(data)
}
