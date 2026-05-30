package codexconfig

import (
	"path/filepath"
	"reflect"
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
