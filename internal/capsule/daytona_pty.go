package capsule

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type DaytonaPtyRunOptions struct {
	SessionID string
	Command   string
	Cwd       string
	Env       map[string]string
	Rows      int
	Cols      int
	Stdin     io.Reader
	Stdout    io.Writer
}

type daytonaPtyCreateResponse struct {
	SessionID string `json:"sessionId"`
}

type daytonaPtyControlMessage struct {
	Type   string `json:"type"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type daytonaPtyExitData struct {
	ExitCode   *int    `json:"exitCode,omitempty"`
	ExitReason *string `json:"exitReason,omitempty"`
	Error      *string `json:"error,omitempty"`
}

func (c DaytonaClient) RunPtyCommand(ctx context.Context, sandboxID, toolboxProxyURL string, opts DaytonaPtyRunOptions) (int, error) {
	if strings.TrimSpace(opts.Command) == "" {
		return 0, fmt.Errorf("daytona PTY command is required")
	}
	baseURL, err := daytonaToolboxSandboxURL(toolboxProxyURL, sandboxID)
	if err != nil {
		return 0, err
	}
	sessionID := opts.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("liza-%d", time.Now().UnixNano())
	}
	rows := opts.Rows
	if rows <= 0 {
		rows = 24
	}
	cols := opts.Cols
	if cols <= 0 {
		cols = 80
	}
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	if opts.Stdin == nil {
		opts.Stdin = strings.NewReader("")
	}

	created, err := c.createPtySession(ctx, baseURL, daytonaPtyCreateRequest{
		ID:        sessionID,
		Cwd:       opts.Cwd,
		Envs:      opts.Env,
		LazyStart: true,
		Rows:      rows,
		Cols:      cols,
	})
	if err != nil {
		return 0, err
	}
	if created.SessionID != "" {
		sessionID = created.SessionID
	}

	headers := c.daytonaToolboxHeaders()
	wsURL := strings.TrimRight(httpToWebSocketURL(baseURL), "/") + "/process/pty/" + url.PathEscape(sessionID) + "/connect"
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		return 0, fmt.Errorf("failed to connect to Daytona PTY: %w", err)
	}
	defer conn.Close()

	connected := make(chan error, 1)
	readDone := make(chan daytonaPtyExitData, 1)
	var writeMu sync.Mutex
	go readDaytonaPty(ctx, conn, opts.Stdout, connected, readDone)

	select {
	case err := <-connected:
		if err != nil {
			return 0, err
		}
	case <-time.After(10 * time.Second):
		return 0, fmt.Errorf("timed out waiting for Daytona PTY connection")
	case <-ctx.Done():
		return 0, ctx.Err()
	}

	if err := writeDaytonaPty(&writeMu, conn, []byte(opts.Command+"\n")); err != nil {
		return 0, err
	}
	inputDone := make(chan error, 1)
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, readErr := opts.Stdin.Read(buf)
			if n > 0 {
				if err := writeDaytonaPty(&writeMu, conn, buf[:n]); err != nil {
					inputDone <- err
					return
				}
			}
			if readErr != nil {
				if readErr == io.EOF {
					inputDone <- nil
				} else {
					inputDone <- readErr
				}
				return
			}
		}
	}()

	select {
	case exit := <-readDone:
		if exit.Error != nil && *exit.Error != "" {
			return exitCodeOrDefault(exit.ExitCode), fmt.Errorf("daytona PTY failed: %s", *exit.Error)
		}
		// Only treat ExitReason as failure if exit code is non-zero
		// Successful exits may have ExitReason like "completed"
		if exit.ExitReason != nil && *exit.ExitReason != "" && exitCodeOrDefault(exit.ExitCode) != 0 {
			return exitCodeOrDefault(exit.ExitCode), fmt.Errorf("daytona PTY exited: %s", *exit.ExitReason)
		}
		return exitCodeOrDefault(exit.ExitCode), nil
	case err := <-inputDone:
		if err != nil {
			return 0, fmt.Errorf("failed to copy input to Daytona PTY: %w", err)
		}
		exit := <-readDone
		return exitCodeOrDefault(exit.ExitCode), nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

type daytonaPtyCreateRequest struct {
	ID        string            `json:"id,omitempty"`
	Cwd       string            `json:"cwd,omitempty"`
	Envs      map[string]string `json:"envs,omitempty"`
	LazyStart bool              `json:"lazyStart,omitempty"`
	Rows      int               `json:"rows,omitempty"`
	Cols      int               `json:"cols,omitempty"`
}

func (c DaytonaClient) createPtySession(ctx context.Context, baseURL string, body daytonaPtyCreateRequest) (daytonaPtyCreateResponse, error) {
	var response daytonaPtyCreateResponse
	if err := c.doToolboxJSON(ctx, http.MethodPost, baseURL, "/process/pty", body, &response); err != nil {
		return daytonaPtyCreateResponse{}, err
	}
	return response, nil
}

func (c DaytonaClient) doToolboxJSON(ctx context.Context, method, baseURL, path string, body any, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to encode Daytona toolbox request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(baseURL, "/")+path, strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	for key, values := range c.daytonaToolboxHeaders() {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("daytona toolbox request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("failed to read daytona toolbox response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("daytona toolbox request failed with HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("failed to decode daytona toolbox response: %w", err)
	}
	return nil
}

func (c DaytonaClient) daytonaToolboxHeaders() http.Header {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+c.APIKey)
	headers.Set("User-Agent", "liza-capsule/1")
	if c.OrganizationID != "" {
		headers.Set("X-Daytona-Organization-ID", c.OrganizationID)
	}
	return headers
}

func daytonaToolboxSandboxURL(toolboxProxyURL, sandboxID string) (string, error) {
	if strings.TrimSpace(toolboxProxyURL) == "" {
		return "", fmt.Errorf("daytona sandbox is missing toolbox proxy URL; run capsule doctor or start again to refresh metadata")
	}
	if strings.TrimSpace(sandboxID) == "" {
		return "", fmt.Errorf("daytona sandbox ID is required")
	}
	return strings.TrimRight(toolboxProxyURL, "/") + "/" + url.PathEscape(sandboxID), nil
}

func httpToWebSocketURL(value string) string {
	if strings.HasPrefix(value, "https://") {
		return "wss://" + strings.TrimPrefix(value, "https://")
	}
	if strings.HasPrefix(value, "http://") {
		return "ws://" + strings.TrimPrefix(value, "http://")
	}
	return value
}

func readDaytonaPty(ctx context.Context, conn *websocket.Conn, stdout io.Writer, connected chan<- error, done chan<- daytonaPtyExitData) {
	connectedSent := false
	sendConnected := func(err error) {
		if connectedSent {
			return
		}
		connectedSent = true
		connected <- err
	}
	for {
		select {
		case <-ctx.Done():
			sendConnected(ctx.Err())
			done <- daytonaPtyExitData{Error: stringPtr(ctx.Err().Error())}
			return
		default:
		}
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			sendConnected(nil)
			if closeErr, ok := err.(*websocket.CloseError); ok {
				done <- parseDaytonaPtyClose(closeErr.Text)
				return
			}
			done <- daytonaPtyExitData{Error: stringPtr(err.Error())}
			return
		}
		if messageType == websocket.TextMessage {
			var ctrl daytonaPtyControlMessage
			if err := json.Unmarshal(data, &ctrl); err == nil && ctrl.Type == "control" {
				switch ctrl.Status {
				case "connected":
					sendConnected(nil)
				case "error":
					if ctrl.Error == "" {
						ctrl.Error = "unknown Daytona PTY connection error"
					}
					sendConnected(fmt.Errorf("daytona PTY connection failed: %s", ctrl.Error))
					done <- daytonaPtyExitData{Error: &ctrl.Error}
					return
				}
				continue
			}
		}
		sendConnected(nil)
		if len(data) > 0 {
			_, _ = stdout.Write(data)
		}
	}
}

func writeDaytonaPty(mu *sync.Mutex, conn *websocket.Conn, data []byte) error {
	mu.Lock()
	defer mu.Unlock()
	if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return fmt.Errorf("failed to write Daytona PTY input: %w", err)
	}
	return nil
}

func parseDaytonaPtyClose(reason string) daytonaPtyExitData {
	if reason == "" {
		code := 0
		return daytonaPtyExitData{ExitCode: &code}
	}
	var exit daytonaPtyExitData
	if err := json.Unmarshal([]byte(reason), &exit); err == nil {
		return exit
	}
	code := 0
	return daytonaPtyExitData{ExitCode: &code}
}

func exitCodeOrDefault(code *int) int {
	if code == nil {
		return 0
	}
	return *code
}

func stringPtr(value string) *string {
	return &value
}
