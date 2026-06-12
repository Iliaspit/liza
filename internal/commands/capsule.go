package commands

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/capsule"
	"github.com/liza-mas/liza/internal/gitenv"
	"golang.org/x/term"
)

type CapsuleCreateParams struct {
	ProjectRoot       string
	Name              string
	Preset            string
	Runtime           string
	Image             string
	StoreRoot         string
	Context           context.Context
	ModelsDevProvider string
	APIKeyEnv         string
	PreferredModels   []string
	DaytonaAPIURL     string
	DaytonaTarget     string
	DaytonaSnapshot   string
	DaytonaCPU        int
	DaytonaMemoryGB   int
	DaytonaDiskGB     int
	DaytonaAutoStop   int
	DaytonaAutoDelete int
	NoProvision       bool
	HTTPClient        capsule.HTTPDoer
	LookupEnv         func(string) (string, bool)
	Now               func() time.Time
}

type CapsuleDoctorParams struct {
	ProjectRoot string
	Name        string
	Tool        string
	StoreRoot   string
}

type CapsuleStartParams struct {
	ProjectRoot string
	Name        string
	Command     []string
	StoreRoot   string
	Stdout      io.Writer
	Stderr      io.Writer
	Stdin       io.Reader
	Context     context.Context
	HTTPClient  capsule.HTTPDoer
	LookupEnv   func(string) (string, bool)
	Now         func() time.Time
}

type CapsuleStopParams struct {
	ProjectRoot string
	Name        string
	StoreRoot   string
	Force       bool
	Context     context.Context
	HTTPClient  capsule.HTTPDoer
	LookupEnv   func(string) (string, bool)
	Now         func() time.Time
}

type CapsuleSnapshotCreateParams struct {
	Name           string
	ImageName      string
	APIURL         string
	RegionID       string
	SandboxClass   string
	Entrypoint     []string
	CPU            int
	MemoryGB       int
	DiskGB         int
	Context        context.Context
	HTTPClient     capsule.HTTPDoer
	LookupEnv      func(string) (string, bool)
	OrganizationID string
}

type CapsuleReportParams struct {
	ProjectRoot string
	Name        string
	StoreRoot   string
}

func CapsuleCreateCommand(params CapsuleCreateParams) (*capsule.CapsuleMetadata, error) {
	storeRoot, err := resolveCapsuleStoreRoot(params.StoreRoot)
	if err != nil {
		return nil, err
	}
	if capsule.RuntimeMode(params.Runtime) == capsule.RuntimeDaytona && !params.NoProvision {
		apiKeyEnv := capsule.DefaultDaytonaAPIKeyEnv
		lookupEnv := params.LookupEnv
		if lookupEnv == nil {
			lookupEnv = os.LookupEnv
		}
		apiKey, ok := lookupEnv(apiKeyEnv)
		if !ok || strings.TrimSpace(apiKey) == "" {
			return nil, fmt.Errorf("daytona API key not found; set %s or use --no-provision", apiKeyEnv)
		}
	}
	var openCodePreset *capsule.OpenCodePreset
	if params.ModelsDevProvider != "" {
		ctx := params.Context
		if ctx == nil {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
		}
		provider, err := capsule.FetchModelsDevProvider(ctx, http.DefaultClient, params.ModelsDevProvider)
		if err != nil {
			return nil, err
		}
		apiKeyEnv := params.APIKeyEnv
		if apiKeyEnv == "" {
			apiKeyEnv = strings.ToUpper(strings.ReplaceAll(params.ModelsDevProvider, "-", "_")) + "_API_KEY"
		}
		preset, err := capsule.PresetFromProviderMetadata("models.dev", provider, apiKeyEnv, params.PreferredModels)
		if err != nil {
			return nil, err
		}
		openCodePreset = &preset
	}
	meta, err := capsule.Create(capsule.CreateOptions{
		Name:        params.Name,
		ProjectRoot: params.ProjectRoot,
		Runtime:     capsule.RuntimeMode(params.Runtime),
		Preset:      params.Preset,
		Image:       params.Image,
		StoreRoot:   storeRoot,
		OpenCode:    openCodePreset,
		Daytona: capsule.DaytonaCreateOptions{
			APIURL:            params.DaytonaAPIURL,
			Target:            params.DaytonaTarget,
			Snapshot:          params.DaytonaSnapshot,
			CPU:               params.DaytonaCPU,
			MemoryGB:          params.DaytonaMemoryGB,
			DiskGB:            params.DaytonaDiskGB,
			AutoStopMinutes:   params.DaytonaAutoStop,
			AutoDeleteMinutes: params.DaytonaAutoDelete,
		},
		Now: params.Now,
	})
	if err != nil {
		return nil, err
	}
	if meta.Runtime == capsule.RuntimeDaytona && !params.NoProvision {
		client, err := daytonaClientFromMeta(meta, params.LookupEnv, params.HTTPClient)
		if err != nil {
			return nil, err
		}
		ctx := params.Context
		if ctx == nil {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
		}
		sandbox, err := client.CreateSandbox(ctx, *meta.Daytona, daytonaSandboxEnv(meta))
		if err != nil {
			return nil, err
		}
		capsule.ApplyDaytonaSandbox(meta, sandbox, nowOrUTC(params.Now))
		if err := capsule.SaveMetadata(meta); err != nil {
			return nil, err
		}
	}
	fmt.Fprintf(os.Stdout, "Created capsule %q\n", meta.Name)
	fmt.Fprintf(os.Stdout, "  runtime: %s (%s/%s guest)\n", meta.Runtime, meta.Guest.OS, meta.Guest.Arch)
	fmt.Fprintf(os.Stdout, "  virtual .liza: %s\n", meta.Paths.ProjectLiza)
	fmt.Fprintf(os.Stdout, "  OpenCode config: %s\n", filepath.Join(meta.Paths.OpenCodeConfig, "opencode.json"))
	if meta.Daytona != nil {
		fmt.Fprintf(os.Stdout, "  Daytona sandbox: %s\n", stringOrDefault(meta.Daytona.SandboxID, "not provisioned"))
	}
	return meta, nil
}

