package capsule

import "time"

type RuntimeMode string

const (
	RuntimeDocker  RuntimeMode = "docker"
	RuntimePodman  RuntimeMode = "podman"
	RuntimeNative  RuntimeMode = "native"
	RuntimeDaytona RuntimeMode = "daytona"
)

type ToolName string

const (
	ToolOpenCode ToolName = "opencode"
	ToolCodex    ToolName = "codex"
	ToolClaude   ToolName = "claude"
)

type Platform struct {
	OS   string `yaml:"os" json:"os"`
	Arch string `yaml:"arch" json:"arch"`
	WSL  bool   `yaml:"wsl,omitempty" json:"wsl,omitempty"`
}

type MountPlan struct {
	Source   string `yaml:"source" json:"source"`
	Target   string `yaml:"target" json:"target"`
	ReadOnly bool   `yaml:"read_only,omitempty" json:"read_only,omitempty"`
	Required bool   `yaml:"required,omitempty" json:"required,omitempty"`
	Purpose  string `yaml:"purpose,omitempty" json:"purpose,omitempty"`
}

type ToolchainRecord struct {
	Tool         ToolName    `yaml:"tool" json:"tool"`
	Host         Platform    `yaml:"host" json:"host"`
	Guest        Platform    `yaml:"guest" json:"guest"`
	Runtime      RuntimeMode `yaml:"runtime" json:"runtime"`
	Supported    bool        `yaml:"supported" json:"supported"`
	Install      string      `yaml:"install" json:"install"`
	Binary       string      `yaml:"binary" json:"binary"`
	AuthMounts   []MountPlan `yaml:"auth_mounts,omitempty" json:"auth_mounts,omitempty"`
	EnvFallbacks []string    `yaml:"env_fallbacks,omitempty" json:"env_fallbacks,omitempty"`
	Notes        []string    `yaml:"notes,omitempty" json:"notes,omitempty"`
}

type OpenCodePreset struct {
	ID               string            `yaml:"id" json:"id"`
	ProviderID       string            `yaml:"provider_id" json:"provider_id"`
	ProviderName     string            `yaml:"provider_name" json:"provider_name"`
	BaseURL          string            `yaml:"base_url" json:"base_url"`
	APIKeyEnv        string            `yaml:"api_key_env" json:"api_key_env"`
	Model            string            `yaml:"model" json:"model"`
	SmallModel       string            `yaml:"small_model,omitempty" json:"small_model,omitempty"`
	Models           map[string]string `yaml:"models" json:"models"`
	EnabledProviders []string          `yaml:"enabled_providers,omitempty" json:"enabled_providers,omitempty"`
	ExtraOptions     map[string]string `yaml:"extra_options,omitempty" json:"extra_options,omitempty"`
	Source           string            `yaml:"source" json:"source"`
}

type CapsuleMetadata struct {
	Version         int                          `yaml:"version" json:"version"`
	Name            string                       `yaml:"name" json:"name"`
	ProjectRoot     string                       `yaml:"project_root" json:"project_root"`
	RepoFingerprint string                       `yaml:"repo_fingerprint" json:"repo_fingerprint"`
	CreatedAt       time.Time                    `yaml:"created_at" json:"created_at"`
	Runtime         RuntimeMode                  `yaml:"runtime" json:"runtime"`
	Image           string                       `yaml:"image" json:"image"`
	Host            Platform                     `yaml:"host" json:"host"`
	Guest           Platform                     `yaml:"guest" json:"guest"`
	Paths           CapsulePaths                 `yaml:"paths" json:"paths"`
	Tools           map[ToolName]ToolchainRecord `yaml:"tools" json:"tools"`
	OpenCode        OpenCodePreset               `yaml:"opencode" json:"opencode"`
	Daytona         *DaytonaMetadata             `yaml:"daytona,omitempty" json:"daytona,omitempty"`
	Env             map[string]string            `yaml:"env" json:"env"`
	Doctor          DoctorSummary                `yaml:"doctor,omitempty" json:"doctor,omitempty"`
}

type DaytonaMetadata struct {
	APIURL            string            `yaml:"api_url,omitempty" json:"api_url,omitempty"`
	APIKeyEnv         string            `yaml:"api_key_env,omitempty" json:"api_key_env,omitempty"`
	OrganizationIDEnv string            `yaml:"organization_id_env,omitempty" json:"organization_id_env,omitempty"`
	Target            string            `yaml:"target,omitempty" json:"target,omitempty"`
	Snapshot          string            `yaml:"snapshot,omitempty" json:"snapshot,omitempty"`
	SandboxID         string            `yaml:"sandbox_id,omitempty" json:"sandbox_id,omitempty"`
	SandboxName       string            `yaml:"sandbox_name,omitempty" json:"sandbox_name,omitempty"`
	SandboxState      string            `yaml:"sandbox_state,omitempty" json:"sandbox_state,omitempty"`
	ToolboxProxyURL   string            `yaml:"toolbox_proxy_url,omitempty" json:"toolbox_proxy_url,omitempty"`
	CPU               int               `yaml:"cpu,omitempty" json:"cpu,omitempty"`
	MemoryGB          int               `yaml:"memory_gb,omitempty" json:"memory_gb,omitempty"`
	DiskGB            int               `yaml:"disk_gb,omitempty" json:"disk_gb,omitempty"`
	AutoStopMinutes   int               `yaml:"auto_stop_minutes,omitempty" json:"auto_stop_minutes,omitempty"`
	AutoDeleteMinutes int               `yaml:"auto_delete_minutes,omitempty" json:"auto_delete_minutes,omitempty"`
	Labels            map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	LastSyncedAt      time.Time         `yaml:"last_synced_at,omitempty" json:"last_synced_at,omitempty"`
}

type CapsulePaths struct {
	Root           string `yaml:"root" json:"root"`
	Metadata       string `yaml:"metadata" json:"metadata"`
	ProjectLiza    string `yaml:"project_liza" json:"project_liza"`
	HomeLiza       string `yaml:"home_liza" json:"home_liza"`
	OpenCodeConfig string `yaml:"opencode_config" json:"opencode_config"`
	OpenCodeData   string `yaml:"opencode_data" json:"opencode_data"`
	Cache          string `yaml:"cache" json:"cache"`
	Reports        string `yaml:"reports" json:"reports"`
	SecretsEnv     string `yaml:"secrets_env" json:"secrets_env"`
	SecretsExample string `yaml:"secrets_example" json:"secrets_example"`
}

type DoctorSummary struct {
	CheckedAt time.Time     `yaml:"checked_at,omitempty" json:"checked_at,omitempty"`
	OK        bool          `yaml:"ok" json:"ok"`
	Checks    []DoctorCheck `yaml:"checks,omitempty" json:"checks,omitempty"`
}

type DoctorCheck struct {
	Name    string `yaml:"name" json:"name"`
	OK      bool   `yaml:"ok" json:"ok"`
	Message string `yaml:"message" json:"message"`
}
