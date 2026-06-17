package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"heyai/guard"
	"io"
	"net/http"
	"strings"
)

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Tools    []ToolDef `json:"tools,omitempty"`
}

type AutoCheckResult struct {
	Safe   bool   `json:"safe"`
	Reason string `json:"reason"`
}

type ToolDef struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

type FunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type OpenAIClient struct {
	HTTPClient *http.Client
	BaseURL    string
	APIKey     string
	Model      string
}

func NewOpenAIClient(cfg Config) *OpenAIClient {
	return &OpenAIClient{HTTPClient: http.DefaultClient, BaseURL: strings.TrimRight(cfg.BaseURL, "/"), APIKey: cfg.APIKey, Model: cfg.Model}
}

func NewAutoCheckClient(cfg Config) *OpenAIClient {
	apiKey := cfg.AutoCheck.APIKey
	if apiKey == "" {
		apiKey = cfg.APIKey
	}
	baseURL := cfg.AutoCheck.BaseURL
	if baseURL == "" {
		baseURL = cfg.BaseURL
	}
	model := cfg.AutoCheck.Model
	if model == "" {
		model = cfg.Model
	}
	return &OpenAIClient{HTTPClient: http.DefaultClient, BaseURL: strings.TrimRight(baseURL, "/"), APIKey: apiKey, Model: model}
}

func (c *OpenAIClient) Chat(ctx context.Context, messages []Message) (Message, error) {
	body := chatRequest{Model: c.Model, Messages: messages, Tools: []ToolDef{bashToolDef()}}
	return c.chat(ctx, body)
}

func (c *OpenAIClient) CheckBashSafety(ctx context.Context, args BashArgs, guardResult guard.GuardResult) (AutoCheckResult, error) {
	messages := []Message{
		{Role: "system", Content: "You are a strict command safety reviewer for a local CLI. Decide whether a bash command that the static guard marked as needing confirmation is safe to run automatically. Approve only when the command is clearly limited to the current project/workdir, has understandable effects, and does not access secrets, credentials, private system files, network services, package installs, privilege escalation, destructive broad deletes, or paths outside the project. Respond only as JSON: {\"safe\":true|false,\"reason\":\"short reason\"}."},
		{Role: "user", Content: fmt.Sprintf("Command: %s\nWorkdir: %s\nTool description: %s\nStatic guard reason: %s", args.Command, args.Workdir, args.Description, guardResult.Reason)},
	}
	body := chatRequest{Model: c.Model, Messages: messages}
	msg, err := c.chat(ctx, body)
	if err != nil {
		return AutoCheckResult{}, err
	}
	var result AutoCheckResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(msg.Content)), &result); err != nil {
		return AutoCheckResult{}, fmt.Errorf("auto check returned invalid JSON: %w", err)
	}
	if result.Reason == "" {
		result.Reason = "no reason provided"
	}
	return result, nil
}

func (c *OpenAIClient) chat(ctx context.Context, body chatRequest) (Message, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return Message{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatCompletionsURL(c.BaseURL), bytes.NewReader(b))
	if err != nil {
		return Message{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return Message{}, err
	}
	defer resp.Body.Close()
	rb, err := io.ReadAll(resp.Body)
	if err != nil {
		return Message{}, err
	}
	trimmed := strings.TrimSpace(string(rb))
	if trimmed == "" {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return Message{}, fmt.Errorf("api error: %s with empty response body", resp.Status)
		}
		return Message{}, fmt.Errorf("api returned empty response body for %s", chatCompletionsURL(c.BaseURL))
	}

	var parsed chatResponse
	if err := json.Unmarshal(rb, &parsed); err != nil {
		return Message{}, fmt.Errorf("api returned invalid JSON (%s): %s: %w", resp.Status, responsePreview(trimmed), err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if parsed.Error != nil && parsed.Error.Message != "" {
			return Message{}, fmt.Errorf("api error: %s", parsed.Error.Message)
		}
		return Message{}, fmt.Errorf("api error: %s", resp.Status)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return Message{}, fmt.Errorf("api error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return Message{}, fmt.Errorf("api returned no choices")
	}
	return parsed.Choices[0].Message, nil
}

func responsePreview(s string) string {
	const max = 200
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

func chatCompletionsURL(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(baseURL, "/v1") {
		return baseURL + "/chat/completions"
	}
	return baseURL + "/v1/chat/completions"
}

func bashToolDef() ToolDef {
	return ToolDef{Type: "function", Function: FunctionDef{Name: "bash", Description: "Run a guarded bash command in the current working directory.", Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command":     map[string]string{"type": "string"},
			"description": map[string]string{"type": "string"},
			"timeout_ms":  map[string]string{"type": "integer"},
			"workdir":     map[string]string{"type": "string"},
		},
		"required": []string{"command"},
	}}}
}
