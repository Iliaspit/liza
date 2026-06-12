package capsule

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
)

type StartOptions struct {
	Command     []string
	Interactive bool
	Stat        func(string) error
}

func BuildStartCommand(meta *CapsuleMetadata, opts StartOptions) (*exec.Cmd, error) {
	if meta.Runtime != RuntimeDocker && meta.Runtime != RuntimePodman {
		return nil, fmt.Errorf("runtime %q is not supported for capsule start; native capsules are reserved for a future backend", meta.Runtime)
	}
	if len(opts.Command) == 0 {
		opts.Command = []string{"liza", "tui"}
	}
	if opts.Stat == nil {
		opts.Stat = func(path string) error {
			_, err := os.Stat(path)
			return err
		}
	}

	args := []string{
		"run", "--rm",
		"--name", ContainerName(meta.Name),
		"-w", "/workspace",
		"-v", meta.ProjectRoot + ":/workspace",
		"-v", meta.Paths.ProjectLiza + ":/workspace/.liza",
		"-v", meta.Paths.HomeLiza + ":/home/liza/.liza",
		"-v", meta.Paths.OpenCodeConfig + ":/home/liza/.config/opencode",
		"-v", meta.Paths.OpenCodeData + ":/home/liza/.local/share/opencode",
		"-v", meta.Paths.Cache + ":/home/liza/.cache",
	}
	if opts.Interactive {
		args = append(args, "-it")
	}
	// Sort env keys for deterministic ordering
	envKeys := make([]string, 0, len(meta.Env))
	for key := range meta.Env {
		envKeys = append(envKeys, key)
	}
	sort.Strings(envKeys)
	for _, key := range envKeys {
		args = append(args, "-e", key+"="+meta.Env[key])
	}
	for _, record := range meta.Tools {
		for _, mount := range record.AuthMounts {
			if opts.Stat(mount.Source) != nil {
				continue
			}
			spec := mount.Source + ":" + mount.Target
			if mount.ReadOnly {
				spec += ":ro"
			}
			args = append(args, "-v", spec)
		}
	}
	if opts.Stat(meta.Paths.SecretsEnv) == nil {
		args = append(args, "--env-file", meta.Paths.SecretsEnv)
	}
	args = append(args, meta.Image)
	args = append(args, opts.Command...)

	return exec.Command(string(meta.Runtime), args...), nil
}