func CapsuleDoctorCommand(params CapsuleDoctorParams) (capsule.DoctorSummary, error) {
	meta, err := loadCapsule(params.ProjectRoot, params.StoreRoot, params.Name)
	if err != nil {
		return capsule.DoctorSummary{}, err
	}
	summary := capsule.Doctor(meta, capsule.DoctorOptions{Tool: capsule.ToolName(params.Tool)})
	meta.Doctor = summary
	_ = capsule.SaveMetadata(meta)
	for _, check := range summary.Checks {
		status := "OK"
		if !check.OK {
			status = "FAIL"
		}
		fmt.Fprintf(os.Stdout, "[%s] %s: %s\n", status, check.Name, check.Message)
	}
	if !summary.OK {
		return summary, fmt.Errorf("capsule %q doctor failed", params.Name)
	}
	return summary, nil
}

func CapsuleStartCommand(params CapsuleStartParams) error {
	meta, err := loadCapsule(params.ProjectRoot, params.StoreRoot, params.Name)
	if err != nil {
		return err
	}
	if meta.Runtime == capsule.RuntimeDaytona {
		return capsuleStartDaytona(meta, params)
	}
	stdin := readerOrDefault(params.Stdin, os.Stdin)
	cmd, err := capsule.BuildStartCommand(meta, capsule.StartOptions{
		Command:     params.Command,
		Interactive: isInteractiveInput(stdin),
	})
	if err != nil {
		return err
	}
	cmd.Stdout = writerOrDefault(params.Stdout, os.Stdout)
	cmd.Stderr = writerOrDefault(params.Stderr, os.Stderr)
	cmd.Stdin = stdin
	return cmd.Run()
}

func CapsuleStopCommand(params CapsuleStopParams) error {
	meta, err := loadCapsule(params.ProjectRoot, params.StoreRoot, params.Name)
	if err != nil {
		return err
	}
	if meta.Runtime != capsule.RuntimeDaytona {
		return fmt.Errorf("capsule %q uses runtime %q; stop is only implemented for daytona capsules", params.Name, meta.Runtime)
	}
	if meta.Daytona == nil || meta.Daytona.SandboxID == "" {
		return fmt.Errorf("capsule %q has no Daytona sandbox ID", params.Name)
	}
	client, err := daytonaClientFromMeta(meta, params.LookupEnv, params.HTTPClient)
	if err != nil {
		return err
	}
	ctx := params.Context
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
	}
	sandbox, err := client.StopSandbox(ctx, meta.Daytona.SandboxID, params.Force)
	if err != nil {
		return err
	}
	capsule.ApplyDaytonaSandbox(meta, sandbox, nowOrUTC(params.Now))
	if err := capsule.SaveMetadata(meta); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Stopped Daytona sandbox %s (%s)\n", meta.Daytona.SandboxID, meta.Daytona.SandboxState)
	return nil
}

