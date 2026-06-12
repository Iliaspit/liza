package capsule

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var capsuleNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

func ValidateName(name string) error {
	if !capsuleNamePattern.MatchString(name) {
		return fmt.Errorf("invalid capsule name %q: use 1-64 letters, numbers, dot, underscore, or hyphen; start with a letter or number", name)
	}
	return nil
}

func DefaultStoreRoot() (string, error) {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "liza", "capsules"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "liza", "capsules"), nil
}

func RepoFingerprint(projectRoot string) string {
	abs, err := filepath.Abs(projectRoot)
	if err == nil {
		projectRoot = abs
	}
	sum := sha256.Sum256([]byte(filepath.Clean(projectRoot)))
	return hex.EncodeToString(sum[:])[:16]
}

func BuildPaths(storeRoot, projectRoot, name string) CapsulePaths {
	fp := RepoFingerprint(projectRoot)
	root := filepath.Join(storeRoot, fp, name)
	return CapsulePaths{
		Root:           root,
		Metadata:       filepath.Join(root, "capsule.yaml"),
		ProjectLiza:    filepath.Join(root, "project-liza"),
		HomeLiza:       filepath.Join(root, "home-liza"),
		OpenCodeConfig: filepath.Join(root, "opencode-config"),
		OpenCodeData:   filepath.Join(root, "opencode-data"),
		Cache:          filepath.Join(root, "cache"),
		Reports:        filepath.Join(root, "reports"),
		SecretsEnv:     filepath.Join(root, "secrets.env"),
		SecretsExample: filepath.Join(root, "secrets.env.example"),
	}
}

func ContainerName(name string) string {
	cleaned := strings.NewReplacer(".", "-", "_", "-").Replace(strings.ToLower(name))
	return "liza-capsule-" + cleaned
}
