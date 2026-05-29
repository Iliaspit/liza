package codexconfig

import (
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestSupportWritableRoots(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "home", "user")
	cache := filepath.Join(home, ".cache")

	got := SupportWritableRoots(home, cache)
	want := []string{
		filepath.Join(home, ".codex"),
		filepath.Join(home, ".liza"),
		filepath.Join(home, ".npm"),
		filepath.Join(home, ".pyenv", "shims"),
		cache,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SupportWritableRoots() = %#v, want %#v", got, want)
	}
}

func TestSupportReadableRoots(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "home", "user")

	got := SupportReadableRoots(home)
	if got != nil {
		t.Fatalf("SupportReadableRoots() = %#v, want nil", got)
	}
}

func TestWorkspaceFilesystemInlineTableIncludesSupportRoots(t *testing.T) {
	cache := filepath.Join(string(filepath.Separator), "home", "user", ".cache")
	lizaRoot := filepath.Join(string(filepath.Separator), "home", "user", ".liza")
	got := WorkspaceFilesystemInlineTable([]string{lizaRoot}, []string{cache})

	for _, want := range []string{
		`":root"="read"`,
		`":tmpdir"="write"`,
		tomlQuotedKey(lizaRoot) + `="read"`,
		tomlQuotedKey(cache) + `="write"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("WorkspaceFilesystemInlineTable() missing %q:\n%s", want, got)
		}
	}
	if runtime.GOOS != "windows" && !strings.Contains(got, `"/tmp"="write"`) {
		t.Fatalf("WorkspaceFilesystemInlineTable() missing /tmp write:\n%s", got)
	}
}
