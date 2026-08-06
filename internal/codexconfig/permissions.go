package codexconfig

import (
	"path/filepath"
	"runtime"

	"github.com/liza-mas/liza/internal/brand"
)

// SupportWritableRoots returns host-level roots Codex commonly needs while
// running under workspace-write sandboxing. These are tool/cache roots, not
// target-project language assumptions.
func SupportWritableRoots(homeDir, cacheDir string) []string {
	var roots []string
	if homeDir != "" {
		values := brand.RuntimeValues()
		roots = append(roots, filepath.Join(homeDir, ".codex"))
		if values.NameLower != "" {
			roots = append(roots, filepath.Join(homeDir, "."+values.NameLower))
		}
		if values.GlobalDirName != "" {
			roots = append(roots, filepath.Join(homeDir, values.GlobalDirName))
		}
		roots = append(roots,
			filepath.Join(homeDir, ".npm"),
			filepath.Join(homeDir, ".pyenv", "shims"),
		)
		if cacheDir == "" && runtime.GOOS != "windows" {
			cacheDir = filepath.Join(homeDir, ".cache")
		}
	}
	if cacheDir != "" {
		roots = append(roots, cacheDir)
	}
	return UniqueNonEmptyStrings(roots)
}

func UniqueNonEmptyStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	var unique []string
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	return unique
}
