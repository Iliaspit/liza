package capsule

import (
	"os"
	"runtime"
	"strings"
)

func DetectHostPlatform() Platform {
	return Platform{
		OS:   runtime.GOOS,
		Arch: normalizeArch(runtime.GOARCH),
		WSL:  detectWSL(),
	}
}

func GuestPlatform(host Platform, mode RuntimeMode) Platform {
	if mode == RuntimeDocker || mode == RuntimePodman {
		return Platform{OS: "linux", Arch: normalizeArch(host.Arch)}
	}
	if mode == RuntimeDaytona {
		return Platform{OS: "linux", Arch: "amd64"}
	}
	return host
}

func normalizeArch(arch string) string {
	switch strings.ToLower(arch) {
	case "x86_64":
		return "amd64"
	case "aarch64":
		return "arm64"
	default:
		return strings.ToLower(arch)
	}
}

func detectWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" || os.Getenv("WSL_INTEROP") != "" {
		return true
	}
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return false
	}
	release := strings.ToLower(string(data))
	return strings.Contains(release, "microsoft") || strings.Contains(release, "wsl")
}
