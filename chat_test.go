package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIClientSendsToolSchema(t *testing.T) {
	var sawTool bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if len(req.Tools) == 1 && req.Tools[0].Function.Name == "bash" {
			sawTool = true
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	client := NewOpenAIClient(Config{APIKey: "k", BaseURL: srv.URL, Model: "m"})
	msg, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "ok" || !sawTool {
		t.Fatalf("content=%q sawTool=%v", msg.Content, sawTool)
	}
}

func TestChatCompletionsURLAcceptsV1Base(t *testing.T) {
	if got := chatCompletionsURL("https://example.test/v1"); got != "https://example.test/v1/chat/completions" {
		t.Fatalf("url=%q", got)
	}
	if got := chatCompletionsURL("https://example.test"); got != "https://example.test/v1/chat/completions" {
		t.Fatalf("url=%q", got)
	}
}

func TestChatToolLoop(t *testing.T) {
	client := &fakeClient{responses: []Message{
		{Role: "assistant", ToolCalls: []ToolCall{{ID: "1", Type: "function", Function: FunctionCall{Name: "bash", Arguments: `{"command":"pwd"}`}}}},
		{Role: "assistant", Content: "done"},
	}}
	var out bytes.Buffer
	chat := Chat{Client: client, Config: Config{MaxTurns: 4, Bash: BashConfig{TimeoutMS: 1000, MaxOutputBytes: 2000}}, Out: &out}
	if err := chat.Run(context.Background(), "run pwd"); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "done" {
		t.Fatalf("out=%q", out.String())
	}
	if len(client.calls) != 2 || len(client.calls[1]) < 3 || client.calls[1][2].Role != "tool" {
		t.Fatalf("tool result was not sent back: %#v", client.calls)
	}
}

func TestChatMalformedToolArgs(t *testing.T) {
	client := &fakeClient{responses: []Message{
		{Role: "assistant", ToolCalls: []ToolCall{{ID: "1", Type: "function", Function: FunctionCall{Name: "bash", Arguments: `{bad`}}}},
		{Role: "assistant", Content: "handled"},
	}}
	var out bytes.Buffer
	chat := Chat{Client: client, Config: Config{MaxTurns: 4, Bash: BashConfig{TimeoutMS: 1000, MaxOutputBytes: 2000}}, Out: &out}
	if err := chat.Run(context.Background(), "run"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(client.calls[1][2].Content, "malformed tool arguments") {
		t.Fatalf("content=%q", client.calls[1][2].Content)
	}
}

func TestChatMaxTurns(t *testing.T) {
	client := &fakeClient{responses: []Message{{Role: "assistant", ToolCalls: []ToolCall{{ID: "1", Type: "function", Function: FunctionCall{Name: "bash", Arguments: `{"command":"pwd"}`}}}}}}
	chat := Chat{Client: client, Config: Config{MaxTurns: 1, Bash: BashConfig{TimeoutMS: 1000, MaxOutputBytes: 2000}}}
	if err := chat.Run(context.Background(), "run"); err == nil || !strings.Contains(err.Error(), "max turns") {
		t.Fatalf("err=%v", err)
	}
}

func TestOpenAIClientAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"bad"}}`, http.StatusBadRequest)
	}))
	defer srv.Close()
	client := NewOpenAIClient(Config{APIKey: "k", BaseURL: srv.URL, Model: "m"})
	_, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("expected error")
	}
}

type fakeClient struct {
	responses []Message
	calls     [][]Message
}

func (f *fakeClient) Chat(ctx context.Context, messages []Message) (Message, error) {
	f.calls = append(f.calls, append([]Message(nil), messages...))
	if len(f.responses) == 0 {
		return Message{Role: "assistant", Content: ""}, nil
	}
	res := f.responses[0]
	f.responses = f.responses[1:]
	return res, nil
}
