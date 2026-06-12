package capsule

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestDaytonaRunPtyCommandUsesWorkspaceCwdAndStreamsCommand(t *testing.T) {
	var createRequest daytonaPtyCreateRequest
	var receivedInput string
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/toolbox/sbx/process/pty":
			if err := json.NewDecoder(r.Body).Decode(&createRequest); err != nil {
				t.Fatalf("decode create request: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"sessionId":"session"}`))
		case r.URL.Path == "/toolbox/sbx/process/pty/session/connect":
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Fatalf("upgrade websocket: %v", err)
			}
			defer conn.Close()
			if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"control","status":"connected"}`)); err != nil {
				t.Fatalf("write control: %v", err)
			}
			_, data, err := conn.ReadMessage()
			if err != nil {
				t.Fatalf("read input: %v", err)
			}
			receivedInput = string(data)
			if err := conn.WriteMessage(websocket.BinaryMessage, []byte("started\n")); err != nil {
				t.Fatalf("write output: %v", err)
			}
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, `{"exitCode":0}`), time.Now().Add(time.Second))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewDaytonaClient("https://app.daytona.io/api", "token", "", server.Client())
	var stdout strings.Builder
	exitCode, err := client.RunPtyCommand(context.Background(), "sbx", server.URL+"/toolbox", DaytonaPtyRunOptions{
		SessionID: "session",
		Command:   "exec liza tui",
		Cwd:       DaytonaWorkspaceDir,
		Env:       map[string]string{"TERM": "xterm-256color"},
		Rows:      40,
		Cols:      120,
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
	})
	if err != nil {
		t.Fatalf("RunPtyCommand failed: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}
	if createRequest.Cwd != DaytonaWorkspaceDir {
		t.Fatalf("cwd = %q, want %q", createRequest.Cwd, DaytonaWorkspaceDir)
	}
	if createRequest.Envs["TERM"] != "xterm-256color" {
		t.Fatalf("envs = %#v", createRequest.Envs)
	}
	if receivedInput != "exec liza tui\n" {
		t.Fatalf("received input = %q", receivedInput)
	}
	if stdout.String() != "started\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
