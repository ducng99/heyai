package tool

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ducng99/heyai/guard"
	"io"
	"os/exec"
	"strings"
	"time"
)

type BashTool struct {
	Config  BashConfig
	Options BashOptions
}

type BashOptions struct {
	Config      BashConfig
	Auto        bool
	AutoChecker BashSafetyChecker
	In          io.Reader
	Err         io.Writer
	Hooks       BashHooks
}

type AutoCheckResult struct {
	Safe   bool   `json:"safe"`
	Reason string `json:"reason"`
}

type BashSafetyChecker interface {
	CheckBashSafety(context.Context, BashArgs, guard.GuardResult) (AutoCheckResult, error)
}

type BashHooks interface {
	BashStart(BashArgs)
	BashRunning()
	BashCompleted(BashResult)
	BashFailed(string)
}

type BashConfig struct {
	TimeoutMS                int  `json:"timeout_ms"`
	AllowRiskyWithoutConfirm bool `json:"allow_risky_without_confirm"`
	MaxOutputBytes           int  `json:"max_output_bytes"`
	ReadOnly                 bool `json:"read_only"`
}

type BashArgs struct {
	Command     string `json:"command"`
	Description string `json:"description,omitempty"`
	TimeoutMS   int    `json:"timeout_ms,omitempty"`
	Workdir     string `json:"workdir,omitempty"`
}

type BashResult struct {
	ExitCode  int    `json:"exit_code"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	TimedOut  bool   `json:"timed_out"`
	Truncated bool   `json:"truncated"`
}

func (t BashTool) Definition() Definition {
	return Definition{Name: "bash", Description: "Run a guarded bash command in the current working directory.", Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command":     map[string]string{"type": "string"},
			"description": map[string]string{"type": "string"},
			"timeout_ms":  map[string]string{"type": "integer"},
			"workdir":     map[string]string{"type": "string"},
		},
		"required": []string{"command"},
	}}
}

func (t BashTool) Run(ctx context.Context, raw json.RawMessage) (any, error) {
	var args BashArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		t.bashFailed("malformed tool arguments")
		return nil, fmt.Errorf("malformed tool arguments: %w", err)
	}
	t.bashStart(args)
	cfg := t.bashConfig()
	guardResult, err := guard.CheckBash(args.Command, args.Workdir)
	if err != nil {
		t.bashFailed(err.Error())
		return nil, err
	}
	if guardResult.Risk == guard.RiskDenied {
		t.bashFailed(guardResult.Reason)
		return nil, errors.New("invalid command: " + guardResult.Reason)
	}
	if cfg.ReadOnly && guardResult.Risk != guard.RiskSafe {
		t.bashFailed(guardResult.Reason)
		return nil, errors.New("readonly mode denied command: " + guardResult.Reason)
	}
	if guardResult.Risk != guard.RiskSafe && !cfg.AllowRiskyWithoutConfirm {
		approved := false
		if t.Options.Auto {
			approved = t.autoApprove(ctx, args, guardResult)
		}
		if !approved && !t.confirm(args, guardResult) {
			t.bashFailed("not approved")
			return nil, errors.New("not approved")
		}
	}

	t.bashRunning()
	result := RunBash(ctx, args, cfg)
	t.bashCompleted(result)
	return result, nil
}

func (t BashTool) bashConfig() BashConfig {
	if t.Options.Config != (BashConfig{}) {
		return t.Options.Config
	}
	return t.Config
}

func (t BashTool) autoApprove(ctx context.Context, args BashArgs, guardResult guard.GuardResult) bool {
	if t.Options.Err == nil {
		t.Options.Err = io.Discard
	}
	if t.Options.AutoChecker == nil {
		fmt.Fprintln(t.Options.Err, "Auto confirmation unavailable: no auto check client is configured")
		return false
	}
	check, err := t.Options.AutoChecker.CheckBashSafety(ctx, args, guardResult)
	if err != nil {
		fmt.Fprintf(t.Options.Err, "Auto confirmation failed: %s\n", err)
		return false
	}
	if !check.Safe {
		fmt.Fprintf(t.Options.Err, "Auto confirmation rejected command: %s\n", check.Reason)
		return false
	}
	fmt.Fprintf(t.Options.Err, "Auto-approved bash command (%s): %s\n", check.Reason, args.Command)
	return true
}

func (t BashTool) confirm(args BashArgs, guardResult guard.GuardResult) bool {
	if t.Options.Err == nil {
		t.Options.Err = io.Discard
	}
	if t.Options.In == nil {
		t.Options.In = strings.NewReader("")
	}
	fmt.Fprintf(t.Options.Err, "Bash command requires confirmation (%s):\n%s\nRun? [y/N] ", guardResult.Reason, args.Command)
	answer, _ := bufio.NewReader(t.Options.In).ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

func (t BashTool) bashStart(args BashArgs) {
	if t.Options.Hooks != nil {
		t.Options.Hooks.BashStart(args)
	}
}

func (t BashTool) bashRunning() {
	if t.Options.Hooks != nil {
		t.Options.Hooks.BashRunning()
	}
}

func (t BashTool) bashCompleted(result BashResult) {
	if t.Options.Hooks != nil {
		t.Options.Hooks.BashCompleted(result)
	}
}

func (t BashTool) bashFailed(reason string) {
	if t.Options.Hooks != nil {
		t.Options.Hooks.BashFailed(reason)
	}
}

func RunBash(parent context.Context, args BashArgs, cfg BashConfig) BashResult {
	timeout := cfg.TimeoutMS
	if args.TimeoutMS > 0 && args.TimeoutMS < timeout {
		timeout = args.TimeoutMS
	}
	if timeout <= 0 {
		timeout = 30000
	}
	limit := cfg.MaxOutputBytes
	if limit <= 0 {
		limit = 20000
	}

	ctx, cancel := context.WithTimeout(parent, time.Duration(timeout)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-lc", args.Command)
	if args.Workdir != "" {
		cmd.Dir = args.Workdir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	result := BashResult{ExitCode: 0, Stdout: stdout.String(), Stderr: stderr.String()}
	if ctx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		result.ExitCode = -1
	} else if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
			result.Stderr += err.Error()
		}
	}
	result.Stdout, result.Stderr, result.Truncated = truncatePair(result.Stdout, result.Stderr, limit)
	return result
}

func truncatePair(stdout, stderr string, limit int) (string, string, bool) {
	if len(stdout)+len(stderr) <= limit {
		return stdout, stderr, false
	}
	remaining := limit
	if len(stdout) > remaining {
		stdout = stdout[:remaining]
		remaining = 0
	} else {
		remaining -= len(stdout)
	}
	if len(stderr) > remaining {
		stderr = stderr[:remaining]
	}
	return stdout, stderr, true
}
