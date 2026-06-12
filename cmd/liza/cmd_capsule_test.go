package main

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liza-mas/liza/internal/capsule"
	"github.com/liza-mas/liza/internal/commands"
)

func TestCapsuleCLI_CreateListReportDeleteKeepsHostLizaUntouched(t *testing.T) {
	projectRoot, statePath := setupMutationTestProject(t, nil)
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read host state before capsule create: %v", err)
	}
	storeRoot := filepath.Join(t.TempDir(), "capsules")

	if err := executeRootCommand(t, projectRoot,
		"capsule", "create", "opencode-lab",
		"--preset", "openai-compatible",
		"--runtime", "docker",
		"--store-root", storeRoot,
	); err != nil {
		t.Fatalf("capsule create failed: %v", err)
	}

	afterCreate, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read host state after capsule create: %v", err)
	}
	if string(afterCreate) != string(before) {
		t.Fatal("capsule create modified the host project .liza/state.yaml")
	}

	if err := executeRootCommand(t, projectRoot, "capsule", "list", "--store-root", storeRoot); err != nil {
		t.Fatalf("capsule list failed: %v", err)
	}

	if err := executeRootCommand(t, projectRoot, "capsule", "report", "opencode-lab", "--store-root", storeRoot); err != nil {
		t.Fatalf("capsule report failed: %v", err)
	}
	paths := capsule.BuildPaths(storeRoot, projectRoot, "opencode-lab")
	reports, err := os.ReadDir(paths.Reports)
	if err != nil {
		t.Fatalf("read capsule reports: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("report count = %d, want 1", len(reports))
	}
	if got := reports[0].Name(); !strings.HasPrefix(got, "opencode-lab-test-user-") || !strings.HasSuffix(got, ".zip") {
		t.Fatalf("report filename = %q, want capsule name, git author, and .zip suffix", got)
	}

	if err := executeRootCommand(t, projectRoot, "capsule", "delete", "opencode-lab", "--store-root", storeRoot); err != nil {
		t.Fatalf("capsule delete failed: %v", err)
	}
	if _, err := os.Stat(paths.Root); !os.IsNotExist(err) {
		t.Fatalf("capsule root still exists after delete or stat failed: %v", err)
	}
}

