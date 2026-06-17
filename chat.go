package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"heyai/tool"
)

type Chat struct {
	Client interface {
		Chat(context.Context, []Message) (Message, error)
	}
	Renderer interface {
		Render(string) (string, error)
	}
	AutoClient interface {
		tool.BashSafetyChecker
	}
	Config  Config
	Tools   []tool.Tool
	Auto    bool
	Verbose bool
	Out     io.Writer
	Err     io.Writer
	In      io.Reader

	verboseStatusStarted bool
	verboseStartedAt     time.Time
}

var (
	verboseAccentStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true)
	verboseCommandStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	verboseMutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	verboseRunningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	verboseSuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	verboseFailStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
)

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
	c.verboseStatusStarted = false
	c.verboseStartedAt = time.Time{}

	messages := []Message{{Role: "user", Content: prompt}}
	tools := c.Tools
	if len(tools) == 0 {
		tools = tool.DefaultToolsWithBashOptions(c.Config.Bash, tool.BashOptions{Auto: c.Auto, AutoChecker: c.AutoClient, In: c.In, Err: c.Err, Hooks: &c})
	}
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
			if call.Type != "function" {
				return fmt.Errorf("unsupported tool call type: %s", call.Type)
			}
			result := tool.Handle(ctx, tools, call.Function.Name, json.RawMessage(call.Function.Arguments))
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

func (c *Chat) writeVerboseBashStart(args tool.BashArgs) {
	if !c.Verbose {
		return
	}
	c.verboseStartedAt = time.Now()
	c.verboseStatusStarted = false
	w := c.verboseWriter()
	fmt.Fprintln(w, verboseAccentStyle.Render("╭─ Bash"))
	if args.Description != "" {
		fmt.Fprintln(w, verboseMutedStyle.Render("│ ")+verboseCommandStyle.Render(args.Description))
	}
	fmt.Fprintln(w, verboseMutedStyle.Render("│ ")+verboseCommandStyle.Render(args.Command))
}

func (c *Chat) BashStart(args tool.BashArgs) {
	c.writeVerboseBashStart(args)
}

func (c *Chat) BashRunning() {
	c.writeVerboseBashRunning()
}

func (c *Chat) BashCompleted(result tool.BashResult) {
	c.writeVerboseBashCompleted(result)
}

func (c *Chat) BashFailed(reason string) {
	c.writeVerboseBashFailed(reason)
}

func (c *Chat) writeVerboseBashRunning() {
	if !c.Verbose {
		return
	}
	c.verboseStatusStarted = true
	c.writeVerboseStatus(verboseRunningStyle.Render("╰─ ● running"), false)
}

func (c *Chat) writeVerboseBashCompleted(res tool.BashResult) {
	if !c.Verbose {
		return
	}
	if res.ExitCode == 0 && !res.TimedOut {
		c.writeVerboseStatus(verboseSuccessStyle.Render("╰─ ✓ completed")+verboseMutedStyle.Render(" "+c.verboseElapsed()), true)
		return
	}
	status := fmt.Sprintf("╰─ ✕ failed exit %d", res.ExitCode)
	if res.TimedOut {
		status = "╰─ ✕ timed out"
	}
	c.writeVerboseStatus(verboseFailStyle.Render(status)+verboseMutedStyle.Render(" "+c.verboseElapsed()), true)
}

func (c *Chat) writeVerboseBashFailed(reason string) {
	if c.Verbose {
		c.writeVerboseStatus(verboseFailStyle.Render("╰─ ✕ blocked")+verboseMutedStyle.Render(" "+reason), true)
	}
}

func (c *Chat) writeVerboseStatus(status string, final bool) {
	w := c.verboseWriter()
	if c.verboseCanUpdate() && c.verboseStatusStarted {
		fmt.Fprint(w, "\r\033[2K")
	}
	if final || !c.verboseCanUpdate() {
		fmt.Fprintln(w, status)
		return
	}
	fmt.Fprint(w, status)
}

func (c *Chat) verboseCanUpdate() bool {
	f, ok := c.Err.(*os.File)
	return ok && isTerminal(f)
}

func (c *Chat) verboseElapsed() string {
	if c.verboseStartedAt.IsZero() {
		return ""
	}
	return "(" + time.Since(c.verboseStartedAt).Round(10*time.Millisecond).String() + ")"
}

func (c Chat) verboseWriter() io.Writer {
	if c.Err != nil {
		return c.Err
	}
	return io.Discard
}
