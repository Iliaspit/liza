package capsule

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

type DoctorOptions struct {
	Tool      ToolName
	Now       func() time.Time
	LookPath  func(string) (string, error)
	Stat      func(string) error
	LookupEnv func(string) (string, bool)
}

func Doctor(meta *CapsuleMetadata, opts DoctorOptions) DoctorSummary {
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now().UTC() }
	}
	if opts.LookPath == nil {
		opts.LookPath = func(name string) (string, error) {
			_, err := exec.LookPath(name)
			return name, err
		}
	}
	if opts.Stat == nil {
		opts.Stat = func(path string) error {
			_, err := os.Stat(path)
			return err
		}
	}
	if opts.LookupEnv == nil {
		opts.LookupEnv = os.LookupEnv
	}

	summary := DoctorSummary{CheckedAt: opts.Now(), OK: true}
	summary.add("runtime", meta.Runtime == RuntimeDocker || meta.Runtime == RuntimePodman || meta.Runtime == RuntimeDaytona, fmt.Sprintf("runtime=%s guest=%s/%s", meta.Runtime, meta.Guest.OS, meta.Guest.Arch))
	if meta.Runtime == RuntimeDocker || meta.Runtime == RuntimePodman {
		_, err := opts.LookPath(string(meta.Runtime))
		summary.add("runtime binary", err == nil, fmt.Sprintf("%s on PATH", meta.Runtime))
	}
	if meta.Runtime == RuntimeDaytona {
		if meta.Daytona == nil {
			summary.add("Daytona config", false, "missing Daytona metadata")
			return summary
		}
		apiKeyEnv := meta.Daytona.APIKeyEnv
		if apiKeyEnv == "" {
			apiKeyEnv = DefaultDaytonaAPIKeyEnv
		}
		_, ok := opts.LookupEnv(apiKeyEnv)
		summary.add("Daytona API key", ok, apiKeyEnv+" must be set in the host environment")
		message := meta.Daytona.SandboxID
		if message == "" {
			message = "sandbox has not been provisioned"
		}
		summary.add("Daytona sandbox", meta.Daytona.SandboxID != "", message)
	}
	summary.add("virtual .liza", opts.Stat(meta.Paths.ProjectLiza) == nil, meta.Paths.ProjectLiza)
	summary.add("OpenCode config", opts.Stat(meta.Paths.OpenCodeConfig) == nil, meta.Paths.OpenCodeConfig)

	for name, record := range meta.Tools {
		if opts.Tool != "" && opts.Tool != "all" && opts.Tool != name {
			continue
		}
		summary.add(string(name)+" supported", record.Supported, stringsOrDefault(record.Notes, "supported"))
		for _, mount := range record.AuthMounts {
			err := opts.Stat(mount.Source)
			if mount.Required {
				summary.add(string(name)+" auth mount", err == nil, mount.Source)
			} else if err == nil {
				summary.add(string(name)+" auth mount", true, mount.Source+" will be mounted read-only")
			} else {
				summary.add(string(name)+" auth fallback", true, mount.Source+" missing; use "+stringsOrDefault(record.EnvFallbacks, "API key env")+" instead")
			}
		}
	}
	return summary
}

func (s *DoctorSummary) add(name string, ok bool, message string) {
	s.Checks = append(s.Checks, DoctorCheck{Name: name, OK: ok, Message: message})
	if !ok {
		s.OK = false
	}
}

func stringsOrDefault(values []string, fallback string) string {
	if len(values) == 0 {
		return fallback
	}
	return values[0]
}
