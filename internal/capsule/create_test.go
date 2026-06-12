package capsule

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreateWritesIsolatedCapsuleMetadataAndOpenCodeConfig(t *testing.T) {
	projectRoot := t.TempDir()
	hostLiza := filepath.Join(projectRoot, ".liza")
	if err := os.MkdirAll(hostLiza, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostLiza, "state.yaml"), []byte("host: true\n"), 0644); err != nil {
		t.Fatal(err)
	}

	storeRoot := filepath.Join(t.TempDir(), "capsules")
	meta, err := Create(CreateOptions{
		Name:        "grok-test",
		ProjectRoot: projectRoot,
		Runtime:     RuntimeDocker,
		Preset:      "openai-compatible",
		StoreRoot:   storeRoot,
		HomeDir:     filepath.Join(t.TempDir(), "home"),
		Host:        Platform{OS: "darwin", Arch: "arm64"},
		Now:         func() time.Time { return time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if meta.Paths.ProjectLiza == hostLiza {
		t.Fatal("capsule project .liza path must not be the host repo .liza")
	}
	if _, err := os.Stat(filepath.Join(meta.Paths.OpenCodeConfig, "opencode.json")); err != nil {
		t.Fatalf("OpenCode config missing: %v", err)
	}
	hostState, err := os.ReadFile(filepath.Join(hostLiza, "state.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(hostState) != "host: true\n" {
		t.Fatalf("host .liza was modified: %q", string(hostState))
	}

	loaded, err := LoadMetadata(storeRoot, projectRoot, "grok-test")
	if err != nil {
		t.Fatalf("LoadMetadata failed: %v", err)
	}
	if loaded.Name != meta.Name || loaded.Guest.OS != "linux" {
		t.Fatalf("loaded metadata = %#v, want name %q and linux guest", loaded, meta.Name)
	}
}

func TestRenderOpenCodeConfigUsesEnvironmentVariableReference(t *testing.T) {
	preset, err := DefaultOpenCodePreset("openai-compatible")
	if err != nil {
		t.Fatal(err)
	}
	data, err := RenderOpenCodeConfig(preset)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`"enabled_providers"`,
		`"apiKey": "{env:CAPSULE_PROVIDER_API_KEY}"`,
		`"gpt-oss-120b"`,
		`"grok-code"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("OpenCode config missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "sk-") {
		t.Fatal("OpenCode config must not contain literal API keys")
	}
}

func TestPresetFromProviderMetadataSelectsPreferredModel(t *testing.T) {
	preset, err := PresetFromProviderMetadata("models.dev", ProviderMetadata{
		ID:      "openrouter",
		Name:    "OpenRouter",
		BaseURL: "https://openrouter.ai/api/v1",
		Models: []ModelMetadata{
			{ID: "other", Name: "Other"},
			{ID: "gpt-oss-120b", Name: "GPT OSS 120B"},
			{ID: "gpt-oss-20b", Name: "GPT OSS 20B"},
		},
	}, "OPENROUTER_API_KEY", []string{"grok-code", "gpt-oss-120b"})
	if err != nil {
		t.Fatalf("PresetFromProviderMetadata failed: %v", err)
	}
	if preset.Model != "gpt-oss-120b" {
		t.Fatalf("selected model = %q, want gpt-oss-120b", preset.Model)
	}
	if preset.SmallModel != "gpt-oss-20b" {
		t.Fatalf("small model = %q, want gpt-oss-20b", preset.SmallModel)
	}
	if preset.EnabledProviders[0] != "openrouter" {
		t.Fatalf("enabled providers = %#v, want openrouter", preset.EnabledProviders)
	}
}

func TestFetchModelsDevProviderParsesProviderMetadata(t *testing.T) {
	client := fakeHTTPDoer{response: `{
		"openrouter": {
			"name": "OpenRouter",
			"api": "https://openrouter.ai/api/v1",
			"models": {
				"openai/gpt-oss-120b": {"name": "GPT OSS 120B"},
				"x-ai/grok-code": {"id": "grok-code", "name": "Grok Code"}
			}
		}
	}`}
	provider, err := FetchModelsDevProvider(context.Background(), client, "openrouter")
	if err != nil {
		t.Fatalf("FetchModelsDevProvider failed: %v", err)
	}
	if provider.BaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("base URL = %q", provider.BaseURL)
	}
	if len(provider.Models) != 2 {
		t.Fatalf("model count = %d, want 2", len(provider.Models))
	}
	for _, model := range provider.Models {
		if model.ID == "" {
			t.Fatalf("model IDs should be populated from keys or explicit IDs: %#v", provider.Models)
		}
	}
}

func TestDaytonaClientCreatesSandboxWithoutPersistingAPIKey(t *testing.T) {
	client := recordingHTTPDoer{response: `{
		"id": "sbx_123",
		"name": "liza-capsule",
		"state": "started",
		"target": "us",
		"snapshot": "liza-capsule:latest",
		"toolboxProxyUrl": "https://proxy.example"
	}`}
	daytona := NewDaytonaClient("https://app.daytona.io/api", "secret-token", "org_123", &client)
	meta := BuildDaytonaMetadata("cloud-test", DaytonaCreateOptions{
		Target:            "us",
		Snapshot:          "liza-capsule:latest",
		CPU:               2,
		MemoryGB:          4,
		DiskGB:            20,
		AutoStopMinutes:   30,
		AutoDeleteMinutes: 120,
	})

	sandbox, err := daytona.CreateSandbox(context.Background(), meta, map[string]string{"OPENCODE_CONFIG": "/home/liza/.config/opencode/opencode.json"})
	if err != nil {
		t.Fatalf("CreateSandbox failed: %v", err)
	}
	if sandbox.ID != "sbx_123" {
		t.Fatalf("sandbox ID = %q", sandbox.ID)
	}
	if client.method != http.MethodPost || client.path != "/api/sandbox" {
		t.Fatalf("request = %s %s, want POST /api/sandbox", client.method, client.path)
	}
	if got := client.authHeader; got != "Bearer secret-token" {
		t.Fatalf("auth header = %q", got)
	}
	var body map[string]any
	if err := json.Unmarshal(client.body, &body); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	if body["snapshot"] != "liza-capsule:latest" || body["target"] != "us" {
		t.Fatalf("request body = %#v", body)
	}
	if strings.Contains(string(client.body), "secret-token") {
		t.Fatal("Daytona API key must not be written into request body")
	}
}

func TestDaytonaClientCreatesSnapshotFromImmutableImage(t *testing.T) {
	client := recordingHTTPDoer{response: `{
		"id": "snap_123",
		"name": "liza-capsule-20260610",
		"imageName": "ghcr.io/example/liza-capsule:20260610",
		"state": "active"
	}`}
	daytona := NewDaytonaClient("https://app.daytona.io/api", "secret-token", "org_123", &client)

	snapshot, err := daytona.CreateSnapshot(context.Background(), DaytonaSnapshotCreateOptions{
		Name:         "liza-capsule-20260610",
		ImageName:    "ghcr.io/example/liza-capsule:20260610",
		RegionID:     "us",
		SandboxClass: "linux-vm",
		Entrypoint:   []string{"sleep", "infinity"},
		CPU:          2,
		MemoryGB:     4,
		DiskGB:       20,
	})
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}
	if snapshot.ID != "snap_123" {
		t.Fatalf("snapshot ID = %q", snapshot.ID)
	}
	if client.method != http.MethodPost || client.path != "/api/snapshots" {
		t.Fatalf("request = %s %s, want POST /api/snapshots", client.method, client.path)
	}
	var body map[string]any
	if err := json.Unmarshal(client.body, &body); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	if body["imageName"] != "ghcr.io/example/liza-capsule:20260610" || body["regionId"] != "us" {
		t.Fatalf("request body = %#v", body)
	}
	if strings.Contains(string(client.body), "secret-token") {
		t.Fatal("Daytona API key must not be written into snapshot request body")
	}
}

func TestDaytonaClientRejectsLatestSnapshotImage(t *testing.T) {
	daytona := NewDaytonaClient("https://app.daytona.io/api", "secret-token", "", nil)
	_, err := daytona.CreateSnapshot(context.Background(), DaytonaSnapshotCreateOptions{
		Name:      "bad",
		ImageName: "ghcr.io/example/liza-capsule:latest",
	})
	if err == nil || !strings.Contains(err.Error(), ":latest") {
		t.Fatalf("CreateSnapshot error = %v, want :latest rejection", err)
	}
}

func TestCreateDaytonaCapsuleStoresOnlyEnvVarNames(t *testing.T) {
	storeRoot := filepath.Join(t.TempDir(), "capsules")
	meta, err := Create(CreateOptions{
		Name:        "cloud-test",
		ProjectRoot: t.TempDir(),
		Runtime:     RuntimeDaytona,
		StoreRoot:   storeRoot,
		HomeDir:     filepath.Join(t.TempDir(), "home"),
		Host:        Platform{OS: "darwin", Arch: "arm64"},
		Daytona:     DaytonaCreateOptions{Target: "eu", Snapshot: "liza-capsule:latest"},
		Now:         func() time.Time { return time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if meta.Daytona == nil {
		t.Fatal("expected Daytona metadata")
	}
	if meta.Guest.OS != "linux" {
		t.Fatalf("guest OS = %q, want linux", meta.Guest.OS)
	}
	if meta.Daytona.APIKeyEnv != "DAYTONA_API_KEY" || meta.Daytona.Target != "eu" {
		t.Fatalf("Daytona metadata = %#v", meta.Daytona)
	}
	data, err := os.ReadFile(meta.Paths.Metadata)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "dtn_") || strings.Contains(string(data), "secret-token") {
		t.Fatal("metadata must not contain literal Daytona credentials")
	}
}

type fakeHTTPDoer struct {
	response string
}

func (f fakeHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(f.response)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

type recordingHTTPDoer struct {
	response   string
	method     string
	path       string
	authHeader string
	body       []byte
}

func (f *recordingHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	f.method = req.Method
	f.path = req.URL.Path
	f.authHeader = req.Header.Get("Authorization")
	if req.Body != nil {
		body, _ := io.ReadAll(req.Body)
		f.body = body
	}
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(f.response)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

func TestDoctorReportsAuthFallbackWhenHostConfigMissing(t *testing.T) {
	meta := &CapsuleMetadata{
		Runtime: RuntimeDocker,
		Guest:   Platform{OS: "linux", Arch: "arm64"},
		Paths: CapsulePaths{
			ProjectLiza:    "/capsule/project-liza",
			OpenCodeConfig: "/capsule/opencode-config",
		},
		Tools: map[ToolName]ToolchainRecord{
			ToolCodex: {
				Tool:         ToolCodex,
				Supported:    true,
				AuthMounts:   []MountPlan{{Source: "/missing/.codex", Target: "/home/liza/.codex", ReadOnly: true}},
				EnvFallbacks: []string{"OPENAI_API_KEY"},
			},
		},
	}
	summary := Doctor(meta, DoctorOptions{
		LookPath: func(string) (string, error) { return "docker", nil },
		Stat: func(path string) error {
			if path == "/capsule/project-liza" || path == "/capsule/opencode-config" {
				return nil
			}
			return os.ErrNotExist
		},
	})
	if !summary.OK {
		t.Fatalf("doctor should allow optional auth fallback: %#v", summary)
	}
	foundFallback := false
	for _, check := range summary.Checks {
		if check.Name == "codex auth fallback" && strings.Contains(check.Message, "OPENAI_API_KEY") {
			foundFallback = true
		}
	}
	if !foundFallback {
		t.Fatalf("missing Codex auth fallback check: %#v", summary.Checks)
	}
}

func TestCreateReportExcludesSecretsAndIncludesManifest(t *testing.T) {
	root := t.TempDir()
	meta := &CapsuleMetadata{
		Name:        "report-test",
		ProjectRoot: root,
		Runtime:     RuntimeDocker,
		Paths: CapsulePaths{
			ProjectLiza:    filepath.Join(root, "project-liza"),
			OpenCodeConfig: filepath.Join(root, "opencode-config"),
			Reports:        filepath.Join(root, "reports"),
		},
		Env: map[string]string{"CAPSULE_PROVIDER_API_KEY": "secret", "OPENCODE_CONFIG": "/config/opencode.json"},
	}
	if err := os.MkdirAll(meta.Paths.ProjectLiza, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(meta.Paths.OpenCodeConfig, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(meta.Paths.ProjectLiza, "state.yaml"), []byte("ok: true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(meta.Paths.OpenCodeConfig, ".env"), []byte("SECRET=value\n"), 0644); err != nil {
		t.Fatal(err)
	}
	nodeModules := filepath.Join(meta.Paths.OpenCodeConfig, "node_modules", "package")
	if err := os.MkdirAll(nodeModules, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nodeModules, "index.js"), []byte("module.exports = true\n"), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := CreateReport(meta, ReportOptions{
		AuthorName: "Livio Gama",
		Now: func() time.Time {
			return time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("CreateReport failed: %v", err)
	}
	if got, want := filepath.Base(report), "report-test-livio-gama-20260610-120000.zip"; got != want {
		t.Fatalf("report filename = %q, want %q", got, want)
	}
	zr, err := zip.OpenReader(report)
	if err != nil {
		t.Fatalf("open report zip: %v", err)
	}
	defer zr.Close()

	entries := map[string]bool{}
	for _, f := range zr.File {
		entries[f.Name] = true
	}
	if !entries["manifest.json"] || !entries["project-liza/state.yaml"] {
		t.Fatalf("report entries = %#v, want manifest and state", entries)
	}
	if entries["opencode-config/.env"] {
		t.Fatalf("report must exclude .env files: %#v", entries)
	}
	if entries["opencode-config/node_modules/package/index.js"] {
		t.Fatalf("report must exclude node_modules directories: %#v", entries)
	}
}
