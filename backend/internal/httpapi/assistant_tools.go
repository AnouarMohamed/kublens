package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"kubelens-backend/internal/ai"
	"kubelens-backend/internal/model"
	"kubelens-backend/internal/redact"
)

const (
	toolGetPodDetails  = "get_pod_details"
	toolGetPodLogs     = "get_pod_logs"
	toolGetNodeState   = "get_node_state"
	toolGetEvents      = "get_events"
	toolRunDiagnostics = "run_diagnostics"
)

var assistantTools = []ai.ToolDefinition{
	{
		Name:        toolGetPodDetails,
		Description: "Get pod details, including status, containers, and node placement.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"namespace": map[string]any{"type": "string", "description": "Pod namespace"},
				"name":      map[string]any{"type": "string", "description": "Pod name"},
			},
			"required": []string{"namespace", "name"},
		},
	},
	{
		Name:        toolGetPodLogs,
		Description: "Fetch recent pod logs for a specific container.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"namespace": map[string]any{"type": "string", "description": "Pod namespace"},
				"name":      map[string]any{"type": "string", "description": "Pod name"},
				"container": map[string]any{"type": "string", "description": "Container name (optional)"},
				"lines":     map[string]any{"type": "integer", "description": "Number of log lines (default 50)"},
			},
			"required": []string{"namespace", "name"},
		},
	},
	{
		Name:        toolGetNodeState,
		Description: "Get node detail and conditions.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string", "description": "Node name"},
			},
			"required": []string{"name"},
		},
	},
	{
		Name:        toolGetEvents,
		Description: "Fetch recent events. Provide namespace+name to scope to a pod, or leave blank for cluster events.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"namespace": map[string]any{"type": "string", "description": "Namespace (optional)"},
				"name":      map[string]any{"type": "string", "description": "Pod name (optional)"},
				"limit":     map[string]any{"type": "integer", "description": "Max events to return (default 10)"},
			},
		},
	},
	{
		Name:        toolRunDiagnostics,
		Description: "Run deterministic diagnostics on the current cluster snapshot.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
}

func (s *Server) generateAssistantWithTools(ctx context.Context, provider ai.ToolingProvider, in ai.Input) (string, error) {
	messages := []ai.ChatMessage{
		{Role: "system", Content: ai.SystemPromptWithContext(in.SystemContext)},
		{Role: "user", Content: ai.UserPrompt(in)},
	}

	const maxIterations = 4
	for i := 0; i < maxIterations; i++ {
		resp, err := provider.Chat(ctx, ai.ChatRequest{
			Messages: messages,
			Tools:    assistantTools,
		})
		if err != nil {
			return "", err
		}

		if len(resp.ToolCalls) == 0 {
			if strings.TrimSpace(resp.Content) == "" {
				return "", fmt.Errorf("assistant returned empty response")
			}
			return resp.Content, nil
		}

		messages = append(messages, ai.ChatMessage{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		for _, call := range resp.ToolCalls {
			result := s.executeAssistantTool(ctx, call)
			messages = append(messages, ai.ChatMessage{
				Role:       "tool",
				ToolCallID: call.ID,
				Content:    result,
			})
		}
	}

	return "", fmt.Errorf("assistant tool loop exceeded %d iterations", maxIterations)
}

func (s *Server) executeAssistantTool(ctx context.Context, call ai.ToolCall) string {
	switch call.Name {
	case toolGetPodDetails:
		var args struct {
			Namespace string `json:"namespace"`
			Name      string `json:"name"`
		}
		if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
			return marshalToolError("invalid arguments", err)
		}
		pod, err := s.cluster.PodDetail(ctx, args.Namespace, args.Name)
		if err != nil {
			return marshalToolError("pod detail failed", err)
		}
		return marshalToolResult(pod)
	case toolGetPodLogs:
		var args struct {
			Namespace string `json:"namespace"`
			Name      string `json:"name"`
			Container string `json:"container"`
			Lines     int    `json:"lines"`
		}
		if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
			return marshalToolError("invalid arguments", err)
		}
		lines := args.Lines
		if lines <= 0 {
			lines = 50
		}
		logs := s.cluster.PodLogs(ctx, args.Namespace, args.Name, strings.TrimSpace(args.Container), lines)
		return marshalToolResult(map[string]string{"logs": truncateString(logs, 4000)})
	case toolGetNodeState:
		var args struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
			return marshalToolError("invalid arguments", err)
		}
		node, err := s.cluster.NodeDetail(ctx, args.Name)
		if err != nil {
			return marshalToolError("node detail failed", err)
		}
		return marshalToolResult(node)
	case toolGetEvents:
		var args struct {
			Namespace string `json:"namespace"`
			Name      string `json:"name"`
			Limit     int    `json:"limit"`
		}
		if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
			return marshalToolError("invalid arguments", err)
		}
		limit := args.Limit
		if limit <= 0 {
			limit = 10
		}
		var events []model.K8sEvent
		if strings.TrimSpace(args.Namespace) != "" && strings.TrimSpace(args.Name) != "" {
			events = s.cluster.PodEvents(ctx, args.Namespace, args.Name)
		} else {
			events = s.cluster.ListClusterEvents(ctx)
		}
		if len(events) > limit {
			events = events[:limit]
		}
		return marshalToolResult(events)
	case toolRunDiagnostics:
		return marshalToolResult(s.mapDiagnosticsReport(s.runDiagnostics(ctx)))
	default:
		return marshalToolError("unknown tool", fmt.Errorf("tool %s not supported", call.Name))
	}
}

func marshalToolResult(data any) string {
	payload, err := json.Marshal(map[string]any{
		"ok":   true,
		"data": data,
	})
	if err != nil {
		return `{"ok":false,"error":"failed to encode tool result"}`
	}
	return string(payload)
}

func marshalToolError(message string, err error) string {
	payload, _ := json.Marshal(map[string]any{
		"ok":    false,
		"error": message + ": " + redact.Error(err),
	})
	return string(payload)
}

func detectIntent(lowerMessage string) assistantIntent {
	switch {
	case strings.Contains(lowerMessage, "manifest"),
		strings.Contains(lowerMessage, "yaml"),
		strings.Contains(lowerMessage, "deployment"):
		return intentManifest
	case strings.Contains(lowerMessage, "health"),
		strings.Contains(lowerMessage, "status"),
		strings.Contains(lowerMessage, "summary"):
		return intentHealth
	case strings.Contains(lowerMessage, "failed"),
		strings.Contains(lowerMessage, "pending"),
		strings.Contains(lowerMessage, "not ready"),
		strings.Contains(lowerMessage, "priority"):
		return intentPriority
	default:
		return intentUnknown
	}
}

func findPodByHint(pods []model.PodSummary, hint string) (model.PodSummary, bool) {
	needle := strings.ToLower(strings.TrimSpace(hint))
	for _, pod := range pods {
		if strings.EqualFold(pod.Name, needle) {
			return pod, true
		}
	}
	for _, pod := range pods {
		if strings.Contains(strings.ToLower(pod.Name), needle) {
			return pod, true
		}
	}
	return model.PodSummary{}, false
}

func collectIssueResources(issues []model.DiagnosticIssue) []string {
	out := make([]string, 0, len(issues))
	for _, issue := range issues {
		if issue.Resource != "" {
			out = append(out, issue.Resource)
		}
	}
	return out
}
