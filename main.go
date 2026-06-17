package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
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
	auto, readOnly, verbose, promptArgs := parseRuntimeFlags(args)
	if len(promptArgs) == 0 {
		return errors.New("missing prompt")
	}
	if readOnly {
		cfg.Bash.ReadOnly = true
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
	  heyai [--auto|-a] [--readonly|-r] [--verbose|-v] "prompt here"
	  heyai --init
	  heyai --config-path

--auto, -a asks an AI safety checker to approve commands that would otherwise need confirmation.
--readonly, -r denies any bash command that is not classified as strictly read-only.
--verbose, -v prints bash tool-call progress.

Configuration is read from $XDG_CONFIG_HOME/heyai/config.json or ~/.config/heyai/config.json.`)
}
