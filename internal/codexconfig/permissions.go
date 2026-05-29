package codexconfig

import (
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// SupportWritableRoots returns host-level roots Codex commonly needs while
// running under workspace-write sandboxing. These are tool/cache roots, not
// target-project language assumptions.
func SupportWritableRoots(homeDir, cacheDir string) []string {
	var roots []string
	if homeDir != "" {
		roots = append(roots,
			filepath.Join(homeDir, ".codex"),
			filepath.Join(homeDir, ".liza"),
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

// SupportReadableRoots returns host-level roots Codex agents need to inspect
// without being allowed to mutate them. The current baseline treats Liza support
// files as writable through sandbox_workspace_write, so there are no global
// read-only support roots.
func SupportReadableRoots(homeDir string) []string {
	return nil
}

// WorkspaceFilesystemInlineTable renders the Codex permissions.workspace
// filesystem profile used by Liza-spawned noninteractive Codex agents.
func WorkspaceFilesystemInlineTable(readRoots, writeRoots []string) string {
	entries := []string{
		`":root"="read"`,
		`":tmpdir"="write"`,
	}
	if runtime.GOOS != "windows" {
		entries = append(entries, `"/tmp"="write"`)
	}
	for _, root := range UniqueNonEmptyStrings(readRoots) {
		entries = append(entries, tomlQuotedKey(root)+`="read"`)
	}
	for _, root := range UniqueNonEmptyStrings(writeRoots) {
		entries = append(entries, tomlQuotedKey(root)+`="write"`)
	}
	return "{" + strings.Join(entries, ", ") + "}"
}

func RenderWorkspaceFilesystemSupportAssignments(readRoots, writeRoots []string) string {
	var lines []string
	for _, root := range UniqueNonEmptyStrings(readRoots) {
		lines = append(lines, tomlQuotedKey(root)+` = "read"`)
	}
	for _, root := range UniqueNonEmptyStrings(writeRoots) {
		lines = append(lines, tomlQuotedKey(root)+` = "write"`)
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
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

func tomlQuotedKey(value string) string {
	return strconv.Quote(value)
}
