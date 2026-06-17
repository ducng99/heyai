package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"heyai/guard"
	"heyai/tool"
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

func TestParseRuntimeFlagsAutoAlias(t *testing.T) {
	auto, readOnly, verbose, promptArgs := parseRuntimeFlags([]string{"-a", "run", "pwd"})
	if !auto {
		t.Fatal("expected auto mode")
	}
	if readOnly {
		t.Fatal("did not expect readonly mode")
	}
	if verbose {
		t.Fatal("did not expect verbose mode")
	}
	if strings.Join(promptArgs, " ") != "run pwd" {
		t.Fatalf("promptArgs=%q", promptArgs)
	}
}

func TestParseRuntimeFlagsReadOnlyAlias(t *testing.T) {
	auto, readOnly, verbose, promptArgs := parseRuntimeFlags([]string{"-r", "run", "pwd"})
	if auto {
		t.Fatal("did not expect auto mode")
	}
	if !readOnly {
		t.Fatal("expected readonly mode")
	}
	if verbose {
		t.Fatal("did not expect verbose mode")
	}
	if strings.Join(promptArgs, " ") != "run pwd" {
		t.Fatalf("promptArgs=%q", promptArgs)
	}
}

func TestParseRuntimeFlagsVerboseAlias(t *testing.T) {
	auto, readOnly, verbose, promptArgs := parseRuntimeFlags([]string{"-v", "run", "pwd"})
	if auto {
		t.Fatal("did not expect auto mode")
	}
	if readOnly {
		t.Fatal("did not expect readonly mode")
	}
	if !verbose {
		t.Fatal("expected verbose mode")
	}
	if strings.Join(promptArgs, " ") != "run pwd" {
		t.Fatalf("promptArgs=%q", promptArgs)
	}
}

