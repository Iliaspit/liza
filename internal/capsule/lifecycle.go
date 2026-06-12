package capsule

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func List(storeRoot, projectRoot string) ([]CapsuleMetadata, error) {
	root := filepath.Join(storeRoot, RepoFingerprint(projectRoot))
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list capsules: %w", err)
	}
	var capsules []CapsuleMetadata
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		meta, err := LoadMetadata(storeRoot, projectRoot, entry.Name())
		if err != nil {
			continue
		}
		capsules = append(capsules, *meta)
	}
	sort.Slice(capsules, func(i, j int) bool {
		return capsules[i].Name < capsules[j].Name
	})
	return capsules, nil
}

func Delete(storeRoot, projectRoot, name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	paths := BuildPaths(storeRoot, projectRoot, name)
	if paths.Root == "" || paths.Root == "/" {
		return fmt.Errorf("refusing to delete invalid capsule root %q", paths.Root)
	}
	if err := os.RemoveAll(paths.Root); err != nil {
		return fmt.Errorf("failed to delete capsule %q: %w", name, err)
	}
	return nil
}
