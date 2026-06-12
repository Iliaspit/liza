package capsule

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultDaytonaAPIURL            = "https://app.daytona.io/api"
	DefaultDaytonaAPIKeyEnv         = "DAYTONA_API_KEY"
	DefaultDaytonaOrganizationIDEnv = "DAYTONA_ORGANIZATION_ID"
	DefaultDaytonaSnapshot          = "liza-capsule:latest"
)

type DaytonaCreateOptions struct {
	APIURL            string
	APIKeyEnv         string
	OrganizationIDEnv string
	Target            string
	Snapshot          string
	CPU               int
	MemoryGB          int
	DiskGB            int
	AutoStopMinutes   int
	AutoDeleteMinutes int
	Labels            map[string]string
}

type DaytonaSandbox struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	State           string            `json:"state"`
	Target          string            `json:"target"`
	Snapshot        string            `json:"snapshot"`
	ToolboxProxyURL string            `json:"toolboxProxyUrl"`
	Labels          map[string]string `json:"labels"`
}

type DaytonaExecuteResponse struct {
	ExitCode int    `json:"exitCode"`
	Result   string `json:"result"`
}

type DaytonaSnapshotCreateOptions struct {
	Name         string
	ImageName    string
	RegionID     string
	SandboxClass string
	Entrypoint   []string
	CPU          int
	MemoryGB     int
	DiskGB       int
}

type DaytonaSnapshot struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ImageName string `json:"imageName"`
	State     string `json:"state"`
	RegionID  string `json:"regionId"`
}

type DaytonaClient struct {
	APIURL         string
	APIKey         string
	OrganizationID string
	HTTPClient     HTTPDoer
}

func BuildDaytonaMetadata(name string, opts DaytonaCreateOptions) DaytonaMetadata {
	apiURL := strings.TrimRight(opts.APIURL, "/")
	if apiURL == "" {
		apiURL = DefaultDaytonaAPIURL
	}
	apiKeyEnv := opts.APIKeyEnv
	if apiKeyEnv == "" {
		apiKeyEnv = DefaultDaytonaAPIKeyEnv
	}
	orgEnv := opts.OrganizationIDEnv
	if orgEnv == "" {
		orgEnv = DefaultDaytonaOrganizationIDEnv
	}
	snapshot := opts.Snapshot
	if snapshot == "" {
		snapshot = DefaultDaytonaSnapshot
	}
	labels := map[string]string{
		"liza.dev/capsule": name,
		"liza.dev/runtime": "daytona",
	}
	for key, value := range opts.Labels {
		labels[key] = value
	}
	return DaytonaMetadata{
		APIURL:            apiURL,
		APIKeyEnv:         apiKeyEnv,
		OrganizationIDEnv: orgEnv,
		Target:            opts.Target,
		Snapshot:          snapshot,
		SandboxName:       "liza-" + ContainerName(name),
		CPU:               opts.CPU,
		MemoryGB:          opts.MemoryGB,
		DiskGB:            opts.DiskGB,
		AutoStopMinutes:   opts.AutoStopMinutes,
		AutoDeleteMinutes: opts.AutoDeleteMinutes,
		Labels:            labels,
	}
}

