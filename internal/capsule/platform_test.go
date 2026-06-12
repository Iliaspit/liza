package capsule

import "testing"

func TestGuestPlatform_ContainerRuntimeUsesLinuxGuest(t *testing.T) {
	tests := []struct {
		name string
		host Platform
		mode RuntimeMode
		want Platform
	}{
		{
			name: "mac docker arm64",
			host: Platform{OS: "darwin", Arch: "arm64"},
			mode: RuntimeDocker,
			want: Platform{OS: "linux", Arch: "arm64"},
		},
		{
			name: "windows podman amd64",
			host: Platform{OS: "windows", Arch: "amd64"},
			mode: RuntimePodman,
			want: Platform{OS: "linux", Arch: "amd64"},
		},
		{
			name: "native preserves host",
			host: Platform{OS: "darwin", Arch: "arm64"},
			mode: RuntimeNative,
			want: Platform{OS: "darwin", Arch: "arm64"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GuestPlatform(tt.host, tt.mode)
			if got != tt.want {
				t.Fatalf("GuestPlatform() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestResolveToolchain_ContainerDoesNotUseHostBinary(t *testing.T) {
	record := ResolveToolchain(ToolCodex, Platform{OS: "darwin", Arch: "arm64"}, RuntimeDocker, "/Users/livio")
	if !record.Supported {
		t.Fatalf("Codex container toolchain should be supported: %#v", record)
	}
	if record.Guest.OS != "linux" {
		t.Fatalf("guest OS = %q, want linux", record.Guest.OS)
	}
	if record.Binary == "codex" {
		t.Fatalf("container toolchain must resolve a guest binary path, got host binary %q", record.Binary)
	}
	if len(record.AuthMounts) != 1 || !record.AuthMounts[0].ReadOnly {
		t.Fatalf("Codex auth mount = %#v, want one read-only mount", record.AuthMounts)
	}
}

func TestResolveToolchain_NativeIsDeferred(t *testing.T) {
	record := ResolveToolchain(ToolClaude, Platform{OS: "darwin", Arch: "arm64"}, RuntimeNative, "/Users/livio")
	if record.Supported {
		t.Fatal("native capsule toolchain should be deferred in v1")
	}
	if record.Binary != "claude" {
		t.Fatalf("native binary = %q, want claude", record.Binary)
	}
}
