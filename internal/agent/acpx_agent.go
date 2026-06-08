package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// ACPXAgent implements LLMAgent through the headless acpx ACP client.
type ACPXAgent struct {
	outputsDir string
	mu         sync.Mutex
	seen       map[string]bool
}

// NewACPXAgent creates an ACP-backed LLM agent using acpx.
func NewACPXAgent(outputsDir string) *ACPXAgent {
	return &ACPXAgent{outputsDir: outputsDir, seen: make(map[string]bool)}
}

func (a *ACPXAgent) Run(ctx context.Context, req LLMAgentRunRequest) (LLMAgentRunResult, error) {
	acpxAgent := acpxAgentName(req.BackendName)
	sessionName := acpxSessionName(req.AgentID)
	a.mu.Lock()
	warm := a.seen[sessionName]
	a.seen[sessionName] = true
	a.mu.Unlock()

	emitLLMAgentEvent(ctx, req.EventSink, LLMAgentEvent{
		Kind:        LLMAgentEventStarted,
		BackendName: req.BackendName,
		AgentID:     req.AgentID,
		TaskID:      req.TaskID,
		SessionID:   sessionName,
		Payload: map[string]any{
			"mode":       "acpx",
			"acpx_agent": acpxAgent,
		},
	})

	if err := a.ensureSession(ctx, req.ProjectRoot, acpxAgent, sessionName); err != nil {
		emitLLMAgentEvent(ctx, req.EventSink, LLMAgentEvent{
			Kind:        LLMAgentEventCompleted,
			BackendName: req.BackendName,
			AgentID:     req.AgentID,
			TaskID:      req.TaskID,
			SessionID:   sessionName,
			Message:     err.Error(),
			Payload: map[string]any{
				"error": err.Error(),
			},
		})
		return LLMAgentRunResult{ExitCode: 1, Output: err.Error(), WarmUsage: warm, SessionID: sessionName}, err
	}

	output, usage, err := a.prompt(ctx, req.ProjectRoot, acpxAgent, sessionName, req.Prompt)
	for _, chunk := range output.Chunks {
		emitLLMAgentEvent(ctx, req.EventSink, LLMAgentEvent{
			Kind:        LLMAgentEventMessage,
			BackendName: req.BackendName,
			AgentID:     req.AgentID,
			TaskID:      req.TaskID,
			SessionID:   sessionName,
			Message:     chunk,
		})
	}
	emitLLMAgentEvent(ctx, req.EventSink, LLMAgentEvent{
		Kind:        LLMAgentEventUsage,
		BackendName: req.BackendName,
		AgentID:     req.AgentID,
		TaskID:      req.TaskID,
		SessionID:   sessionName,
		Payload: map[string]any{
			"usage": usage,
		},
	})

	if err != nil {
		emitLLMAgentEvent(ctx, req.EventSink, LLMAgentEvent{
			Kind:        LLMAgentEventCompleted,
			BackendName: req.BackendName,
			AgentID:     req.AgentID,
			TaskID:      req.TaskID,
			SessionID:   sessionName,
			Message:     err.Error(),
			Payload: map[string]any{
				"error": err.Error(),
			},
		})
		return LLMAgentRunResult{ExitCode: 1, Output: output.Text, Usage: usage, WarmUsage: warm, SessionID: sessionName}, err
	}

	emitLLMAgentEvent(ctx, req.EventSink, LLMAgentEvent{
		Kind:        LLMAgentEventCompleted,
		BackendName: req.BackendName,
		AgentID:     req.AgentID,
		TaskID:      req.TaskID,
		SessionID:   sessionName,
		Payload: map[string]any{
			"exit_code": 0,
		},
	})
	return LLMAgentRunResult{ExitCode: 0, Output: output.Text, Usage: usage, WarmUsage: warm, SessionID: sessionName}, nil
}

func (a *ACPXAgent) RunInteractive(context.Context, LLMAgentInteractiveRequest) (int, error) {
	return 1, fmt.Errorf("interactive mode is not supported by ACPXAgent")
}

func (a *ACPXAgent) ensureSession(ctx context.Context, cwd, acpxAgent, sessionName string) error {
	args := []string{"--cwd", cwd, acpxAgent, "sessions", "ensure", "--name", sessionName}
	out, err := exec.CommandContext(ctx, "acpx", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("acpx sessions ensure: %w\n%s", err, string(out))
	}
	return nil
}

func (a *ACPXAgent) prompt(ctx context.Context, cwd, acpxAgent, sessionName, prompt string) (acpxOutput, LLMAgentUsage, error) {
	args := []string{
		"--cwd", cwd,
		"--format", "json",
		"--approve-all",
		acpxAgent,
		"prompt",
		"-s", sessionName,
		"--file", "-",
	}
	cmd := exec.CommandContext(ctx, "acpx", args...)
	cmd.Stdin = strings.NewReader(prompt)
	out, err := cmd.CombinedOutput()
	output, usage := parseACPXAgentOutput(string(out))
	if err != nil {
		return output, usage, fmt.Errorf("acpx prompt: %w\n%s", err, string(out))
	}
	return output, usage, nil
}

type acpxOutput struct {
	Text   string
	Chunks []string
}

func parseACPXAgentOutput(raw string) (acpxOutput, LLMAgentUsage) {
	var output acpxOutput
	var usage LLMAgentUsage
	scanner := bufio.NewScanner(strings.NewReader(raw))
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg map[string]any
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if params, ok := msg["params"].(map[string]any); ok {
			if update, ok := params["update"].(map[string]any); ok {
				if chunk := acpxAgentMessageChunk(update); chunk != "" {
					output.Chunks = append(output.Chunks, chunk)
					output.Text += chunk
				}
			}
		}
		if result, ok := msg["result"].(map[string]any); ok {
			if parsed, ok := acpxAgentUsage(result["usage"]); ok {
				usage = parsed
			}
		}
	}
	return output, usage
}

func acpxAgentMessageChunk(update map[string]any) string {
	if update["sessionUpdate"] != "agent_message_chunk" {
		return ""
	}
	content, ok := update["content"].(map[string]any)
	if !ok {
		return ""
	}
	text, ok := content["text"].(string)
	if !ok {
		return ""
	}
	return text
}

func acpxAgentUsage(raw any) (LLMAgentUsage, bool) {
	usage, ok := raw.(map[string]any)
	if !ok {
		return LLMAgentUsage{}, false
	}
	return LLMAgentUsage{
		InputTokens:      acpxInt(usage["inputTokens"]),
		OutputTokens:     acpxInt(usage["outputTokens"]),
		CachedReadTokens: acpxInt(usage["cachedReadTokens"]),
	}, true
}

func acpxInt(raw any) int {
	switch v := raw.(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

func acpxAgentName(cliName string) string {
	switch cliName {
	case "codex-acp", "acpx-codex":
		return "codex"
	default:
		if strings.HasPrefix(cliName, "acpx-") {
			return strings.TrimPrefix(cliName, "acpx-")
		}
		if strings.HasSuffix(cliName, "-acp") {
			return strings.TrimSuffix(cliName, "-acp")
		}
		return cliName
	}
}

func acpxSessionName(agentID string) string {
	if agentID == "" {
		return "liza-agent"
	}
	return "liza-" + agentID
}
