package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

func (c *OpenAIClient) Chat(ctx context.Context, messages []Message) (Message, error) {
	body := chatRequest{Model: c.Model, Messages: messages, Tools: []ToolDef{bashToolDef()}}
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

	var parsed chatResponse
	if err := json.Unmarshal(rb, &parsed); err != nil {
		return Message{}, err
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