func NewDaytonaClient(apiURL, apiKey, organizationID string, httpClient HTTPDoer) DaytonaClient {
	if strings.TrimSpace(apiURL) == "" {
		apiURL = DefaultDaytonaAPIURL
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return DaytonaClient{
		APIURL:         strings.TrimRight(apiURL, "/"),
		APIKey:         apiKey,
		OrganizationID: organizationID,
		HTTPClient:     httpClient,
	}
}

func (c DaytonaClient) CreateSandbox(ctx context.Context, meta DaytonaMetadata, env map[string]string) (DaytonaSandbox, error) {
	body := map[string]any{
		"name":               meta.SandboxName,
		"snapshot":           meta.Snapshot,
		"env":                env,
		"labels":             meta.Labels,
		"public":             false,
		"autoStopInterval":   meta.AutoStopMinutes,
		"autoDeleteInterval": meta.AutoDeleteMinutes,
	}
	if meta.Target != "" {
		body["target"] = meta.Target
	}
	if meta.CPU > 0 {
		body["cpu"] = meta.CPU
	}
	if meta.MemoryGB > 0 {
		body["memory"] = meta.MemoryGB
	}
	if meta.DiskGB > 0 {
		body["disk"] = meta.DiskGB
	}
	var sandbox DaytonaSandbox
	if err := c.doJSON(ctx, http.MethodPost, "/sandbox", nil, body, &sandbox); err != nil {
		return DaytonaSandbox{}, err
	}
	return sandbox, nil
}

func (c DaytonaClient) CreateSnapshot(ctx context.Context, opts DaytonaSnapshotCreateOptions) (DaytonaSnapshot, error) {
	if strings.TrimSpace(opts.Name) == "" {
		return DaytonaSnapshot{}, fmt.Errorf("snapshot name is required")
	}
	if strings.TrimSpace(opts.ImageName) == "" {
		return DaytonaSnapshot{}, fmt.Errorf("snapshot image name is required")
	}
	if strings.HasSuffix(opts.ImageName, ":latest") {
		return DaytonaSnapshot{}, fmt.Errorf("daytona snapshots reject image tag :latest; use an immutable tag")
	}
	body := map[string]any{
		"name":      opts.Name,
		"imageName": opts.ImageName,
	}
	if len(opts.Entrypoint) > 0 {
		body["entrypoint"] = opts.Entrypoint
	}
	if opts.RegionID != "" {
		body["regionId"] = opts.RegionID
	}
	if opts.SandboxClass != "" {
		body["sandboxClass"] = opts.SandboxClass
	}
	if opts.CPU > 0 {
		body["cpu"] = opts.CPU
	}
	if opts.MemoryGB > 0 {
		body["memory"] = opts.MemoryGB
	}
	if opts.DiskGB > 0 {
		body["disk"] = opts.DiskGB
	}
	var snapshot DaytonaSnapshot
	if err := c.doJSON(ctx, http.MethodPost, "/snapshots", nil, body, &snapshot); err != nil {
		return DaytonaSnapshot{}, err
	}
	return snapshot, nil
}

func (c DaytonaClient) GetSandbox(ctx context.Context, sandboxIDOrName string) (DaytonaSandbox, error) {
	var sandbox DaytonaSandbox
	if err := c.doJSON(ctx, http.MethodGet, "/sandbox/"+url.PathEscape(sandboxIDOrName), map[string]string{"verbose": "true"}, nil, &sandbox); err != nil {
		return DaytonaSandbox{}, err
	}
	return sandbox, nil
}

func (c DaytonaClient) StartSandbox(ctx context.Context, sandboxIDOrName string) (DaytonaSandbox, error) {
	var sandbox DaytonaSandbox
	if err := c.doJSON(ctx, http.MethodPost, "/sandbox/"+url.PathEscape(sandboxIDOrName)+"/start", nil, nil, &sandbox); err != nil {
		return DaytonaSandbox{}, err
	}
	return sandbox, nil
}

func (c DaytonaClient) StopSandbox(ctx context.Context, sandboxIDOrName string, force bool) (DaytonaSandbox, error) {
	query := map[string]string{}
	if force {
		query["force"] = "true"
	}
	var sandbox DaytonaSandbox
	if err := c.doJSON(ctx, http.MethodPost, "/sandbox/"+url.PathEscape(sandboxIDOrName)+"/stop", query, nil, &sandbox); err != nil {
		return DaytonaSandbox{}, err
	}
	return sandbox, nil
}

func (c DaytonaClient) DeleteSandbox(ctx context.Context, sandboxIDOrName string) (DaytonaSandbox, error) {
	var sandbox DaytonaSandbox
	if err := c.doJSON(ctx, http.MethodDelete, "/sandbox/"+url.PathEscape(sandboxIDOrName), nil, nil, &sandbox); err != nil {
		return DaytonaSandbox{}, err
	}
	return sandbox, nil
}

func (c DaytonaClient) ExecuteCommand(ctx context.Context, sandboxID, command, cwd string, timeoutSeconds int) (DaytonaExecuteResponse, error) {
	body := map[string]any{"command": command}
	if cwd != "" {
		body["cwd"] = cwd
	}
	if timeoutSeconds > 0 {
		body["timeout"] = timeoutSeconds
	}
	var response DaytonaExecuteResponse
	if err := c.doJSON(ctx, http.MethodPost, "/toolbox/"+url.PathEscape(sandboxID)+"/toolbox/process/execute", nil, body, &response); err != nil {
		return DaytonaExecuteResponse{}, err
	}
	return response, nil
}

func (c DaytonaClient) doJSON(ctx context.Context, method, path string, query map[string]string, body any, out any) error {
	if strings.TrimSpace(c.APIKey) == "" {
		return fmt.Errorf("daytona API key is required; set %s", DefaultDaytonaAPIKeyEnv)
	}
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to encode daytona request: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.APIURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "liza-capsule/1")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.OrganizationID != "" {
		req.Header.Set("X-Daytona-Organization-ID", c.OrganizationID)
	}
	if len(query) > 0 {
		values := req.URL.Query()
		for key, value := range query {
			values.Set(key, value)
		}
		req.URL.RawQuery = values.Encode()
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("daytona request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("failed to read daytona response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("daytona request failed with HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("failed to decode daytona response: %w", err)
	}
	return nil
}

func ApplyDaytonaSandbox(meta *CapsuleMetadata, sandbox DaytonaSandbox, now time.Time) {
	if meta.Daytona == nil {
		metadata := BuildDaytonaMetadata(meta.Name, DaytonaCreateOptions{})
		meta.Daytona = &metadata
	}
	meta.Daytona.SandboxID = sandbox.ID
	meta.Daytona.SandboxName = sandbox.Name
	meta.Daytona.SandboxState = sandbox.State
	meta.Daytona.ToolboxProxyURL = sandbox.ToolboxProxyURL
	if sandbox.Target != "" {
		meta.Daytona.Target = sandbox.Target
	}
	if sandbox.Snapshot != "" {
		meta.Daytona.Snapshot = sandbox.Snapshot
	}
	meta.Daytona.LastSyncedAt = now
}

func DaytonaCommand(command []string) string {
	if len(command) == 0 {
		return ""
	}
	quoted := make([]string, len(command))
	for i, part := range command {
		quoted[i] = shellQuote(part)
	}
	return strings.Join(quoted, " ")
}

func ShellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return !(r >= 'A' && r <= 'Z') &&
			!(r >= 'a' && r <= 'z') &&
			!(r >= '0' && r <= '9') &&
			!strings.ContainsRune("_+-=/:.,", r)
	}) == -1 {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func shellQuote(value string) string {
	return ShellQuote(value)
}