func CapsuleSnapshotCreateCommand(params CapsuleSnapshotCreateParams) (capsule.DaytonaSnapshot, error) {
	lookupEnv := params.LookupEnv
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	apiKey, ok := lookupEnv(capsule.DefaultDaytonaAPIKeyEnv)
	if !ok || strings.TrimSpace(apiKey) == "" {
		return capsule.DaytonaSnapshot{}, fmt.Errorf("daytona API key not found; set %s", capsule.DefaultDaytonaAPIKeyEnv)
	}
	orgID := params.OrganizationID
	if orgID == "" {
		orgID, _ = lookupEnv(capsule.DefaultDaytonaOrganizationIDEnv)
	}
	ctx := params.Context
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
	}
	client := capsule.NewDaytonaClient(params.APIURL, apiKey, orgID, params.HTTPClient)
	snapshot, err := client.CreateSnapshot(ctx, capsule.DaytonaSnapshotCreateOptions{
		Name:         params.Name,
		ImageName:    params.ImageName,
		RegionID:     params.RegionID,
		SandboxClass: params.SandboxClass,
		Entrypoint:   params.Entrypoint,
		CPU:          params.CPU,
		MemoryGB:     params.MemoryGB,
		DiskGB:       params.DiskGB,
	})
	if err != nil {
		return capsule.DaytonaSnapshot{}, err
	}
	fmt.Fprintf(os.Stdout, "Created Daytona snapshot %q from %s\n", stringOrDefault(snapshot.Name, params.Name), params.ImageName)
	if snapshot.ID != "" {
		fmt.Fprintf(os.Stdout, "  id: %s\n", snapshot.ID)
	}
	if snapshot.State != "" {
		fmt.Fprintf(os.Stdout, "  state: %s\n", snapshot.State)
	}
	return snapshot, nil
}

func CapsuleReportCommand(ctx context.Context, params CapsuleReportParams) (string, error) {
	meta, err := loadCapsule(params.ProjectRoot, params.StoreRoot, params.Name)
	if err != nil {
		return "", err
	}
	reportPath, err := capsule.CreateReport(meta, capsule.ReportOptions{
		AuthorName: capsuleReportAuthorName(params.ProjectRoot),
	})
	if err != nil {
		return "", err
	}
	fmt.Fprintf(os.Stdout, "Created report: %s\n", reportPath)
	return reportPath, nil
}

func capsuleReportAuthorName(projectRoot string) string {
	output, err := gitenv.Output(projectRoot, "config", "user.name")
	if err == nil && strings.TrimSpace(string(output)) != "" {
		return strings.TrimSpace(string(output))
	}
	output, err = gitenv.Output(projectRoot, "log", "-1", "--format=%an")
	if err == nil {
		return strings.TrimSpace(string(output))
	}
	return ""
}

func CapsuleListCommand(projectRoot, storeRoot string) ([]capsule.CapsuleMetadata, error) {
	root, err := resolveCapsuleStoreRoot(storeRoot)
	if err != nil {
		return nil, err
	}
	items, err := capsule.List(root, projectRoot)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		fmt.Fprintf(os.Stdout, "%s\t%s\t%s/%s\t%s\n", item.Name, item.Runtime, item.Guest.OS, item.Guest.Arch, item.CreatedAt.Format("2006-01-02T15:04:05Z"))
	}
	return items, nil
}

func CapsuleDeleteCommand(projectRoot, storeRoot, name string) error {
	root, err := resolveCapsuleStoreRoot(storeRoot)
	if err != nil {
		return err
	}
	if err := capsule.Delete(root, projectRoot, name); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Deleted capsule %q\n", name)
	return nil
}

func CapsuleDeleteCommandWithRemote(ctx context.Context, projectRoot, storeRoot, name string, localOnly bool) error {
	meta, err := loadCapsule(projectRoot, storeRoot, name)
	if err != nil {
		return err
	}
	if meta.Runtime == capsule.RuntimeDaytona && !localOnly && meta.Daytona != nil && meta.Daytona.SandboxID != "" {
		client, err := daytonaClientFromMeta(meta, nil, nil)
		if err != nil {
			return err
		}
		if ctx == nil {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
		}
		if _, err := client.DeleteSandbox(ctx, meta.Daytona.SandboxID); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Deleted Daytona sandbox %s\n", meta.Daytona.SandboxID)
	}
	return CapsuleDeleteCommand(projectRoot, storeRoot, name)
}

func loadCapsule(projectRoot, storeRoot, name string) (*capsule.CapsuleMetadata, error) {
	root, err := resolveCapsuleStoreRoot(storeRoot)
	if err != nil {
		return nil, err
	}
	return capsule.LoadMetadata(root, projectRoot, name)
}

