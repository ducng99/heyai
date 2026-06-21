package heyai

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

func Run(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printUsage()
		return nil
	}

	switch args[0] {
	case "--config-path":
		path, err := configPath()
		if err != nil {
			return err
		}
		fmt.Println(path)
		return nil
	case "--init":
		return initConfig()
	}

	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	if cfg.APIKey == "" {
		return errors.New("missing api_key in config")
	}
	autoFlag, readOnly, verbose, promptArgs := parseRuntimeFlags(args)
	if len(promptArgs) == 0 {
		return errors.New("missing prompt")
	}
	auto := cfg.Tools.AutoMode || autoFlag
	if readOnly {
		cfg.Tools.ReadOnly = true
	}

	client := NewOpenAIClient(cfg)
	chat := Chat{Client: client, Config: cfg, Auto: auto, Verbose: verbose, Out: os.Stdout, Err: os.Stderr}
	if isTerminal(os.Stdout) {
		if renderer, err := newTerminalMarkdownRenderer(os.Stdout); err == nil {
			chat.Renderer = renderer
		}
	}
	if auto {
		chat.AutoClient = NewAutoCheckClient(cfg)
	}
	return chat.Run(context.Background(), strings.Join(promptArgs, " "))
}

func parseRuntimeFlags(args []string) (bool, bool, bool, []string) {
	auto := false
	readOnly := false
	verbose := false
	promptArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--auto" || arg == "-a" {
			auto = true
			continue
		}
		if arg == "--readonly" || arg == "-r" {
			readOnly = true
			continue
		}
		if arg == "--verbose" || arg == "-v" {
			verbose = true
			continue
		}
		promptArgs = append(promptArgs, arg)
	}
	return auto, readOnly, verbose, promptArgs
}

func printUsage() {
	fmt.Println(`Usage:
	  hey [--auto|-a] [--readonly|-r] [--verbose|-v] "prompt here"
	  hey --init
	  hey --config-path

--auto, -a enables auto mode for this run; set "auto_mode": true under "tools" in config to enable it by default.
--readonly, -r denies any bash command that is not classified as strictly read-only.
--verbose, -v prints bash tool-call progress.

Configuration is read from $XDG_CONFIG_HOME/heyai/config.json or ~/.config/heyai/config.json.`)
}
