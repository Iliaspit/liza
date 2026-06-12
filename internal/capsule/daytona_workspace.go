package capsule

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const DaytonaWorkspaceDir = "/workspace"

func EnsureProjectLizaSeed(projectRoot string, meta *CapsuleMetadata) error {
	if meta == nil {
		return fmt.Errorf("capsule metadata is required")
	}
	if hasSyncableFiles(meta.Paths.ProjectLiza) {
		return nil
	}
	source := filepath.Join(projectRoot, ".liza")
	if !hasSyncableFiles(source) {
		return fmt.Errorf("project %s has no syncable .liza state", projectRoot)
	}
	return copyFilteredDir(source, meta.Paths.ProjectLiza)
}

func BuildProjectLizaArchive(projectLizaDir string) ([]byte, error) {
	var raw bytes.Buffer
	gzipWriter := gzip.NewWriter(&raw)
	tarWriter := tar.NewWriter(gzipWriter)
	err := filepath.WalkDir(projectLizaDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if shouldExcludeCapsuleSyncPath(path) {
			return nil
		}
		rel, err := filepath.Rel(projectLizaDir, path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(filepath.Join(".liza", rel))
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(tarWriter, file)
		return err
	})
	if err != nil {
		_ = tarWriter.Close()
		_ = gzipWriter.Close()
		return nil, err
	}
	if err := tarWriter.Close(); err != nil {
		return nil, err
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, err
	}
	return raw.Bytes(), nil
}

func hasSyncableFiles(root string) bool {
	found := false
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !shouldExcludeCapsuleSyncPath(path) {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

func copyFilteredDir(source, target string) error {
	return filepath.WalkDir(source, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(target, 0755)
		}
		destination := filepath.Join(target, rel)
		if d.IsDir() {
			return os.MkdirAll(destination, 0755)
		}
		if shouldExcludeCapsuleSyncPath(path) {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			return err
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		defer input.Close()
		output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		if _, err := io.Copy(output, input); err != nil {
			_ = output.Close()
			return err
		}
		return output.Close()
	})
}

func shouldExcludeCapsuleSyncPath(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	return name == ".ds_store" ||
		strings.HasPrefix(name, ".env") ||
		strings.Contains(name, "secret") ||
		strings.Contains(name, "token") ||
		strings.Contains(name, "credential") ||
		strings.Contains(name, "auth") ||
		strings.HasSuffix(name, ".lock") ||
		strings.Contains(name, ".lock.")
}

func Base64Chunks(data []byte, chunkSize int) []string {
	if chunkSize <= 0 {
		chunkSize = 32000
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	chunks := make([]string, 0, (len(encoded)+chunkSize-1)/chunkSize)
	for len(encoded) > 0 {
		n := chunkSize
		if len(encoded) < n {
			n = len(encoded)
		}
		chunks = append(chunks, encoded[:n])
		encoded = encoded[n:]
	}
	return chunks
}