func resolveCapsuleStoreRoot(value string) (string, error) {
	if value != "" {
		return value, nil
	}
	return capsule.DefaultStoreRoot()
}

func writerOrDefault(w io.Writer, fallback io.Writer) io.Writer {
	if w != nil {
		return w
	}
	return fallback
}

func readerOrDefault(r io.Reader, fallback io.Reader) io.Reader {
	if r != nil {
		return r
	}
	return fallback
}

func capsuleStartDaytona(meta *capsule.CapsuleMetadata, params CapsuleStartParams) error {
	if meta.Daytona == nil || meta.Daytona.SandboxID == "" {
		return fmt.Errorf("capsule %q has no Daytona sandbox ID; create it without --no-provision first", meta.Name)
	}
	client, err := daytonaClientFromMeta(meta, params.LookupEnv, params.HTTPClient)
	if err != nil {
		return err
	}
	ctx := params.Context
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
	}
	sandbox, err := client.StartSandbox(ctx, meta.Daytona.SandboxID)
	if err != nil {
		return err
	}
	capsule.ApplyDaytonaSandbox(meta, sandbox, nowOrUTC(params.Now))
	if err := capsule.SaveMetadata(meta); err != nil {
		return err
	}
	fmt.Fprintf(writerOrDefault(params.Stdout, os.Stdout), "Started Daytona sandbox %s (%s)\n", meta.Daytona.SandboxID, meta.Daytona.SandboxState)
	if len(params.Command) == 0 {
		return nil
	}
	if isLizaCapsuleCommand(params.Command) {
		if err := ensureDaytonaProjectWorkspace(ctx, client, meta, params.ProjectRoot); err != nil {
			return err
		}
	}
	command := capsule.DaytonaCommand(params.Command)
	if isLizaTUICommand(params.Command) {
		stdin := readerOrDefault(params.Stdin, os.Stdin)
		if !isInteractiveInput(stdin) {
			return fmt.Errorf("daytona interactive command %q requires a terminal; run it from an interactive shell", strings.Join(params.Command, " "))
		}
		rows, cols := 24, 80
		var restore func() error
		if file, ok := stdin.(*os.File); ok {
			width, height, err := term.GetSize(int(file.Fd()))
			if err == nil && width > 0 && height > 0 {
				cols = width
				rows = height
			}
			oldState, err := term.MakeRaw(int(file.Fd()))
			if err != nil {
				return fmt.Errorf("failed to switch terminal to raw mode: %w", err)
			}
			restore = func() error { return term.Restore(int(file.Fd()), oldState) }
			defer restore()
		}
		exitCode, err := client.RunPtyCommand(ctx, meta.Daytona.SandboxID, meta.Daytona.ToolboxProxyURL, capsule.DaytonaPtyRunOptions{
			Command: "exec " + command,
			Cwd:     capsule.DaytonaWorkspaceDir,
			Env:     daytonaSandboxEnv(meta),
			Rows:    rows,
			Cols:    cols,
			Stdin:   stdin,
			Stdout:  writerOrDefault(params.Stdout, os.Stdout),
		})
		if err != nil {
			return err
		}
		if exitCode != 0 {
			return fmt.Errorf("daytona PTY command exited with status %d", exitCode)
		}
		return nil
	}
	result, err := client.ExecuteCommand(ctx, meta.Daytona.SandboxID, command, capsule.DaytonaWorkspaceDir, 0)
	if err != nil {
		return err
	}
	if result.Result != "" {
		fmt.Fprint(writerOrDefault(params.Stdout, os.Stdout), result.Result)
		if !strings.HasSuffix(result.Result, "\n") {
			fmt.Fprintln(writerOrDefault(params.Stdout, os.Stdout))
		}
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("daytona command exited with status %d", result.ExitCode)
	}
	return nil
}