func TestChatToolLoop(t *testing.T) {
	client := &fakeClient{responses: []Message{
		{Role: "assistant", ToolCalls: []ToolCall{{ID: "1", Type: "function", Function: FunctionCall{Name: "bash", Arguments: `{"command":"pwd"}`}}}},
		{Role: "assistant", Content: "done"},
	}}
	var out bytes.Buffer
	chat := Chat{Client: client, Config: Config{MaxTurns: 4, Bash: tool.BashConfig{TimeoutMS: 1000, MaxOutputBytes: 2000}}, Out: &out}
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

func TestChatRendersAssistantMarkdown(t *testing.T) {
	client := &fakeClient{responses: []Message{{Role: "assistant", Content: "# Done"}}}
	var out bytes.Buffer
	chat := Chat{Client: client, Config: Config{MaxTurns: 1}, Out: &out, Renderer: fakeRenderer{rendered: "rendered"}}
	if err := chat.Run(context.Background(), "render"); err != nil {
		t.Fatal(err)
	}
	if out.String() != "rendered\n" {
		t.Fatalf("out=%q", out.String())
	}
}

func TestChatFallsBackWhenMarkdownRenderFails(t *testing.T) {
	client := &fakeClient{responses: []Message{{Role: "assistant", Content: "# Done"}}}
	var out bytes.Buffer
	chat := Chat{Client: client, Config: Config{MaxTurns: 1}, Out: &out, Renderer: fakeRenderer{err: errors.New("render failed")}}
	if err := chat.Run(context.Background(), "render"); err != nil {
		t.Fatal(err)
	}
	if out.String() != "# Done\n" {
		t.Fatalf("out=%q", out.String())
	}
}

func TestChatMalformedToolArgs(t *testing.T) {
	client := &fakeClient{responses: []Message{
		{Role: "assistant", ToolCalls: []ToolCall{{ID: "1", Type: "function", Function: FunctionCall{Name: "bash", Arguments: `{bad`}}}},
		{Role: "assistant", Content: "handled"},
	}}
	var out bytes.Buffer
	chat := Chat{Client: client, Config: Config{MaxTurns: 4, Bash: tool.BashConfig{TimeoutMS: 1000, MaxOutputBytes: 2000}}, Out: &out}
	if err := chat.Run(context.Background(), "run"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(client.calls[1][2].Content, "malformed tool arguments") {
		t.Fatalf("content=%q", client.calls[1][2].Content)
	}
}

func TestChatFormerlyDeniedCommandRequiresConfirmation(t *testing.T) {
	call := ToolCall{ID: "1", Type: "function", Function: FunctionCall{Name: "bash", Arguments: `{"command":"sudo true"}`}}
	var errOut bytes.Buffer
	chat := Chat{
		Config: Config{Bash: tool.BashConfig{TimeoutMS: 1000, MaxOutputBytes: 2000}},
		Err:    &errOut,
		In:     strings.NewReader("n\n"),
	}

	result := chat.handleBash(context.Background(), call)
	if strings.Contains(result, "denied") || !strings.Contains(result, "not approved") {
		t.Fatalf("result=%q", result)
	}
	if !strings.Contains(errOut.String(), "requires confirmation") {
		t.Fatalf("err=%q", errOut.String())
	}
}

func TestChatReadOnlyDeniesNeedsConfirmCommand(t *testing.T) {
	call := ToolCall{ID: "1", Type: "function", Function: FunctionCall{Name: "bash", Arguments: `{"command":"echo hi > file.txt"}`}}
	var errOut bytes.Buffer
	chat := Chat{
		Config: Config{Bash: tool.BashConfig{TimeoutMS: 1000, MaxOutputBytes: 2000, ReadOnly: true}},
		Err:    &errOut,
		In:     strings.NewReader("y\n"),
	}

	result := chat.handleBash(context.Background(), call)
	if !strings.Contains(result, "readonly mode denied command") {
		t.Fatalf("result=%q", result)
	}
	if strings.Contains(errOut.String(), "requires confirmation") {
		t.Fatalf("readonly mode prompted for confirmation: %q", errOut.String())
	}
}

func TestChatAutoApprovesNeedsConfirmCommand(t *testing.T) {
	call := ToolCall{ID: "1", Type: "function", Function: FunctionCall{Name: "bash", Arguments: `{"command":"false"}`}}
	checker := &fakeAutoChecker{result: AutoCheckResult{Safe: true, Reason: "harmless exit status check"}}
	var errOut bytes.Buffer
	chat := Chat{
		Auto:       true,
		AutoClient: checker,
		Config:     Config{Bash: tool.BashConfig{TimeoutMS: 1000, MaxOutputBytes: 2000}},
		Err:        &errOut,
	}

	result := chat.handleBash(context.Background(), call)
	if !strings.Contains(result, `"exit_code":1`) {
		t.Fatalf("result=%q", result)
	}
	if checker.calls != 1 {
		t.Fatalf("auto checker calls=%d", checker.calls)
	}
	if !strings.Contains(errOut.String(), "Auto-approved") {
		t.Fatalf("err=%q", errOut.String())
	}
}

func TestChatVerboseBashLifecycle(t *testing.T) {
	call := ToolCall{ID: "1", Type: "function", Function: FunctionCall{Name: "bash", Arguments: `{"command":"pwd","description":"print directory"}`}}
	var errOut bytes.Buffer
	chat := Chat{
		Verbose: true,
		Config:  Config{Bash: tool.BashConfig{TimeoutMS: 1000, MaxOutputBytes: 2000}},
		Err:     &errOut,
	}

	result := chat.handleBash(context.Background(), call)
	if !strings.Contains(result, `"exit_code":0`) {
		t.Fatalf("result=%q", result)
	}
	for _, want := range []string{"╭─ Bash", "print directory", "│ ", "pwd", "running", "completed"} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("verbose output missing %q: %q", want, errOut.String())
		}
	}
}

func TestChatAutoRejectsNeedsConfirmCommand(t *testing.T) {
	call := ToolCall{ID: "1", Type: "function", Function: FunctionCall{Name: "bash", Arguments: `{"command":"false"}`}}
	checker := &fakeAutoChecker{result: AutoCheckResult{Safe: false, Reason: "unknown command"}}
	var errOut bytes.Buffer
	chat := Chat{
		Auto:       true,
		AutoClient: checker,
		Config:     Config{Bash: tool.BashConfig{TimeoutMS: 1000, MaxOutputBytes: 2000}},
		Err:        &errOut,
		In:         strings.NewReader("y\n"),
	}

	result := chat.handleBash(context.Background(), call)
	if !strings.Contains(result, `"exit_code":1`) {
		t.Fatalf("result=%q", result)
	}
	if checker.calls != 1 {
		t.Fatalf("auto checker calls=%d", checker.calls)
	}
	if !strings.Contains(errOut.String(), "Auto confirmation rejected command") || !strings.Contains(errOut.String(), "Run? [y/N]") {
		t.Fatalf("err=%q", errOut.String())
	}
}

