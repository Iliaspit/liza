package codexconfig

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/liza-mas/liza/internal/brand"
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

func TestSupportWritableRootsIncludesNameAndGlobalNamespacesWhenDistinct(t *testing.T) {
	previousNameLower, previousGlobalDirName := brand.NameLower, brand.GlobalDirName
	brand.NameLower = "omni"
	brand.GlobalDirName = ".omni-ee"
	t.Cleanup(func() {
		brand.NameLower = previousNameLower
		brand.GlobalDirName = previousGlobalDirName
	})

	home := filepath.Join(string(filepath.Separator), "home", "user")
	got := SupportWritableRoots(home, filepath.Join(home, ".cache"))
	wantRoots := []string{
		filepath.Join(home, ".omni"),
		filepath.Join(home, ".omni-ee"),
	}
	for _, want := range wantRoots {
		if !containsString(got, want) {
			t.Errorf("SupportWritableRoots() missing %q: %#v", want, got)
		}
	}
}

func TestSupportWritableRootsDoesNotGrantHomeForEmptyName(t *testing.T) {
	previousNameLower := brand.NameLower
	brand.NameLower = ""
	t.Cleanup(func() {
		brand.NameLower = previousNameLower
	})

	home := filepath.Join(string(filepath.Separator), "home", "user")
	got := SupportWritableRoots(home, filepath.Join(home, ".cache"))
	if containsString(got, home) {
		t.Fatalf("SupportWritableRoots() granted the entire home directory: %#v", got)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