func ensureDaytonaProjectWorkspace(ctx context.Context, client capsule.DaytonaClient, meta *capsule.CapsuleMetadata, projectRoot string) error {
	if err := capsule.EnsureProjectLizaSeed(projectRoot, meta); err != nil {
		return err
	}
	archive, err := capsule.BuildProjectLizaArchive(meta.Paths.ProjectLiza)
	if err != nil {
		return fmt.Errorf("failed to package capsule .liza for Daytona: %w", err)
	}
	remoteBase := "/tmp/liza-capsule-" + meta.Name + "-project-liza"
	remoteB64 := remoteBase + ".tgz.b64"
	remoteTGZ := remoteBase + ".tgz"
	initCommand := strings.Join([]string{
		"set -e",
		"mkdir -p " + capsule.DaytonaCommand([]string{capsule.DaytonaWorkspaceDir}),
		"if [ ! -d " + capsule.DaytonaCommand([]string{filepath.Join(capsule.DaytonaWorkspaceDir, ".git")}) + " ]; then git init -q " + capsule.DaytonaCommand([]string{capsule.DaytonaWorkspaceDir}) + "; fi",
		"git config --global --add safe.directory " + capsule.DaytonaCommand([]string{capsule.DaytonaWorkspaceDir}) + " >/dev/null 2>&1 || true",
		"touch ~/.zshrc",
		"mkdir -p " + capsule.DaytonaCommand([]string{filepath.Join(capsule.DaytonaWorkspaceDir, ".liza")}),
		"rm -f " + capsule.DaytonaCommand([]string{remoteB64}) + " " + capsule.DaytonaCommand([]string{remoteTGZ}),
	}, "; ")
	if err := executeDaytonaOK(ctx, client, meta.Daytona.SandboxID, initCommand, "/"); err != nil {
		return err
	}
	for _, chunk := range capsule.Base64Chunks(archive, 32000) {
		appendCommand := capsule.DaytonaCommand([]string{"printf", "%s", chunk}) + " >> " + capsule.DaytonaCommand([]string{remoteB64})
		if err := executeDaytonaOK(ctx, client, meta.Daytona.SandboxID, appendCommand, "/"); err != nil {
			return err
		}
	}
	applyCommand := strings.Join([]string{
		"set -e",
		"base64 -d " + capsule.DaytonaCommand([]string{remoteB64}) + " > " + capsule.DaytonaCommand([]string{remoteTGZ}),
		"tar -xzf " + capsule.DaytonaCommand([]string{remoteTGZ}) + " -C " + capsule.DaytonaCommand([]string{capsule.DaytonaWorkspaceDir}),
		"rm -f " + capsule.DaytonaCommand([]string{remoteB64}) + " " + capsule.DaytonaCommand([]string{remoteTGZ}),
	}, "; ")
	return executeDaytonaOK(ctx, client, meta.Daytona.SandboxID, applyCommand, "/")
}

func executeDaytonaOK(ctx context.Context, client capsule.DaytonaClient, sandboxID, command, cwd string) error {
	result, err := client.ExecuteCommand(ctx, sandboxID, command, cwd, 120)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("daytona workspace command exited with status %d: %s", result.ExitCode, strings.TrimSpace(result.Result))
	}
	return nil
}

func isLizaCapsuleCommand(command []string) bool {
	return len(command) > 0 && filepath.Base(command[0]) == "liza"
}

func isLizaTUICommand(command []string) bool {
	return len(command) >= 2 && filepath.Base(command[0]) == "liza" && command[1] == "tui"
}

func daytonaClientFromMeta(meta *capsule.CapsuleMetadata, lookupEnv func(string) (string, bool), httpClient capsule.HTTPDoer) (capsule.DaytonaClient, error) {
	if meta.Daytona == nil {
		return capsule.DaytonaClient{}, fmt.Errorf("missing daytona metadata")
	}
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	apiKeyEnv := meta.Daytona.APIKeyEnv
	if apiKeyEnv == "" {
		apiKeyEnv = capsule.DefaultDaytonaAPIKeyEnv
	}
	apiKey, ok := lookupEnv(apiKeyEnv)
	if !ok || strings.TrimSpace(apiKey) == "" {
		return capsule.DaytonaClient{}, fmt.Errorf("daytona API key not found; set %s", apiKeyEnv)
	}
	orgID := ""
	if meta.Daytona.OrganizationIDEnv != "" {
		orgID, _ = lookupEnv(meta.Daytona.OrganizationIDEnv)
	}
	return capsule.NewDaytonaClient(meta.Daytona.APIURL, apiKey, orgID, httpClient), nil
}

func daytonaSandboxEnv(meta *capsule.CapsuleMetadata) map[string]string {
	env := map[string]string{}
	for key, value := range meta.Env {
		env[key] = value
	}
	env["LIZA_CAPSULE_NAME"] = meta.Name
	env["LIZA_CAPSULE_RUNTIME"] = string(meta.Runtime)
	return env
}

func nowOrUTC(now func() time.Time) time.Time {
	if now != nil {
		return now()
	}
	return time.Now().UTC()
}

func stringOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func isInteractiveInput(r io.Reader) bool {
	file, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0 && term.IsTerminal(int(file.Fd()))
}
