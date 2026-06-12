package capsule

import (
	"path/filepath"
)

func ResolveToolchain(tool ToolName, host Platform, mode RuntimeMode, homeDir string) ToolchainRecord {
	guest := GuestPlatform(host, mode)
	record := ToolchainRecord{
		Tool:    tool,
		Host:    host,
		Guest:   guest,
		Runtime: mode,
	}

	if mode == RuntimeNative {
		record.Supported = false
		record.Install = "host-binary"
		record.Binary = string(tool)
		record.Notes = []string{"native capsules are reserved for a future backend; host binaries are not executed by container capsules"}
		return record
	}

	if mode == RuntimeDaytona {
		record.Supported = true
		record.Install = "daytona-snapshot"
		record.Binary = "/usr/local/bin/" + string(tool)
		switch tool {
		case ToolOpenCode:
			record.EnvFallbacks = []string{"OPENCODE_CONFIG", "OPENCODE_CONFIG_DIR"}
			record.Notes = []string{"OpenCode config is written into the cloud capsule; credentials must be injected through sandbox environment variables"}
		case ToolCodex:
			record.EnvFallbacks = []string{"OPENAI_API_KEY"}
			record.Notes = []string{"Daytona capsules cannot mount host Codex auth; use sandbox env or a pre-authenticated private snapshot"}
		case ToolClaude:
			record.EnvFallbacks = []string{"ANTHROPIC_API_KEY"}
			record.Notes = []string{"Daytona capsules cannot mount host Claude auth; use sandbox env or a pre-authenticated private snapshot"}
		default:
			record.Supported = false
			record.Notes = []string{"unknown tool"}
		}
		return record
	}

	if guest.OS != "linux" {
		record.Supported = false
		record.Notes = []string{"container capsules require a Linux guest runtime"}
		return record
	}

	record.Supported = true
	switch tool {
	case ToolOpenCode:
		record.Install = "npm install -g opencode-ai"
		record.Binary = "/usr/local/bin/opencode"
		record.EnvFallbacks = []string{"OPENCODE_CONFIG", "OPENCODE_CONFIG_DIR"}
		record.Notes = []string{"OpenCode config and auth stay capsule-local by default"}
	case ToolCodex:
		record.Install = "curl -fsSL https://chatgpt.com/codex/install.sh | CODEX_NON_INTERACTIVE=1 sh"
		record.Binary = "/usr/local/bin/codex"
		record.AuthMounts = []MountPlan{{
			Source:   filepath.Join(homeDir, ".codex"),
			Target:   "/home/liza/.codex",
			ReadOnly: true,
			Purpose:  "reuse host Codex auth/config when compatible with the Linux guest CLI",
		}}
		record.EnvFallbacks = []string{"OPENAI_API_KEY"}
		record.Notes = []string{"host auth is mounted read-only only when present; API key env is the fallback"}
	case ToolClaude:
		record.Install = "curl -fsSL https://claude.ai/install.sh | bash"
		record.Binary = "/usr/local/bin/claude"
		record.AuthMounts = []MountPlan{{
			Source:   filepath.Join(homeDir, ".claude"),
			Target:   "/home/liza/.claude",
			ReadOnly: true,
			Purpose:  "reuse host Claude Code auth/config when compatible with the Linux guest CLI",
		}}
		record.EnvFallbacks = []string{"ANTHROPIC_API_KEY"}
		record.Notes = []string{"host auth is mounted read-only only when present; API key env is the fallback"}
	default:
		record.Supported = false
		record.Notes = []string{"unknown tool"}
	}

	return record
}

func ResolveAllToolchains(host Platform, mode RuntimeMode, homeDir string) map[ToolName]ToolchainRecord {
	return map[ToolName]ToolchainRecord{
		ToolOpenCode: ResolveToolchain(ToolOpenCode, host, mode, homeDir),
		ToolCodex:    ResolveToolchain(ToolCodex, host, mode, homeDir),
		ToolClaude:   ResolveToolchain(ToolClaude, host, mode, homeDir),
	}
}
