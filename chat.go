package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"heyai/guard"
	"io"
	"strings"
)

type Chat struct {
	Client interface {
		Chat(context.Context, []Message) (Message, error)
	}
	Renderer interface {
		Render(string) (string, error)
	}
	AutoClient interface {
		CheckBashSafety(context.Context, BashArgs, guard.GuardResult) (AutoCheckResult, error)
	}
	Config Config
	Auto   bool
	Out    io.Writer
	Err    io.Writer
	In     io.Reader
}

func (c Chat) Run(ctx context.Context, prompt string) error {
	if c.Out == nil {
		c.Out = io.Discard
	}
	if c.Err == nil {
		c.Err = io.Discard
	}
	if c.In == nil {
		c.In = strings.NewReader("")
	}
	if c.Config.MaxTurns <= 0 {
		c.Config.MaxTurns = defaultConfig().MaxTurns
	}

	messages := []Message{{Role: "user", Content: prompt}}
	for turn := 0; turn < c.Config.MaxTurns; turn++ {
		msg, err := c.Client.Chat(ctx, messages)
		if err != nil {
			return err
		}
		messages = append(messages, msg)
		if len(msg.ToolCalls) == 0 {
			if msg.Content != "" {
				c.writeAssistantContent(msg.Content)
			}
			return nil
		}

		for _, call := range msg.ToolCalls {
			if call.Type != "function" || call.Function.Name != "bash" {
				return fmt.Errorf("unsupported tool call: %s", call.Function.Name)
			}
			result := c.handleBash(ctx, call)
			messages = append(messages, Message{Role: "tool", ToolCallID: call.ID, Content: result})
		}
	}
	return fmt.Errorf("max turns exceeded")
}

func (c Chat) writeAssistantContent(content string) {
	if c.Renderer != nil {
		if rendered, err := c.Renderer.Render(content); err == nil {
			fmt.Fprint(c.Out, rendered)
			if !strings.HasSuffix(rendered, "\n") {
				fmt.Fprintln(c.Out)
			}
			return
		}
	}
	fmt.Fprintln(c.Out, content)
}

func (c Chat) handleBash(ctx context.Context, call ToolCall) string {
	var args BashArgs
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return toolError("malformed tool arguments: " + err.Error())
	}
	guardResult, err := guard.CheckBash(args.Command, args.Workdir)
	if err != nil {
		return toolError(err.Error())
	}
	if guardResult.Risk == guard.RiskDenied {
		return toolError("invalid command: " + guardResult.Reason)
	}
	if c.Config.Bash.ReadOnly && guardResult.Risk != guard.RiskSafe {
		return toolError("readonly mode denied command: " + guardResult.Reason)
	}
	if guardResult.Risk != guard.RiskSafe && !c.Config.Bash.AllowRiskyWithoutConfirm {
		approved := false
		if c.Auto {
			if c.AutoClient == nil {
				fmt.Fprintln(c.Err, "Auto confirmation unavailable: no auto check client is configured")
			} else {
				check, err := c.AutoClient.CheckBashSafety(ctx, args, guardResult)
				if err != nil {
					fmt.Fprintf(c.Err, "Auto confirmation failed: %s\n", err)
				} else if !check.Safe {
					fmt.Fprintf(c.Err, "Auto confirmation rejected command: %s\n", check.Reason)
				} else {
					approved = true
					fmt.Fprintf(c.Err, "Auto-approved bash command (%s): %s\n", check.Reason, args.Command)
				}
			}
		}
		if !approved {
			fmt.Fprintf(c.Err, "Bash command requires confirmation (%s):\n%s\nRun? [y/N] ", guardResult.Reason, args.Command)
			answer, _ := bufio.NewReader(c.In).ReadString('\n')
			answer = strings.ToLower(strings.TrimSpace(answer))
			if answer != "y" && answer != "yes" {
				return toolError("not approved")
			}
		}
	}

	res := RunBash(ctx, args, c.Config.Bash)
	b, err := json.Marshal(res)
	if err != nil {
		return toolError(err.Error())
	}
	return string(b)
}

func toolError(msg string) string {
	b, _ := json.Marshal(map[string]any{"error": msg})
	return string(b)
}