func TestChatAutoErrorFallsBackToConfirmation(t *testing.T) {
	call := ToolCall{ID: "1", Type: "function", Function: FunctionCall{Name: "bash", Arguments: `{"command":"false"}`}}
	checker := &fakeAutoChecker{err: errors.New("api down")}
	var errOut bytes.Buffer
	chat := Chat{
		Auto:       true,
		AutoClient: checker,
		Config:     Config{Bash: tool.BashConfig{TimeoutMS: 1000, MaxOutputBytes: 2000}},
		Err:        &errOut,
		In:         strings.NewReader("n\n"),
	}

	result := chat.handleBash(context.Background(), call)
	if !strings.Contains(result, "not approved") {
		t.Fatalf("result=%q", result)
	}
	if checker.calls != 1 {
		t.Fatalf("auto checker calls=%d", checker.calls)
	}
	if !strings.Contains(errOut.String(), "Auto confirmation failed: api down") || !strings.Contains(errOut.String(), "Run? [y/N]") {
		t.Fatalf("err=%q", errOut.String())
	}
}

func TestOpenAIClientChecksBashSafetyWithoutTools(t *testing.T) {
	var sawTools bool
	var sawModel string
	var sawAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		sawModel = req.Model
		sawTools = len(req.Tools) > 0
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{\"safe\":true,\"reason\":\"scoped\"}"}}]}`))
	}))
	defer srv.Close()

	client := NewAutoCheckClient(Config{APIKey: "primary-key", BaseURL: srv.URL, Model: "primary-model", AutoCheck: AutoCheckConfig{Model: "check-model"}})
	result, err := client.CheckBashSafety(context.Background(), tool.BashArgs{Command: "false"}, guard.GuardResult{Reason: "command is not in allowlist"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Safe || result.Reason != "scoped" {
		t.Fatalf("result=%#v", result)
	}
	if sawTools {
		t.Fatal("auto check request included tools")
	}
	if sawModel != "check-model" {
		t.Fatalf("model=%q", sawModel)
	}
	if sawAuth != "Bearer primary-key" {
		t.Fatalf("auth=%q", sawAuth)
	}
}

func TestChatMaxTurns(t *testing.T) {
	client := &fakeClient{responses: []Message{{Role: "assistant", ToolCalls: []ToolCall{{ID: "1", Type: "function", Function: FunctionCall{Name: "bash", Arguments: `{"command":"pwd"}`}}}}}}
	chat := Chat{Client: client, Config: Config{MaxTurns: 1, Bash: tool.BashConfig{TimeoutMS: 1000, MaxOutputBytes: 2000}}}
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

func TestOpenAIClientEmptyResponseBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	client := NewOpenAIClient(Config{APIKey: "k", BaseURL: srv.URL, Model: "m"})
	_, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err == nil || !strings.Contains(err.Error(), "api returned empty response body") {
		t.Fatalf("err=%v", err)
	}
}

func TestOpenAIClientInvalidJSONResponseBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer srv.Close()
	client := NewOpenAIClient(Config{APIKey: "k", BaseURL: srv.URL, Model: "m"})
	_, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err == nil || !strings.Contains(err.Error(), "api returned invalid JSON (200 OK): not json") {
		t.Fatalf("err=%v", err)
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

type fakeAutoChecker struct {
	result AutoCheckResult
	err    error
	calls  int
}

type fakeRenderer struct {
	rendered string
	err      error
}

func (f fakeRenderer) Render(markdown string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.rendered, nil
}

func (f *fakeAutoChecker) CheckBashSafety(ctx context.Context, args tool.BashArgs, guardResult guard.GuardResult) (AutoCheckResult, error) {
	f.calls++
	if f.err != nil {
		return AutoCheckResult{}, f.err
	}
	return f.result, nil
}
