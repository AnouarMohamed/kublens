package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"kubelens-backend/internal/redact"
)

const defaultBaseURL = "https://api.openai.com/v1"

type OpenAICompatibleConfig struct {
	BaseURL     string
	APIKey      string
	Model       string
	Temperature float64
	MaxTokens   int
	HTTPClient  *http.Client
}

type OpenAICompatibleProvider struct {
	baseURL     string
	apiKey      string
	model       string
	temperature float64
	maxTokens   int
	client      *http.Client
}

func NewOpenAICompatibleProvider(cfg OpenAICompatibleConfig) (*OpenAICompatibleProvider, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("missing API key")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, errors.New("missing model")
	}

	temperature := cfg.Temperature
	if temperature == 0 {
		temperature = 0.2
	}
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 2048
	}

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}

	return &OpenAICompatibleProvider{
		baseURL:     baseURL,
		apiKey:      cfg.APIKey,
		model:       cfg.Model,
		temperature: temperature,
		maxTokens:   maxTokens,
		client:      client,
	}, nil
}

func (p *OpenAICompatibleProvider) Name() string {
	return "openai-compatible"
}

func (p *OpenAICompatibleProvider) Generate(ctx context.Context, in Input) (string, error) {
	body, err := json.Marshal(chatCompletionsRequest{
		Model: p.model,
		Messages: []chatMessage{
			{Role: "system", Content: SystemPromptWithContext(in.SystemContext)},
			{Role: "user", Content: UserPrompt(in)},
		},
		Temperature: p.temperature,
		MaxTokens:   p.maxTokens,
	})
	if err != nil {
		return "", fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request provider: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
		return "", fmt.Errorf("provider status %d: %s", resp.StatusCode, redact.SensitiveText(string(payload)))
	}

	var out chatCompletionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode provider response: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", errors.New("provider returned no choices")
	}

	answer := strings.TrimSpace(out.Choices[0].Message.Content)
	if answer == "" {
		return "", errors.New("provider returned empty answer")
	}
	return answer, nil
}

func (p *OpenAICompatibleProvider) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = p.model
	}
	temperature := req.Temperature
	if temperature == 0 {
		temperature = p.temperature
	}
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = p.maxTokens
	}

	messages := make([]chatMessage, 0, len(req.Messages))
	for _, msg := range req.Messages {
		messages = append(messages, chatMessage{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCalls:  mapToolCalls(msg.ToolCalls),
			ToolCallID: msg.ToolCallID,
		})
	}

	tools := make([]toolDefinition, 0, len(req.Tools))
	for _, tool := range req.Tools {
		tools = append(tools, toolDefinition{
			Type: "function",
			Function: toolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}

	payload := chatCompletionsRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
	}
	if len(tools) > 0 {
		payload.Tools = tools
		payload.ToolChoice = "auto"
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("encode request: %w", err)
	}

	reqHTTP, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ChatResponse{}, fmt.Errorf("build request: %w", err)
	}

	reqHTTP.Header.Set("Authorization", "Bearer "+p.apiKey)
	reqHTTP.Header.Set("Content-Type", "application/json")
	reqHTTP.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(reqHTTP)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("request provider: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
		return ChatResponse{}, fmt.Errorf("provider status %d: %s", resp.StatusCode, redact.SensitiveText(string(payload)))
	}

	var out chatCompletionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ChatResponse{}, fmt.Errorf("decode provider response: %w", err)
	}
	if len(out.Choices) == 0 {
		return ChatResponse{}, errors.New("provider returned no choices")
	}

	msg := out.Choices[0].Message
	return ChatResponse{
		Content:   strings.TrimSpace(msg.Content),
		ToolCalls: readToolCalls(msg.ToolCalls),
	}, nil
}

type chatCompletionsRequest struct {
	Model       string           `json:"model"`
	Messages    []chatMessage    `json:"messages"`
	Temperature float64          `json:"temperature,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Tools       []toolDefinition `json:"tools,omitempty"`
	ToolChoice  any              `json:"tool_choice,omitempty"`
}

type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type chatCompletionsResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

type toolDefinition struct {
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

type toolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type toolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function toolCallFunction `json:"function"`
}

type toolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func mapToolCalls(in []ToolCall) []toolCall {
	if len(in) == 0 {
		return nil
	}
	out := make([]toolCall, 0, len(in))
	for _, call := range in {
		out = append(out, toolCall{
			ID:   call.ID,
			Type: "function",
			Function: toolCallFunction{
				Name:      call.Name,
				Arguments: call.Arguments,
			},
		})
	}
	return out
}

func readToolCalls(in []toolCall) []ToolCall {
	if len(in) == 0 {
		return nil
	}
	out := make([]ToolCall, 0, len(in))
	for _, call := range in {
		out = append(out, ToolCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: call.Function.Arguments,
		})
	}
	return out
}
