package capsule

import (
	"os/exec"
	"testing"
)

func TestBuildStartCommandOnlyAddsTTYWhenInteractive(t *testing.T) {
	meta := startCommandMetadata()

	nonInteractive, err := BuildStartCommand(meta, StartOptions{Command: []string{"true"}})
	if err != nil {
		t.Fatalf("BuildStartCommand() error = %v", err)
	}
	if hasArg(nonInteractive, "-it") {
		t.Fatalf("non-interactive command should not include -it: %#v", nonInteractive.Args)
	}

	interactive, err := BuildStartCommand(meta, StartOptions{Command: []string{"true"}, Interactive: true})
	if err != nil {
		t.Fatalf("BuildStartCommand() error = %v", err)
	}
	if !hasArg(interactive, "-it") {
		t.Fatalf("interactive command should include -it: %#v", interactive.Args)
	}
}

func startCommandMetadata() *CapsuleMetadata {
	return &CapsuleMetadata{
		Name:        "smoke",
		Runtime:     RuntimeDocker,
		ProjectRoot: "/repo",
		Image:       "liza-capsule:test",
		Paths: CapsulePaths{
			ProjectLiza:    "/capsule/.liza",
			HomeLiza:       "/capsule/home-liza",
			OpenCodeConfig: "/capsule/opencode-config",
			OpenCodeData:   "/capsule/opencode-data",
			Cache:          "/capsule/cache",
			SecretsEnv:     "/capsule/secrets.env",
		},
	}
}

func hasArg(cmd *exec.Cmd, want string) bool {
	for _, arg := range cmd.Args {
		if arg == want {
			return true
		}
	}
	return false
}
