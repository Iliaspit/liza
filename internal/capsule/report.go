package capsule

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ReportOptions struct {
	Now        func() time.Time
	AuthorName string
}

func CreateReport(meta *CapsuleMetadata, opts ReportOptions) (string, error) {
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now().UTC() }
	}
	if meta == nil {
		return "", fmt.Errorf("report metadata is nil")
	}
	if meta.Paths.Reports == "" {
		return "", fmt.Errorf("report output path not set")
	}
	if err := os.MkdirAll(meta.Paths.Reports, 0755); err != nil {
		return "", fmt.Errorf("failed to create reports directory: %w", err)
	}
	reportPath := filepath.Join(meta.Paths.Reports, reportFileName(meta.Name, opts.AuthorName, opts.Now()))
	file, err := os.Create(reportPath)
	if err != nil {
		return "", fmt.Errorf("failed to create report zip: %w", err)
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	defer zw.Close()

	if err := addManifest(zw, meta); err != nil {
		return "", err
	}
	for _, item := range []struct {
		prefix string
		path   string
	}{
		{"project-liza", meta.Paths.ProjectLiza},
		{"opencode-config", meta.Paths.OpenCodeConfig},
	} {
		if err := addPathToZip(zw, item.prefix, item.path); err != nil {
			return "", err
		}
	}
	return reportPath, nil
}

func reportFileName(capsuleName, authorName string, now time.Time) string {
	authorSlug := filenameSlug(authorName)
	if authorSlug == "" {
		authorSlug = "unknown-author"
	}
	return fmt.Sprintf("%s-%s-%s.zip", filenameSlug(capsuleName), authorSlug, now.Format("20060102-150405"))
}

func filenameSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func addManifest(zw *zip.Writer, meta *CapsuleMetadata) error {
	redacted := *meta
	redacted.Env = map[string]string{}
	for k, v := range meta.Env {
		if isSecretName(k) {
			redacted.Env[k] = "***REDACTED***"
		} else {
			redacted.Env[k] = v
		}
	}
	data, err := json.MarshalIndent(redacted, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report manifest: %w", err)
	}
	w, err := zw.Create("manifest.json")
	if err != nil {
		return fmt.Errorf("failed to add report manifest: %w", err)
	}
	_, err = w.Write(append(data, '\n'))
	return err
}

func addPathToZip(zw *zip.Writer, prefix, root string) error {
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil
	}
	return filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if shouldExcludeReportPath(path) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		w, err := zw.Create(filepath.ToSlash(filepath.Join(prefix, rel)))
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(w, f)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func shouldExcludeReportPath(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	return name == "node_modules" || name == "reports" || isSecretName(name) || strings.Contains(name, "auth") || strings.Contains(name, "token")
}

func isSecretName(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "secret") ||
		strings.Contains(lower, "token") ||
		strings.Contains(lower, "api_key") ||
		strings.Contains(lower, "apikey") ||
		strings.HasPrefix(lower, ".env") ||
		strings.Contains(lower, "credential")
}