func TestCapsuleCommands_CreateAndStartDaytonaWithFakeAPI(t *testing.T) {
	projectRoot, _ := setupMutationTestProject(t, nil)
	storeRoot := filepath.Join(t.TempDir(), "capsules")
	httpClient := &daytonaCommandHTTPDoer{}
	lookupEnv := func(key string) (string, bool) {
		if key == "DAYTONA_API_KEY" {
			return "secret-token", true
		}
		return "", false
	}

	meta, err := commands.CapsuleCreateCommand(commands.CapsuleCreateParams{
		ProjectRoot:       projectRoot,
		Name:              "cloud-lab",
		Runtime:           "daytona",
		StoreRoot:         storeRoot,
		DaytonaAPIURL:     "https://app.daytona.io/api",
		DaytonaTarget:     "us",
		DaytonaSnapshot:   "liza-capsule:latest",
		DaytonaAutoStop:   30,
		DaytonaAutoDelete: 60,
		HTTPClient:        httpClient,
		LookupEnv:         lookupEnv,
	})
	if err != nil {
		t.Fatalf("CapsuleCreateCommand failed: %v", err)
	}
	if meta.Daytona == nil || meta.Daytona.SandboxID != "sbx_cloud_lab" {
		t.Fatalf("Daytona metadata = %#v", meta.Daytona)
	}
	if strings.Contains(string(httpClient.lastBody), "secret-token") {
		t.Fatal("Daytona API key must not be sent in sandbox env/body")
	}

	var stdout strings.Builder
	if err := commands.CapsuleStartCommand(commands.CapsuleStartParams{
		ProjectRoot: projectRoot,
		Name:        "cloud-lab",
		StoreRoot:   storeRoot,
		Command:     []string{"echo", "ok"},
		Stdout:      &stdout,
		HTTPClient:  httpClient,
		LookupEnv:   lookupEnv,
	}); err != nil {
		t.Fatalf("CapsuleStartCommand failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "ok from daytona") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if httpClient.executedCommand != "echo ok" {
		t.Fatalf("executed command = %q", httpClient.executedCommand)
	}
}

func TestCapsuleCommands_StartDaytonaLizaCommandBootstrapsWorkspace(t *testing.T) {
	projectRoot, _ := setupMutationTestProject(t, nil)
	storeRoot := filepath.Join(t.TempDir(), "capsules")
	httpClient := &daytonaCommandHTTPDoer{}
	lookupEnv := func(key string) (string, bool) {
		if key == "DAYTONA_API_KEY" {
			return "secret-token", true
		}
		return "", false
	}

	if _, err := commands.CapsuleCreateCommand(commands.CapsuleCreateParams{
		ProjectRoot:     projectRoot,
		Name:            "cloud-liza",
		Runtime:         "daytona",
		StoreRoot:       storeRoot,
		DaytonaAPIURL:   "https://app.daytona.io/api",
		DaytonaTarget:   "us",
		DaytonaSnapshot: "liza-capsule:latest",
		HTTPClient:      httpClient,
		LookupEnv:       lookupEnv,
	}); err != nil {
		t.Fatalf("CapsuleCreateCommand failed: %v", err)
	}

	var stdout strings.Builder
	if err := commands.CapsuleStartCommand(commands.CapsuleStartParams{
		ProjectRoot: projectRoot,
		Name:        "cloud-liza",
		StoreRoot:   storeRoot,
		Command:     []string{"liza", "status"},
		Stdout:      &stdout,
		HTTPClient:  httpClient,
		LookupEnv:   lookupEnv,
	}); err != nil {
		t.Fatalf("CapsuleStartCommand failed: %v", err)
	}
	if httpClient.executedCommand != "liza status" {
		t.Fatalf("executed command = %q", httpClient.executedCommand)
	}
	if httpClient.executedCWD != "/workspace" {
		t.Fatalf("executed cwd = %q", httpClient.executedCWD)
	}
	if !containsCommandFragment(httpClient.executedCommands, "git init -q /workspace") {
		t.Fatalf("bootstrap commands did not initialize /workspace: %#v", httpClient.executedCommands)
	}
	if !containsCommandFragment(httpClient.executedCommands, "tar -xzf") {
		t.Fatalf("bootstrap commands did not extract .liza archive: %#v", httpClient.executedCommands)
	}
}

func TestCapsuleCommands_StartDaytonaLizaTUIRequiresInteractiveTerminal(t *testing.T) {
	projectRoot, _ := setupMutationTestProject(t, nil)
	storeRoot := filepath.Join(t.TempDir(), "capsules")
	httpClient := &daytonaCommandHTTPDoer{}
	lookupEnv := func(key string) (string, bool) {
		if key == "DAYTONA_API_KEY" {
			return "secret-token", true
		}
		return "", false
	}

	if _, err := commands.CapsuleCreateCommand(commands.CapsuleCreateParams{
		ProjectRoot:     projectRoot,
		Name:            "cloud-tui",
		Runtime:         "daytona",
		StoreRoot:       storeRoot,
		DaytonaAPIURL:   "https://app.daytona.io/api",
		DaytonaTarget:   "us",
		DaytonaSnapshot: "liza-capsule:latest",
		HTTPClient:      httpClient,
		LookupEnv:       lookupEnv,
	}); err != nil {
		t.Fatalf("CapsuleCreateCommand failed: %v", err)
	}

	err := commands.CapsuleStartCommand(commands.CapsuleStartParams{
		ProjectRoot: projectRoot,
		Name:        "cloud-tui",
		StoreRoot:   storeRoot,
		Command:     []string{"liza", "tui"},
		Stdin:       strings.NewReader(""),
		HTTPClient:  httpClient,
		LookupEnv:   lookupEnv,
	})
	if err == nil || !strings.Contains(err.Error(), "requires a terminal") {
		t.Fatalf("CapsuleStartCommand error = %v, want requires a terminal", err)
	}
	if containsCommandFragment(httpClient.executedCommands, "liza tui") {
		t.Fatalf("liza tui must not run through process/execute: %#v", httpClient.executedCommands)
	}
}

func TestCapsuleCommands_CreateDaytonaSnapshotWithFakeAPI(t *testing.T) {
	httpClient := &daytonaCommandHTTPDoer{}
	lookupEnv := func(key string) (string, bool) {
		if key == "DAYTONA_API_KEY" {
			return "secret-token", true
		}
		return "", false
	}

	snapshot, err := commands.CapsuleSnapshotCreateCommand(commands.CapsuleSnapshotCreateParams{
		Name:         "liza-capsule-20260610",
		ImageName:    "ghcr.io/example/liza-capsule:20260610",
		RegionID:     "us",
		SandboxClass: "linux-vm",
		Entrypoint:   []string{"sleep", "infinity"},
		CPU:          2,
		MemoryGB:     4,
		DiskGB:       20,
		HTTPClient:   httpClient,
		LookupEnv:    lookupEnv,
	})
	if err != nil {
		t.Fatalf("CapsuleSnapshotCreateCommand failed: %v", err)
	}
	if snapshot.ID != "snap_liza_capsule" {
		t.Fatalf("snapshot ID = %q", snapshot.ID)
	}
	if !strings.Contains(string(httpClient.lastBody), `"imageName":"ghcr.io/example/liza-capsule:20260610"`) {
		t.Fatalf("snapshot body = %s", string(httpClient.lastBody))
	}
	if strings.Contains(string(httpClient.lastBody), "secret-token") {
		t.Fatal("Daytona API key must not be sent in snapshot request body")
	}
}

type daytonaCommandHTTPDoer struct {
	lastBody         []byte
	executedCommand  string
	executedCWD      string
	executedCommands []string
}

func (f *daytonaCommandHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		f.lastBody, _ = io.ReadAll(req.Body)
	}
	body := `{"id":"sbx_cloud_lab","name":"liza-liza-capsule-cloud-lab","state":"started","target":"us","snapshot":"liza-capsule:latest","toolboxProxyUrl":"https://proxy.example"}`
	if strings.HasSuffix(req.URL.Path, "/snapshots") {
		body = `{"id":"snap_liza_capsule","name":"liza-capsule-20260610","imageName":"ghcr.io/example/liza-capsule:20260610","state":"active"}`
	}
	if strings.Contains(req.URL.Path, "/toolbox/process/execute") {
		f.executedCommand = stringValueFromJSON(f.lastBody, "command")
		f.executedCWD = stringValueFromJSON(f.lastBody, "cwd")
		f.executedCommands = append(f.executedCommands, f.executedCommand)
		body = `{"exitCode":0,"result":"ok from daytona\n"}`
	}
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

func containsCommandFragment(commands []string, fragment string) bool {
	for _, command := range commands {
		if strings.Contains(command, fragment) {
			return true
		}
	}
	return false
}

func stringValueFromJSON(body []byte, key string) string {
	needle := `"` + key + `":"`
	text := string(body)
	start := strings.Index(text, needle)
	if start == -1 {
		return ""
	}
	start += len(needle)
	end := strings.Index(text[start:], `"`)
	if end == -1 {
		return ""
	}
	return text[start : start+end]
}
