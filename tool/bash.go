package tool

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
)

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
