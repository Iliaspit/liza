package capsule

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type CreateOptions struct {
	Name        string
	ProjectRoot string
	Runtime     RuntimeMode
	Preset      string
	Image       string
	StoreRoot   string
	HomeDir     string
	Host        Platform
	OpenCode    *OpenCodePreset
	Daytona     DaytonaCreateOptions
	Now         func() time.Time
	Force       bool
}

func Create(opts CreateOptions) (*CapsuleMetadata, error) {
	if err := ValidateName(opts.Name); err != nil {
		return nil, err
	}
	if opts.Runtime == "" {
		opts.Runtime = RuntimeDocker
	}
	if opts.Runtime != RuntimeDocker && opts.Runtime != RuntimePodman && opts.Runtime != RuntimeDaytona {
		return nil, fmt.Errorf("unsupported capsule runtime %q: supported runtimes are docker, podman, or daytona", opts.Runtime)
	}
	if opts.Image == "" {
		opts.Image = defaultImage(opts.Runtime)
	}
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now().UTC() }
	}
	if opts.Host.OS == "" {
		opts.Host = DetectHostPlatform()
	}
	if opts.HomeDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		opts.HomeDir = home
	}
	if opts.StoreRoot == "" {
		root, err := DefaultStoreRoot()
		if err != nil {
			return nil, err
		}
		opts.StoreRoot = root
	}
	projectRoot, err := filepath.Abs(opts.ProjectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve project root: %w", err)
	}

	var preset OpenCodePreset
	if opts.OpenCode != nil {
		preset = *opts.OpenCode
	} else {
		var err error
		preset, err = DefaultOpenCodePreset(opts.Preset)
		if err != nil {
			return nil, err
		}
	}
	paths := BuildPaths(opts.StoreRoot, projectRoot, opts.Name)
	// Check if capsule already exists
	if _, err := os.Stat(paths.Metadata); err == nil {
		if !opts.Force {
			return nil, fmt.Errorf("capsule %q already exists; use --force to overwrite", opts.Name)
		}
		// Force mode: remove existing capsule
		if err := os.RemoveAll(paths.ProjectLiza); err != nil {
			return nil, fmt.Errorf("failed to remove existing capsule %q: %w", opts.Name, err)
		}
		if err := os.RemoveAll(paths.HomeLiza); err != nil {
			return nil, fmt.Errorf("failed to remove existing capsule home %q: %w", opts.Name, err)
		}
		if err := os.RemoveAll(paths.OpenCodeConfig); err != nil {
			return nil, fmt.Errorf("failed to remove existing capsule config %q: %w", opts.Name, err)
		}
		if err := os.RemoveAll(paths.OpenCodeData); err != nil {
			return nil, fmt.Errorf("failed to remove existing capsule data %q: %w", opts.Name, err)
		}
		if err := os.RemoveAll(paths.Cache); err != nil {
			return nil, fmt.Errorf("failed to remove existing capsule cache %q: %w", opts.Name, err)
		}
		if err := os.RemoveAll(paths.Reports); err != nil {
			return nil, fmt.Errorf("failed to remove existing capsule reports %q: %w", opts.Name, err)
		}
	}
	for _, dir := range []string{paths.ProjectLiza, paths.HomeLiza, paths.OpenCodeConfig, paths.OpenCodeData, paths.Cache, paths.Reports} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create capsule directory %s: %w", dir, err)
		}
	}
	if _, err := WriteOpenCodeConfig(paths.OpenCodeConfig, preset); err != nil {
		return nil, err
	}
	if err := writeSecretsExample(paths.SecretsExample, preset.APIKeyEnv); err != nil {
		return nil, err
	}

	var daytona *DaytonaMetadata
	if opts.Runtime == RuntimeDaytona {
		metadata := BuildDaytonaMetadata(opts.Name, opts.Daytona)
		daytona = &metadata
	}

	meta := &CapsuleMetadata{
		Version:         1,
		Name:            opts.Name,
		ProjectRoot:     projectRoot,
		RepoFingerprint: RepoFingerprint(projectRoot),
		CreatedAt:       opts.Now(),
		Runtime:         opts.Runtime,
		Image:           opts.Image,
		Host:            opts.Host,
		Guest:           GuestPlatform(opts.Host, opts.Runtime),
		Paths:           paths,
		Tools:           ResolveAllToolchains(opts.Host, opts.Runtime, opts.HomeDir),
		OpenCode:        preset,
		Daytona:         daytona,
		Env: map[string]string{
			"OPENCODE_CONFIG":     "/home/liza/.config/opencode/opencode.json",
			"OPENCODE_CONFIG_DIR": "/home/liza/.config/opencode",
			"OPENCODE_DATA_DIR":   "/home/liza/.local/share/opencode",
		},
	}
	if err := SaveMetadata(meta); err != nil {
		return nil, err
	}
	return meta, nil
}

func defaultImage(mode RuntimeMode) string {
	switch mode {
	case RuntimePodman:
		return "localhost/liza-capsule:latest"
	default:
		return "liza-capsule:latest"
	}
}

func writeSecretsExample(path, apiKeyEnv string) error {
	content := "# Capsule provider secrets. Copy to secrets.env and fill locally; this file is safe to commit nowhere.\n" + apiKeyEnv + "=\nOPENAI_API_KEY=\nANTHROPIC_API_KEY=\nSLACK_BOT_TOKEN=\n"
	return os.WriteFile(path, []byte(content), 0600)
}

func SaveMetadata(meta *CapsuleMetadata) error {
	data, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal capsule metadata: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(meta.Paths.Metadata), 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}
	if err := os.WriteFile(meta.Paths.Metadata, data, 0644); err != nil {
		return fmt.Errorf("failed to write capsule metadata: %w", err)
	}
	return nil
}

func LoadMetadata(storeRoot, projectRoot, name string) (*CapsuleMetadata, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	paths := BuildPaths(storeRoot, projectRoot, name)
	data, err := os.ReadFile(paths.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to read capsule metadata for %q: %w", name, err)
	}
	var meta CapsuleMetadata
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse capsule metadata for %q: %w", name, err)
	}
	return &meta, nil
}
